package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/app/payments"
	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/order"
	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

// DashboardView is the S1 screen from the artifact: today's numbers, the
// capacity grid and the actions that need a human.
type DashboardView struct {
	DateLabel      string
	MealsToday     int
	Deliveries     int
	Delivered      int
	OnTheRoad      int
	AwaitingVerify int
	OldestWaitMin  int
	RevenueIDR     money.IDR
	OutOfRange     int
	CutOffIn       string
	Capacity       []CapacityRow
	Actions        []ActionRow
}

// CapacityRow is one kitchen's row in the "terpakai / kuota" grid.
type CapacityRow struct {
	Code  string
	Name  string
	Cells []CapacityCell
}

// CapacityCell is one slot's usage. Closed means the kitchen does not serve
// that slot, which the artifact renders as "tutup" rather than 0/0.
type CapacityCell struct {
	SlotLabel string
	Used      int
	Quota     int
	Closed    bool
	Full      bool
}

// ActionRow is one item in "Perlu tindakan".
type ActionRow struct {
	Title  string
	Detail string
	Link   string
	CTA    string
	Kind   string // info | warn
}

// MenuDay is one column of the back-office menu calendar (S2).
type MenuDay struct {
	Date     time.Time
	Weekday  string
	DayMonth string
	Slots    []MenuSlot
}

// MenuSlot groups the meals scheduled at one time on one day.
type MenuSlot struct {
	SlotLabel string
	SlotAlias string
	Meals     []MenuMeal
}

// MenuMeal is one scheduled meal in the calendar.
type MenuMeal struct {
	ID         uuid.UUID
	Name       string
	DietName   string
	Status     string
	Components int
	Kcal       int
	Complete   bool
	Used       int
	Quota      int
	Full       bool
}

// Dashboard renders the back-office landing screen.
func (h *AppHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "", "Dasbor — Back office Evermore", "")
	d.NoIndex = true
	d.Principal = PrincipalOf(c)

	dash, err := h.dashboard(ctx, params)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d.Dash = dash
	h.render(c, "bo-dashboard", http.StatusOK, d)
}

func (h *AppHandler) dashboard(ctx context.Context, params map[string]string) (*DashboardView, error) {
	today := h.today()
	dv := &DashboardView{
		DateLabel: h.longDate(today),
		CutOffIn:  h.cutOffCountdown(params),
	}
	date := today.Format("2006-01-02")

	// One round trip for the headline numbers rather than six.
	err := h.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE((SELECT sum(dl.qty) FROM delivery d
		              JOIN delivery_line dl ON dl.delivery_id = d.id
		             WHERE d.service_date = $1::date AND d.status <> 'CANCELLED'), 0)::int,
		  (SELECT count(*) FROM delivery WHERE service_date = $1::date AND status <> 'CANCELLED')::int,
		  (SELECT count(*) FROM delivery WHERE service_date = $1::date AND status = 'DELIVERED')::int,
		  (SELECT count(*) FROM delivery WHERE service_date = $1::date AND status = 'OUT_FOR_DELIVERY')::int,
		  (SELECT count(*) FROM payment WHERE status = 'SUBMITTED')::int,
		  COALESCE((SELECT EXTRACT(EPOCH FROM (now() - min(submitted_at)))/60
		              FROM payment WHERE status = 'SUBMITTED'), 0)::int,
		  COALESCE((SELECT sum(o.total_idr) FROM customer_order o
		             WHERE o.status IN ('PAID','COMPLETED')
		               AND o.placed_at >= $1::date), 0)::bigint,
		  (SELECT count(*) FROM out_of_range_attempt WHERE occurred_at >= $1::date)::int`,
		date).Scan(&dv.MealsToday, &dv.Deliveries, &dv.Delivered, &dv.OnTheRoad,
		&dv.AwaitingVerify, &dv.OldestWaitMin, (*int64)(&dv.RevenueIDR), &dv.OutOfRange)
	if err != nil {
		return nil, err
	}

	// The capacity grid.
	slotRows, err := h.db.QueryContext(ctx, `
		SELECT to_char(slot_time, 'HH24.MI') FROM delivery_time_slot
		 WHERE is_active ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	var slotLabels []string
	for slotRows.Next() {
		var s string
		if err := slotRows.Scan(&s); err != nil {
			slotRows.Close()
			return nil, err
		}
		slotLabels = append(slotLabels, s)
	}
	slotRows.Close()

	capRows, err := h.db.QueryContext(ctx, `
		SELECT k.code, k.name, to_char(sl.slot_time, 'HH24.MI'),
		       COALESCE(kc.reserved_portions, 0), COALESCE(kc.max_portions, 0),
		       EXISTS (SELECT 1 FROM kitchen_slot ks
		                WHERE ks.kitchen_id = k.id AND ks.slot_id = sl.id AND ks.is_active)
		  FROM kitchen k
		 CROSS JOIN delivery_time_slot sl
		  LEFT JOIN kitchen_capacity kc
		         ON kc.kitchen_id = k.id AND kc.slot_id = sl.id AND kc.service_date = $1::date
		 WHERE k.is_active AND sl.is_active
		 ORDER BY k.priority, k.code, sl.sort_order`, date)
	if err != nil {
		return nil, err
	}
	defer capRows.Close()

	byKitchen := map[string]*CapacityRow{}
	var kitchenOrder []string
	for capRows.Next() {
		var code, name, slot string
		var used, quota int
		var serves bool
		if err := capRows.Scan(&code, &name, &slot, &used, &quota, &serves); err != nil {
			return nil, err
		}
		row, ok := byKitchen[code]
		if !ok {
			row = &CapacityRow{Code: code, Name: name}
			byKitchen[code] = row
			kitchenOrder = append(kitchenOrder, code)
		}
		row.Cells = append(row.Cells, CapacityCell{
			SlotLabel: slot, Used: used, Quota: quota,
			Closed: !serves, Full: quota > 0 && used >= quota,
		})
	}
	if err := capRows.Err(); err != nil {
		return nil, err
	}
	for _, code := range kitchenOrder {
		dv.Capacity = append(dv.Capacity, *byKitchen[code])
	}
	_ = slotLabels

	// "Perlu tindakan" — only things a human has to decide.
	if dv.AwaitingVerify > 0 {
		dv.Actions = append(dv.Actions, ActionRow{
			Title:  plural(dv.AwaitingVerify, "bukti transfer menunggu"),
			Detail: "tertua " + minutesLabel(dv.OldestWaitMin),
			Link:   "/bo/pembayaran", CTA: "Verifikasi", Kind: "warn",
		})
	}
	var drafts int
	if err := h.db.QueryRowContext(ctx, `
		SELECT count(*) FROM scheduled_meal
		 WHERE status = 'DRAFT' AND service_date >= $1::date`, date).Scan(&drafts); err == nil && drafts > 0 {
		dv.Actions = append(dv.Actions, ActionRow{
			Title:  plural(drafts, "menu masih DRAFT"),
			Detail: "belum terbit untuk pelanggan",
			Link:   "/bo/menu", CTA: "Tinjau", Kind: "info",
		})
	}
	var fullSlots int
	if err := h.db.QueryRowContext(ctx, `
		SELECT count(*) FROM kitchen_capacity
		 WHERE service_date = $1::date AND max_portions > 0
		   AND reserved_portions >= max_portions`, date).Scan(&fullSlots); err == nil && fullSlots > 0 {
		dv.Actions = append(dv.Actions, ActionRow{
			Title:  plural(fullSlots, "slot dapur penuh hari ini"),
			Detail: "pesanan baru dialihkan ke dapur lain",
			Link:   "/bo/menu", CTA: "Lihat", Kind: "warn",
		})
	}
	// A diet with no price configured blocks checkout with
	// PRICE_NOT_CONFIGURED rather than guessing, so surface it here.
	var unpriced int
	if err := h.db.QueryRowContext(ctx, `
		SELECT count(*) FROM diet_type d
		 WHERE d.is_active
		   AND NOT EXISTS (SELECT 1 FROM meal_price_normal p
		                    WHERE p.diet_type_id = d.id AND p.is_active
		                      AND p.validity @> CURRENT_DATE)`).Scan(&unpriced); err == nil && unpriced > 0 {
		dv.Actions = append(dv.Actions, ActionRow{
			Title:  plural(unpriced, "tipe diet belum punya harga"),
			Detail: "checkout diblokir dengan PRICE_NOT_CONFIGURED",
			Link:   "/bo/menu", CTA: "Isi", Kind: "warn",
		})
	}
	return dv, nil
}

// PaymentQueue renders the verification queue (S4).
func (h *AppHandler) PaymentQueue(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	search := c.Query("q")
	if search != "" {
		if search, err = sanitize.Text("q", search, 0, 60); err != nil {
			Fail(c, apierror.Validation("Kata kunci terlalu panjang."))
			return
		}
	}
	d := h.base(c, params, "", "Verifikasi pembayaran — Back office", "")
	d.NoIndex = true
	d.Principal = PrincipalOf(c)
	d.Search = search
	d.Flash = c.Query("ok")
	d.FlashKind = "ok"
	if msg := c.Query("err"); msg != "" {
		d.Flash, d.FlashKind = msg, "warn"
	}

	if d.PayQueue, err = h.payments.Queue(ctx, search, 100); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "bo-payments", http.StatusOK, d)
}

// VerifyPayment approves a transfer.
func (h *AppHandler) VerifyPayment(c *gin.Context) {
	p := PrincipalOf(c)
	paymentID, err := uuid.Parse(c.PostForm("payment_id"))
	if err != nil {
		Fail(c, apierror.Validation("Pembayaran tidak dikenali."))
		return
	}
	err = h.payments.Verify(c.Request.Context(), p.UserID, paymentID, actorFor(p))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bo/pembayaran?err="+urlSafe(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/bo/pembayaran?ok="+urlSafe("Pembayaran diverifikasi."))
}

// RejectPayment sends a transfer back with a reason.
func (h *AppHandler) RejectPayment(c *gin.Context) {
	p := PrincipalOf(c)
	paymentID, err := uuid.Parse(c.PostForm("payment_id"))
	if err != nil {
		Fail(c, apierror.Validation("Pembayaran tidak dikenali."))
		return
	}
	reason, err := sanitize.Required("reason", c.PostForm("reason"), 240)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bo/pembayaran?err="+urlSafe("Alasan penolakan wajib diisi."))
		return
	}
	err = h.payments.Reject(c.Request.Context(), p.UserID, paymentID, actorFor(p), reason)
	if errors.Is(err, payments.ErrReasonRequired) {
		c.Redirect(http.StatusSeeOther, "/bo/pembayaran?err="+urlSafe("Alasan penolakan wajib diisi."))
		return
	}
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bo/pembayaran?err="+urlSafe(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/bo/pembayaran?ok="+urlSafe("Pembayaran ditolak dan pelanggan diberi tahu."))
}

// MenuCalendar renders the DRAFT/PUBLISHED grid (S2).
func (h *AppHandler) MenuCalendar(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	monday := mondayOf(h.today(), h.cfg.Location)
	if raw := c.Query("minggu"); raw != "" {
		parsed, perr := time.ParseInLocation("2006-01-02", raw, h.cfg.Location)
		if perr != nil {
			Fail(c, apierror.Validation("Format tanggal tidak valid."))
			return
		}
		monday = mondayOf(parsed, h.cfg.Location)
	}

	d := h.base(c, params, "", "Jadwal menu — Back office", "")
	d.NoIndex = true
	d.Principal = PrincipalOf(c)
	d.ThisWeek = monday.Format("2006-01-02")
	d.PrevWeek = monday.AddDate(0, 0, -7).Format("2006-01-02")
	d.NextWeek = monday.AddDate(0, 0, 7).Format("2006-01-02")
	sunday := monday.AddDate(0, 0, 6)
	d.WeekLabel = shortDate(monday) + " – " + shortDate(sunday) + " " + intToStr(monday.Year())
	d.Flash = c.Query("ok")
	d.FlashKind = "ok"

	if d.MenuWeek, err = h.menuWeek(ctx, monday); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "bo-menu", http.StatusOK, d)
}

// PublishMeal moves a DRAFT meal to PUBLISHED.
func (h *AppHandler) PublishMeal(c *gin.Context) {
	mealID, err := uuid.Parse(c.PostForm("meal_id"))
	if err != nil {
		Fail(c, apierror.Validation("Menu tidak dikenali."))
		return
	}
	week := c.PostForm("minggu")

	// A meal with an incomplete nutrition panel is not published: the customer
	// screen would under-report it, and 01-domain-model.md §5.2b says a meal
	// with a missing component panel is incomplete rather than summed.
	var complete bool
	if err := h.db.QueryRowContext(c.Request.Context(), `
		SELECT count(smi.id) > 0 AND count(smi.id) = count(n.id)
		  FROM scheduled_meal_item smi
		  LEFT JOIN food_nutrition n ON n.food_id = smi.food_id
		 WHERE smi.scheduled_meal_id = $1`, mealID).Scan(&complete); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if !complete {
		c.Redirect(http.StatusSeeOther, "/bo/menu?minggu="+week+"&ok="+
			urlSafe("Menu belum bisa terbit: gizi salah satu komponen belum lengkap."))
		return
	}

	res, err := h.db.ExecContext(c.Request.Context(), `
		UPDATE scheduled_meal SET status = 'PUBLISHED', published_at = now()
		 WHERE id = $1 AND status = 'DRAFT'`, mealID)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	// Assert it moved. A conditional UPDATE that matched nothing means it was
	// already published or the id was wrong, and reporting success either way
	// would be a lie.
	n, err := res.RowsAffected()
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	msg := "Menu diterbitkan."
	if n == 0 {
		msg = "Menu itu sudah terbit."
	}
	c.Redirect(http.StatusSeeOther, "/bo/menu?minggu="+week+"&ok="+urlSafe(msg))
}

func (h *AppHandler) menuWeek(ctx context.Context, monday time.Time) ([]MenuDay, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT sm.service_date, to_char(sl.slot_time, 'HH24.MI'), sl.alias, sl.sort_order,
		       sm.id, sm.name, dt.name, sm.status,
		       count(smi.id)::int,
		       COALESCE(sum(n.calories_kcal), 0)::int,
		       count(smi.id) > 0 AND count(smi.id) = count(n.id),
		       COALESCE(max(kc.reserved_portions), 0)::int,
		       COALESCE(max(kc.max_portions), 0)::int
		  FROM scheduled_meal sm
		  JOIN diet_type dt          ON dt.id = sm.diet_type_id
		  JOIN delivery_time_slot sl ON sl.id = sm.slot_id
		  LEFT JOIN scheduled_meal_item smi ON smi.scheduled_meal_id = sm.id
		  LEFT JOIN food_nutrition n        ON n.food_id = smi.food_id
		  LEFT JOIN kitchen_capacity kc     ON kc.service_date = sm.service_date AND kc.slot_id = sm.slot_id
		 WHERE sm.service_date >= $1::date AND sm.service_date < ($1::date + 7)
		 GROUP BY sm.service_date, sl.slot_time, sl.alias, sl.sort_order,
		          sm.id, sm.name, dt.name, dt.sort_order, sm.status
		 ORDER BY sm.service_date, sl.sort_order, dt.sort_order`,
		monday.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := make([]MenuDay, 7)
	for i := range days {
		d := monday.AddDate(0, 0, i)
		days[i] = MenuDay{
			Date: d, Weekday: weekdayID[d.Weekday()],
			DayMonth: intToStr(d.Day()) + " " + monthShortID[d.Month()],
		}
	}

	for rows.Next() {
		var date time.Time
		var slot, alias string
		var sortOrder int
		var m MenuMeal
		if err := rows.Scan(&date, &slot, &alias, &sortOrder, &m.ID, &m.Name,
			&m.DietName, &m.Status, &m.Components, &m.Kcal, &m.Complete,
			&m.Used, &m.Quota); err != nil {
			return nil, err
		}
		m.Full = m.Quota > 0 && m.Used >= m.Quota
		idx := int(date.In(h.cfg.Location).Sub(monday).Hours() / 24)
		if idx < 0 || idx > 6 {
			continue
		}
		day := &days[idx]
		var target *MenuSlot
		for i := range day.Slots {
			if day.Slots[i].SlotLabel == slot {
				target = &day.Slots[i]
			}
		}
		if target == nil {
			day.Slots = append(day.Slots, MenuSlot{SlotLabel: slot, SlotAlias: alias})
			target = &day.Slots[len(day.Slots)-1]
		}
		target.Meals = append(target.Meals, m)
	}
	return days, rows.Err()
}

// actorFor maps a principal to the domain's actor, so the state machine gets
// the right permission level rather than a hard-coded one.
func actorFor(p *auth.Principal) order.Actor {
	if p == nil {
		return order.ActorSystem
	}
	for _, r := range p.Roles {
		if r == "admin" {
			return order.ActorAdmin
		}
	}
	if p.IsStaff {
		return order.ActorStaff
	}
	return order.ActorCustomer
}

func mondayOf(t time.Time, loc *time.Location) time.Time {
	d := t.In(loc)
	offset := (int(d.Weekday()) + 6) % 7
	return time.Date(d.Year(), d.Month(), d.Day()-offset, 0, 0, 0, 0, loc)
}

func shortDate(t time.Time) string {
	return intToStr(t.Day()) + " " + monthShortID[t.Month()]
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func plural(n int, noun string) string { return intToStr(n) + " " + noun }

func minutesLabel(m int) string {
	if m < 60 {
		return intToStr(m) + " menit"
	}
	return intToStr(m/60) + " jam " + intToStr(m%60) + " menit"
}

// urlSafe percent-encodes a flash message for a redirect query string, so a
// message containing an ampersand cannot forge extra parameters.
func urlSafe(s string) string { return url.QueryEscape(s) }

var _ = sql.ErrNoRows
var _ = money.IDR(0)
