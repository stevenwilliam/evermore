package http

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// AddressView is a saved delivery address as the checkout screen shows it.
type AddressView struct {
	ID        uuid.UUID
	Label     string
	Line      string
	Recipient string
	Phone     string
	Note      string
	IsDefault bool
}

func (h *AppHandler) addresses(ctx context.Context, customerID uuid.UUID) ([]AddressView, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, label, address_line, recipient_name, recipient_phone, driver_note, is_default
		  FROM customer_address
		 WHERE customer_id = $1 AND is_active
		 ORDER BY is_default DESC, label`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AddressView
	for rows.Next() {
		var a AddressView
		if err := rows.Scan(&a.ID, &a.Label, &a.Line, &a.Recipient, &a.Phone, &a.Note, &a.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// OrderView is one row of the customer's order history.
type OrderView struct {
	ID          uuid.UUID
	Number      string
	Status      string
	StatusLabel string
	TotalIDR    money.IDR
	PayIDR      money.IDR
	PlacedAt    time.Time
	Meals       int
	Deliveries  int
	Deadline    *time.Time
}

// statusLabelID renders an order status in Indonesian. Colour is never the
// only signal (CLAUDE.md §7), so the label carries the meaning.
var statusLabelID = map[string]string{
	"DRAFT":             "Draf",
	"AWAITING_PAYMENT":  "Menunggu pembayaran",
	"PAYMENT_SUBMITTED": "Menunggu verifikasi",
	"PAID":              "Lunas",
	"COMPLETED":         "Selesai",
	"CANCELLED":         "Dibatalkan",
	"EXPIRED":           "Kedaluwarsa",
	"REFUNDED":          "Dikembalikan",
}

func (h *AppHandler) customerOrders(ctx context.Context, customerID uuid.UUID, search string) ([]OrderView, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT o.id, o.order_number, o.status, o.total_idr, o.payment_amount_idr,
		       o.placed_at, o.payment_deadline_at,
		       COALESCE((SELECT sum(ol.qty) FROM order_line ol WHERE ol.order_id = o.id), 0)::int,
		       (SELECT count(*) FROM delivery d WHERE d.order_id = o.id)::int
		  FROM customer_order o
		 WHERE o.customer_id = $1
		   AND o.status <> 'DRAFT'
		   AND ($2 = '' OR o.order_number ILIKE '%' || $2 || '%')
		 ORDER BY o.created_at DESC
		 LIMIT 100`, customerID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderView
	for rows.Next() {
		var o OrderView
		var placed, deadline sql.NullTime
		if err := rows.Scan(&o.ID, &o.Number, &o.Status, (*int64)(&o.TotalIDR),
			(*int64)(&o.PayIDR), &placed, &deadline, &o.Meals, &o.Deliveries); err != nil {
			return nil, err
		}
		if placed.Valid {
			o.PlacedAt = placed.Time
		}
		if deadline.Valid {
			t := deadline.Time
			o.Deadline = &t
		}
		o.StatusLabel = statusLabelID[o.Status]
		if o.StatusLabel == "" {
			o.StatusLabel = o.Status
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// PaymentLine is one line on the payment summary.
type PaymentLine struct {
	Name      string
	Qty       int
	SlotLabel string
	TotalIDR  money.IDR
}

func (h *AppHandler) orderLines(ctx context.Context, orderID uuid.UUID) ([]PaymentLine, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT sm.name, ol.qty, to_char(sl.slot_time, 'HH24.MI'),
		       to_char(sm.service_date, 'DD Mon'), ol.line_total_idr
		  FROM order_line ol
		  JOIN scheduled_meal sm      ON sm.id = ol.scheduled_meal_id
		  JOIN delivery_time_slot sl  ON sl.id = sm.slot_id
		 WHERE ol.order_id = $1
		 ORDER BY sm.service_date, sl.slot_time`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentLine
	for rows.Next() {
		var l PaymentLine
		var slot, date string
		if err := rows.Scan(&l.Name, &l.Qty, &slot, &date, (*int64)(&l.TotalIDR)); err != nil {
			return nil, err
		}
		l.SlotLabel = date + " · " + slot
		out = append(out, l)
	}
	return out, rows.Err()
}

// PackageView is a purchased package with its live balance.
type PackageView struct {
	ID          uuid.UUID
	Number      string
	Name        string
	Credits     int
	Balance     int
	Status      string
	StatusLabel string
	ExpiresAt   *time.Time
	DaysLeft    int
}

// LedgerRow is one credit movement.
type LedgerRow struct {
	OccurredAt time.Time
	TypeLabel  string
	Qty        int
	Note       string
	Reference  string
	Balance    int
}

var ledgerLabelID = map[string]string{
	"PURCHASE":   "Pembelian paket",
	"REDEEM":     "Terpakai",
	"REFUND":     "Dikembalikan",
	"EXPIRE":     "Hangus",
	"ADJUSTMENT": "Penyesuaian",
}

// creditView loads packages and the ledger.
//
// The balance is SUM(qty) computed here, never a stored column
// (01-domain-model.md §3.6).
func (h *AppHandler) creditView(ctx context.Context, customerID uuid.UUID) ([]PackageView, []LedgerRow, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT cp.id, cp.package_number, p.name, cp.meal_credits, cp.status, cp.expires_at,
		       COALESCE((SELECT sum(cl.qty) FROM credit_ledger cl
		                  WHERE cl.customer_package_id = cp.id), 0)::int
		  FROM customer_package cp
		  JOIN package p ON p.id = cp.package_id
		 WHERE cp.customer_id = $1
		 ORDER BY cp.created_at DESC`, customerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var pkgs []PackageView
	today := time.Now().In(h.cfg.Location)
	for rows.Next() {
		var p PackageView
		var expires sql.NullTime
		if err := rows.Scan(&p.ID, &p.Number, &p.Name, &p.Credits, &p.Status, &expires, &p.Balance); err != nil {
			return nil, nil, err
		}
		if expires.Valid {
			e := expires.Time
			p.ExpiresAt = &e
			p.DaysLeft = int(e.Sub(today).Hours() / 24)
			if p.DaysLeft < 0 {
				p.DaysLeft = 0
			}
		}
		switch p.Status {
		case "ACTIVE":
			p.StatusLabel = "Aktif"
		case "PENDING":
			p.StatusLabel = "Menunggu pembayaran"
		case "EXHAUSTED":
			p.StatusLabel = "Habis"
		case "EXPIRED":
			p.StatusLabel = "Kedaluwarsa"
		case "CANCELLED":
			p.StatusLabel = "Dibatalkan"
		default:
			p.StatusLabel = p.Status
		}
		pkgs = append(pkgs, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	lRows, err := h.db.QueryContext(ctx, `
		SELECT cl.occurred_at, cl.entry_type, cl.qty, cl.note,
		       COALESCE(o.order_number, cp.package_number, '')
		  FROM credit_ledger cl
		  JOIN customer_package cp ON cp.id = cl.customer_package_id
		  LEFT JOIN customer_order o ON o.id = cl.reference_id
		 WHERE cl.customer_id = $1
		 ORDER BY cl.occurred_at DESC
		 LIMIT 50`, customerID)
	if err != nil {
		return nil, nil, err
	}
	defer lRows.Close()

	var ledger []LedgerRow
	for lRows.Next() {
		var r LedgerRow
		var typ string
		if err := lRows.Scan(&r.OccurredAt, &typ, &r.Qty, &r.Note, &r.Reference); err != nil {
			return nil, nil, err
		}
		r.TypeLabel = ledgerLabelID[typ]
		if r.TypeLabel == "" {
			r.TypeLabel = typ
		}
		ledger = append(ledger, r)
	}
	return pkgs, ledger, lRows.Err()
}

// anyRetailCustomer returns a retail customer id so an anonymous cart can be
// priced at the DEFAULT scope. It reads no customer data — only the scope the
// pricing resolver needs.
func (h *AppHandler) anyRetailCustomer(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := h.db.QueryRowContext(ctx, `
		SELECT c.id FROM customer c
		  JOIN customer_type ct ON ct.id = c.customer_type_id
		 WHERE ct.slug = 'retail' LIMIT 1`).Scan(&id)
	return id, err
}

// readFull fills buf, tolerating short reads.
func readFull(r io.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
	}
	return total, nil
}

// fsFS is io/fs.FS, aliased so handler signatures stay short.
type fsFS = fs.FS
