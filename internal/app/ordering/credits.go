// Credit-paid meal ordering (BR-9.x, D86).
//
// Meals are not sold for money. A customer buys credits as a package, and
// every meal on an order spends exactly one credit — one for lunch, one for
// dinner, whatever the meal contains (D-32). The money transaction happens
// once, when the package is bought; placing a meal order moves no rupiah at
// all, which is why these orders are written with every amount at zero and go
// straight to PAID rather than AWAITING_PAYMENT.
//
// Credits are spent SOONEST-EXPIRING FIRST. Any other order would quietly
// forfeit credits that still had value, and the customer would have no way to
// see why.
package ordering

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/evermore/internal/domain/credit"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
)

// ErrNoCredits means the basket needs more credits than the customer holds, or
// holds that are still valid on the service date.
var ErrNoCredits = errors.New("INSUFFICIENT_CREDIT")

// CreditQuote is what a basket costs: meals, not rupiah.
type CreditQuote struct {
	Lines     []CreditLine
	Meals     int // credits this basket will spend
	Balance   int // credits the customer holds right now
	Shortfall int // Meals - Balance when short, else 0
}

// CreditLine is one meal on the basket.
type CreditLine struct {
	ScheduledMealID uuid.UUID
	MealName        string
	ServiceDate     time.Time
	SlotTime        string
	SlotPeriod      string
	DietName        string
	Qty             int // credits this line spends
	// What the kitchen routing needs, beyond the fields above: reserveAndRoute
	// reads slotID, dietTypeID, ServiceDate, Qty, ScheduledMealID and
	// MealSnapshot. Leaving slotID out made every address look
	// un-serviceable; leaving dietTypeID out broke delivery's foreign key
	// (D86). They are carried here so the credit path reuses routing whole.
	slotID       uuid.UUID
	dietTypeID   uuid.UUID
	MealSnapshot map[string]any
}

// creditPackage is one spendable package, read inside the lock.
type creditPackage struct {
	pkg     credit.Package
	entries []credit.Entry
}

// CreditQuoteFor prices a basket in credits and reports the balance beside it,
// so the page can say "3 credits, you have 12" without a second round trip.
func (s *Service) CreditQuoteFor(ctx context.Context, customerID uuid.UUID, items []CartItem) (*CreditQuote, error) {
	q := &CreditQuote{}
	for _, it := range items {
		if it.Qty <= 0 {
			continue
		}
		var l CreditLine
		err := s.db.QueryRowContext(ctx, `
			SELECT sm.id, sm.name, sm.service_date, to_char(sl.slot_time,'HH24.MI'),
			       sl.meal_period, dt.name, sl.id, dt.id
			  FROM scheduled_meal sm
			  JOIN delivery_time_slot sl ON sl.id = sm.slot_id
			  JOIN diet_type dt          ON dt.id = sm.diet_type_id
			 WHERE sm.id = $1 AND sm.status = 'PUBLISHED'`, it.ScheduledMealID).
			Scan(&l.ScheduledMealID, &l.MealName, &l.ServiceDate, &l.SlotTime,
				&l.SlotPeriod, &l.DietName, &l.slotID, &l.dietTypeID)
		if err == sql.ErrNoRows {
			continue // an unpublished meal silently leaves the basket
		}
		if err != nil {
			return nil, err
		}
		l.Qty = it.Qty
		l.MealSnapshot = map[string]any{
			"name": l.MealName, "diet": l.DietName, "slot": l.SlotTime,
			"period": l.SlotPeriod, "service_date": l.ServiceDate.Format("2006-01-02"),
		}
		q.Lines = append(q.Lines, l)
		q.Meals += it.Qty
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(sum(cl.qty), 0)::int
		  FROM credit_ledger cl
		 WHERE cl.customer_id = $1`, customerID).Scan(&q.Balance); err != nil {
		return nil, err
	}
	if q.Meals > q.Balance {
		q.Shortfall = q.Meals - q.Balance
	}
	return q, nil
}

// lockSpendablePackages reads every package a customer can spend from, in the
// order they must be spent, with the rows locked for the rest of the
// transaction. SELECT … FOR UPDATE is what makes two concurrent checkouts
// safe; the ledger's own sign constraints are what make it true even if this
// code is wrong.
func (s *Service) lockSpendablePackages(ctx context.Context, tx *sql.Tx, customerID uuid.UUID) ([]*creditPackage, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, customer_id, meal_credits, expires_at, status
		  FROM customer_package
		 WHERE customer_id = $1 AND status = 'ACTIVE' AND expires_at IS NOT NULL
		 ORDER BY expires_at ASC, created_at ASC
		   FOR UPDATE`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*creditPackage
	for rows.Next() {
		var p credit.Package
		var status string
		if err := rows.Scan(&p.ID, &p.CustomerID, &p.MealCredits, &p.ExpiresAt, &status); err != nil {
			return nil, err
		}
		p.Status = credit.Status(status)
		out = append(out, &creditPackage{pkg: p})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, cp := range out {
		ledger, err := tx.QueryContext(ctx, `
			SELECT entry_type, qty FROM credit_ledger
			 WHERE customer_package_id = $1`, cp.pkg.ID)
		if err != nil {
			return nil, err
		}
		for ledger.Next() {
			var e credit.Entry
			var t string
			if err := ledger.Scan(&t, &e.Qty); err != nil {
				ledger.Close()
				return nil, err
			}
			e.EntryType = credit.EntryType(t)
			cp.entries = append(cp.entries, e)
		}
		err = ledger.Err()
		ledger.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// spendCredits turns the basket into REDEEM entries, soonest-expiring package
// first, refusing the whole order rather than half-spending it.
func spendCredits(packs []*creditPackage, lines []CreditLine, orderID uuid.UUID, now time.Time, actor *uuid.UUID) ([]credit.Entry, error) {
	// Latest service date first is NOT what we want: a credit that expires
	// soonest should go to the EARLIEST delivery it can still cover, so sort
	// the demand by date and walk it forwards.
	demand := append([]CreditLine(nil), lines...)
	sort.Slice(demand, func(i, j int) bool { return demand[i].ServiceDate.Before(demand[j].ServiceDate) })

	var out []credit.Entry
	for _, l := range demand {
		remaining := l.Qty
		for _, cp := range packs {
			if remaining == 0 {
				break
			}
			avail := credit.Balance(cp.entries)
			if avail <= 0 {
				continue
			}
			// D-27: a delivery may not be scheduled after the package expires.
			if l.ServiceDate.After(cp.pkg.ExpiresAt) {
				continue
			}
			take := remaining
			if take > avail {
				take = avail
			}
			entries, err := credit.Redeem(credit.RedeemRequest{
				Package:      cp.pkg,
				Entries:      cp.entries,
				Meals:        take,
				ServiceDate:  l.ServiceDate,
				OccurredAt:   now,
				ReferenceID:  orderID,
				ReferenceTyp: "customer_order",
				ActorID:      actor,
			})
			if err != nil {
				return nil, err
			}
			cp.entries = append(cp.entries, entries...)
			out = append(out, entries...)
			remaining -= take
		}
		if remaining > 0 {
			return nil, fmt.Errorf("%w: %d credit(s) short for %s",
				ErrNoCredits, remaining, l.ServiceDate.Format("2006-01-02"))
		}
	}
	return out, nil
}

// writeLedger appends the REDEEM rows and marks any package that has just run
// out as EXHAUSTED, so it stops being offered as spendable.
func writeLedger(ctx context.Context, tx *sql.Tx, entries []credit.Entry, packs []*creditPackage) error {
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_ledger
			  (id, customer_id, customer_package_id, entry_type, qty,
			   reference_type, reference_id, occurred_at, created_by, note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'')`,
			e.ID, e.CustomerID, e.CustomerPackageID, string(e.EntryType), e.Qty,
			e.ReferenceType, e.ReferenceID, e.OccurredAt, e.CreatedBy); err != nil {
			return err
		}
	}
	for _, cp := range packs {
		if credit.Balance(cp.entries) == 0 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE customer_package SET status = 'EXHAUSTED', updated_at = now()
				 WHERE id = $1 AND status = 'ACTIVE'`, cp.pkg.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckoutWithCredits places a meal order paid for with credits.
//
// No payment row, no transfer suffix, no deadline: the money moved when the
// package was bought. The order is PAID from the moment it is placed, and the
// only thing that can refuse it is the cut-off, the kitchen's capacity, or an
// empty credit balance.
func (s *Service) CheckoutWithCredits(ctx context.Context, in CheckoutInput) (uuid.UUID, string, error) {
	quote, err := s.CreditQuoteFor(ctx, in.CustomerID, in.Items)
	if err != nil {
		return uuid.Nil, "", err
	}
	if len(quote.Lines) == 0 {
		return uuid.Nil, "", errors.New("ordering: the basket is empty")
	}

	params, err := s.params(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	cutOff := params["order.cutoff_time"]
	prefix := params["order.number_prefix"]
	if prefix == "" {
		prefix = "EVM"
	}
	for _, l := range quote.Lines {
		ok, err := s.beforeCutOff(l.ServiceDate, cutOff)
		if err != nil {
			return uuid.Nil, "", err
		}
		if !ok {
			return uuid.Nil, "", fmt.Errorf("%w: the cut-off for %s has passed",
				ErrPastCutOff, l.ServiceDate.Format("2006-01-02"))
		}
	}

	addr, err := s.address(ctx, in.CustomerID, in.AddressID)
	if err != nil {
		return uuid.Nil, "", err
	}

	var orderID uuid.UUID
	var orderNumber string

	err = database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Idempotency: a retried checkout must not place a second order, and
		// must not spend the credits twice.
		if in.IdempotencyKey != "" {
			var existingID uuid.UUID
			var existingNumber string
			err := tx.QueryRowContext(ctx, `
				SELECT id, order_number FROM customer_order
				 WHERE customer_id = $1 AND idempotency_key = $2`,
				in.CustomerID, in.IdempotencyKey).Scan(&existingID, &existingNumber)
			if err == nil {
				orderID, orderNumber = existingID, existingNumber
				return nil
			}
			if err != sql.ErrNoRows {
				return err
			}
		}

		period := s.now().In(s.loc).Format("0601")
		if err := tx.QueryRowContext(ctx,
			`SELECT next_order_number($1, $2)`, prefix, period).Scan(&orderNumber); err != nil {
			return err
		}
		orderID = id.New()

		// Every amount is zero. The constraints still hold: base + tax = total
		// and payment_amount = total + rounding are both 0 = 0.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_order
			  (id, order_number, customer_id, order_type, status,
			   subtotal_idr, delivery_fee_idr, discount_idr, total_idr,
			   tax_base_idr, tax_idr, tax_rate_bps,
			   payment_amount_idr, payment_rounding_idr,
			   idempotency_key, notes, placed_at)
			VALUES ($1,$2,$3,'MEAL','PAID',0,0,0,0,0,0,0,0,0,$4,$5,now())`,
			orderID, orderNumber, in.CustomerID,
			nullIfEmpty(in.IdempotencyKey), in.CourierNote); err != nil {
			return err
		}

		packs, err := s.lockSpendablePackages(ctx, tx, in.CustomerID)
		if err != nil {
			return err
		}
		entries, err := spendCredits(packs, quote.Lines, orderID, s.now(), nil)
		if err != nil {
			return err
		}

		for _, l := range quote.Lines {
			lineID := id.New()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO order_line
				  (id, order_id, scheduled_meal_id, qty, unit_price_idr, normal_price_idr,
				   line_total_idr, line_tax_base_idr, line_tax_idr, is_promo,
				   meal_snapshot)
				-- price_table stays NULL: a credit-paid line was resolved
				-- against no price table at all, and inventing a 'CREDIT'
				-- value would be a lie the CHECK rightly refused.
				VALUES ($1,$2,$3,$4,0,0,0,0,0,false,$5)`,
				lineID, orderID, l.ScheduledMealID, l.Qty,
				snapshotJSON(l.MealSnapshot)); err != nil {
				return err
			}
			if err := s.reserveAndRoute(ctx, tx, orderID, lineID, in.CustomerID, addr, QuoteLine{
				ScheduledMealID: l.ScheduledMealID,
				ServiceDate:     l.ServiceDate,
				SlotTime:        l.SlotTime,
				Qty:             l.Qty,
				slotID:          l.slotID,
				dietTypeID:      l.dietTypeID,
				MealSnapshot:    l.MealSnapshot,
			}); err != nil {
				return err
			}
		}

		return writeLedger(ctx, tx, entries, packs)
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return orderID, orderNumber, nil
}
