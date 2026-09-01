// Package credit implements the credit ledger rules from 01-domain-model.md
// §3.6 and §5.2.
//
// Two invariants govern everything here:
//
//   - The balance is never stored. It is SUM(qty) over an append-only ledger.
//     There is no UPDATE and no DELETE path.
//   - One credit buys one meal (D-32), whatever that meal contains. Nothing in
//     the ledger ever counts foods.
//
// Credits are never money (D-31). A REFUND entry returns a credit, not rupiah.
package credit

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// EntryType is the kind of ledger movement.
type EntryType string

const (
	EntryPurchase   EntryType = "PURCHASE"   // +N, package bought and paid
	EntryRedeem     EntryType = "REDEEM"     // -1 per meal
	EntryRefund     EntryType = "REFUND"     // +1, skip before cut-off or kitchen failure
	EntryExpire     EntryType = "EXPIRE"     // -remainder at expiry, forfeited
	EntryAdjustment EntryType = "ADJUSTMENT" // signed, staff, reason required
)

// Entry is one append-only ledger row.
type Entry struct {
	ID                uuid.UUID
	CustomerID        uuid.UUID
	CustomerPackageID uuid.UUID
	EntryType         EntryType
	Qty               int // signed
	ReferenceType     string
	ReferenceID       *uuid.UUID
	OccurredAt        time.Time
	CreatedBy         *uuid.UUID
	Note              string
}

var (
	// ErrInsufficientCredit is returned when a redemption would take the
	// balance below zero. A CHECK constraint cannot express SUM(...) >= 0
	// across rows, so this rule plus the row lock is the enforcement point.
	ErrInsufficientCredit = errors.New("INSUFFICIENT_CREDIT")

	// ErrPackageNotActive is returned when the package cannot be spent from.
	ErrPackageNotActive = errors.New("PACKAGE_NOT_ACTIVE")

	// ErrPackageExpired is returned when a delivery is scheduled past
	// expires_at (D-27).
	ErrPackageExpired = errors.New("PACKAGE_EXPIRED")

	// ErrReasonRequired is returned for an ADJUSTMENT with no note.
	ErrReasonRequired = errors.New("ADJUSTMENT_REASON_REQUIRED")
)

// Status is the customer_package lifecycle state (01-domain-model.md §4.3).
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusActive    Status = "ACTIVE"
	StatusExhausted Status = "EXHAUSTED"
	StatusExpired   Status = "EXPIRED"
	StatusCancelled Status = "CANCELLED"
)

// Package is the purchased instance, with the snapshot fields that must not
// move when the catalogue changes underneath it.
type Package struct {
	ID          uuid.UUID
	CustomerID  uuid.UUID
	MealCredits int       // snapshot at purchase
	ExpiresAt   time.Time // a DATE in Asia/Jakarta
	Status      Status
}

// Balance is SUM(qty). It is the only way to know a balance; nothing caches it.
func Balance(entries []Entry) int {
	total := 0
	for _, e := range entries {
		total += e.Qty
	}
	return total
}

// RedeemRequest asks to spend credits for meals.
type RedeemRequest struct {
	Package      Package
	Entries      []Entry // every entry for this package, read inside the lock
	Meals        int     // one credit per meal (D-32)
	ServiceDate  time.Time
	OccurredAt   time.Time
	ReferenceID  uuid.UUID
	ReferenceTyp string
	ActorID      *uuid.UUID
}

// Redeem decides whether a redemption is allowed and returns the entries to
// append. It is pure — the caller performs it inside a transaction that holds
// SELECT ... FOR UPDATE on the customer_package row and has re-read Entries
// inside that lock. That lock is what makes the concurrent case safe; this
// function only encodes the rules.
func Redeem(req RedeemRequest) ([]Entry, error) {
	if req.Meals <= 0 {
		return nil, errors.New("credit: meals must be positive")
	}
	if req.Package.Status != StatusActive {
		return nil, ErrPackageNotActive
	}
	// D-27: a delivery may not be scheduled after the package expires.
	if req.ServiceDate.After(req.Package.ExpiresAt) {
		return nil, ErrPackageExpired
	}
	if Balance(req.Entries) < req.Meals {
		return nil, ErrInsufficientCredit
	}

	out := make([]Entry, 0, req.Meals)
	for i := 0; i < req.Meals; i++ {
		ref := req.ReferenceID
		out = append(out, Entry{
			ID:                uuid.New(),
			CustomerID:        req.Package.CustomerID,
			CustomerPackageID: req.Package.ID,
			EntryType:         EntryRedeem,
			Qty:               -1, // one credit per meal, never per food
			ReferenceType:     req.ReferenceTyp,
			ReferenceID:       &ref,
			OccurredAt:        req.OccurredAt,
			CreatedBy:         req.ActorID,
		})
	}
	return out, nil
}

// SkipRequest asks to return a credit for a skipped delivery.
type SkipRequest struct {
	Package     Package
	Now         time.Time
	CutOffAt    time.Time // the cut-off for the delivery's service date
	ReferenceID uuid.UUID
	ActorID     *uuid.UUID
}

// Skip returns a credit if the skip happens before cut-off, and nothing if it
// happens after. §5.2: "skip before cut-off returns a credit · skip after
// cut-off does not". Returning no entry and no error is the documented
// behaviour after cut-off: the skip is allowed, the credit is simply spent.
func Skip(req SkipRequest) ([]Entry, error) {
	if !req.Now.Before(req.CutOffAt) {
		return nil, nil
	}
	ref := req.ReferenceID
	return []Entry{{
		ID:                uuid.New(),
		CustomerID:        req.Package.CustomerID,
		CustomerPackageID: req.Package.ID,
		EntryType:         EntryRefund,
		Qty:               +1,
		ReferenceType:     "delivery",
		ReferenceID:       &ref,
		OccurredAt:        req.Now,
		CreatedBy:         req.ActorID,
	}}, nil
}

// Expire posts the negative remainder at expiry. The remainder is forfeited and
// is never refunded in money (D-31). If the balance is already zero it posts
// nothing.
func Expire(pkg Package, entries []Entry, at time.Time) []Entry {
	bal := Balance(entries)
	if bal <= 0 {
		return nil
	}
	return []Entry{{
		ID:                uuid.New(),
		CustomerID:        pkg.CustomerID,
		CustomerPackageID: pkg.ID,
		EntryType:         EntryExpire,
		Qty:               -bal,
		ReferenceType:     "customer_package",
		OccurredAt:        at,
		Note:              "kredit hangus pada tanggal berakhir",
	}}
}

// ExtendExpiry reactivates a package whose expiry has passed. §4.3 requires the
// EXPIRE entry to be reversed with a compensating ADJUSTMENT rather than
// deleted, because the ledger is append-only and the balance history has to
// reconcile.
func ExtendExpiry(pkg Package, entries []Entry, at time.Time, actor uuid.UUID, reason string) ([]Entry, error) {
	if reason == "" {
		return nil, ErrReasonRequired
	}
	var reversal int
	for _, e := range entries {
		if e.EntryType == EntryExpire {
			reversal -= e.Qty // EXPIRE qty is negative, so this adds back
		}
	}
	if reversal == 0 {
		return nil, nil
	}
	return []Entry{{
		ID:                uuid.New(),
		CustomerID:        pkg.CustomerID,
		CustomerPackageID: pkg.ID,
		EntryType:         EntryAdjustment,
		Qty:               reversal,
		ReferenceType:     "customer_package",
		OccurredAt:        at,
		CreatedBy:         &actor,
		Note:              reason,
	}}, nil
}

// Adjust posts a signed staff adjustment. A reason is mandatory.
func Adjust(pkg Package, qty int, at time.Time, actor uuid.UUID, reason string) ([]Entry, error) {
	if reason == "" {
		return nil, ErrReasonRequired
	}
	if qty == 0 {
		return nil, errors.New("credit: adjustment of zero")
	}
	return []Entry{{
		ID:                uuid.New(),
		CustomerID:        pkg.CustomerID,
		CustomerPackageID: pkg.ID,
		EntryType:         EntryAdjustment,
		Qty:               qty,
		ReferenceType:     "customer_package",
		OccurredAt:        at,
		CreatedBy:         &actor,
		Note:              reason,
	}}, nil
}

// NextStatus derives the package status from its balance and the clock. The
// caller persists it; the balance itself stays underived.
func NextStatus(pkg Package, entries []Entry, today time.Time) Status {
	switch pkg.Status {
	case StatusPending, StatusCancelled:
		return pkg.Status
	}
	if today.After(pkg.ExpiresAt) {
		return StatusExpired
	}
	if Balance(entries) <= 0 {
		return StatusExhausted
	}
	return StatusActive
}
