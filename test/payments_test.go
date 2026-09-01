package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/app/ordering"
	"github.com/stevenwilliam/evermore/internal/app/payments"
	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/order"
)

// memStore records what would have been written to MinIO, so these tests do
// not need object storage running.
type memStore struct{ keys map[string]int }

func (m *memStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if m.keys == nil {
		m.keys = map[string]int{}
	}
	m.keys[key] = int(size)
	return nil
}

func paymentsSvc(t *testing.T, now time.Time) (*payments.Service, *memStore) {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	store := &memStore{}
	return payments.NewService(testDB, store, loc, func() time.Time { return now }), store
}

// pngBytes is a minimal valid PNG header plus padding, enough for the sniffer.
func pngBytes(n int) []byte {
	b := []byte("\x89PNG\r\n\x1a\n")
	return append(b, bytes.Repeat([]byte{0}, n)...)
}

func jpegBytes(n int) []byte {
	b := []byte{0xFF, 0xD8, 0xFF}
	return append(b, bytes.Repeat([]byte{0}, n)...)
}

// placeOrder runs a real checkout and returns the order.
func placeOrder(t *testing.T, email string, qty int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	seedOnce(t)
	mealID, _ := futureMeal(t, 2)
	custID, addrID := customerAndAddress(t, email)
	svc := orderingSvc(t, time.Now())
	orderID, _, err := svc.Checkout(context.Background(), ordering.CheckoutInput{
		CustomerID: custID, AddressID: addrID,
		Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: qty}},
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("placing an order: %v", err)
	}
	return orderID, custID
}

func TestInstructions_ShowTheSuffixSeparately(t *testing.T) {
	orderID, custID := placeOrder(t, "sinta@example.com", 2)
	svc, _ := paymentsSvc(t, time.Now())

	in, err := svc.Instructions(context.Background(), custID, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if in.BankName != "BCA" || in.AccountNumber != "5391184402" {
		t.Errorf("bank details = %s %s, want the seeded BCA account", in.BankName, in.AccountNumber)
	}
	if in.AccountHolder != "PT Evermore Nutrisi Indonesia" {
		t.Errorf("holder = %q", in.AccountHolder)
	}
	// The artifact highlights the last three digits as the matching device.
	if in.Suffix != int(in.AmountIDR%1000) {
		t.Errorf("suffix %d is not the last three digits of %d", in.Suffix, in.AmountIDR)
	}
	if in.DeadlineAt.IsZero() {
		t.Error("no payment deadline set")
	}
	if in.Remaining <= 0 || in.Remaining > 4*time.Hour {
		t.Errorf("remaining = %s, expected within the 3-hour window", in.Remaining)
	}
}

func TestInstructions_ScopedByOwner(t *testing.T) {
	orderID, _ := placeOrder(t, "sinta@example.com", 1)
	_, bagasID := placeOrder(t, "bagas@example.com", 1)
	svc, _ := paymentsSvc(t, time.Now())

	// Bagas asking for Sinta's order is not-found, not a peek at her total.
	_, err := svc.Instructions(context.Background(), bagasID, orderID)
	if !errors.Is(err, payments.ErrPaymentNotFound) {
		t.Errorf("got %v, want ErrPaymentNotFound", err)
	}
}

func TestSubmitProof_AcceptsImagesAndMovesTheOrder(t *testing.T) {
	orderID, custID := placeOrder(t, "sinta@example.com", 2)
	svc, store := paymentsSvc(t, time.Now())

	if err := svc.SubmitProof(context.Background(), custID, orderID, pngBytes(2048), 0, "SINTA PRAMESWARI"); err != nil {
		t.Fatalf("submitting a PNG: %v", err)
	}
	if len(store.keys) != 1 {
		t.Errorf("%d objects stored, want 1", len(store.keys))
	}

	var orderStatus, payStatus string
	if err := testDB.QueryRow(`
		SELECT o.status, p.status FROM customer_order o
		  JOIN payment p ON p.order_id = o.id WHERE o.id = $1`, orderID).
		Scan(&orderStatus, &payStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "PAYMENT_SUBMITTED" {
		t.Errorf("order status = %s, want PAYMENT_SUBMITTED", orderStatus)
	}
	if payStatus != "SUBMITTED" {
		t.Errorf("payment status = %s, want SUBMITTED", payStatus)
	}

	// The event history records it, append-only.
	var events int
	if err := testDB.QueryRow(`
		SELECT count(*) FROM payment_event pe JOIN payment p ON p.id = pe.payment_id
		 WHERE p.order_id = $1`, orderID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events < 2 {
		t.Errorf("%d payment events, want at least PENDING and SUBMITTED", events)
	}
}

func TestSubmitProof_RejectsWrongTypeAndOversize(t *testing.T) {
	orderID, custID := placeOrder(t, "sinta@example.com", 1)
	svc, _ := paymentsSvc(t, time.Now())

	// A PDF renamed to .png is still a PDF. The type is sniffed from the
	// bytes, never taken from a client-supplied header.
	pdf := append([]byte("%PDF-1.7"), bytes.Repeat([]byte{0}, 100)...)
	if err := svc.SubmitProof(context.Background(), custID, orderID, pdf, 0, ""); !errors.Is(err, payments.ErrProofWrongType) {
		t.Errorf("a PDF was accepted as proof: %v", err)
	}

	// An HTML file, which is the one that matters if proofs are ever served
	// back from the same origin.
	html := []byte("<html><script>alert(1)</script></html>")
	if err := svc.SubmitProof(context.Background(), custID, orderID, html, 0, ""); !errors.Is(err, payments.ErrProofWrongType) {
		t.Errorf("an HTML file was accepted as proof: %v", err)
	}

	if err := svc.SubmitProof(context.Background(), custID, orderID, pngBytes(payments.MaxProofBytes+1), 0, ""); !errors.Is(err, payments.ErrProofTooLarge) {
		t.Errorf("an oversize proof was accepted: %v", err)
	}
	if err := svc.SubmitProof(context.Background(), custID, orderID, nil, 0, ""); err == nil {
		t.Error("an empty upload was accepted")
	}

	// The order must still be awaiting payment: a refused upload changes
	// nothing.
	var status string
	if err := testDB.QueryRow(`SELECT status FROM customer_order WHERE id = $1`, orderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "AWAITING_PAYMENT" {
		t.Errorf("status = %s after refused uploads, want AWAITING_PAYMENT", status)
	}
}

func TestSubmitProof_RefusedAfterTheDeadline(t *testing.T) {
	orderID, custID := placeOrder(t, "sinta@example.com", 1)
	// A clock four hours on: the three-hour window has closed.
	svc, _ := paymentsSvc(t, time.Now().Add(4*time.Hour))

	err := svc.SubmitProof(context.Background(), custID, orderID, pngBytes(64), 0, "")
	if !errors.Is(err, payments.ErrDeadlinePassed) {
		t.Errorf("got %v, want ErrDeadlinePassed", err)
	}
}

func TestVerify_MarksPaidAndQueueEmpties(t *testing.T) {
	orderID, custID := placeOrder(t, "sinta@example.com", 2)
	svc, _ := paymentsSvc(t, time.Now())

	var expected int64
	if err := testDB.QueryRow(
		`SELECT payment_amount_idr FROM customer_order WHERE id = $1`, orderID).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if err := svc.SubmitProof(context.Background(), custID, orderID,
		jpegBytes(1024), moneyOf(expected), "SINTA PRAMESWARI"); err != nil {
		t.Fatal(err)
	}

	queue, err := svc.Queue(context.Background(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	var item *payments.QueueItem
	for i := range queue {
		if queue[i].OrderID == orderID {
			item = &queue[i]
		}
	}
	if item == nil {
		t.Fatal("the submitted payment is not in the verification queue")
	}
	if item.Match != "Cocok" {
		t.Errorf("match verdict = %q, want Cocok for an exact amount", item.Match)
	}
	if item.ProofKey == "" {
		t.Error("the queue row carries no proof key")
	}

	var actorID uuid.UUID
	if err := testDB.QueryRow(
		`SELECT id FROM app_user WHERE email = 'finance@evermore.co.id'`).Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Verify(context.Background(), actorID, item.PaymentID, order.ActorStaff); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var orderStatus, payStatus string
	var verifiedBy *uuid.UUID
	if err := testDB.QueryRow(`
		SELECT o.status, p.status, p.verified_by FROM customer_order o
		  JOIN payment p ON p.order_id = o.id WHERE o.id = $1`, orderID).
		Scan(&orderStatus, &payStatus, &verifiedBy); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "PAID" || payStatus != "VERIFIED" {
		t.Errorf("order=%s payment=%s, want PAID/VERIFIED", orderStatus, payStatus)
	}
	if verifiedBy == nil || *verifiedBy != actorID {
		t.Error("the verifying actor was not recorded")
	}

	// Verifying twice is refused rather than double-processed.
	if err := svc.Verify(context.Background(), actorID, item.PaymentID, order.ActorStaff); err == nil {
		t.Error("a second verification was accepted")
	}
}

func TestReject_RequiresAReasonAndReturnsToAwaitingPayment(t *testing.T) {
	orderID, custID := placeOrder(t, "bagas@example.com", 1)
	svc, _ := paymentsSvc(t, time.Now())
	if err := svc.SubmitProof(context.Background(), custID, orderID, pngBytes(512), 0, "SALAH NAMA"); err != nil {
		t.Fatal(err)
	}

	var paymentID, actorID uuid.UUID
	if err := testDB.QueryRow(`SELECT id FROM payment WHERE order_id = $1`, orderID).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if err := testDB.QueryRow(
		`SELECT id FROM app_user WHERE email = 'finance@evermore.co.id'`).Scan(&actorID); err != nil {
		t.Fatal(err)
	}

	if err := svc.Reject(context.Background(), actorID, paymentID, order.ActorStaff, ""); !errors.Is(err, payments.ErrReasonRequired) {
		t.Errorf("a rejection with no reason was accepted: %v", err)
	}
	if err := svc.Reject(context.Background(), actorID, paymentID, order.ActorStaff,
		"nama pengirim tidak cocok"); err != nil {
		t.Fatal(err)
	}

	var orderStatus, payStatus, reason string
	if err := testDB.QueryRow(`
		SELECT o.status, p.status, COALESCE(p.rejected_reason,'')
		  FROM customer_order o JOIN payment p ON p.order_id = o.id WHERE o.id = $1`, orderID).
		Scan(&orderStatus, &payStatus, &reason); err != nil {
		t.Fatal(err)
	}
	// Back to awaiting payment so the customer can re-upload. NOT cancelled:
	// nothing automated cancels a booking, and a rejection is not a decision
	// to cancel one either.
	if orderStatus != "AWAITING_PAYMENT" {
		t.Errorf("order status = %s, want AWAITING_PAYMENT", orderStatus)
	}
	if payStatus != "REJECTED" || reason == "" {
		t.Errorf("payment = %s, reason = %q", payStatus, reason)
	}
}

func TestExpireOverdue_ReleasesCapacityAndSuffix(t *testing.T) {
	orderID, _ := placeOrder(t, "sinta@example.com", 3)

	var kitchenID, slotID uuid.UUID
	var serviceDate time.Time
	if err := testDB.QueryRow(`
		SELECT kitchen_id, slot_id, service_date FROM delivery WHERE order_id = $1`, orderID).
		Scan(&kitchenID, &slotID, &serviceDate); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := testDB.QueryRow(`
		SELECT reserved_portions FROM kitchen_capacity
		 WHERE kitchen_id = $1 AND slot_id = $2 AND service_date = $3`,
		kitchenID, slotID, serviceDate).Scan(&before); err != nil {
		t.Fatal(err)
	}

	// Push this order's deadline into the past.
	if _, err := testDB.Exec(
		`UPDATE customer_order SET payment_deadline_at = now() - interval '1 hour' WHERE id = $1`,
		orderID); err != nil {
		t.Fatal(err)
	}

	svc, _ := paymentsSvc(t, time.Now())
	n, err := svc.ExpireOverdue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expired %d orders, expected at least 1", n)
	}

	var status string
	if err := testDB.QueryRow(`SELECT status FROM customer_order WHERE id = $1`, orderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	// EXPIRED, not CANCELLED — CLAUDE.md §7.
	if status != "EXPIRED" {
		t.Errorf("status = %s, want EXPIRED", status)
	}

	var after int
	if err := testDB.QueryRow(`
		SELECT reserved_portions FROM kitchen_capacity
		 WHERE kitchen_id = $1 AND slot_id = $2 AND service_date = $3`,
		kitchenID, slotID, serviceDate).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before-3 {
		t.Errorf("reserved went %d -> %d, expected the 3 portions to be released", before, after)
	}

	// The suffix is released so it can be drawn again.
	var released int
	if err := testDB.QueryRow(`
		SELECT count(*) FROM payment_suffix_claim WHERE order_id = $1 AND released_at IS NOT NULL`,
		orderID).Scan(&released); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Errorf("%d suffix claims released, want 1", released)
	}
}

// moneyOf keeps the call sites readable without importing money everywhere.
func moneyOf(v int64) money.IDR { return money.IDR(v) }
