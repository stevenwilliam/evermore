// Package postgres holds the repositories. Money paths use explicit SQL with
// placeholders and integer arithmetic — CLAUDE.md §3 forbids the ORM there,
// and in practice every query in this package is hand-written SQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/domain/nutrition"
)

// PublicRepo serves the anonymous, SEO-indexed surface.
type PublicRepo struct{ db *sql.DB }

func NewPublicRepo(db *sql.DB) *PublicRepo { return &PublicRepo{db: db} }

// DietType is a diet as the public site shows it.
type DietType struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
}

// Slot is a delivery time slot.
type Slot struct {
	ID     uuid.UUID
	Time   string // HH.MM, Indonesian style
	Alias  string
	Period string
}

// MealCard is a published meal as a listing shows it.
type MealCard struct {
	ID          uuid.UUID
	Name        string
	ServiceDate time.Time
	DietSlug    string
	DietName    string
	SlotTime    string
	SlotAlias   string
	Components  int
	Panel       nutrition.Panel
	PriceIDR    money.IDR
	Allergens   []string
	SoldOut     bool
}

// Kitchen is a kitchen as the coverage page shows it.
type Kitchen struct {
	Code            string
	Name            string
	AddressLine     string
	Lat, Lng        float64
	ServiceRadiusKM float64
	Priority        int
}

// PackageCard is a credit package.
type PackageCard struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	MealCredits  int
	ValidityDays int
	PriceIDR     money.IDR
	PerMealIDR   money.IDR
	IsFeatured   bool
}

// DietTypes lists the active diets in display order.
func (r *PublicRepo) DietTypes(ctx context.Context) ([]DietType, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, slug, description
		  FROM diet_type
		 WHERE is_active
		 ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DietType
	for rows.Next() {
		var d DietType
		if err := rows.Scan(&d.ID, &d.Name, &d.Slug, &d.Description); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A scan into a column that does not exist leaves the zero value rather
	// than erroring, so assert the query actually returned something usable.
	if len(out) > 0 && out[0].Slug == "" {
		return nil, errors.New("postgres: diet_type rows scanned with an empty slug")
	}
	return out, nil
}

// ServiceDates returns the dates that have at least one published meal,
// from today forward, within the horizon.
func (r *PublicRepo) ServiceDates(ctx context.Context, from time.Time, horizonDays int) ([]time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT service_date
		  FROM scheduled_meal
		 WHERE status = 'PUBLISHED'
		   AND service_date >= $1::date
		   AND service_date < ($1::date + $2::int)
		 ORDER BY service_date`,
		from.Format("2006-01-02"), horizonDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// mealQuery is shared by the listing and the detail page. The nutrition panel
// is aggregated in SQL so a listing of twelve meals is one round trip rather
// than thirteen; the pure aggregator in domain/nutrition remains the authority
// for the detail page and for anything that has to report completeness.
const mealSelect = `
SELECT sm.id,
       sm.name,
       sm.service_date,
       dt.slug,
       dt.name,
       to_char(sl.slot_time, 'HH24.MI'),
       sl.alias,
       count(smi.id)::int                                AS components,
       COALESCE(sum(n.calories_kcal), 0)::int            AS kcal,
       COALESCE(sum(n.protein_mg), 0)::int               AS protein_mg,
       COALESCE(sum(n.fat_mg), 0)::int                   AS fat_mg,
       COALESCE(sum(n.carbohydrate_mg), 0)::int          AS carb_mg,
       COALESCE(sum(n.fibre_mg), 0)::int                 AS fibre_mg,
       COALESCE(sum(n.sodium_mg), 0)::int                AS sodium_mg,
       count(smi.id) = count(n.id)                       AS panel_complete,
       COALESCE(sm.qty_capacity, 0)::int                 AS capacity
  FROM scheduled_meal sm
  JOIN diet_type dt          ON dt.id = sm.diet_type_id
  JOIN delivery_time_slot sl ON sl.id = sm.slot_id
  LEFT JOIN scheduled_meal_item smi ON smi.scheduled_meal_id = sm.id
  LEFT JOIN food_nutrition n        ON n.food_id = smi.food_id
 WHERE sm.status = 'PUBLISHED'`

// PublishedMeals lists published meals for a date, optionally filtered by
// diet slug and by a free-text search over the meal and component names.
//
// The search box is on every list (CLAUDE.md §7), so the query supports it
// directly rather than filtering in Go after fetching everything.
func (r *PublicRepo) PublishedMeals(ctx context.Context, date time.Time, dietSlug, search string) ([]MealCard, error) {
	q := mealSelect + `
   AND sm.service_date = $1::date
   AND ($2 = '' OR dt.slug = $2)
   AND ($3 = '' OR sm.id IN (
         SELECT sm2.id FROM scheduled_meal sm2
         LEFT JOIN scheduled_meal_item i2 ON i2.scheduled_meal_id = sm2.id
         LEFT JOIN food f2 ON f2.id = i2.food_id
         WHERE sm2.id = sm.id
           AND (sm2.name ILIKE '%' || $3 || '%' OR f2.name ILIKE '%' || $3 || '%')))
 GROUP BY sm.id, sm.name, sm.service_date, dt.slug, dt.name, sl.slot_time, sl.alias, sl.sort_order
 ORDER BY sl.sort_order, sm.name`

	rows, err := r.db.QueryContext(ctx, q, date.Format("2006-01-02"), dietSlug, search)
	if err != nil {
		return nil, fmt.Errorf("postgres: published meals: %w", err)
	}
	defer rows.Close()
	return scanMealCards(rows)
}

func scanMealCards(rows *sql.Rows) ([]MealCard, error) {
	var out []MealCard
	for rows.Next() {
		var m MealCard
		var capacity int
		if err := rows.Scan(
			&m.ID, &m.Name, &m.ServiceDate, &m.DietSlug, &m.DietName,
			&m.SlotTime, &m.SlotAlias, &m.Components,
			&m.Panel.CaloriesKcal, &m.Panel.ProteinMG, &m.Panel.FatMG,
			&m.Panel.CarbohydrateMG, &m.Panel.FibreMG, &m.Panel.SodiumMG,
			&m.Panel.Complete, &capacity,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MealDetail is one meal with its components, for the detail page.
type MealDetail struct {
	MealCard
	Description string
	Items       []MealItem
	Allergens   []string
}

// MealItem is one component of a meal.
type MealItem struct {
	FoodName    string
	Role        string
	PortionSize string
	Panel       *nutrition.Panel
}

// Meal loads one published meal by id.
func (r *PublicRepo) Meal(ctx context.Context, mealID uuid.UUID) (*MealDetail, error) {
	q := mealSelect + `
   AND sm.id = $1
 GROUP BY sm.id, sm.name, sm.service_date, dt.slug, dt.name, sl.slot_time, sl.alias`

	rows, err := r.db.QueryContext(ctx, q, mealID)
	if err != nil {
		return nil, err
	}
	cards, err := scanMealCards(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, sql.ErrNoRows
	}
	d := &MealDetail{MealCard: cards[0]}

	itemRows, err := r.db.QueryContext(ctx, `
		SELECT f.name, smi.item_role, f.portion_size,
		       n.calories_kcal, n.protein_mg, n.fat_mg, n.saturated_fat_mg,
		       n.carbohydrate_mg, n.sugar_mg, n.fibre_mg, n.sodium_mg, n.cholesterol_mg
		  FROM scheduled_meal_item smi
		  JOIN food f ON f.id = smi.food_id
		  LEFT JOIN food_nutrition n ON n.food_id = f.id
		 WHERE smi.scheduled_meal_id = $1
		 ORDER BY smi.sort_order`, mealID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	// Rebuild the panel through the pure aggregator so the detail page reports
	// completeness the way domain/nutrition defines it, rather than trusting
	// the SQL sum to have noticed a missing component.
	var components []nutrition.Component
	for itemRows.Next() {
		var it MealItem
		var kcal, protein, fat, satFat, carb, sugar, fibre, sodium, chol sql.NullInt64
		if err := itemRows.Scan(&it.FoodName, &it.Role, &it.PortionSize,
			&kcal, &protein, &fat, &satFat, &carb, &sugar, &fibre, &sodium, &chol); err != nil {
			return nil, err
		}
		if kcal.Valid {
			it.Panel = &nutrition.Panel{
				CaloriesKcal: int(kcal.Int64), ProteinMG: int(protein.Int64),
				FatMG: int(fat.Int64), SaturatedFatMG: int(satFat.Int64),
				CarbohydrateMG: int(carb.Int64), SugarMG: int(sugar.Int64),
				FibreMG: int(fibre.Int64), SodiumMG: int(sodium.Int64),
				CholesterolMG: int(chol.Int64),
			}
		}
		components = append(components, nutrition.Component{FoodName: it.FoodName, Panel: it.Panel})
		d.Items = append(d.Items, it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}
	d.Panel = nutrition.Aggregate(components)

	allergenRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT a.name_id
		  FROM scheduled_meal_item smi
		  JOIN food_allergen fa ON fa.food_id = smi.food_id
		  JOIN allergen a       ON a.id = fa.allergen_id
		 WHERE smi.scheduled_meal_id = $1
		 ORDER BY a.name_id`, mealID)
	if err != nil {
		return nil, err
	}
	defer allergenRows.Close()
	for allergenRows.Next() {
		var name string
		if err := allergenRows.Scan(&name); err != nil {
			return nil, err
		}
		d.Allergens = append(d.Allergens, name)
	}
	return d, allergenRows.Err()
}

// SinglePortionPrice returns the DEFAULT-scope unit price a customer buying
// ONE portion actually pays: the row on the tier that contains qty 1.
//
// Not min() across the tiers. The cheapest tier is the 10+ band, and showing
// Rp 71.000 on a card next to an "Add" button that charges Rp 78.000 for one
// portion is a price the customer will not be offered. The artifact shows
// Rp 78.000 on the menu cards for exactly this reason.
func (r *PublicRepo) SinglePortionPrice(ctx context.Context, dietSlug string, on time.Time) (money.IDR, error) {
	var v sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT p.unit_price_idr
		  FROM meal_price_normal p
		  JOIN diet_type d       ON d.id = p.diet_type_id
		  JOIN meal_price_tier t ON t.id = p.tier_id
		 WHERE p.is_active AND t.is_active
		   AND p.scope_key = 'DEFAULT'
		   AND d.slug = $1
		   AND p.validity @> $2::date
		   AND t.min_qty <= 1
		   AND (t.max_qty IS NULL OR t.max_qty >= 1)
		 LIMIT 1`, dietSlug, on.Format("2006-01-02")).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !v.Valid {
		// No configured price. The caller shows nothing rather than a zero,
		// because "Rp 0" is worse than an absent price.
		return 0, nil
	}
	return money.IDR(v.Int64), nil
}

// TierPrices returns the configured tier ladder for a diet, which the cart
// and the marketing pages both show.
type TierPrice struct {
	Label    string
	MinQty   int
	MaxQty   *int
	PriceIDR money.IDR
}

func (r *PublicRepo) TierPrices(ctx context.Context, dietSlug string, on time.Time) ([]TierPrice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.name, t.min_qty, t.max_qty, p.unit_price_idr
		  FROM meal_price_normal p
		  JOIN meal_price_tier t ON t.id = p.tier_id
		  JOIN diet_type d       ON d.id = p.diet_type_id
		 WHERE p.is_active AND t.is_active
		   AND p.scope_key = 'DEFAULT'
		   AND d.slug = $1
		   AND p.validity @> $2::date
		 ORDER BY t.min_qty`, dietSlug, on.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TierPrice
	for rows.Next() {
		var t TierPrice
		var max sql.NullInt64
		var price int64
		if err := rows.Scan(&t.Label, &t.MinQty, &max, &price); err != nil {
			return nil, err
		}
		if max.Valid {
			m := int(max.Int64)
			t.MaxQty = &m
		}
		t.PriceIDR = money.IDR(price)
		out = append(out, t)
	}
	return out, rows.Err()
}

// Packages lists the credit packages with their prices.
func (r *PublicRepo) Packages(ctx context.Context, on time.Time) ([]PackageCard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pk.id, pk.name, pk.slug, pk.meal_credits, pk.validity_days,
		       pp.total_price_idr, pk.is_featured
		  FROM package pk
		  JOIN package_price_normal pp ON pp.package_id = pk.id
		 WHERE pk.is_active AND pp.is_active
		   AND pp.scope_key = 'DEFAULT'
		   AND pp.validity @> $1::date
		 ORDER BY pk.sort_order`, on.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PackageCard
	for rows.Next() {
		var p PackageCard
		var price int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.MealCredits, &p.ValidityDays, &price, &p.IsFeatured); err != nil {
			return nil, err
		}
		p.PriceIDR = money.IDR(price)
		if p.MealCredits > 0 {
			// Integer division: the per-meal figure is displayed, never used
			// to compute a charge, so truncation here cannot cost anyone money.
			p.PerMealIDR = money.IDR(price / int64(p.MealCredits))
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Kitchens lists the active kitchens for the coverage page.
func (r *PublicRepo) Kitchens(ctx context.Context) ([]Kitchen, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT code, name, address_line, latitude::float8, longitude::float8,
		       service_radius_km::float8, priority
		  FROM kitchen
		 WHERE is_active
		 ORDER BY priority, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Kitchen
	for rows.Next() {
		var k Kitchen
		if err := rows.Scan(&k.Code, &k.Name, &k.AddressLine, &k.Lat, &k.Lng,
			&k.ServiceRadiusKM, &k.Priority); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Params loads sys_parameters as a map, masking secret-flagged values so a
// template cannot accidentally render one (CLAUDE.md §7).
func (r *PublicRepo) Params(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT key, value, is_secret FROM sys_parameters WHERE is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		var secret bool
		if err := rows.Scan(&k, &v, &secret); err != nil {
			return nil, err
		}
		if secret {
			v = "••••••"
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A missing catalogue key renders as the key, so prove the load worked
	// rather than discovering it on a page that says "company.name".
	if len(out) == 0 {
		return nil, errors.New("postgres: sys_parameters returned no rows")
	}
	return out, nil
}
