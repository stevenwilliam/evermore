package ordering

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/order"
	"github.com/stevenwilliam/evermore/internal/domain/pricing"
	"github.com/stevenwilliam/evermore/internal/domain/routing"
	"github.com/stevenwilliam/evermore/internal/platform/id"
)

// today is the current business date in the operating timezone. Business-day
// logic converts explicitly and never uses the server's zone (CLAUDE.md §4).
func (s *Service) today() time.Time {
	n := s.now().In(s.loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, s.loc)
}

func (s *Service) params(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM sys_parameters WHERE is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ordering: sys_parameters is empty")
	}
	return out, nil
}

func paramInt(p map[string]string, key string, def int) int {
	v, ok := p[key]
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (s *Service) customerTypeOf(ctx context.Context, customerID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx,
		`SELECT customer_type_id FROM customer WHERE id = $1`, customerID).Scan(&id)
	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("ordering: customer %s not found", customerID)
	}
	return id, err
}

func (s *Service) tiers(ctx context.Context) ([]pricing.Tier, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, min_qty, max_qty FROM meal_price_tier WHERE is_active ORDER BY min_qty`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pricing.Tier
	for rows.Next() {
		var t pricing.Tier
		var max sql.NullInt64
		if err := rows.Scan(&t.ID, &t.MinQty, &max); err != nil {
			return nil, err
		}
		if max.Valid {
			m := int(max.Int64)
			t.MaxQty = &m
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// priceRows loads every candidate row for a diet, from BOTH the normal and the
// promo table, so the resolver can apply promo-over-normal within a scope.
func (s *Service) priceRows(ctx context.Context, dietTypeID uuid.UUID, on time.Time) ([]pricing.Row, error) {
	q := `
		SELECT id, scope_key, diet_type_id, tier_id, unit_price_idr,
		       lower(validity), upper(validity), '' AS promo_label, is_active, 'meal_normal' AS tbl
		  FROM meal_price_normal
		 WHERE is_active AND diet_type_id = $1
		UNION ALL
		SELECT id, scope_key, diet_type_id, tier_id, unit_price_idr,
		       lower(validity), upper(validity), promo_label, is_active, 'meal_promo' AS tbl
		  FROM meal_price_promo
		 WHERE is_active AND diet_type_id = $1`
	rows, err := s.db.QueryContext(ctx, q, dietTypeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []pricing.Row
	for rows.Next() {
		var r pricing.Row
		var lower time.Time
		var upper sql.NullTime
		var price int64
		var tbl string
		if err := rows.Scan(&r.ID, &r.ScopeKey, &r.DietTypeID, &r.TierID, &price,
			&lower, &upper, &r.PromoLabel, &r.IsActive, &tbl); err != nil {
			return nil, err
		}
		r.UnitPriceIDR = money.IDR(price)
		// daterange bounds come back as UTC midnights; the resolver compares
		// them against a date in the operating zone, so both are normalised
		// to a bare calendar date here.
		r.ValidFrom = dateOnly(lower, s.loc)
		if upper.Valid {
			u := dateOnly(upper.Time, s.loc)
			r.ValidTo = &u
		}
		r.Table = pricing.Table(tbl)
		out = append(out, r)
	}
	return out, rows.Err()
}

func dateOnly(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// mealRow is a published meal, with what checkout needs to price and route it.
type mealRow struct {
	ID          uuid.UUID
	Name        string
	ServiceDate time.Time
	DietTypeID  uuid.UUID
	DietName    string
	DietSlug    string
	SlotID      uuid.UUID
	SlotTime    string
}

// mealForSale loads a meal and refuses one that is not published. A DRAFT meal
// is not on sale, and letting a client checkout against one by id would sell
// something the kitchen has not committed to.
func (s *Service) mealForSale(ctx context.Context, mealID uuid.UUID) (*mealRow, error) {
	var m mealRow
	err := s.db.QueryRowContext(ctx, `
		SELECT sm.id, sm.name, sm.service_date, sm.diet_type_id, dt.name, dt.slug,
		       sm.slot_id, to_char(sl.slot_time, 'HH24.MI')
		  FROM scheduled_meal sm
		  JOIN diet_type dt          ON dt.id = sm.diet_type_id
		  JOIN delivery_time_slot sl ON sl.id = sm.slot_id
		 WHERE sm.id = $1 AND sm.status = 'PUBLISHED'`, mealID).
		Scan(&m.ID, &m.Name, &m.ServiceDate, &m.DietTypeID, &m.DietName, &m.DietSlug,
			&m.SlotID, &m.SlotTime)
	if err == sql.ErrNoRows {
		return nil, ErrMealNotPublished
	}
	if err != nil {
		return nil, err
	}
	m.ServiceDate = dateOnly(m.ServiceDate, s.loc)
	return &m, nil
}

// mealSnapshot captures every food in the meal at the moment of sale, so a
// later recipe edit cannot rewrite what a customer bought.
func (s *Service) mealSnapshot(ctx context.Context, mealID uuid.UUID) (map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.name, smi.item_role, f.portion_size,
		       COALESCE(n.calories_kcal,0), COALESCE(n.protein_mg,0),
		       COALESCE(n.carbohydrate_mg,0), COALESCE(n.fat_mg,0),
		       COALESCE(n.fibre_mg,0), COALESCE(n.sodium_mg,0)
		  FROM scheduled_meal_item smi
		  JOIN food f ON f.id = smi.food_id
		  LEFT JOIN food_nutrition n ON n.food_id = f.id
		 WHERE smi.scheduled_meal_id = $1
		 ORDER BY smi.sort_order`, mealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]any
	total := map[string]int{}
	for rows.Next() {
		var name, role, portion string
		var kcal, protein, carb, fat, fibre, sodium int
		if err := rows.Scan(&name, &role, &portion, &kcal, &protein, &carb, &fat, &fibre, &sodium); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"name": name, "role": role, "portion": portion,
			"calories_kcal": kcal, "protein_mg": protein,
		})
		total["calories_kcal"] += kcal
		total["protein_mg"] += protein
		total["carbohydrate_mg"] += carb
		total["fat_mg"] += fat
		total["fibre_mg"] += fibre
		total["sodium_mg"] += sodium
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var allergens []string
	aRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.name_id FROM scheduled_meal_item smi
		  JOIN food_allergen fa ON fa.food_id = smi.food_id
		  JOIN allergen a ON a.id = fa.allergen_id
		 WHERE smi.scheduled_meal_id = $1 ORDER BY a.name_id`, mealID)
	if err != nil {
		return nil, err
	}
	defer aRows.Close()
	for aRows.Next() {
		var n string
		if err := aRows.Scan(&n); err != nil {
			return nil, err
		}
		allergens = append(allergens, n)
	}
	return map[string]any{
		"items": items, "nutrition": total, "allergens": allergens,
		"captured_at": s.now().UTC().Format(time.RFC3339),
	}, aRows.Err()
}

// addressRow is a delivery address with what routing needs.
type addressRow struct {
	ID        uuid.UUID
	Line      string
	District  string
	Recipient string
	Phone     string
	Note      string
	Lat, Lng  float64
}

func (a *addressRow) snapshotJSON() []byte {
	b, _ := json.Marshal(map[string]any{
		"address_line": a.Line, "district": a.District,
		"recipient_name": a.Recipient, "recipient_phone": a.Phone,
		"driver_note": a.Note, "latitude": a.Lat, "longitude": a.Lng,
	})
	return b
}

// address loads an address scoped by owner. The customer id comes from the
// token, so asking for someone else's address id returns not-found rather
// than their address (IDOR).
func (s *Service) address(ctx context.Context, customerID, addressID uuid.UUID) (*addressRow, error) {
	var a addressRow
	err := s.db.QueryRowContext(ctx, `
		SELECT id, address_line, district, recipient_name, recipient_phone,
		       driver_note, latitude::float8, longitude::float8
		  FROM customer_address
		 WHERE id = $1 AND customer_id = $2 AND is_active`, addressID, customerID).
		Scan(&a.ID, &a.Line, &a.District, &a.Recipient, &a.Phone, &a.Note, &a.Lat, &a.Lng)
	if err == sql.ErrNoRows {
		return nil, ErrAddressNotOwned
	}
	return &a, err
}

// candidates asks the database which kitchens could serve this point, letting
// PostGIS do the geometry. The polygon-over-radius decision is NOT made here:
// the raw verdicts are handed to routing.Route, which owns that rule and is
// unit-tested on it.
func (s *Service) candidates(ctx context.Context, tx *sql.Tx, addr *addressRow, l QuoteLine) ([]routing.Candidate, error) {
	pt := fmt.Sprintf("SRID=4326;POINT(%f %f)", addr.Lng, addr.Lat)
	weekday := int(l.ServiceDate.In(s.loc).Weekday())
	if weekday == 0 {
		weekday = 7 // ISO-8601: Sunday is 7
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT k.id, k.code, k.priority,
		       ST_Distance(k.geom, $1::geography)::int              AS distance_m,
		       (k.service_area IS NOT NULL)                          AS has_polygon,
		       COALESCE(ST_Covers(k.service_area, $1::geography), false) AS inside_polygon,
		       ST_DWithin(k.geom, $1::geography, k.service_radius_km * 1000) AS inside_radius,
		       k.is_active,
		       EXISTS (SELECT 1 FROM kitchen_slot ks
		                WHERE ks.kitchen_id = k.id AND ks.slot_id = $2 AND ks.is_active) AS serves_slot,
		       COALESCE((SELECT kod.is_open FROM kitchen_operating_day kod
		                  WHERE kod.kitchen_id = k.id AND kod.weekday = $3), false)      AS open_that_day,
		       COALESCE((SELECT kc.max_portions - kc.reserved_portions
		                   FROM kitchen_capacity kc
		                  WHERE kc.kitchen_id = k.id AND kc.service_date = $4::date
		                    AND kc.slot_id = $2), 0)                                     AS free_portions
		  FROM kitchen k
		 ORDER BY k.priority, k.code`,
		pt, l.slotID, weekday, l.ServiceDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []routing.Candidate
	for rows.Next() {
		var c routing.Candidate
		if err := rows.Scan(&c.KitchenID, &c.Code, &c.Priority, &c.DistanceM,
			&c.HasPolygon, &c.InsidePolygon, &c.InsideRadius, &c.IsActive,
			&c.ServesSlot, &c.OpenThatDay, &c.FreePortions); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// deliveryFee evaluates the band engine. Every band is currently zero (D14),
// but the engine runs so that charging later is a settings edit.
func (s *Service) deliveryFee(ctx context.Context) (money.IDR, error) {
	var fee sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT fee_idr FROM delivery_fee_band
		 WHERE is_active AND min_distance_m <= 0
		 ORDER BY min_distance_m LIMIT 1`).Scan(&fee)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !fee.Valid {
		return 0, nil
	}
	return money.IDR(fee.Int64), nil
}

// beforeCutOff reports whether an order for serviceDate may still be placed.
//
// The rule is "before HH:MM the day before", evaluated in the operating
// timezone. A same-day order is refused outright: the kitchen snapshot is
// taken at cut-off and same-day capacity is already committed.
func (s *Service) beforeCutOff(serviceDate time.Time, cutOff string) (bool, error) {
	hh, mm, ok := parseHHMM(cutOff)
	if !ok {
		return false, fmt.Errorf("ordering: order.cutoff_time %q is not HH:MM", cutOff)
	}
	sd := dateOnly(serviceDate, s.loc)
	deadline := time.Date(sd.Year(), sd.Month(), sd.Day(), hh, mm, 0, 0, s.loc).AddDate(0, 0, -1)
	return s.now().In(s.loc).Before(deadline), nil
}

func parseHHMM(v string) (h, m int, ok bool) {
	var hh, mm int
	if n, err := fmt.Sscanf(v, "%d:%d", &hh, &mm); n != 2 || err != nil {
		return 0, 0, false
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	return hh, mm, true
}

// claimSuffix takes a free three-digit payment suffix for today and returns
// the amount to transfer.
//
// The unique index on (claim_date, suffix) is what makes this safe under
// concurrency: two checkouts racing for the same suffix means one insert
// fails and retries.
func (s *Service) claimSuffix(ctx context.Context, tx *sql.Tx, orderID uuid.UUID, total money.IDR) (int, money.IDR, money.IDR, error) {
	today := s.today().Format("2006-01-02")
	for attempt := 0; attempt < 40; attempt++ {
		suffix, err := order.RandomSuffix()
		if err != nil {
			return 0, 0, 0, err
		}
		var claimed bool
		err = tx.QueryRowContext(ctx, `
			INSERT INTO payment_suffix_claim (id, order_id, claim_date, suffix)
			VALUES ($1, $2, $3::date, $4)
			ON CONFLICT (claim_date, suffix) WHERE released_at IS NULL DO NOTHING
			RETURNING true`, id.New(), orderID, today, suffix).Scan(&claimed)
		if err == sql.ErrNoRows {
			continue // taken; draw another
		}
		if err != nil {
			return 0, 0, 0, err
		}
		amount, rounding, err := order.PaymentAmount(total, suffix)
		if err != nil {
			return 0, 0, 0, err
		}
		return suffix, amount, rounding, nil
	}
	// 40 collisions out of 1000 slots means the day is genuinely saturated.
	return 0, 0, 0, fmt.Errorf("ordering: no free payment suffix for %s", today)
}

func traceJSON(q *Quote) []byte {
	traces := make([]pricing.Trace, 0, len(q.Lines))
	for _, l := range q.Lines {
		traces = append(traces, l.Trace)
	}
	b, _ := json.Marshal(map[string]any{
		"total_qty":    q.TotalQty,
		"tax_rate_bps": q.TaxRateBPS,
		"lines":        traces,
	})
	return b
}

func snapshotJSON(m map[string]any) []byte {
	b, _ := json.Marshal(m)
	return b
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
