// Package test holds integration tests that need a real PostgreSQL.
//
// These exist because CLAUDE.md §4 says the database enforces the invariant,
// not just the application. A CHECK constraint that was never exercised is a
// comment. Every test here attempts a write that must fail, and asserts that
// the database refused it.
//
// Run with:  TEST_DATABASE_URL=... go test ./test/...
package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/db"
	"github.com/stevenwilliam/evermore/internal/platform/database"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Skipping silently would let a green run mean nothing. Say why.
		println("test: TEST_DATABASE_URL is not set; integration tests cannot run")
		os.Exit(0)
	}
	ctx := context.Background()
	conn, err := database.Open(ctx, database.Options{DSN: dsn})
	if err != nil {
		panic(err)
	}
	migrations, err := database.Load(db.Migrations, "migrations")
	if err != nil {
		panic(err)
	}
	if _, err := database.Up(ctx, conn, migrations); err != nil {
		panic(err)
	}
	testDB = conn
	// Start from a known state. Without this the suite is not re-runnable:
	// the second run inherits the first run's tier bands and every exclusion
	// constraint fires during setup rather than during the assertion.
	if err := truncateAll(ctx, conn); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = conn.Close()
	os.Exit(code)
}

// mustFail runs a statement that the database is required to refuse, and
// fails the test if it succeeded. It returns the error message so a test can
// assert on which constraint fired.
func mustFail(t *testing.T, why, stmt string, args ...any) string {
	t.Helper()
	_, err := testDB.ExecContext(context.Background(), stmt, args...)
	if err == nil {
		t.Fatalf("%s: the database ACCEPTED a write it must refuse", why)
	}
	return err.Error()
}

func mustExec(t *testing.T, stmt string, args ...any) {
	t.Helper()
	if _, err := testDB.ExecContext(context.Background(), stmt, args...); err != nil {
		t.Fatalf("setup failed: %v\nstatement: %s", err, stmt)
	}
}

func assertMentions(t *testing.T, errText, constraint string) {
	t.Helper()
	if !strings.Contains(errText, constraint) {
		t.Errorf("the write was refused, but by %q rather than %s", errText, constraint)
	}
}

// ---------------------------------------------------------------------------
// Money invariants.
// ---------------------------------------------------------------------------

func TestOrderTaxMustReconcile(t *testing.T) {
	custID := seedCustomer(t)
	id := uuid.New()
	// base + tax must equal total. This is §3.11 stated where the database can
	// refuse a violation rather than trusting the code that computes it.
	e := mustFail(t, "order whose tax split does not sum to the total", `
		INSERT INTO customer_order
		  (id, order_number, customer_id, order_type, status,
		   total_idr, tax_base_idr, tax_idr, tax_rate_bps,
		   payment_amount_idr, payment_rounding_idr)
		VALUES ($1, $2, $3, 'MEAL', 'DRAFT', 500000, 450450, 40000, 1100, 500000, 0)`,
		id, "EVM-TEST-0001", custID)
	assertMentions(t, e, "customer_order_tax_reconciles")

	// The correct split from the worked example is accepted.
	mustExec(t, `
		INSERT INTO customer_order
		  (id, order_number, customer_id, order_type, status,
		   total_idr, tax_base_idr, tax_idr, tax_rate_bps,
		   payment_amount_idr, payment_rounding_idr)
		VALUES ($1, $2, $3, 'MEAL', 'DRAFT', 500000, 450450, 49550, 1100, 500148, 148)`,
		uuid.New(), "EVM-TEST-0002", custID)
}

func TestOrderPaymentAmountMustReconcile(t *testing.T) {
	custID := seedCustomer(t)
	e := mustFail(t, "payment amount that is not total plus the rounding", `
		INSERT INTO customer_order
		  (id, order_number, customer_id, order_type, status,
		   total_idr, tax_base_idr, tax_idr, tax_rate_bps,
		   payment_amount_idr, payment_rounding_idr)
		VALUES ($1, $2, $3, 'MEAL', 'DRAFT', 500000, 450450, 49550, 1100, 999999, 148)`,
		uuid.New(), "EVM-TEST-0003", custID)
	assertMentions(t, e, "customer_order_payment_reconciles")
}

func TestOrderLineTotalMustBeUnitTimesQty(t *testing.T) {
	custID := seedCustomer(t)
	orderID := seedOrder(t, custID, "EVM-TEST-0010")
	mealID := seedScheduledMeal(t)

	e := mustFail(t, "line total that is not unit x qty", `
		INSERT INTO order_line
		  (id, order_id, scheduled_meal_id, qty, unit_price_idr, normal_price_idr,
		   line_total_idr, line_tax_base_idr, line_tax_idr)
		VALUES ($1, $2, $3, 4, 75000, 78000, 999999, 900900, 99099)`,
		uuid.New(), orderID, mealID)
	assertMentions(t, e, "order_line_total_is_unit_times_qty")
}

func TestOrderLineIsMealOrPackageNeverBoth(t *testing.T) {
	custID := seedCustomer(t)
	orderID := seedOrder(t, custID, "EVM-TEST-0011")
	mealID := seedScheduledMeal(t)
	pkgID := seedPackage(t)

	e := mustFail(t, "a line that is both a meal and a package", `
		INSERT INTO order_line
		  (id, order_id, scheduled_meal_id, package_id, qty, unit_price_idr,
		   normal_price_idr, line_total_idr, line_tax_base_idr, line_tax_idr)
		VALUES ($1, $2, $3, $4, 1, 1000, 1000, 1000, 901, 99)`,
		uuid.New(), orderID, mealID, pkgID)
	assertMentions(t, e, "order_line_exactly_one_subject")

	e = mustFail(t, "a line that is neither a meal nor a package", `
		INSERT INTO order_line
		  (id, order_id, qty, unit_price_idr, normal_price_idr,
		   line_total_idr, line_tax_base_idr, line_tax_idr)
		VALUES ($1, $2, 1, 1000, 1000, 1000, 901, 99)`,
		uuid.New(), orderID)
	assertMentions(t, e, "order_line_exactly_one_subject")
}

// ---------------------------------------------------------------------------
// Capacity — the invariant the oversell test attacks.
// ---------------------------------------------------------------------------

func TestCapacityCannotBeOversold(t *testing.T) {
	kitchenID := seedKitchen(t, "OVR-01")
	slotID := seedSlot(t, "11:30:00")
	capID := uuid.New()
	mustExec(t, `
		INSERT INTO kitchen_capacity (id, kitchen_id, service_date, slot_id, max_portions, reserved_portions)
		VALUES ($1, $2, DATE '2026-09-01', $3, 40, 40)`, capID, kitchenID, slotID)

	// Reserving one more than the maximum must be refused by the database,
	// not merely avoided by the application.
	e := mustFail(t, "reserving beyond max_portions", `
		UPDATE kitchen_capacity SET reserved_portions = reserved_portions + 1 WHERE id = $1`, capID)
	assertMentions(t, e, "kitchen_capacity_not_oversold")
}

func TestCapacityConcurrentReservationsCannotOversell(t *testing.T) {
	// CLAUDE.md §4: "Anything reserving a limited resource takes
	// SELECT ... FOR UPDATE inside one transaction and ships with a test that
	// proves it cannot oversell." This is that test.
	kitchenID := seedKitchen(t, "OVR-02")
	slotID := seedSlot(t, "12:00:00")
	capID := uuid.New()
	const maxPortions = 20
	mustExec(t, `
		INSERT INTO kitchen_capacity (id, kitchen_id, service_date, slot_id, max_portions, reserved_portions)
		VALUES ($1, $2, DATE '2026-09-02', $3, $4, 0)`, capID, kitchenID, slotID, maxPortions)

	const writers = 40 // twice the capacity, so half must lose
	type result struct{ ok bool }
	results := make(chan result, writers)
	start := make(chan struct{})

	for i := 0; i < writers; i++ {
		go func() {
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			err := database.InTx(ctx, testDB, nil, func(tx *sql.Tx) error {
				var reserved, max int
				// The lock. Without FOR UPDATE two readers see the same value
				// and both believe there is room.
				if err := tx.QueryRowContext(ctx,
					`SELECT reserved_portions, max_portions FROM kitchen_capacity WHERE id = $1 FOR UPDATE`,
					capID).Scan(&reserved, &max); err != nil {
					return err
				}
				if reserved+1 > max {
					return sql.ErrNoRows // no room; this writer loses
				}
				_, err := tx.ExecContext(ctx,
					`UPDATE kitchen_capacity SET reserved_portions = reserved_portions + 1 WHERE id = $1`, capID)
				return err
			})
			results <- result{ok: err == nil}
		}()
	}
	close(start)

	won := 0
	for i := 0; i < writers; i++ {
		if (<-results).ok {
			won++
		}
	}

	var reserved int
	if err := testDB.QueryRow(`SELECT reserved_portions FROM kitchen_capacity WHERE id = $1`, capID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if reserved > maxPortions {
		t.Fatalf("OVERSOLD: reserved_portions = %d with a maximum of %d", reserved, maxPortions)
	}
	if won != maxPortions {
		t.Errorf("%d writers succeeded, want exactly %d", won, maxPortions)
	}
	if reserved != maxPortions {
		t.Errorf("reserved_portions = %d, want the capacity to be exactly filled at %d", reserved, maxPortions)
	}
	t.Logf("%d concurrent writers, %d won, reserved=%d/%d", writers, won, reserved, maxPortions)
}

// ---------------------------------------------------------------------------
// Append-only history.
// ---------------------------------------------------------------------------

func TestAuditLogIsAppendOnly(t *testing.T) {
	id := uuid.New()
	mustExec(t, `INSERT INTO audit_log (id, action, entity_type) VALUES ($1, 'test.write', 'test')`, id)

	e := mustFail(t, "updating an audit row", `UPDATE audit_log SET action = 'tampered' WHERE id = $1`, id)
	assertMentions(t, e, "append-only")

	e = mustFail(t, "deleting an audit row", `DELETE FROM audit_log WHERE id = $1`, id)
	assertMentions(t, e, "append-only")
}

func TestCreditLedgerIsAppendOnly(t *testing.T) {
	custID := seedCustomer(t)
	pkgID := seedCustomerPackage(t, custID)
	id := uuid.New()
	mustExec(t, `
		INSERT INTO credit_ledger (id, customer_id, customer_package_id, entry_type, qty)
		VALUES ($1, $2, $3, 'PURCHASE', 20)`, id, custID, pkgID)

	e := mustFail(t, "updating a ledger entry", `UPDATE credit_ledger SET qty = 999 WHERE id = $1`, id)
	assertMentions(t, e, "append-only")

	e = mustFail(t, "deleting a ledger entry", `DELETE FROM credit_ledger WHERE id = $1`, id)
	assertMentions(t, e, "append-only")
}

func TestCreditLedgerSignDiscipline(t *testing.T) {
	custID := seedCustomer(t)
	pkgID := seedCustomerPackage(t, custID)

	// A REDEEM that adds credit is nonsense the database must refuse.
	e := mustFail(t, "a REDEEM with a positive quantity", `
		INSERT INTO credit_ledger (id, customer_id, customer_package_id, entry_type, qty)
		VALUES ($1, $2, $3, 'REDEEM', 5)`, uuid.New(), custID, pkgID)
	assertMentions(t, e, "credit_ledger_sign_matches_type")

	e = mustFail(t, "a PURCHASE with a negative quantity", `
		INSERT INTO credit_ledger (id, customer_id, customer_package_id, entry_type, qty)
		VALUES ($1, $2, $3, 'PURCHASE', -5)`, uuid.New(), custID, pkgID)
	assertMentions(t, e, "credit_ledger_sign_matches_type")

	e = mustFail(t, "an ADJUSTMENT with no reason", `
		INSERT INTO credit_ledger (id, customer_id, customer_package_id, entry_type, qty, note)
		VALUES ($1, $2, $3, 'ADJUSTMENT', 3, '')`, uuid.New(), custID, pkgID)
	assertMentions(t, e, "credit_ledger_adjustment_has_note")
}

// ---------------------------------------------------------------------------
// Pricing: the exclusion constraints.
// ---------------------------------------------------------------------------

func TestPriceRowsCannotOverlap(t *testing.T) {
	dietID := seedDietType(t, "excl-test")
	tierID := seedTier(t, 1, 3)

	mustExec(t, `
		INSERT INTO meal_price_normal (id, diet_type_id, tier_id, unit_price_idr, validity)
		VALUES ($1, $2, $3, 78000, daterange(DATE '2026-09-01', NULL, '[)'))`,
		uuid.New(), dietID, tierID)

	// A second open-ended DEFAULT row for the same diet and tier overlaps.
	// This is the case that a nullable customer_type_id could not catch,
	// because NULL <> NULL — the generated scope_key is what makes it work.
	e := mustFail(t, "an overlapping DEFAULT price row", `
		INSERT INTO meal_price_normal (id, diet_type_id, tier_id, unit_price_idr, validity)
		VALUES ($1, $2, $3, 71000, daterange(DATE '2026-10-01', NULL, '[)'))`,
		uuid.New(), dietID, tierID)
	assertMentions(t, e, "meal_price_normal_no_overlap")

	// A row that starts after the first one is closed is fine.
	mustExec(t, `
		UPDATE meal_price_normal SET validity = daterange(DATE '2026-09-01', DATE '2026-10-01', '[)')
		WHERE diet_type_id = $1 AND tier_id = $2`, dietID, tierID)
	mustExec(t, `
		INSERT INTO meal_price_normal (id, diet_type_id, tier_id, unit_price_idr, validity)
		VALUES ($1, $2, $3, 71000, daterange(DATE '2026-10-01', NULL, '[)'))`,
		uuid.New(), dietID, tierID)
}

func TestScopeKeyIsGeneratedAndCannotDrift(t *testing.T) {
	dietID := seedDietType(t, "scope-test")
	tierID := seedTier(t, 10, 0)
	ctID := seedCustomerType(t, "scope-corp")

	id := uuid.New()
	mustExec(t, `
		INSERT INTO meal_price_normal (id, customer_type_id, diet_type_id, tier_id, unit_price_idr, validity)
		VALUES ($1, $2, $3, $4, 70000, daterange(DATE '2026-09-01', NULL, '[)'))`,
		id, ctID, dietID, tierID)

	var scope string
	if err := testDB.QueryRow(`SELECT scope_key FROM meal_price_normal WHERE id = $1`, id).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	want := "CT:" + ctID.String()
	if scope != want {
		t.Errorf("scope_key = %q, want %q", scope, want)
	}

	// It is generated, so it cannot be written directly and therefore cannot
	// drift from customer_type_id.
	e := mustFail(t, "writing scope_key directly", `
		UPDATE meal_price_normal SET scope_key = 'DEFAULT' WHERE id = $1`, id)
	// PostgreSQL 18 words this as "can only be updated to DEFAULT". Asserting
	// on the column name plus the refusal keeps the test from being tied to
	// one release's phrasing while still proving the right thing was refused.
	if !strings.Contains(e, "scope_key") {
		t.Errorf("expected the refusal to name scope_key, got %q", e)
	}
}

func TestTierBandsCannotOverlap(t *testing.T) {
	// A band of this test's own, so the collision it asserts is the one it
	// created and not one inherited from another test.
	base := 100000 + int(tierBand.Add(1))*1000
	mustExec(t, `INSERT INTO meal_price_tier (id, name, min_qty, max_qty) VALUES ($1, 'A', $2, $3)`,
		uuid.New(), base+500, base+599)
	e := mustFail(t, "overlapping tier bands", `
		INSERT INTO meal_price_tier (id, name, min_qty, max_qty) VALUES ($1, 'B', $2, $3)`,
		uuid.New(), base+550, base+650)
	assertMentions(t, e, "meal_price_tier_no_overlap")
}

// ---------------------------------------------------------------------------
// Other invariants worth proving.
// ---------------------------------------------------------------------------

func TestOneDefaultAddressPerCustomer(t *testing.T) {
	custID := seedCustomer(t)
	mustExec(t, `
		INSERT INTO customer_address
		  (id, customer_id, recipient_name, recipient_phone, address_line, latitude, longitude, is_default)
		VALUES ($1, $2, 'Sinta', '0812', 'Jl. Wijaya IX No. 12', -6.2400000, 106.7980000, true)`,
		uuid.New(), custID)

	e := mustFail(t, "a second default address", `
		INSERT INTO customer_address
		  (id, customer_id, recipient_name, recipient_phone, address_line, latitude, longitude, is_default)
		VALUES ($1, $2, 'Sinta', '0812', 'Menara Sudirman', -6.2200000, 106.8100000, true)`,
		uuid.New(), custID)
	assertMentions(t, e, "customer_address_one_default_uk")

	// A non-default second address is fine.
	mustExec(t, `
		INSERT INTO customer_address
		  (id, customer_id, recipient_name, recipient_phone, address_line, latitude, longitude, is_default)
		VALUES ($1, $2, 'Sinta', '0812', 'Menara Sudirman', -6.2200000, 106.8100000, false)`,
		uuid.New(), custID)
}

func TestPasswordHashMustBeArgon2id(t *testing.T) {
	e := mustFail(t, "a bcrypt hash in the password column", `
		INSERT INTO app_user (id, email, password_hash)
		VALUES ($1, 'bcrypt@example.com', '$2a$10$abcdefghijklmnopqrstuv')`, uuid.New())
	assertMentions(t, e, "app_user_password_is_argon2id")

	e = mustFail(t, "a plaintext password", `
		INSERT INTO app_user (id, email, password_hash)
		VALUES ($1, 'plain@example.com', 'hunter2')`, uuid.New())
	assertMentions(t, e, "app_user_password_is_argon2id")
}

func TestPaymentProofRejectsOversizeAndWrongType(t *testing.T) {
	custID := seedCustomer(t)
	orderID := seedOrder(t, custID, "EVM-TEST-0020")
	payID := uuid.New()
	mustExec(t, `
		INSERT INTO payment (id, order_id, expected_amount_idr) VALUES ($1, $2, 480148)`, payID, orderID)

	// The artifact says JPG or PNG, maximum 5 MB. The handler enforces it too,
	// but the handler can be bypassed with curl.
	e := mustFail(t, "a PDF proof", `
		INSERT INTO payment_proof (id, payment_id, object_key, mime_type, size_bytes, sha256)
		VALUES ($1, $2, 'k', 'application/pdf', 1000, 'x')`, uuid.New(), payID)
	assertMentions(t, e, "payment_proof_mime_allowed")

	e = mustFail(t, "a proof over 5 MB", `
		INSERT INTO payment_proof (id, payment_id, object_key, mime_type, size_bytes, sha256)
		VALUES ($1, $2, 'k', 'image/png', 5242881, 'x')`, uuid.New(), payID)
	assertMentions(t, e, "payment_proof_size_limit")
}

func TestScheduledMealPublishedStateIsConsistent(t *testing.T) {
	dietID := seedDietType(t, "pub-test")
	slotID := seedSlot(t, "07:00:00")

	e := mustFail(t, "PUBLISHED with no published_at", `
		INSERT INTO scheduled_meal (id, service_date, diet_type_id, slot_id, name, status)
		VALUES ($1, DATE '2026-09-10', $2, $3, 'Test', 'PUBLISHED')`, uuid.New(), dietID, slotID)
	assertMentions(t, e, "scheduled_meal_published_at_consistent")

	e = mustFail(t, "DRAFT that carries a published_at", `
		INSERT INTO scheduled_meal (id, service_date, diet_type_id, slot_id, name, status, published_at)
		VALUES ($1, DATE '2026-09-11', $2, $3, 'Test', 'DRAFT', now())`, uuid.New(), dietID, slotID)
	assertMentions(t, e, "scheduled_meal_published_at_consistent")
}

func TestSlotMustBeOnTheFifteenMinuteGrid(t *testing.T) {
	e := mustFail(t, "a slot at 11:37", `
		INSERT INTO delivery_time_slot (id, slot_time, alias) VALUES ($1, TIME '11:37', 'Aneh')`, uuid.New())
	assertMentions(t, e, "delivery_time_slot_on_grid")
}

func TestCancelledOrderMustCarryAReason(t *testing.T) {
	custID := seedCustomer(t)
	e := mustFail(t, "a cancelled order with no reason", `
		INSERT INTO customer_order
		  (id, order_number, customer_id, order_type, status, total_idr, tax_base_idr, tax_idr, payment_amount_idr)
		VALUES ($1, $2, $3, 'MEAL', 'CANCELLED', 0, 0, 0, 0)`,
		uuid.New(), "EVM-TEST-0030", custID)
	assertMentions(t, e, "customer_order_cancelled_has_reason")
}

func TestOrderNumberSequenceIsUniqueUnderConcurrency(t *testing.T) {
	const n = 50
	got := make(chan string, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			<-start
			var num string
			if err := testDB.QueryRow(`SELECT next_order_number('CONC', '2699')`).Scan(&num); err != nil {
				got <- ""
				return
			}
			got <- num
		}()
	}
	close(start)

	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		num := <-got
		if num == "" {
			t.Fatal("next_order_number returned an error")
		}
		if seen[num] {
			t.Fatalf("DUPLICATE order number %s under %d concurrent callers", num, n)
		}
		seen[num] = true
	}
	if len(seen) != n {
		t.Errorf("got %d distinct numbers, want %d", len(seen), n)
	}
}

// ---------------------------------------------------------------------------
// Seed helpers. Each returns the id of a freshly created row so tests do not
// depend on each other's data.
// ---------------------------------------------------------------------------

func uniq(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8]
}

func seedCustomerType(t *testing.T, slug string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExec(t, `INSERT INTO customer_type (id, name, slug) VALUES ($1, $2, $3)`, id, slug, uniq(slug))
	return id
}

func seedCustomer(t *testing.T) uuid.UUID {
	t.Helper()
	userID, custID, ctID := uuid.New(), uuid.New(), seedCustomerType(t, "retail")
	mustExec(t, `
		INSERT INTO app_user (id, email, password_hash)
		VALUES ($1, $2, '$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA')`,
		userID, uniq("user")+"@example.com")
	mustExec(t, `
		INSERT INTO customer (id, user_id, customer_type_id, full_name)
		VALUES ($1, $2, $3, 'Sinta Prameswari')`, custID, userID, ctID)
	return custID
}

func seedOrder(t *testing.T, custID uuid.UUID, number string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExec(t, `
		INSERT INTO customer_order
		  (id, order_number, customer_id, order_type, status,
		   total_idr, tax_base_idr, tax_idr, tax_rate_bps, payment_amount_idr, payment_rounding_idr)
		VALUES ($1, $2, $3, 'MEAL', 'DRAFT', 500000, 450450, 49550, 1100, 500148, 148)`,
		id, uniq(number), custID)
	return id
}

func seedDietType(t *testing.T, slug string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExec(t, `INSERT INTO diet_type (id, name, slug) VALUES ($1, 'Balanced', $2)`, id, uniq(slug))
	return id
}

func seedSlot(t *testing.T, at string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	// Slots are unique by time, so reuse one if it already exists.
	var existing uuid.UUID
	err := testDB.QueryRow(`SELECT id FROM delivery_time_slot WHERE slot_time = $1::time`, at).Scan(&existing)
	if err == nil {
		return existing
	}
	mustExec(t, `INSERT INTO delivery_time_slot (id, slot_time, alias) VALUES ($1, $2::time, 'Slot')`, id, at)
	return id
}

func seedKitchen(t *testing.T, code string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExec(t, `
		INSERT INTO kitchen (id, code, name, latitude, longitude)
		VALUES ($1, $2, 'Dapur Tes', -6.2260000, 106.8480000)`, id, uniq(code))
	return id
}

func seedScheduledMeal(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	dietID := seedDietType(t, "meal")
	slotID := seedSlot(t, "11:30:00")
	mustExec(t, `
		INSERT INTO scheduled_meal (id, service_date, diet_type_id, slot_id, name, status, published_at)
		VALUES ($1, $2, $3, $4, 'Ayam panggang lemon & quinoa', 'PUBLISHED', now())`,
		id, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), dietID, slotID)
	return id
}

func seedPackage(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExec(t, `
		INSERT INTO package (id, name, slug, meal_credits, validity_days)
		VALUES ($1, 'Paket 20', $2, 20, 90)`, id, uniq("paket-20"))
	return id
}

func seedCustomerPackage(t *testing.T, custID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	pkgID := seedPackage(t)
	mustExec(t, `
		INSERT INTO customer_package
		  (id, customer_id, package_id, package_number, meal_credits, validity_days,
		   price_paid_idr, status, activated_at, expires_at)
		VALUES ($1, $2, $3, $4, 20, 90, 1420000, 'ACTIVE', now(), DATE '2026-11-30')`,
		id, custID, pkgID, uniq("PKG-2609"))
	return id
}

// seedTier creates a tier band in a private, non-overlapping quantity range.
//
// meal_price_tier_no_overlap is global across the table, so two tests that
// both wanted "1-3" would collide during setup rather than during their
// assertion. Each call takes the next band from an atomic counter, which makes
// the tests order-independent — verified with `go test -shuffle=on`.
//
// Bands here are always bounded. An unbounded band would cover every band
// allocated after it and reintroduce exactly the ordering dependency this
// helper exists to remove; unbounded-tier semantics are covered by the pure
// unit tests in internal/domain/pricing.
func seedTier(t *testing.T, minQty, maxQty int) uuid.UUID {
	t.Helper()
	const bandWidth = 1000
	base := 100000 + int(tierBand.Add(1))*bandWidth
	if maxQty <= 0 || maxQty < minQty {
		maxQty = minQty + 100
	}
	id := uuid.New()
	mustExec(t, `INSERT INTO meal_price_tier (id, name, min_qty, max_qty) VALUES ($1, 'T', $2, $3)`,
		id, base+minQty, base+maxQty)
	return id
}

// tierBand hands out a fresh quantity band per seedTier call.
var tierBand atomic.Int64

// truncateAll empties every data table, leaving the schema and the migration
// history intact. It is driven off the catalogue rather than a hand-kept list,
// so a table added in a later migration is cleaned without anyone remembering
// to add it here.
func truncateAll(ctx context.Context, conn *sql.DB) error {
	rows, err := conn.QueryContext(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename NOT IN ('schema_migration', 'spatial_ref_sys')`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		names = append(names, `"`+n+`"`)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("test: no tables found to truncate — is the schema migrated?")
	}

	// The append-only triggers refuse DELETE, but TRUNCATE is a different
	// operation and is not caught by a row-level trigger, which is what makes
	// a clean slate possible here without weakening the production guarantee.
	_, err = conn.ExecContext(ctx,
		"TRUNCATE TABLE "+strings.Join(names, ", ")+" RESTART IDENTITY CASCADE")
	return err
}
