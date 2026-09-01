// Package money holds every arithmetic operation this system performs on
// rupiah. Amounts are whole rupiah in int64 — CLAUDE.md §4 prohibits floating
// point on any code path touching money, and sen is obsolete in retail so the
// rupiah is the minor unit (D7).
//
// Nothing in this package does I/O, and nothing outside it may compute a tax
// split or apply a basis-point rate.
package money

import "errors"

// IDR is a whole-rupiah amount. It is a distinct type so that a bare int64
// cannot be passed where an amount is expected without saying so.
type IDR int64

// ErrNegative is returned when an operation would produce a negative amount in
// a context where that cannot be meaningful.
var ErrNegative = errors.New("money: negative amount")

// ErrOverflow is returned when an operation would exceed int64.
var ErrOverflow = errors.New("money: int64 overflow")

const maxIDR = IDR(1<<62 - 1)

// ApplyBPS applies a basis-point rate, rounded half-up, per CLAUDE.md §4:
//
//	floor((amount * bps + 5000) / 10000)
//
// It is used for discounts and any other rate applied *to* an amount. It is
// NOT how tax is split out of an inclusive price — see SplitInclusive.
func ApplyBPS(amount IDR, bps int) (IDR, error) {
	if amount < 0 {
		return 0, ErrNegative
	}
	if bps < 0 {
		return 0, ErrNegative
	}
	if bps != 0 && int64(amount) > (int64(maxIDR)-5000)/int64(bps) {
		return 0, ErrOverflow
	}
	return IDR((int64(amount)*int64(bps) + 5000) / 10000), nil
}

// TaxSplit is the decomposition of a tax-inclusive amount.
//
// Base+Tax always equals the original amount exactly, because Tax is taken as
// the residue rather than computed independently. 01-domain-model.md §3.11.
type TaxSplit struct {
	Inclusive IDR // what the customer pays
	Base      IDR // the taxable base
	Tax       IDR // the residue: Inclusive - Base
	RateBPS   int // the rate in force, snapshotted by the caller
}

// SplitInclusive back-calculates the tax base out of a tax-inclusive amount,
// integer-only and half-up, with D = 10000 + rateBPS:
//
//	base = (inclusive * 10000 + D/2) / D
//	tax  = inclusive - base
//
// Worked, from 01-domain-model.md §3.11: Rp 500.000 at 1100 bps gives
// base 450.450 and tax 49.550, which sum to 500.000 exactly.
//
// Callers MUST pass a line total, never a unit price — computing the split per
// unit and multiplying multiplies the rounding error by the quantity.
func SplitInclusive(inclusive IDR, rateBPS int) (TaxSplit, error) {
	if inclusive < 0 {
		return TaxSplit{}, ErrNegative
	}
	if rateBPS < 0 {
		return TaxSplit{}, ErrNegative
	}
	d := int64(10000 + rateBPS)
	if int64(inclusive) > (int64(maxIDR)-d/2)/10000 {
		return TaxSplit{}, ErrOverflow
	}
	base := IDR((int64(inclusive)*10000 + d/2) / d)
	return TaxSplit{
		Inclusive: inclusive,
		Base:      base,
		Tax:       inclusive - base,
		RateBPS:   rateBPS,
	}, nil
}

// Mul multiplies an amount by a non-negative quantity, refusing to overflow.
func Mul(amount IDR, qty int) (IDR, error) {
	if amount < 0 || qty < 0 {
		return 0, ErrNegative
	}
	if qty != 0 && int64(amount) > int64(maxIDR)/int64(qty) {
		return 0, ErrOverflow
	}
	return amount * IDR(qty), nil
}

// Sum adds amounts, refusing to overflow.
func Sum(amounts ...IDR) (IDR, error) {
	var total IDR
	for _, a := range amounts {
		if a < 0 {
			return 0, ErrNegative
		}
		if total > maxIDR-a {
			return 0, ErrOverflow
		}
		total += a
	}
	return total, nil
}

// SumSplits totals a set of line-level splits. The order's tax is the SUM of
// the line taxes and is never re-derived from the order total — re-deriving
// reintroduces a rounding difference between an invoice and its own lines
// (01-domain-model.md §3.11).
func SumSplits(splits []TaxSplit) (TaxSplit, error) {
	var out TaxSplit
	if len(splits) == 0 {
		return out, nil
	}
	out.RateBPS = splits[0].RateBPS
	for _, s := range splits {
		if s.RateBPS != out.RateBPS {
			return TaxSplit{}, errors.New("money: mixed tax rates in one order")
		}
		var err error
		if out.Inclusive, err = Sum(out.Inclusive, s.Inclusive); err != nil {
			return TaxSplit{}, err
		}
		if out.Base, err = Sum(out.Base, s.Base); err != nil {
			return TaxSplit{}, err
		}
		if out.Tax, err = Sum(out.Tax, s.Tax); err != nil {
			return TaxSplit{}, err
		}
	}
	return out, nil
}
