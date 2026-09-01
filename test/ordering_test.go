package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/app/ordering"
	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// orderingSvc builds the service with a fixed clock, so cut-off behaviour is
// deterministic rather than depending on when the suite happens to run.
func orderingSvc(t *testing.T, now time.Time) *ordering.Service {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return ordering.NewService(testDB, loc, func() time.Time { return now })
}

// jakartaClock returns a time on a service date, before the 15:00 cut-off for
// the day after.
func jakartaClock(t *testing.T, y int, m time.Month, d, hh, mm int) time.Time {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	return time.Date(y, m, d, hh, mm, 0, 0, loc)
}

// futureMeal finds a published meal at least two days out, so the cut-off for
// its service date has not passed relative to the test clock.
func futureMeal(t *testing.T, minDaysAhead int) (uuid.UUID, time.Time) {
	t.Helper()
	seedOnce(t)
	var id uuid.UUID
	var date time.Time
	err := testDB.QueryRow(`
		SELECT sm.id, sm.service_date
		  FROM scheduled_meal sm
		  JOIN diet_type dt ON dt.id = sm.diet_type_id
		 WHERE sm.status = 'PUBLISHED'
		   AND dt.slug = 'balanced'
		   AND sm.service_date >= (CURRENT_DATE + $1::int)
		 ORDER BY sm.service_date, sm.id
		 LIMIT 1`, minDaysAhead).Scan(&id, &date)
	if err != nil {
		t.Skipf("no published meal at least %d days out: %v", minDaysAhead, err)
	}
	return id, date
}

func customerAndAddress(t *testing.T, email string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var custID, addrID uuid.UUID
	if err := testDB.QueryRow(`
		SELECT c.id, a.id FROM customer c
		  JOIN app_user u ON u.id = c.user_id
		  JOIN customer_address a ON a.customer_id = c.id AND a.is_default
		 WHERE u.email = $1`, email).Scan(&custID, &addrID); err != nil {
		t.Fatalf("loading %s: %v", email, err)
	}
	return custID, addrID
}

func TestQuote_TierResolvesOnTheWholeCart(t *testing.T) {
	seedOnce(t)
	mealID, date := futureMeal(t, 2)
	custID, _ := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	// One portion sits in the 1-3 band at Rp 78.000.
	q1, err := svc.Quote(context.Background(), custID, []ordering.CartItem{{ScheduledMealID: mealID, Qty: 1}})
	if err != nil {
		t.Fatalf("quoting 1: %v", err)
	}
	if q1.Lines[0].UnitPrice != 78000 {
		t.Errorf("qty 1 priced at %d, want 78000", q1.Lines[0].UnitPrice)
	}

	// Six portions crosses into 4-9 at Rp 75.000 — the artifact's cart.
	q6, err := svc.Quote(context.Background(), custID, []ordering.CartItem{{ScheduledMealID: mealID, Qty: 6}})
	if err != nil {
		t.Fatalf("quoting 6: %v", err)
	}
	if q6.Lines[0].UnitPrice != 75000 {
		t.Errorf("qty 6 priced at %d, want 75000", q6.Lines[0].UnitPrice)
	}
	if q6.Subtotal != 450000 {
		t.Errorf("subtotal = %d, want 450000 — the artifact's cart", q6.Subtotal)
	}
	// The single-portion price is carried so the UI can show what was saved.
	if q6.Lines[0].NormalPrice != 78000 {
		t.Errorf("normal price = %d, want 78000", q6.Lines[0].NormalPrice)
	}
	// Ten crosses into 10+.
	q10, err := svc.Quote(context.Background(), custID, []ordering.CartItem{{ScheduledMealID: mealID, Qty: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if q10.Lines[0].UnitPrice != 71000 {
		t.Errorf("qty 10 priced at %d, want 71000", q10.Lines[0].UnitPrice)
	}
	_ = date
}

func TestQuote_TierSpansDates_D14(t *testing.T) {
	seedOnce(t)
	custID, _ := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	// Two different meals on different dates, 2 + 2 = 4 meals total. D14: the
	// tier resolves on the ORDER's total quantity, so this reaches the 4-9
	// band even though no single line does.
	rows, err := testDB.Query(`
		SELECT sm.id FROM scheduled_meal sm
		  JOIN diet_type dt ON dt.id = sm.diet_type_id
		 WHERE sm.status='PUBLISHED' AND dt.slug='balanced'
		   AND sm.service_date >= CURRENT_DATE + 2
		 ORDER BY sm.service_date, sm.id LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 2 {
		t.Skip("need two published meals at least two days out")
	}

	q, err := svc.Quote(context.Background(), custID, []ordering.CartItem{
		{ScheduledMealID: ids[0], Qty: 2},
		{ScheduledMealID: ids[1], Qty: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q.TotalQty != 4 {
		t.Fatalf("total qty = %d, want 4", q.TotalQty)
	}
	for i, l := range q.Lines {
		if l.UnitPrice != 75000 {
			t.Errorf("line %d priced at %d, want 75000 — the tier is the cart's total, not the line's",
				i, l.UnitPrice)
		}
	}
}

func TestQuote_TaxIsTheSumOfLineTaxes(t *testing.T) {
	seedOnce(t)
	mealID, _ := futureMeal(t, 2)
	custID, _ := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	q, err := svc.Quote(context.Background(), custID, []ordering.CartItem{{ScheduledMealID: mealID, Qty: 6}})
	if err != nil {
		t.Fatal(err)
	}
	if q.TaxRateBPS != 1100 {
		t.Fatalf("tax rate = %d bps, want 1100", q.TaxRateBPS)
	}

	var sum money.IDR
	for _, l := range q.Lines {
		sum += l.LineTax.Tax
	}
	if q.Tax.Tax != sum {
		t.Errorf("order tax %d is not the sum of line taxes %d", q.Tax.Tax, sum)
	}
	// The invariant the database also enforces.
	if q.Tax.Base+q.Tax.Tax != q.Total {
		t.Errorf("base %d + tax %d != total %d", q.Tax.Base, q.Tax.Tax, q.Total)
	}
	// The artifact: Rp 450.000 at 11% inclusive is Rp 44.595 of tax.
	if q.Total != 450000 {
		t.Errorf("total = %d, want 450000", q.Total)
	}
}

func TestCheckout_WritesAConsistentOrder(t *testing.T) {
	seedOnce(t)
	mealID, _ := futureMeal(t, 2)
	custID, addrID := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	orderID, number, err := svc.Checkout(context.Background(), ordering.CheckoutInput{
		CustomerID: custID, AddressID: addrID,
		Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: 4}},
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if number == "" {
		t.Fatal("no order number allocated")
	}

	var (
		status                              string
		total, taxBase, tax, payAmt, payRnd int64
		rate                                int
	)
	if err := testDB.QueryRow(`
		SELECT status, total_idr, tax_base_idr, tax_idr, tax_rate_bps,
		       payment_amount_idr, payment_rounding_idr
		  FROM customer_order WHERE id = $1`, orderID).
		Scan(&status, &total, &taxBase, &tax, &rate, &payAmt, &payRnd); err != nil {
		t.Fatal(err)
	}
	if status != "AWAITING_PAYMENT" {
		t.Errorf("status = %s, want AWAITING_PAYMENT", status)
	}
	if taxBase+tax != total {
		t.Errorf("base %d + tax %d != total %d", taxBase, tax, total)
	}
	if payAmt != total+payRnd {
		t.Errorf("payment %d != total %d + rounding %d", payAmt, total, payRnd)
	}
	if payAmt < total {
		t.Errorf("the payment amount %d is LESS than the total %d", payAmt, total)
	}
	// The suffix is the last three digits and is excluded from the tax base.
	if payRnd >= 2000 {
		t.Errorf("rounding %d is larger than the suffix mechanism allows", payRnd)
	}

	// A delivery was created and routed, and capacity was taken.
	var deliveries, reservedAfter int
	if err := testDB.QueryRow(
		`SELECT count(*) FROM delivery WHERE order_id = $1`, orderID).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if deliveries != 1 {
		t.Errorf("%d deliveries created, want 1", deliveries)
	}
	if err := testDB.QueryRow(`
		SELECT kc.reserved_portions FROM kitchen_capacity kc
		  JOIN delivery d ON d.kitchen_id = kc.kitchen_id
		                 AND d.service_date = kc.service_date
		                 AND d.slot_id = kc.slot_id
		 WHERE d.order_id = $1`, orderID).Scan(&reservedAfter); err != nil {
		t.Fatal(err)
	}
	if reservedAfter < 4 {
		t.Errorf("reserved_portions = %d, expected at least the 4 just taken", reservedAfter)
	}

	// The price resolution trace is stored, so "why did they pay that?" is
	// answerable from the record.
	var trace string
	if err := testDB.QueryRow(
		`SELECT price_resolution_trace::text FROM customer_order WHERE id = $1`, orderID).Scan(&trace); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"scope_matched", "tier_id", "row_id", "meal_normal"} {
		if !contains(trace, want) {
			t.Errorf("the trace does not record %q: %s", want, trace)
		}
	}

	// The meal snapshot is on the line, so a later recipe edit cannot rewrite
	// what was sold.
	var snapshot string
	if err := testDB.QueryRow(
		`SELECT meal_snapshot::text FROM order_line WHERE order_id = $1`, orderID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if !contains(snapshot, "items") || !contains(snapshot, "nutrition") {
		t.Errorf("the line snapshot is incomplete: %s", snapshot)
	}
}

func TestCheckout_IsIdempotent(t *testing.T) {
	seedOnce(t)
	mealID, _ := futureMeal(t, 2)
	custID, addrID := customerAndAddress(t, "bagas@example.com")
	svc := orderingSvc(t, time.Now())

	key := uuid.NewString()
	in := ordering.CheckoutInput{
		CustomerID: custID, AddressID: addrID,
		Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: 2}},
		IdempotencyKey: key,
	}

	id1, num1, err := svc.Checkout(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	id2, num2, err := svc.Checkout(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || num1 != num2 {
		t.Errorf("a retried checkout placed a SECOND order: %s/%s vs %s/%s", id1, num1, id2, num2)
	}

	var n int
	if err := testDB.QueryRow(
		`SELECT count(*) FROM customer_order WHERE customer_id = $1 AND idempotency_key = $2`,
		custID, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d orders share the idempotency key, want 1", n)
	}
}

func TestCheckout_RefusesAfterCutOff(t *testing.T) {
	seedOnce(t)
	mealID, serviceDate := futureMeal(t, 2)
	custID, addrID := customerAndAddress(t, "sinta@example.com")

	// A clock set to the service date itself: the 15:00 cut-off the day before
	// has long passed.
	loc, _ := time.LoadLocation("Asia/Jakarta")
	tooLate := time.Date(serviceDate.Year(), serviceDate.Month(), serviceDate.Day(), 9, 0, 0, 0, loc)
	svc := orderingSvc(t, tooLate)

	_, _, err := svc.Checkout(context.Background(), ordering.CheckoutInput{
		CustomerID: custID, AddressID: addrID,
		Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: 1}},
		IdempotencyKey: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("checkout after cut-off was ACCEPTED")
	}
	if !contains(err.Error(), "PAST_CUTOFF") {
		t.Errorf("got %v, want PAST_CUTOFF", err)
	}
}

func TestCheckout_RefusesAnotherCustomersAddress(t *testing.T) {
	seedOnce(t)
	mealID, _ := futureMeal(t, 2)
	sintaID, _ := customerAndAddress(t, "sinta@example.com")
	_, bagasAddr := customerAndAddress(t, "bagas@example.com")
	svc := orderingSvc(t, time.Now())

	// Sinta checking out to Bagas's address. The address is scoped by owner in
	// the QUERY, so this is not-found rather than a successful delivery to
	// someone else's house (IDOR).
	_, _, err := svc.Checkout(context.Background(), ordering.CheckoutInput{
		CustomerID: sintaID, AddressID: bagasAddr,
		Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: 1}},
		IdempotencyKey: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("checkout to another customer's address was ACCEPTED")
	}
	if !contains(err.Error(), "ADDRESS_NOT_FOUND") {
		t.Errorf("got %v, want ADDRESS_NOT_FOUND", err)
	}
}

func TestCheckout_RefusesADraftMeal(t *testing.T) {
	seedOnce(t)
	custID, addrID := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	var draftID uuid.UUID
	err := testDB.QueryRow(
		`SELECT id FROM scheduled_meal WHERE status = 'DRAFT' LIMIT 1`).Scan(&draftID)
	if err != nil {
		t.Skip("no draft meal seeded")
	}

	_, _, err = svc.Checkout(context.Background(), ordering.CheckoutInput{
		CustomerID: custID, AddressID: addrID,
		Items:          []ordering.CartItem{{ScheduledMealID: draftID, Qty: 1}},
		IdempotencyKey: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("a DRAFT meal was sold")
	}
	if !contains(err.Error(), "MENU_NOT_PUBLISHED") {
		t.Errorf("got %v, want MENU_NOT_PUBLISHED", err)
	}
}

// TestCheckout_ConcurrentCannotOversell is the end-to-end version of the
// capacity guarantee: not a synthetic UPDATE loop, but real concurrent
// checkouts competing for the last portions.
func TestCheckout_ConcurrentCannotOversell(t *testing.T) {
	seedOnce(t)
	mealID, serviceDate := futureMeal(t, 3)
	custID, addrID := customerAndAddress(t, "sinta@example.com")
	svc := orderingSvc(t, time.Now())

	// Squeeze EVERY kitchen's capacity for this date+slot, so the total is
	// genuinely scarce. Capping one kitchen is not enough: the router
	// correctly spills to the next kitchen as each fills, which is the
	// "6 pesanan dialihkan ke KBY-02" behaviour the dashboard shows.
	const perKitchen = 2
	res, err := testDB.Exec(`
		UPDATE kitchen_capacity kc
		   SET max_portions = $1, reserved_portions = 0
		  FROM scheduled_meal sm
		 WHERE sm.id = $2
		   AND kc.service_date = sm.service_date
		   AND kc.slot_id = sm.slot_id`, perKitchen, mealID)
	if err != nil {
		t.Fatal(err)
	}
	capped, err := res.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if capped == 0 {
		t.Fatal("no capacity rows matched — the test would prove nothing")
	}

	// Put the quota back afterwards. Leaving it squeezed starves every later
	// checkout test of capacity, which showed up only under -shuffle=on.
	// reserved_portions is deliberately left alone: it is real reservation
	// history, and raising max is enough to free the date up again.
	t.Cleanup(func() {
		if _, err := testDB.Exec(`
			UPDATE kitchen_capacity kc
			   SET max_portions = 40
			  FROM scheduled_meal sm
			 WHERE sm.id = $1
			   AND kc.service_date = sm.service_date
			   AND kc.slot_id = sm.slot_id`, mealID); err != nil {
			t.Errorf("restoring capacity: %v", err)
		}
	})

	// Only the kitchens that actually SERVE this address contribute capacity.
	// Counting every capacity row overstates it: Sinta's address in Kebayoran
	// is outside Kelapa Gading's radius, so that kitchen's portions are not
	// reachable. The predicate here mirrors the one the router uses.
	var reachableKitchens int
	if err := testDB.QueryRow(`
		SELECT count(*)
		  FROM kitchen k, customer_address a, scheduled_meal sm
		 WHERE a.id = $1 AND sm.id = $2
		   AND k.is_active
		   AND CASE WHEN k.service_area IS NOT NULL
		            THEN ST_Covers(k.service_area, a.geom)
		            ELSE ST_DWithin(k.geom, a.geom, k.service_radius_km * 1000)
		       END
		   AND EXISTS (SELECT 1 FROM kitchen_slot ks
		                WHERE ks.kitchen_id = k.id AND ks.slot_id = sm.slot_id AND ks.is_active)
		   AND COALESCE((SELECT kod.is_open FROM kitchen_operating_day kod
		                  WHERE kod.kitchen_id = k.id
		                    AND kod.weekday = EXTRACT(ISODOW FROM sm.service_date)::int), false)
		   AND EXISTS (SELECT 1 FROM kitchen_capacity kc
		                WHERE kc.kitchen_id = k.id AND kc.service_date = sm.service_date
		                  AND kc.slot_id = sm.slot_id)`,
		addrID, mealID).Scan(&reachableKitchens); err != nil {
		t.Fatal(err)
	}
	if reachableKitchens == 0 {
		t.Fatal("no kitchen serves this address for this slot — the test would prove nothing")
	}
	totalCapacity := reachableKitchens * perKitchen

	const writers = 12 // more than the total capacity, so the limit must bind
	var wg sync.WaitGroup
	results := make(chan error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := svc.Checkout(context.Background(), ordering.CheckoutInput{
				CustomerID: custID, AddressID: addrID,
				Items:          []ordering.CartItem{{ScheduledMealID: mealID, Qty: 1}},
				IdempotencyKey: uuid.NewString(),
			})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	won := 0
	for err := range results {
		if err == nil {
			won++
		}
	}

	// The invariant: no capacity row anywhere exceeded its maximum.
	var oversold int
	if err := testDB.QueryRow(`
		SELECT count(*) FROM kitchen_capacity WHERE reserved_portions > max_portions`).Scan(&oversold); err != nil {
		t.Fatal(err)
	}
	if oversold != 0 {
		t.Fatalf("OVERSOLD: %d capacity rows exceed their maximum", oversold)
	}

	var reserved int
	if err := testDB.QueryRow(`
		SELECT COALESCE(sum(kc.reserved_portions), 0)
		  FROM kitchen_capacity kc
		  JOIN scheduled_meal sm ON sm.service_date = kc.service_date AND sm.slot_id = kc.slot_id
		 WHERE sm.id = $1`, mealID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	_ = capped
	t.Logf("%d concurrent checkouts against %d reachable portions (%d serving kitchens x %d): %d succeeded, %d reserved",
		writers, totalCapacity, reachableKitchens, perKitchen, won, reserved)
	if won == 0 {
		t.Fatal("no checkout succeeded at all — the race did not exercise the happy path")
	}
	if won > totalCapacity {
		t.Errorf("%d checkouts succeeded against a total capacity of %d", won, totalCapacity)
	}
	if reserved != won {
		t.Errorf("%d portions reserved but %d checkouts succeeded — they must match", reserved, won)
	}
	if won != totalCapacity {
		t.Errorf("%d succeeded but %d portions were available; the limit should be filled exactly",
			won, totalCapacity)
	}
	_ = serviceDate
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
