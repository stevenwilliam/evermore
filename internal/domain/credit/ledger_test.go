package credit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Matrix from 01-domain-model.md §5.2. The concurrency half of that matrix —
// "two concurrent redemptions of the last credit, one wins" — needs a real
// database and lives in the integration tests; these cover the pure rules.

var (
	cust = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	jkt  = time.FixedZone("WIB", 7*3600)
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, jkt)
}

func activePkg(credits int) Package {
	return Package{
		ID: uuid.New(), CustomerID: cust, MealCredits: credits,
		ExpiresAt: day(2026, 11, 30), Status: StatusActive,
	}
}

func purchase(pkg Package, n int) Entry {
	return Entry{
		ID: uuid.New(), CustomerID: cust, CustomerPackageID: pkg.ID,
		EntryType: EntryPurchase, Qty: n, OccurredAt: day(2026, 8, 24),
	}
}

func TestBalance_PurchaseThenRedeem(t *testing.T) {
	pkg := activePkg(20)
	entries := []Entry{purchase(pkg, 20)}
	if got := Balance(entries); got != 20 {
		t.Fatalf("balance = %d, want 20", got)
	}
	out, err := Redeem(RedeemRequest{
		Package: pkg, Entries: entries, Meals: 2,
		ServiceDate: day(2026, 9, 1), OccurredAt: day(2026, 9, 1),
		ReferenceID: uuid.New(), ReferenceTyp: "delivery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d entries, want 2 — one per meal", len(out))
	}
	for _, e := range out {
		if e.Qty != -1 {
			t.Errorf("redeem qty = %d, want -1", e.Qty)
		}
	}
	if got := Balance(append(entries, out...)); got != 18 {
		t.Errorf("balance after = %d, want 18", got)
	}
}

func TestRedeem_OneCreditPerMealRegardlessOfFoods_D32(t *testing.T) {
	// A four-component meal and a single-dish meal both cost exactly one
	// credit. The ledger never sees a food count at all, which is the point:
	// there is no parameter here that could carry one.
	pkg := activePkg(20)
	out, err := Redeem(RedeemRequest{
		Package: pkg, Entries: []Entry{purchase(pkg, 20)}, Meals: 1,
		ServiceDate: day(2026, 9, 1), OccurredAt: day(2026, 9, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Qty != -1 {
		t.Errorf("one meal must cost exactly one credit, got %d entries", len(out))
	}
}

func TestRedeem_BalanceNeverBelowZero(t *testing.T) {
	pkg := activePkg(20)
	entries := []Entry{purchase(pkg, 1)}
	_, err := Redeem(RedeemRequest{
		Package: pkg, Entries: entries, Meals: 2,
		ServiceDate: day(2026, 9, 1), OccurredAt: day(2026, 9, 1),
	})
	if err != ErrInsufficientCredit {
		t.Errorf("got %v, want ErrInsufficientCredit", err)
	}
	// Exactly enough is fine.
	if _, err := Redeem(RedeemRequest{
		Package: pkg, Entries: entries, Meals: 1,
		ServiceDate: day(2026, 9, 1), OccurredAt: day(2026, 9, 1),
	}); err != nil {
		t.Errorf("spending the last credit must be allowed: %v", err)
	}
}

func TestRedeem_RefusedAfterExpiry_D27(t *testing.T) {
	pkg := activePkg(20)
	_, err := Redeem(RedeemRequest{
		Package: pkg, Entries: []Entry{purchase(pkg, 20)}, Meals: 1,
		ServiceDate: day(2026, 12, 1), // one day past expires_at
		OccurredAt:  day(2026, 9, 1),
	})
	if err != ErrPackageExpired {
		t.Errorf("got %v, want ErrPackageExpired", err)
	}
	// The expiry date itself is still serviceable.
	if _, err := Redeem(RedeemRequest{
		Package: pkg, Entries: []Entry{purchase(pkg, 20)}, Meals: 1,
		ServiceDate: day(2026, 11, 30), OccurredAt: day(2026, 9, 1),
	}); err != nil {
		t.Errorf("the expiry date itself must be serviceable: %v", err)
	}
}

func TestRedeem_RefusedWhenNotActive(t *testing.T) {
	for _, st := range []Status{StatusPending, StatusExpired, StatusCancelled, StatusExhausted} {
		pkg := activePkg(20)
		pkg.Status = st
		_, err := Redeem(RedeemRequest{
			Package: pkg, Entries: []Entry{purchase(pkg, 20)}, Meals: 1,
			ServiceDate: day(2026, 9, 1), OccurredAt: day(2026, 9, 1),
		})
		if err != ErrPackageNotActive {
			t.Errorf("status %s: got %v, want ErrPackageNotActive", st, err)
		}
	}
}

func TestSkip_BeforeCutOffReturnsCredit_AfterDoesNot(t *testing.T) {
	pkg := activePkg(20)
	cutOff := time.Date(2026, 8, 31, 15, 0, 0, 0, jkt)

	before, err := Skip(SkipRequest{
		Package: pkg, Now: cutOff.Add(-time.Minute), CutOffAt: cutOff,
		ReferenceID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Qty != +1 || before[0].EntryType != EntryRefund {
		t.Errorf("a skip before cut-off must return one credit, got %+v", before)
	}

	after, err := Skip(SkipRequest{
		Package: pkg, Now: cutOff, CutOffAt: cutOff, ReferenceID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Errorf("a skip at or after cut-off returns nothing, got %+v", after)
	}
}

func TestExpire_ForfeitsRemainder(t *testing.T) {
	pkg := activePkg(20)
	entries := []Entry{purchase(pkg, 20), {EntryType: EntryRedeem, Qty: -7}}
	out := Expire(pkg, entries, day(2026, 12, 1))
	if len(out) != 1 {
		t.Fatalf("got %d entries, want 1", len(out))
	}
	if out[0].Qty != -13 {
		t.Errorf("expire qty = %d, want -13", out[0].Qty)
	}
	if got := Balance(append(entries, out...)); got != 0 {
		t.Errorf("balance after expiry = %d, want 0", got)
	}
	// Nothing to forfeit posts nothing.
	if got := Expire(pkg, []Entry{purchase(pkg, 5), {EntryType: EntryRedeem, Qty: -5}}, day(2026, 12, 1)); got != nil {
		t.Errorf("a zero balance must post no EXPIRE entry, got %+v", got)
	}
}

func TestExtendExpiry_ReversesWithCompensatingEntry_NeverDeletes(t *testing.T) {
	pkg := activePkg(20)
	entries := []Entry{purchase(pkg, 20), {EntryType: EntryRedeem, Qty: -7}}
	entries = append(entries, Expire(pkg, entries, day(2026, 12, 1))...)
	if got := Balance(entries); got != 0 {
		t.Fatalf("precondition: balance = %d, want 0", got)
	}

	out, err := ExtendExpiry(pkg, entries, day(2026, 12, 2), uuid.New(), "pelanggan minta perpanjangan")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].EntryType != EntryAdjustment || out[0].Qty != 13 {
		t.Fatalf("want one compensating ADJUSTMENT of +13, got %+v", out)
	}
	final := append(entries, out...)
	if got := Balance(final); got != 13 {
		t.Errorf("balance after extension = %d, want 13", got)
	}
	// The EXPIRE row is still there — append-only, nothing deleted.
	expires := 0
	for _, e := range final {
		if e.EntryType == EntryExpire {
			expires++
		}
	}
	if expires != 1 {
		t.Errorf("the EXPIRE entry must survive; found %d", expires)
	}
}

func TestExtendExpiry_RequiresReason(t *testing.T) {
	pkg := activePkg(20)
	if _, err := ExtendExpiry(pkg, nil, day(2026, 12, 2), uuid.New(), ""); err != ErrReasonRequired {
		t.Errorf("got %v, want ErrReasonRequired", err)
	}
}

func TestAdjust_RequiresReason(t *testing.T) {
	pkg := activePkg(20)
	if _, err := Adjust(pkg, 3, day(2026, 9, 1), uuid.New(), ""); err != ErrReasonRequired {
		t.Errorf("got %v, want ErrReasonRequired", err)
	}
	if _, err := Adjust(pkg, 0, day(2026, 9, 1), uuid.New(), "alasan"); err == nil {
		t.Error("an adjustment of zero should be rejected")
	}
	out, err := Adjust(pkg, -2, day(2026, 9, 1), uuid.New(), "koreksi input")
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Qty != -2 || out[0].Note != "koreksi input" {
		t.Errorf("got %+v", out[0])
	}
}

func TestNextStatus(t *testing.T) {
	pkg := activePkg(20)
	full := []Entry{purchase(pkg, 20)}
	spent := []Entry{purchase(pkg, 20), {Qty: -20}}

	if got := NextStatus(pkg, full, day(2026, 9, 1)); got != StatusActive {
		t.Errorf("got %s, want ACTIVE", got)
	}
	if got := NextStatus(pkg, spent, day(2026, 9, 1)); got != StatusExhausted {
		t.Errorf("got %s, want EXHAUSTED", got)
	}
	if got := NextStatus(pkg, full, day(2026, 12, 1)); got != StatusExpired {
		t.Errorf("got %s, want EXPIRED", got)
	}
	// Expiry beats exhaustion when both apply, and PENDING/CANCELLED are inert.
	pending := pkg
	pending.Status = StatusPending
	if got := NextStatus(pending, full, day(2026, 12, 1)); got != StatusPending {
		t.Errorf("PENDING must not be derived away, got %s", got)
	}
}
