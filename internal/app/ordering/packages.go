package ordering

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
)

// Package purchase (D36).
//
// A package is a prepaid bundle of meal credits. It deliberately does NOT go
// through the meal cart: it has no service date, no address, no kitchen and no
// delivery, so cut-off, routing and capacity — the three things Checkout
// exists to enforce — have nothing to act on. Forcing it through that path
// would mean teaching every one of them to skip themselves.
//
// What it DOES share is everything after the order exists: the same
// customer_order, the same payment with its matching suffix, and the same
// verification queue. payments.activatePackages already turns a verified
// payment into an ACTIVE customer_package plus a PURCHASE ledger entry, so
// this function's job ends at writing the PENDING row it looks for.

var (
	ErrPackageNotForSale = errors.New("PACKAGE_NOT_FOR_SALE")
	ErrNoPackagePrice    = errors.New("PACKAGE_PRICE_MISSING")
)

// packageForSale is the package as resolved for one customer on one day.
type packageForSale struct {
	id           uuid.UUID
	name         string
	credits      int
	validityDays int
	price        money.IDR
	normalPrice  money.IDR
	isPromo      bool
	priceRowID   uuid.UUID
	priceTable   string
}

// resolvePackage reads the package and its price for this customer.
//
// Scope is resolved FIRST and the promo applied within it (02-business-rules
// §5.2): a customer-type normal price beats a DEFAULT promo. Resolving the
// other way round would quietly hand corporate customers the retail discount.
func (s *Service) resolvePackage(ctx context.Context, q queryer, customerID, packageID uuid.UUID) (*packageForSale, error) {
	p := &packageForSale{id: packageID}
	err := q.QueryRowContext(ctx, `
		SELECT name, meal_credits, validity_days
		  FROM package
		 WHERE id = $1 AND is_active`, packageID).Scan(&p.name, &p.credits, &p.validityDays)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPackageNotForSale
	}
	if err != nil {
		return nil, err
	}

	ctID, err := s.customerTypeOf(ctx, customerID)
	if err != nil {
		return nil, err
	}
	today := s.today().Format("2006-01-02")

	// Scope preference: the customer's own type, then DEFAULT.
	scopes := []string{"CT:" + ctID.String(), "DEFAULT"}
	for _, scope := range scopes {
		var normalID uuid.UUID
		var normal int64
		err := q.QueryRowContext(ctx, `
			SELECT id, total_price_idr FROM package_price_normal
			 WHERE package_id = $1 AND scope_key = $2 AND is_active AND validity @> $3::date`,
			packageID, scope, today).Scan(&normalID, &normal)
		if errors.Is(err, sql.ErrNoRows) {
			continue // try the next scope
		}
		if err != nil {
			return nil, err
		}
		p.normalPrice = money.IDR(normal)
		p.price = money.IDR(normal)
		p.priceRowID = normalID
		p.priceTable = "pkg_normal"

		// A promo in the SAME scope overrides it.
		var promoID uuid.UUID
		var promo int64
		err = q.QueryRowContext(ctx, `
			SELECT id, total_price_idr FROM package_price_promo
			 WHERE package_id = $1 AND scope_key = $2 AND is_active AND validity @> $3::date`,
			packageID, scope, today).Scan(&promoID, &promo)
		if err == nil {
			p.price = money.IDR(promo)
			p.priceRowID = promoID
			p.priceTable = "pkg_promo"
			p.isPromo = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return p, nil
	}
	// Never guess a price (02-business-rules §5.1): block the purchase.
	return nil, ErrNoPackagePrice
}

// queryer is satisfied by both *sql.DB and *sql.Tx, so price resolution can be
// reused for a quote outside a transaction and for the checkout inside one.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// PackageQuote prices a package for display without writing anything.
func (s *Service) PackageQuote(ctx context.Context, customerID, packageID uuid.UUID) (money.IDR, bool, error) {
	p, err := s.resolvePackage(ctx, s.db, customerID, packageID)
	if err != nil {
		return 0, false, err
	}
	return p.price, p.isPromo, nil
}

// BuyPackage places an order for one package and returns it awaiting payment.
func (s *Service) BuyPackage(ctx context.Context, customerID, packageID uuid.UUID, idempotencyKey string) (uuid.UUID, string, error) {
	params, err := s.params(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	windowHours := paramInt(params, "order.payment_window_hours", 3)
	rateBPS := paramInt(params, "tax.rate_bps", 0)
	prefix := params["order.number_prefix"]
	if prefix == "" {
		prefix = "EVM"
	}

	var orderID uuid.UUID
	var orderNumber string

	err = database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Idempotency: a retried submit must not buy the package twice.
		if idempotencyKey != "" {
			var existingID uuid.UUID
			var existingNumber string
			err := tx.QueryRowContext(ctx, `
				SELECT id, order_number FROM customer_order
				 WHERE customer_id = $1 AND idempotency_key = $2`,
				customerID, idempotencyKey).Scan(&existingID, &existingNumber)
			if err == nil {
				orderID, orderNumber = existingID, existingNumber
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}

		// Priced inside the transaction, never from the client. The form
		// carries only a package id; the amount is resolved here.
		p, err := s.resolvePackage(ctx, tx, customerID, packageID)
		if err != nil {
			return err
		}

		// Tax is inclusive of the displayed price, as it is on the meal path.
		split, err := money.SplitInclusive(p.price, rateBPS)
		if err != nil {
			return err
		}

		period := s.now().In(s.loc).Format("0601")
		if err := tx.QueryRowContext(ctx,
			`SELECT next_order_number($1, $2)`, prefix, period).Scan(&orderNumber); err != nil {
			return err
		}
		orderID = id.New()
		deadline := s.now().Add(time.Duration(windowHours) * time.Hour)

		// No delivery fee and no address: a package is not delivered, the
		// meals it later pays for are.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_order
			  (id, order_number, customer_id, order_type, status,
			   subtotal_idr, delivery_fee_idr, discount_idr, total_idr,
			   tax_base_idr, tax_idr, tax_rate_bps,
			   payment_amount_idr, payment_rounding_idr, payment_deadline_at,
			   idempotency_key, price_resolution_trace, notes, placed_at)
			VALUES ($1,$2,$3,'PACKAGE','AWAITING_PAYMENT',
			        $4,0,0,$4,$5,$6,$7,$4,0,$8,$9,$10,'',now())`,
			orderID, orderNumber, customerID,
			int64(p.price), int64(split.Base), int64(split.Tax), rateBPS,
			deadline, nullIfEmpty(idempotencyKey),
			packageTraceJSON(p)); err != nil {
			return err
		}

		// The matching transfer suffix, exactly as the meal path claims it.
		_, paymentAmount, rounding, err := s.claimSuffix(ctx, tx, orderID, p.price)
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
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("ordering: attaching the payment suffix affected %d rows, want 1", n)
		}

		// order_line_exactly_one_subject: package_id set, scheduled_meal_id null.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_line
			  (id, order_id, package_id, qty, unit_price_idr, normal_price_idr,
			   line_total_idr, line_tax_base_idr, line_tax_idr, is_promo,
			   price_row_id, price_table, meal_snapshot)
			VALUES ($1,$2,$3,1,$4,$5,$4,$6,$7,$8,$9,$10,$11)`,
			id.New(), orderID, packageID,
			int64(p.price), int64(p.normalPrice),
			int64(split.Base), int64(split.Tax), p.isPromo,
			p.priceRowID, p.priceTable, packageSnapshotJSON(p)); err != nil {
			return err
		}

		// The PENDING row payments.activatePackages looks for. It carries no
		// activated_at or expires_at yet — the validity window starts when the
		// payment is verified, not when the order is placed (D-14), and
		// customer_package_active_is_dated permits exactly that while PENDING.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_package
			  (id, customer_id, package_id, order_id, package_number,
			   meal_credits, validity_days, price_paid_idr, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PENDING')`,
			id.New(), customerID, packageID, orderID,
			packageNumber(orderNumber), p.credits, p.validityDays,
			int64(p.price)); err != nil {
			return err
		}

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
			VALUES ($1, $2, NULL, 'PENDING', 'package order placed')`, id.New(), paymentID)
		return err
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return orderID, orderNumber, nil
}

// packageNumber derives the package's human reference from the order's, so the
// two can be read against each other in the back office. Order numbers are
// unique per period, so this is too.
func packageNumber(orderNumber string) string {
	parts := strings.SplitN(orderNumber, "-", 2)
	if len(parts) == 2 {
		return "PKG-" + parts[1]
	}
	return "PKG-" + orderNumber
}

func packageTraceJSON(p *packageForSale) string {
	return fmt.Sprintf(
		`{"kind":"package","package_id":%q,"price_table":%q,"price_row_id":%q,"promo":%t,"normal_idr":%d,"charged_idr":%d}`,
		p.id, p.priceTable, p.priceRowID, p.isPromo, int64(p.normalPrice), int64(p.price))
}

func packageSnapshotJSON(p *packageForSale) string {
	return fmt.Sprintf(`{"name":%q,"meal_credits":%d,"validity_days":%d}`,
		p.name, p.credits, p.validityDays)
}
