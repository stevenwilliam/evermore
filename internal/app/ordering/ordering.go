// Package ordering is the checkout use-case: it prices a cart, reserves
// capacity, routes each delivery to a kitchen and writes the order.
//
// Everything money-related delegates to internal/domain/money and
// internal/domain/pricing; nothing here does arithmetic on a price directly.
// The one thing this package owns that the domain cannot is the transaction:
// capacity is a limited resource, so reservation happens inside a single
// transaction holding SELECT … FOR UPDATE (CLAUDE.md §4).
package ordering

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/pricing"
	"github.com/stevenwilliam/evermore/internal/domain/routing"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
)

// Service performs checkout.
type Service struct {
	db  *sql.DB
	loc *time.Location
	now func() time.Time
}

func NewService(db *sql.DB, loc *time.Location, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, loc: loc, now: now}
}

var (
	ErrEmptyCart        = errors.New("CART_EMPTY")
	ErrPastCutOff       = errors.New("PAST_CUTOFF")
	ErrMealNotPublished = errors.New("MENU_NOT_PUBLISHED")
	ErrCapacityFull     = errors.New("CAPACITY_FULL")
	ErrAddressNotOwned  = errors.New("ADDRESS_NOT_FOUND")
	ErrQtyOutOfRange    = errors.New("QTY_OUT_OF_RANGE")
)

// CartItem is one line a customer asked for.
type CartItem struct {
	ScheduledMealID uuid.UUID
	Qty             int
}

// CheckoutInput is a complete checkout request.
type CheckoutInput struct {
	CustomerID     uuid.UUID
	AddressID      uuid.UUID
	Items          []CartItem
	CourierNote    string
	IdempotencyKey string
}

// Quote is a priced cart, before anything is written. The cart screen renders
// this, and checkout recomputes it rather than trusting a client total.
type Quote struct {
	Lines        []QuoteLine
	TotalQty     int
	Subtotal     money.IDR
	DeliveryFee  money.IDR
	Tax          money.TaxSplit
	Total        money.IDR
	TaxRateBPS   int
	Deliveries   int
	NextTierQty  int       // how many more meals to reach the next tier, 0 if none
	NextTierIDR  money.IDR // the unit price at that tier
	NextTierSave money.IDR // what the whole cart would save
}

// QuoteLine is one priced line.
type QuoteLine struct {
	ScheduledMealID uuid.UUID
	MealName        string
	ServiceDate     time.Time
	SlotTime        string
	DietName        string
	DietSlug        string
	Qty             int
	UnitPrice       money.IDR
	NormalPrice     money.IDR
	LineTotal       money.IDR
	LineTax         money.TaxSplit
	Trace           pricing.Trace
	MealSnapshot    map[string]any

	// Plumbing the cart screen does not render but reservation needs.
	slotID     uuid.UUID
	dietTypeID uuid.UUID
}

// Quote prices a cart without writing anything.
//
// The tier resolves on the order's TOTAL quantity across every date (D14): a
// cart of Monday×2 and Tuesday×2 is four meals and reaches the 4-9 band, which
// is what the artifact's "Tambah 4 porsi lagi" prompt is computing against.
func (s *Service) Quote(ctx context.Context, customerID uuid.UUID, items []CartItem) (*Quote, error) {
	if len(items) == 0 {
		return nil, ErrEmptyCart
	}

	params, err := s.params(ctx)
	if err != nil {
		return nil, err
	}
	maxQty := paramInt(params, "order.max_qty", 999)
	taxBPS := paramInt(params, "tax.rate_bps", 0)

	totalQty := 0
	for _, it := range items {
		if it.Qty <= 0 {
			return nil, ErrQtyOutOfRange
		}
		totalQty += it.Qty
	}
	if totalQty > maxQty {
		return nil, fmt.Errorf("%w: %d meals exceeds the maximum of %d", ErrQtyOutOfRange, totalQty, maxQty)
	}

	customerTypeID, err := s.customerTypeOf(ctx, customerID)
	if err != nil {
		return nil, err
	}
	tiers, err := s.tiers(ctx)
	if err != nil {
		return nil, err
	}
	// Coverage is validated on save, but a gap that slipped through would
	// otherwise surface as a confusing checkout failure.
	if err := pricing.ValidateTiers(tiers, maxQty); err != nil {
		return nil, err
	}

	q := &Quote{TotalQty: totalQty, TaxRateBPS: taxBPS}
	var splits []money.TaxSplit
	dates := map[string]bool{}

	for _, it := range items {
		meal, err := s.mealForSale(ctx, it.ScheduledMealID)
		if err != nil {
			return nil, err
		}

		rows, err := s.priceRows(ctx, meal.DietTypeID, meal.ServiceDate)
		if err != nil {
			return nil, err
		}
		// The order date is today, not the service date: a price is what was
		// in force when the customer bought, and validity ranges are read
		// against that.
		orderDate := s.today()
		resolved, err := pricing.Resolve(pricing.Request{
			CustomerTypeID: customerTypeID,
			DietTypeID:     meal.DietTypeID,
			Qty:            totalQty, // the whole cart, not this line (D14)
			OrderDate:      orderDate,
		}, tiers, rows)
		if err != nil {
			return nil, err
		}

		// The single-portion price, so the UI can show what the tier saved.
		normal := resolved.UnitPriceIDR
		if single, err := pricing.Resolve(pricing.Request{
			CustomerTypeID: customerTypeID, DietTypeID: meal.DietTypeID,
			Qty: 1, OrderDate: orderDate,
		}, tiers, rows); err == nil {
			normal = single.UnitPriceIDR
		}

		lineTotal, err := money.Mul(resolved.UnitPriceIDR, it.Qty)
		if err != nil {
			return nil, err
		}
		// The split is computed on the LINE TOTAL, never per unit: computing
		// per unit and multiplying multiplies the rounding error by the
		// quantity (01-domain-model.md §3.11).
		split, err := money.SplitInclusive(lineTotal, taxBPS)
		if err != nil {
			return nil, err
		}
		splits = append(splits, split)

		snapshot, err := s.mealSnapshot(ctx, it.ScheduledMealID)
		if err != nil {
			return nil, err
		}

		q.Lines = append(q.Lines, QuoteLine{
			ScheduledMealID: it.ScheduledMealID,
			MealName:        meal.Name,
			ServiceDate:     meal.ServiceDate,
			SlotTime:        meal.SlotTime,
			DietName:        meal.DietName,
			DietSlug:        meal.DietSlug,
			Qty:             it.Qty,
			UnitPrice:       resolved.UnitPriceIDR,
			NormalPrice:     normal,
			LineTotal:       lineTotal,
			LineTax:         split,
			Trace:           resolved.Trace,
			MealSnapshot:    snapshot,
			slotID:          meal.SlotID,
			dietTypeID:      meal.DietTypeID,
		})
		dates[meal.ServiceDate.Format("2006-01-02")+"|"+meal.SlotTime] = true
	}

	q.Deliveries = len(dates)

	// One delivery fee per distinct date+slot, evaluated through the band
	// engine even though every band is currently zero (D14), so switching it
	// on later is a settings edit rather than a code change.
	perDelivery, err := s.deliveryFee(ctx)
	if err != nil {
		return nil, err
	}
	q.DeliveryFee, err = money.Mul(perDelivery, q.Deliveries)
	if err != nil {
		return nil, err
	}
	if q.DeliveryFee > 0 {
		// The delivery fee is a taxable supply too, split at the same rate.
		feeSplit, err := money.SplitInclusive(q.DeliveryFee, taxBPS)
		if err != nil {
			return nil, err
		}
		splits = append(splits, feeSplit)
	}

	for _, l := range q.Lines {
		if q.Subtotal, err = money.Sum(q.Subtotal, l.LineTotal); err != nil {
			return nil, err
		}
	}
	// The order's tax is the SUM of the line taxes, never re-derived from the
	// total — re-deriving reintroduces a rounding difference between an
	// invoice and its own lines.
	if q.Tax, err = money.SumSplits(splits); err != nil {
		return nil, err
	}
	q.Total = q.Tax.Inclusive

	s.attachNextTier(q, tiers, customerTypeID)
	return q, nil
}

// attachNextTier computes the artifact's "add N more and the whole cart drops
// to Rp X" prompt.
func (s *Service) attachNextTier(q *Quote, tiers []pricing.Tier, ctID uuid.UUID) {
	if len(q.Lines) == 0 {
		return
	}
	// The next band up, by minimum quantity.
	best := -1
	for _, t := range tiers {
		if t.MinQty > q.TotalQty && (best == -1 || t.MinQty < best) {
			best = t.MinQty
		}
	}
	if best == -1 {
		return
	}
	q.NextTierQty = best - q.TotalQty
}

// Checkout prices the cart again, reserves capacity, routes deliveries and
// writes the order — all in one transaction.
func (s *Service) Checkout(ctx context.Context, in CheckoutInput) (uuid.UUID, string, error) {
	quote, err := s.Quote(ctx, in.CustomerID, in.Items)
	if err != nil {
		return uuid.Nil, "", err
	}

	params, err := s.params(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	cutOff := params["order.cutoff_time"]
	windowHours := paramInt(params, "order.payment_window_hours", 3)
	prefix := params["order.number_prefix"]
	if prefix == "" {
		prefix = "EVM"
	}

	// Cut-off applies per service date, before anything is written.
	for _, l := range quote.Lines {
		ok, err := s.beforeCutOff(l.ServiceDate, cutOff)
		if err != nil {
			return uuid.Nil, "", err
		}
		if !ok {
			return uuid.Nil, "", fmt.Errorf("%w: %s sudah lewat batas pesan",
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
		// Idempotency: a retried checkout must not place a second order.
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

		deadline := s.now().Add(time.Duration(windowHours) * time.Hour)

		// The order row is written first, with the payment amount equal to the
		// total and no rounding. payment_suffix_claim carries a foreign key to
		// customer_order, so the claim cannot precede the row it references —
		// and customer_order_payment_reconciles holds at this intermediate
		// state as well as at the final one.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_order
			  (id, order_number, customer_id, order_type, status,
			   subtotal_idr, delivery_fee_idr, discount_idr, total_idr,
			   tax_base_idr, tax_idr, tax_rate_bps,
			   payment_amount_idr, payment_rounding_idr, payment_deadline_at,
			   idempotency_key, price_resolution_trace, notes, placed_at)
			VALUES ($1,$2,$3,'MEAL','AWAITING_PAYMENT',
			        $4,$5,0,$6,$7,$8,$9,$6,0,$10,$11,$12,$13,now())`,
			orderID, orderNumber, in.CustomerID,
			int64(quote.Subtotal), int64(quote.DeliveryFee), int64(quote.Total),
			int64(quote.Tax.Base), int64(quote.Tax.Tax), quote.TaxRateBPS,
			deadline, nullIfEmpty(in.IdempotencyKey), traceJSON(quote), in.CourierNote); err != nil {
			return err
		}

		// Now the matching suffix, claimed under a unique index so two
		// concurrent checkouts cannot take the same one on the same day.
		_, paymentAmount, rounding, err := s.claimSuffix(ctx, tx, orderID, quote.Total)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE customer_order
			   SET payment_amount_idr = $2, payment_rounding_idr = $3
			 WHERE id = $1`, orderID, int64(paymentAmount), int64(rounding))
		if err != nil {
			return err
		}
		// Assert the update landed. An UPDATE that matched nothing would leave
		// the order asking for the un-suffixed amount, and no incoming transfer
		// would ever match it.
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("ordering: attaching the payment suffix affected %d rows, want 1", n)
		}

		for _, l := range quote.Lines {
			lineID := id.New()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO order_line
				  (id, order_id, scheduled_meal_id, qty, unit_price_idr, normal_price_idr,
				   line_total_idr, line_tax_base_idr, line_tax_idr, is_promo,
				   price_row_id, price_table, meal_snapshot)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
				lineID, orderID, l.ScheduledMealID, l.Qty,
				int64(l.UnitPrice), int64(l.NormalPrice), int64(l.LineTotal),
				int64(l.LineTax.Base), int64(l.LineTax.Tax), l.Trace.PromoApplied,
				l.Trace.RowID, string(l.Trace.Table), snapshotJSON(l.MealSnapshot)); err != nil {
				return err
			}

			// Route and reserve, per line, inside this transaction.
			if err := s.reserveAndRoute(ctx, tx, orderID, lineID, in.CustomerID, addr, l); err != nil {
				return err
			}
		}

		// Payment record.
		var bankID uuid.UUID
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM bank_account WHERE is_active ORDER BY sort_order LIMIT 1`).Scan(&bankID); err != nil {
			return fmt.Errorf("no active bank account is configured: %w", err)
		}
		paymentID := id.New()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment (id, order_id, bank_account_id, provider, status, expected_amount_idr)
			VALUES ($1, $2, $3, 'MANUAL_TRANSFER', 'PENDING', $4)`,
			paymentID, orderID, bankID, int64(paymentAmount)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO payment_event (id, payment_id, from_status, to_status, reason)
			VALUES ($1, $2, NULL, 'PENDING', 'order placed')`, id.New(), paymentID)
		return err
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return orderID, orderNumber, nil
}

// reserveAndRoute picks a kitchen for a line and takes its capacity.
//
// The SELECT … FOR UPDATE is what makes concurrent checkouts safe; the CHECK
// constraint on kitchen_capacity is what makes it true even if this code is
// wrong.
func (s *Service) reserveAndRoute(ctx context.Context, tx *sql.Tx, orderID, lineID, customerID uuid.UUID, addr *addressRow, l QuoteLine) error {
	candidates, err := s.candidates(ctx, tx, addr, l)
	if err != nil {
		return err
	}
	ranked, err := routing.Rank(routing.Request{
		Candidates: candidates, PortionsNeeded: l.Qty,
	})
	if err != nil {
		if errors.Is(err, routing.ErrNotServiceable) {
			// Log the attempt so the dashboard's "di luar jangkauan" count is
			// real rather than an estimate.
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO out_of_range_attempt
				  (id, customer_id, latitude, longitude, district, service_date, occurred_at)
				VALUES ($1,$2,$3,$4,$5,$6, now())`,
				id.New(), customerID, addr.Lat, addr.Lng, addr.District, l.ServiceDate)
		}
		return err
	}

	// Walk the ranked kitchens, taking capacity under a row lock, and fall
	// through to the next when one turns out to be full.
	//
	// The free-capacity figure in a candidate was read before the lock, so a
	// concurrent checkout may have taken the last portions in between. Giving
	// up at that point would tell a customer there is no capacity while the
	// next kitchen still had room — which is what happened before this loop
	// existed: 12 checkouts against 6 available portions sold only 4.
	var (
		assignment routing.Assignment
		capID      uuid.UUID
		taken      bool
		lastFull   string
	)
	for _, cand := range ranked {
		var reserved, max int
		var rowID uuid.UUID
		err := tx.QueryRowContext(ctx, `
			SELECT id, reserved_portions, max_portions
			  FROM kitchen_capacity
			 WHERE kitchen_id = $1 AND service_date = $2::date AND slot_id = $3
			 FOR UPDATE`,
			cand.KitchenID, l.ServiceDate.Format("2006-01-02"), l.slotID).
			Scan(&rowID, &reserved, &max)
		if err == sql.ErrNoRows {
			lastFull = cand.Code
			continue // this kitchen has no quota row for the date at all
		}
		if err != nil {
			return err
		}
		if reserved+l.Qty > max {
			lastFull = cand.Code
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE kitchen_capacity SET reserved_portions = reserved_portions + $2 WHERE id = $1`,
			rowID, l.Qty); err != nil {
			return err
		}
		assignment, capID, taken = cand, rowID, true
		break
	}
	if !taken {
		return fmt.Errorf("%w: semua dapur yang melayani alamat ini sudah penuh untuk %s (terakhir dicoba %s)",
			ErrCapacityFull, l.ServiceDate.Format("2006-01-02"), lastFull)
	}
	_ = capID

	deliveryID := id.New()
	// The delivery number comes from the same monthly sequence the order
	// numbers use. Deriving it from the uuid was wrong: UUIDv7 is time-ordered,
	// so two deliveries created in the same millisecond share their leading
	// characters and collide on delivery_number_uk.
	var deliveryNumber string
	if err := tx.QueryRowContext(ctx,
		`SELECT next_order_number('DLV', $1)`, s.now().In(s.loc).Format("0601")).
		Scan(&deliveryNumber); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO delivery
		  (id, delivery_number, order_id, customer_id, service_date, slot_id,
		   diet_type_id, kitchen_id, address_id, address_snapshot,
		   assigned_distance_m, assignment_mode, assignment_reason, status)
		VALUES ($1,$2,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,'AUTO',$12,'SCHEDULED')`,
		deliveryID, deliveryNumber, orderID, customerID,
		l.ServiceDate.Format("2006-01-02"), l.slotID, l.dietTypeID,
		assignment.KitchenID, addr.ID, addr.snapshotJSON(),
		assignment.DistanceM, assignment.Reason)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO delivery_line (id, delivery_id, scheduled_meal_id, order_line_id, qty, meal_snapshot)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		id.New(), deliveryID, l.ScheduledMealID, lineID, l.Qty, snapshotJSON(l.MealSnapshot))
	return err
}
