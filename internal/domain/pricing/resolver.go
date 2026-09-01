// Package pricing implements the price resolver from 01-domain-model.md §3.5
// and §5.1. It is pure: given the candidate rows it decides which one applies
// and why. It never reads a database and never guesses.
package pricing

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// ErrPriceNotConfigured is returned when no row matches in either the
// customer-type scope or DEFAULT. Checkout blocks on this rather than guessing
// a price — 01-domain-model.md §3.5 step 3, and the back office surfaces it as
// "Harga belum diisi".
var ErrPriceNotConfigured = errors.New("PRICE_NOT_CONFIGURED")

// ErrNoTier is returned when the quantity falls outside every configured tier.
// Tier coverage is validated on save, so reaching this at checkout means the
// tier table has a gap that validation missed.
var ErrNoTier = errors.New("PRICE_TIER_NOT_FOUND")

// Table names a price table. The four tables of 01-domain-model.md §3.5.
type Table string

const (
	TableMealNormal    Table = "meal_normal"
	TableMealPromo     Table = "meal_promo"
	TablePackageNormal Table = "pkg_normal"
	TablePackagePromo  Table = "pkg_promo"
)

// ScopeDefault is the scope_key of a row that applies to every customer type.
const ScopeDefault = "DEFAULT"

// ScopeForCustomerType builds the scope_key for a customer type. It mirrors the
// generated column in the database so the two cannot drift.
func ScopeForCustomerType(id uuid.UUID) string { return "CT:" + id.String() }

// Tier is a quantity band, counted in meals and never in foods (D-32).
// MaxQty nil means unbounded.
type Tier struct {
	ID     uuid.UUID
	MinQty int
	MaxQty *int
}

// Contains reports whether qty falls in this tier.
func (t Tier) Contains(qty int) bool {
	if qty < t.MinQty {
		return false
	}
	if t.MaxQty != nil && qty > *t.MaxQty {
		return false
	}
	return true
}

// Row is one row from any of the four price tables. Validity is a half-open
// date range [ValidFrom, ValidTo); ValidTo nil means open-ended.
type Row struct {
	ID           uuid.UUID
	Table        Table
	ScopeKey     string
	DietTypeID   uuid.UUID
	TierID       uuid.UUID
	UnitPriceIDR money.IDR
	ValidFrom    time.Time
	ValidTo      *time.Time
	PromoLabel   string
	IsActive     bool
}

// CoversDate reports whether the row's validity contains d, with [) bounds.
// Both are compared as dates in the operating timezone; the caller normalises.
func (r Row) CoversDate(d time.Time) bool {
	if d.Before(r.ValidFrom) {
		return false
	}
	if r.ValidTo != nil && !d.Before(*r.ValidTo) {
		return false
	}
	return true
}

// Request is what the resolver is asked to price.
type Request struct {
	CustomerTypeID uuid.UUID
	DietTypeID     uuid.UUID
	Qty            int // meals, not foods
	OrderDate      time.Time
}

// Trace records why a price was chosen. It is stored on the order so that
// "why did this customer pay that?" is answerable from the record without
// re-running the resolver (01-domain-model.md §3.5).
type Trace struct {
	ScopeMatched string    `json:"scope_matched"`
	Table        Table     `json:"table"`
	TierID       uuid.UUID `json:"tier_id"`
	TierMinQty   int       `json:"tier_min_qty"`
	TierMaxQty   *int      `json:"tier_max_qty"`
	RowID        uuid.UUID `json:"row_id"`
	PromoApplied bool      `json:"promo_applied"`
	PromoLabel   string    `json:"promo_label,omitempty"`
	Qty          int       `json:"qty"`
	OrderDate    string    `json:"order_date"`
}

// Resolved is the outcome: a unit price, tax-inclusive, plus the trace.
type Resolved struct {
	UnitPriceIDR money.IDR
	Trace        Trace
}

// Resolve implements the documented resolution order:
//
//  1. Resolve scope: rows whose scope_key is CT:<customer type>. If that scope
//     has no candidate, fall back to DEFAULT.
//  2. Within the resolved scope, promo beats normal (D-9).
//  3. Nothing in either scope: block with ErrPriceNotConfigured.
//
// The D-9 subtlety that the test matrix calls out: a customer-type NORMAL row
// beats a DEFAULT PROMO row, because scope is resolved *before* promo. Step 2
// only ever compares rows within one scope.
func Resolve(req Request, tiers []Tier, rows []Row) (Resolved, error) {
	if req.Qty <= 0 {
		return Resolved{}, fmt.Errorf("pricing: qty must be positive, got %d", req.Qty)
	}

	tier, ok := findTier(tiers, req.Qty)
	if !ok {
		return Resolved{}, ErrNoTier
	}

	scopes := []string{ScopeForCustomerType(req.CustomerTypeID), ScopeDefault}
	for _, scope := range scopes {
		normal, promo := candidates(rows, scope, req, tier.ID)
		// Step 2: promo beats normal, but only inside this scope.
		if promo != nil {
			return resolved(*promo, tier, req, true), nil
		}
		if normal != nil {
			return resolved(*normal, tier, req, false), nil
		}
		// Step 1 fallback: this scope had nothing, try the next.
	}
	return Resolved{}, ErrPriceNotConfigured
}

func resolved(r Row, tier Tier, req Request, isPromo bool) Resolved {
	return Resolved{
		UnitPriceIDR: r.UnitPriceIDR,
		Trace: Trace{
			ScopeMatched: r.ScopeKey,
			Table:        r.Table,
			TierID:       tier.ID,
			TierMinQty:   tier.MinQty,
			TierMaxQty:   tier.MaxQty,
			RowID:        r.ID,
			PromoApplied: isPromo,
			PromoLabel:   r.PromoLabel,
			Qty:          req.Qty,
			OrderDate:    req.OrderDate.Format("2006-01-02"),
		},
	}
}

// candidates returns the single applicable normal row and promo row for a
// scope. The database's EXCLUDE constraint guarantees at most one of each is
// valid on any given date, so finding a second is a data-integrity failure
// rather than something to arbitrate here.
func candidates(rows []Row, scope string, req Request, tierID uuid.UUID) (normal, promo *Row) {
	for i := range rows {
		r := rows[i]
		if !r.IsActive ||
			r.ScopeKey != scope ||
			r.DietTypeID != req.DietTypeID ||
			r.TierID != tierID ||
			!r.CoversDate(req.OrderDate) {
			continue
		}
		switch r.Table {
		case TableMealNormal, TablePackageNormal:
			normal = &rows[i]
		case TableMealPromo, TablePackagePromo:
			promo = &rows[i]
		}
	}
	return normal, promo
}

func findTier(tiers []Tier, qty int) (Tier, bool) {
	for _, t := range tiers {
		if t.Contains(qty) {
			return t, true
		}
	}
	return Tier{}, false
}

// ValidateTiers checks that the tier set covers 1..maxQty with no gap and no
// overlap. 01-domain-model.md §3.5 requires this on save so that a gap is
// refused at configuration time rather than discovered at checkout.
func ValidateTiers(tiers []Tier, maxQty int) error {
	if len(tiers) == 0 {
		return errors.New("pricing: no tiers configured")
	}
	// Walk the range once. A tier set is valid iff every quantity from 1 to
	// maxQty is matched by exactly one tier. Checking every value is O(maxQty)
	// with maxQty=999, which is cheap and cannot be fooled by clever bounds.
	for qty := 1; qty <= maxQty; qty++ {
		matches := 0
		for _, t := range tiers {
			if t.Contains(qty) {
				matches++
			}
		}
		if matches == 0 {
			return fmt.Errorf("pricing: no tier covers qty %d", qty)
		}
		if matches > 1 {
			return fmt.Errorf("pricing: %d tiers overlap at qty %d", matches, qty)
		}
	}
	return nil
}
