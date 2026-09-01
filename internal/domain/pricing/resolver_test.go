package pricing

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// The test matrix is 01-domain-model.md §5.1, item by item:
//
//	scope hit · scope miss falling back to DEFAULT · both missing (blocks) ·
//	promo overriding normal in the same scope · customer-type normal beating a
//	DEFAULT promo (D-9) · tier boundary at min · at max · at max+1 · open-ended
//	max_qty · validity boundary on valid_from · on valid_to (exclusive) ·
//	overlapping rows rejected by the constraint (that one is an integration
//	test, the constraint lives in the database) · qty above the configured max.

var (
	ctRetail    = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ctCorporate = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	dietBal     = uuid.MustParse("33333333-3333-4333-8333-333333333333")
	dietWL      = uuid.MustParse("44444444-4444-4444-8444-444444444444")

	tier1 = Tier{ID: uuid.MustParse("aaaaaaaa-0001-4000-8000-000000000000"), MinQty: 1, MaxQty: ptr(3)}
	tier2 = Tier{ID: uuid.MustParse("aaaaaaaa-0002-4000-8000-000000000000"), MinQty: 4, MaxQty: ptr(9)}
	tier3 = Tier{ID: uuid.MustParse("aaaaaaaa-0003-4000-8000-000000000000"), MinQty: 10, MaxQty: nil}

	allTiers = []Tier{tier1, tier2, tier3}

	sep1 = date(2026, 9, 1)
)

func ptr(i int) *int { return &i }

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func row(tbl Table, scope string, tier Tier, price money.IDR, from time.Time, to *time.Time) Row {
	return Row{
		ID:           uuid.New(),
		Table:        tbl,
		ScopeKey:     scope,
		DietTypeID:   dietBal,
		TierID:       tier.ID,
		UnitPriceIDR: price,
		ValidFrom:    from,
		ValidTo:      to,
		IsActive:     true,
	}
}

func req(ct uuid.UUID, qty int) Request {
	return Request{CustomerTypeID: ct, DietTypeID: dietBal, Qty: qty, OrderDate: sep1}
}

func TestResolve_ScopeHit(t *testing.T) {
	rows := []Row{
		row(TableMealNormal, ScopeDefault, tier2, 75000, sep1, nil),
		row(TableMealNormal, ScopeForCustomerType(ctCorporate), tier2, 70000, sep1, nil),
	}
	got, err := Resolve(req(ctCorporate, 6), allTiers, rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitPriceIDR != 70000 {
		t.Errorf("price = %d, want 70000 (the corporate row)", got.UnitPriceIDR)
	}
	if got.Trace.ScopeMatched != ScopeForCustomerType(ctCorporate) {
		t.Errorf("scope = %q, want the corporate scope", got.Trace.ScopeMatched)
	}
}

func TestResolve_ScopeMissFallsBackToDefault(t *testing.T) {
	rows := []Row{row(TableMealNormal, ScopeDefault, tier2, 75000, sep1, nil)}
	got, err := Resolve(req(ctCorporate, 6), allTiers, rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitPriceIDR != 75000 {
		t.Errorf("price = %d, want 75000", got.UnitPriceIDR)
	}
	if got.Trace.ScopeMatched != ScopeDefault {
		t.Errorf("scope = %q, want DEFAULT", got.Trace.ScopeMatched)
	}
}

func TestResolve_BothMissingBlocks(t *testing.T) {
	// A row exists, but for a different diet type. Nothing matches, so the
	// resolver must block rather than fall through to any other price.
	rows := []Row{{
		ID: uuid.New(), Table: TableMealNormal, ScopeKey: ScopeDefault,
		DietTypeID: dietWL, TierID: tier2.ID, UnitPriceIDR: 75000,
		ValidFrom: sep1, IsActive: true,
	}}
	_, err := Resolve(req(ctRetail, 6), allTiers, rows)
	if err != ErrPriceNotConfigured {
		t.Errorf("got %v, want ErrPriceNotConfigured", err)
	}
}

func TestResolve_PromoBeatsNormalInSameScope(t *testing.T) {
	rows := []Row{
		row(TableMealNormal, ScopeDefault, tier2, 75000, sep1, nil),
		row(TableMealPromo, ScopeDefault, tier2, 69000, sep1, nil),
	}
	got, err := Resolve(req(ctRetail, 6), allTiers, rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitPriceIDR != 69000 {
		t.Errorf("price = %d, want the promo 69000", got.UnitPriceIDR)
	}
	if !got.Trace.PromoApplied {
		t.Error("trace should record that a promo applied")
	}
}

func TestResolve_CustomerTypeNormalBeatsDefaultPromo_D9(t *testing.T) {
	// The D-9 case, and the one most likely to be got wrong: scope is resolved
	// FIRST. A cheaper DEFAULT promo must not win over the corporate normal row.
	rows := []Row{
		row(TableMealPromo, ScopeDefault, tier2, 60000, sep1, nil),
		row(TableMealNormal, ScopeForCustomerType(ctCorporate), tier2, 70000, sep1, nil),
	}
	got, err := Resolve(req(ctCorporate, 6), allTiers, rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitPriceIDR != 70000 {
		t.Fatalf("price = %d, want 70000: the corporate normal row must beat the DEFAULT promo", got.UnitPriceIDR)
	}
	if got.Trace.PromoApplied {
		t.Error("no promo applies in the corporate scope")
	}
}

func TestResolve_TierBoundaries(t *testing.T) {
	rows := []Row{
		row(TableMealNormal, ScopeDefault, tier1, 78000, sep1, nil),
		row(TableMealNormal, ScopeDefault, tier2, 75000, sep1, nil),
		row(TableMealNormal, ScopeDefault, tier3, 71000, sep1, nil),
	}
	cases := []struct {
		qty  int
		want money.IDR
		note string
	}{
		{1, 78000, "tier 1 min"},
		{3, 78000, "tier 1 max"},
		{4, 75000, "tier 1 max+1 crosses into tier 2"},
		{9, 75000, "tier 2 max"},
		{10, 71000, "tier 2 max+1 crosses into tier 3"},
		{999, 71000, "open-ended max_qty"},
	}
	for _, c := range cases {
		got, err := Resolve(req(ctRetail, c.qty), allTiers, rows)
		if err != nil {
			t.Fatalf("qty=%d (%s): %v", c.qty, c.note, err)
		}
		if got.UnitPriceIDR != c.want {
			t.Errorf("qty=%d (%s): price = %d, want %d", c.qty, c.note, got.UnitPriceIDR, c.want)
		}
	}
}

func TestResolve_ValidityBoundaries(t *testing.T) {
	from := date(2026, 9, 1)
	to := date(2026, 10, 1)
	rows := []Row{row(TableMealNormal, ScopeDefault, tier1, 78000, from, &to)}

	// On valid_from: inclusive.
	if _, err := Resolve(Request{ctRetail, dietBal, 1, from}, allTiers, rows); err != nil {
		t.Errorf("valid_from should be inclusive, got %v", err)
	}
	// Day before: outside.
	if _, err := Resolve(Request{ctRetail, dietBal, 1, from.AddDate(0, 0, -1)}, allTiers, rows); err != ErrPriceNotConfigured {
		t.Errorf("the day before valid_from should not match, got %v", err)
	}
	// Day before valid_to: inside.
	if _, err := Resolve(Request{ctRetail, dietBal, 1, to.AddDate(0, 0, -1)}, allTiers, rows); err != nil {
		t.Errorf("the day before valid_to should match, got %v", err)
	}
	// On valid_to: EXCLUSIVE, the [) bound.
	if _, err := Resolve(Request{ctRetail, dietBal, 1, to}, allTiers, rows); err != ErrPriceNotConfigured {
		t.Errorf("valid_to must be exclusive, got %v", err)
	}
}

func TestResolve_QtyAboveConfiguredMax(t *testing.T) {
	// Tiers that stop at 99. A quantity of 100 has no tier at all.
	bounded := []Tier{{ID: tier1.ID, MinQty: 1, MaxQty: ptr(99)}}
	rows := []Row{row(TableMealNormal, ScopeDefault, tier1, 78000, sep1, nil)}
	if _, err := Resolve(req(ctRetail, 100), bounded, rows); err != ErrNoTier {
		t.Errorf("got %v, want ErrNoTier", err)
	}
}

func TestResolve_RejectsNonPositiveQty(t *testing.T) {
	rows := []Row{row(TableMealNormal, ScopeDefault, tier1, 78000, sep1, nil)}
	for _, qty := range []int{0, -1} {
		if _, err := Resolve(req(ctRetail, qty), allTiers, rows); err == nil {
			t.Errorf("qty=%d should be rejected", qty)
		}
	}
}

func TestResolve_InactiveRowIgnored(t *testing.T) {
	r := row(TableMealNormal, ScopeDefault, tier1, 78000, sep1, nil)
	r.IsActive = false
	if _, err := Resolve(req(ctRetail, 1), allTiers, []Row{r}); err != ErrPriceNotConfigured {
		t.Errorf("an archived row must not price an order, got %v", err)
	}
}

func TestResolve_TraceIsComplete(t *testing.T) {
	r := row(TableMealPromo, ScopeDefault, tier2, 69000, sep1, nil)
	r.PromoLabel = "Promo September"
	got, err := Resolve(req(ctRetail, 6), allTiers, []Row{r})
	if err != nil {
		t.Fatal(err)
	}
	tr := got.Trace
	if tr.RowID != r.ID {
		t.Error("trace must name the row that priced the order")
	}
	if tr.Table != TableMealPromo {
		t.Errorf("trace table = %q", tr.Table)
	}
	if tr.TierID != tier2.ID || tr.TierMinQty != 4 || *tr.TierMaxQty != 9 {
		t.Error("trace must carry the tier that matched")
	}
	if tr.PromoLabel != "Promo September" {
		t.Errorf("trace promo label = %q", tr.PromoLabel)
	}
	if tr.OrderDate != "2026-09-01" {
		t.Errorf("trace order date = %q", tr.OrderDate)
	}
}

func TestValidateTiers(t *testing.T) {
	if err := ValidateTiers(allTiers, 999); err != nil {
		t.Errorf("the documented tier set should validate: %v", err)
	}
	gap := []Tier{
		{ID: uuid.New(), MinQty: 1, MaxQty: ptr(3)},
		{ID: uuid.New(), MinQty: 5, MaxQty: nil}, // 4 is uncovered
	}
	if err := ValidateTiers(gap, 999); err == nil {
		t.Error("a gap at qty 4 must be rejected")
	}
	overlap := []Tier{
		{ID: uuid.New(), MinQty: 1, MaxQty: ptr(5)},
		{ID: uuid.New(), MinQty: 4, MaxQty: nil}, // 4 and 5 are covered twice
	}
	if err := ValidateTiers(overlap, 999); err == nil {
		t.Error("an overlap at qty 4-5 must be rejected")
	}
	if err := ValidateTiers(nil, 999); err == nil {
		t.Error("an empty tier set must be rejected")
	}
}
