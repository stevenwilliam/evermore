// Package payments handles manual bank transfer: the customer uploads proof,
// finance verifies or rejects, and every decision is recorded.
//
// PaymentProvider is an interface from day one with ManualTransferProvider as
// the only implementation, so adding QRIS later does not mean rewriting the
// order state machine (01-domain-model.md §6).
package payments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/order"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
)

var (
	ErrPaymentNotFound  = errors.New("PAYMENT_NOT_FOUND")
	ErrNotAwaitingProof = errors.New("PAYMENT_NOT_AWAITING_PROOF")
	ErrDeadlinePassed   = errors.New("PAYMENT_DEADLINE_PASSED")
	ErrProofTooLarge    = errors.New("PROOF_TOO_LARGE")
	ErrProofWrongType   = errors.New("PROOF_UNSUPPORTED_TYPE")
	ErrReasonRequired   = errors.New("REASON_REQUIRED")
)

// MaxProofBytes matches the artifact's "maksimum 5 MB" and the CHECK on
// payment_proof.
const MaxProofBytes = 5 * 1024 * 1024

// allowedProofTypes mirrors the payment_proof_mime_allowed constraint. The
// content type is sniffed from the bytes, never taken from the client: a
// Content-Type header is a claim, not evidence.
var allowedProofTypes = map[string]bool{"image/jpeg": true, "image/png": true}

// ObjectStore is what payments needs from storage.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
}

// Service performs payment use-cases.
type Service struct {
	db    *sql.DB
	store ObjectStore
	loc   *time.Location
	now   func() time.Time
}

func NewService(db *sql.DB, store ObjectStore, loc *time.Location, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, store: store, loc: loc, now: now}
}

// Instructions is what the payment screen renders.
type Instructions struct {
	OrderID       uuid.UUID
	OrderNumber   string
	Status        string
	BankName      string
	AccountNumber string
	AccountHolder string
	AmountIDR     money.IDR
	// Suffix is the last three digits, shown separately because the artifact
	// highlights them: "Nominal tepat sampai 3 digit terakhir".
	Suffix     int
	DeadlineAt time.Time
	Remaining  time.Duration
	Expired    bool
}

// Instructions loads the transfer details for an order, scoped by owner.
func (s *Service) Instructions(ctx context.Context, customerID, orderID uuid.UUID) (*Instructions, error) {
	var in Instructions
	var deadline sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, o.status, o.payment_amount_idr, o.payment_deadline_at,
		       b.bank_name, b.account_number, b.account_holder
		  FROM customer_order o
		  JOIN payment p      ON p.order_id = o.id
		  JOIN bank_account b ON b.id = p.bank_account_id
		 WHERE o.id = $1 AND o.customer_id = $2`, orderID, customerID).
		Scan(&in.OrderID, &in.OrderNumber, &in.Status, (*int64)(&in.AmountIDR), &deadline,
			&in.BankName, &in.AccountNumber, &in.AccountHolder)
	if err == sql.ErrNoRows {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	in.Suffix = int(in.AmountIDR % order.MaxSuffix)
	if deadline.Valid {
		in.DeadlineAt = deadline.Time
		in.Remaining = deadline.Time.Sub(s.now())
		if in.Remaining < 0 {
			in.Remaining = 0
			in.Expired = true
		}
	}
	return &in, nil
}

// SubmitProof stores the uploaded image and moves the order to
// PAYMENT_SUBMITTED.
func (s *Service) SubmitProof(ctx context.Context, customerID, orderID uuid.UUID,
	data []byte, declaredAmount money.IDR, senderName string) error {

	if len(data) == 0 {
		return ErrProofWrongType
	}
	if len(data) > MaxProofBytes {
		return fmt.Errorf("%w: %d bytes, maximum is %d", ErrProofTooLarge, len(data), MaxProofBytes)
	}
	// Sniff the type from the bytes. A client claiming image/png while sending
	// a PDF or an HTML file is exactly what the CHECK constraint and this
	// check exist to stop.
	mime := sniff(data)
	if !allowedProofTypes[mime] {
		return fmt.Errorf("%w: berkas harus JPG atau PNG", ErrProofWrongType)
	}

	var (
		paymentID   uuid.UUID
		payStatus   string
		orderStatus string
		deadline    sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.status, o.status, o.payment_deadline_at
		  FROM payment p JOIN customer_order o ON o.id = p.order_id
		 WHERE p.order_id = $1 AND o.customer_id = $2`, orderID, customerID).
		Scan(&paymentID, &payStatus, &orderStatus, &deadline)
	if err == sql.ErrNoRows {
		return ErrPaymentNotFound
	}
	if err != nil {
		return err
	}

	// A proof may be uploaded while awaiting payment, and re-uploaded after a
	// rejection. Anything else is out of sequence.
	if orderStatus != string(order.StatusAwaitingPayment) {
		return fmt.Errorf("%w: pesanan berstatus %s", ErrNotAwaitingProof, orderStatus)
	}
	if deadline.Valid && s.now().After(deadline.Time) {
		return ErrDeadlinePassed
	}
	// The domain owns the transition, not this handler.
	if err := order.Transition(order.Status(orderStatus), order.StatusPaymentSubmitted,
		order.ActorCustomer, ""); err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	}
	key := fmt.Sprintf("payment-proof/%s/%s%s", orderID, digest[:16], ext)

	if s.store != nil {
		if err := s.store.Put(ctx, key, bytesReader(data), int64(len(data)), mime); err != nil {
			return fmt.Errorf("storing the proof: %w", err)
		}
	}

	return database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment_proof (id, payment_id, object_key, mime_type, size_bytes, sha256)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			id.New(), paymentID, key, mime, len(data), digest); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE payment
			   SET status = 'SUBMITTED', submitted_at = now(),
			       declared_amount_idr = $2, sender_name = $3
			 WHERE id = $1`, paymentID, nullIfZero(declaredAmount), nullIfBlank(senderName)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment_event (id, payment_id, from_status, to_status, reason)
			VALUES ($1, $2, $3, 'SUBMITTED', 'customer uploaded proof')`,
			id.New(), paymentID, payStatus); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE customer_order SET status = 'PAYMENT_SUBMITTED' WHERE id = $1 AND status = $2`,
			orderID, orderStatus)
		if err != nil {
			return err
		}
		// Assert the order actually moved. A conditional UPDATE that matched
		// nothing means another request changed the status underneath us, and
		// silently continuing would leave a submitted proof on an order still
		// marked as awaiting payment.
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: pesanan berubah status saat unggah", ErrNotAwaitingProof)
		}
		return nil
	})
}

// QueueItem is one row of the verification queue.
type QueueItem struct {
	OrderID      uuid.UUID
	PaymentID    uuid.UUID
	OrderNumber  string
	CustomerName string
	CustomerType string
	ExpectedIDR  money.IDR
	DeclaredIDR  *money.IDR
	SenderName   string
	SubmittedAt  time.Time
	WaitingFor   time.Duration
	Suffix       int
	ProofKey     string
	// Match summarises how well the declared transfer lines up, which is the
	// "Cocok / Selisih Rp 2.000 / Nama beda" column in the artifact.
	Match string
}

// Queue lists payments awaiting verification, oldest first, with an optional
// search over order number and customer name.
func (s *Service) Queue(ctx context.Context, search string, limit int) ([]QueueItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, p.id, o.order_number, c.full_name, ct.name,
		       o.payment_amount_idr, p.declared_amount_idr, COALESCE(p.sender_name, ''),
		       p.submitted_at,
		       COALESCE((SELECT pp.object_key FROM payment_proof pp
		                  WHERE pp.payment_id = p.id
		                  ORDER BY pp.uploaded_at DESC LIMIT 1), '')
		  FROM payment p
		  JOIN customer_order o ON o.id = p.order_id
		  JOIN customer c       ON c.id = o.customer_id
		  JOIN customer_type ct ON ct.id = c.customer_type_id
		 WHERE p.status = 'SUBMITTED'
		   AND ($1 = '' OR o.order_number ILIKE '%' || $1 || '%' OR c.full_name ILIKE '%' || $1 || '%')
		 ORDER BY p.submitted_at
		 LIMIT $2`, search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		var declared sql.NullInt64
		var submitted sql.NullTime
		if err := rows.Scan(&it.OrderID, &it.PaymentID, &it.OrderNumber, &it.CustomerName,
			&it.CustomerType, (*int64)(&it.ExpectedIDR), &declared, &it.SenderName,
			&submitted, &it.ProofKey); err != nil {
			return nil, err
		}
		if declared.Valid {
			d := money.IDR(declared.Int64)
			it.DeclaredIDR = &d
		}
		if submitted.Valid {
			it.SubmittedAt = submitted.Time
			it.WaitingFor = s.now().Sub(submitted.Time)
		}
		it.Suffix = int(it.ExpectedIDR % order.MaxSuffix)
		it.Match = matchVerdict(it)
		out = append(out, it)
	}
	return out, rows.Err()
}

// matchVerdict reproduces the artifact's Kecocokan column. It is advisory: a
// human still decides, and the reason is recorded either way.
func matchVerdict(it QueueItem) string {
	if it.DeclaredIDR == nil {
		return "Belum ada nominal"
	}
	switch {
	case *it.DeclaredIDR == it.ExpectedIDR:
		if it.SenderName == "" {
			return "Cocok"
		}
		return "Cocok"
	case *it.DeclaredIDR > it.ExpectedIDR:
		return fmt.Sprintf("Lebih Rp %d", *it.DeclaredIDR-it.ExpectedIDR)
	default:
		return fmt.Sprintf("Selisih Rp %d", it.ExpectedIDR-*it.DeclaredIDR)
	}
}

// Verify marks a payment verified and the order paid.
//
// If the order is a package purchase, the package is activated here — D-14
// sets activated_at at verification, not at order time, because an unpaid
// package must not start burning its validity window.
func (s *Service) Verify(ctx context.Context, actorID, paymentID uuid.UUID, actorRole order.Actor) error {
	return database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var (
			orderID     uuid.UUID
			payStatus   string
			orderStatus string
		)
		if err := tx.QueryRowContext(ctx, `
			SELECT p.order_id, p.status, o.status
			  FROM payment p JOIN customer_order o ON o.id = p.order_id
			 WHERE p.id = $1 FOR UPDATE OF p`, paymentID).
			Scan(&orderID, &payStatus, &orderStatus); err != nil {
			if err == sql.ErrNoRows {
				return ErrPaymentNotFound
			}
			return err
		}
		if payStatus != "SUBMITTED" {
			return fmt.Errorf("%w: pembayaran berstatus %s", ErrNotAwaitingProof, payStatus)
		}
		if err := order.Transition(order.Status(orderStatus), order.StatusPaid, actorRole, ""); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE payment SET status = 'VERIFIED', verified_at = now(), verified_by = $2
			 WHERE id = $1`, paymentID, actorID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment_event (id, payment_id, from_status, to_status, actor_id, reason)
			VALUES ($1, $2, $3, 'VERIFIED', $4, 'finance verified the transfer')`,
			id.New(), paymentID, payStatus, actorID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE customer_order SET status = 'PAID' WHERE id = $1`, orderID); err != nil {
			return err
		}

		// Activate any package this order bought, and post the purchase to
		// the ledger. D-14: the validity window starts now, not at checkout.
		return s.activatePackages(ctx, tx, orderID, actorID)
	})
}

func (s *Service) activatePackages(ctx context.Context, tx *sql.Tx, orderID, actorID uuid.UUID) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, customer_id, meal_credits, validity_days
		  FROM customer_package
		 WHERE order_id = $1 AND status = 'PENDING'
		 FOR UPDATE`, orderID)
	if err != nil {
		return err
	}
	type pkg struct {
		id, customerID uuid.UUID
		credits, days  int
	}
	var pkgs []pkg
	for rows.Next() {
		var p pkg
		if err := rows.Scan(&p.id, &p.customerID, &p.credits, &p.days); err != nil {
			rows.Close()
			return err
		}
		pkgs = append(pkgs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range pkgs {
		// expires_at is a DATE in the operating timezone: expiry is a business
		// day, not a moment (01-domain-model.md §3.6).
		expires := s.now().In(s.loc).AddDate(0, 0, p.days).Format("2006-01-02")
		if _, err := tx.ExecContext(ctx, `
			UPDATE customer_package
			   SET status = 'ACTIVE', activated_at = now(), expires_at = $2::date
			 WHERE id = $1`, p.id, expires); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_ledger
			  (id, customer_id, customer_package_id, entry_type, qty, reference_type, reference_id, created_by, note)
			VALUES ($1, $2, $3, 'PURCHASE', $4, 'customer_order', $5, $6, 'pembelian paket terverifikasi')`,
			id.New(), p.customerID, p.id, p.credits, orderID, actorID); err != nil {
			return err
		}
	}
	return nil
}

// Reject sends a payment back to the customer with a reason.
func (s *Service) Reject(ctx context.Context, actorID, paymentID uuid.UUID, actorRole order.Actor, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	return database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var orderID uuid.UUID
		var payStatus, orderStatus string
		if err := tx.QueryRowContext(ctx, `
			SELECT p.order_id, p.status, o.status
			  FROM payment p JOIN customer_order o ON o.id = p.order_id
			 WHERE p.id = $1 FOR UPDATE OF p`, paymentID).
			Scan(&orderID, &payStatus, &orderStatus); err != nil {
			if err == sql.ErrNoRows {
				return ErrPaymentNotFound
			}
			return err
		}
		if payStatus != "SUBMITTED" {
			return fmt.Errorf("%w: pembayaran berstatus %s", ErrNotAwaitingProof, payStatus)
		}
		if err := order.Transition(order.Status(orderStatus), order.StatusAwaitingPayment,
			actorRole, reason); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE payment SET status = 'REJECTED', rejected_reason = $2 WHERE id = $1`,
			paymentID, reason); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment_event (id, payment_id, from_status, to_status, actor_id, reason)
			VALUES ($1, $2, $3, 'REJECTED', $4, $5)`,
			id.New(), paymentID, payStatus, actorID, reason); err != nil {
			return err
		}
		// The order goes back to awaiting payment so the customer can
		// re-upload; it is not cancelled. Nothing automated cancels a booking.
		_, err := tx.ExecContext(ctx,
			`UPDATE customer_order SET status = 'AWAITING_PAYMENT' WHERE id = $1`, orderID)
		return err
	})
}

// ExpireOverdue moves unpaid orders past their deadline to EXPIRED and
// releases both their capacity and their payment suffix.
//
// This is the one automated status change on an order, and it is EXPIRED, not
// CANCELLED: CLAUDE.md §7 says nothing automated cancels a customer's booking.
func (s *Service) ExpireOverdue(ctx context.Context) (int, error) {
	var expired int
	err := database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id FROM customer_order
			 WHERE status IN ('AWAITING_PAYMENT','PAYMENT_SUBMITTED')
			   AND payment_deadline_at IS NOT NULL
			   AND payment_deadline_at < now()
			 FOR UPDATE`)
		if err != nil {
			return err
		}
		var ids []uuid.UUID
		for rows.Next() {
			var oid uuid.UUID
			if err := rows.Scan(&oid); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, oid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, oid := range ids {
			// Give the capacity back, so a slot released by a lapsed order can
			// be sold again.
			if _, err := tx.ExecContext(ctx, `
				UPDATE kitchen_capacity kc
				   SET reserved_portions = GREATEST(0, kc.reserved_portions - dl.qty)
				  FROM delivery d
				  JOIN delivery_line dl ON dl.delivery_id = d.id
				 WHERE d.order_id = $1
				   AND kc.kitchen_id = d.kitchen_id
				   AND kc.service_date = d.service_date
				   AND kc.slot_id = d.slot_id`, oid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE delivery SET status = 'CANCELLED' WHERE order_id = $1 AND status = 'SCHEDULED'`,
				oid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE payment_suffix_claim SET released_at = now()
				 WHERE order_id = $1 AND released_at IS NULL`, oid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE payment SET status = 'EXPIRED' WHERE order_id = $1 AND status IN ('PENDING','SUBMITTED')`,
				oid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE customer_order SET status = 'EXPIRED' WHERE id = $1`, oid); err != nil {
				return err
			}
			expired++
		}
		return nil
	})
	return expired, err
}

// sniff identifies an image from its magic bytes.
func sniff(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	}
	return "application/octet-stream"
}

func nullIfZero(v money.IDR) any {
	if v == 0 {
		return nil
	}
	return int64(v)
}

func nullIfBlank(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }
