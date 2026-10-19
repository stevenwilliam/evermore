package http

import (
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/evermore/internal/adapter/postgres"
	"github.com/stevenwilliam/evermore/internal/app/ordering"
	"github.com/stevenwilliam/evermore/internal/app/payments"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/web"
)

// TestEveryTemplateRenders executes every page with EVERY SLICE POPULATED.
//
// html/template resolves field names and function arity at execution time, not
// at parse time, so `newRenderer` happily accepts a template that cannot run.
// Worse, a `{{range}}` over an empty slice never executes its body — which is
// exactly how a broken `tierRange` call sat in app-cart.html unnoticed: the
// tier table only renders once there is something in the cart (D85).
//
// So every slice here carries at least one element. An empty fixture would
// pass while the page is still broken for the first customer who uses it.
func TestEveryTemplateRenders(t *testing.T) {
	cfg := &config.Config{BaseURL: "http://test.local", Location: time.UTC}

	pages := allPages()
	r, err := newRenderer(cfg, web.Templates, pages)
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}

	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			tpl, ok := r.tpl[page]
			if !ok {
				t.Fatalf("template %q was not parsed", page)
			}
			if err := tpl.ExecuteTemplate(io.Discard, "base", fullPageData()); err != nil {
				t.Fatalf("%s: %v", page, err)
			}
		})
	}
}

// TestEveryTemplateRendersWithNothingToList is the other half: a page must
// also survive a quiet day. Every slice is nil, but the pointers and
// parameters a handler always sets are still there — an all-zero fixture would
// only prove that templates fail without their handler, which is not news.
func TestEveryTemplateRendersWithNothingToList(t *testing.T) {
	cfg := &config.Config{BaseURL: "http://test.local", Location: time.UTC}
	r, err := newRenderer(cfg, web.Templates, allPages())
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	for _, page := range allPages() {
		t.Run(page, func(t *testing.T) {
			d := fullPageData()
			d.Diets, d.Kitchens, d.Meals, d.Week = nil, nil, nil, nil
			d.Packages, d.Tiers, d.Dates, d.Certs = nil, nil, nil, nil
			d.Paragraphs, d.Alternates = nil, nil
			if err := r.tpl[page].ExecuteTemplate(io.Discard, "base", d); err != nil {
				t.Fatalf("%s with nothing to list: %v", page, err)
			}
		})
	}
}

// allPages is every template the binary parses, in one place, so a page added
// to a handler but forgotten here is obvious.
func allPages() []string {
	pages := append([]string{
		"home", "menu", "meal", "packages", "coverage", "how", "corporate",
		"legal", "error",
	}, appPages...)
	pages = append(pages, "bo-menu")
	// appPages already names the bo screens, so dedupe rather than parse twice.
	seen := map[string]bool{}
	out := pages[:0]
	for _, p := range pages {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func fullPageData() pageData {
	id := uuid.New()
	now := time.Date(2026, 10, 19, 11, 30, 0, 0, time.UTC)
	max := 9

	meal := postgres.MealCard{
		ID: id, Name: "Test meal", ServiceDate: now, DietSlug: "balanced",
		DietName: "Balanced", SlotTime: "11.30", SlotAlias: "Lunch",
		SlotPeriod: "lunch", Components: 4, PriceIDR: 78000,
	}

	return pageData{
		L: "en", Lang: "en", Title: "t", Description: "d",
		CanonicalURL: "http://test.local/", BaseURL: "http://test.local",
		SiteName: "Evermore", Public: true, SignedIn: true,
		Year:       2026,
		Params:     map[string]string{"company_phone": "0811-1055-1734", "company_whatsapp": "6281110551734"},
		Alternates: []alternate{{Lang: "en", URL: "http://test.local/?lang=en"}},
		Heading:    "h", Message: "m", Code: "404", TraceID: "t",
		Paragraphs: []string{"one", "two"},
		CutOff:     "15:00",
		Diets:      []postgres.DietType{{Slug: "balanced", Name: "Balanced"}},
		Kitchens:   []postgres.Kitchen{{Code: "KTC-01", Name: "Dapur Tebet", AddressLine: "Jl."}},
		Meals:      []postgres.MealCard{meal},
		Week:       []postgres.DayMenu{{Date: now, IsToday: true, Meals: []postgres.MealCard{meal}, Lunch: &meal, Dinner: &meal}},
		Packages:   []postgres.PackageCard{{ID: id, Name: "Basic", Slug: "basic", MealCredits: 10, ValidityDays: 60, PriceIDR: 750000, PerMealIDR: 75000, IsFeatured: true, FreeDelivery: true, FreeAllergenCustom: true, DieticianSessions: 2, DailyFruit: true}},
		Tiers:      []postgres.TierPrice{{MinQty: 1, MaxQty: &max, PriceIDR: 78000}},
		Dates:      []dayLink{{URL: "/menu?date=2026-10-19", Weekday: "Mon", DayMonth: "19 Oct", Selected: true}},
		Certs:      []certBadge{{Key: "halal", Image: "/images/certs/seal-halal.svg", Label: "Halal", Note: "ID0001"}},

		// The pointer fields a handler always sets. Without them four pages
		// cannot render at all, and leaving them nil would only prove that a
		// template needs its handler.
		Meal: &postgres.MealDetail{MealCard: meal, Description: "d",
			Items:     []postgres.MealItem{{Role: "MAIN", FoodName: "Ayam", PortionSize: "150 g"}},
			Allergens: []string{"telur"}},
		Quote: &ordering.Quote{
			Lines: []ordering.QuoteLine{{ScheduledMealID: id, MealName: "Test meal", ServiceDate: now,
				SlotTime: "11.30", DietName: "Balanced", DietSlug: "balanced", Qty: 2,
				UnitPrice: 78000, NormalPrice: 78000, LineTotal: 156000}},
			TotalQty: 2, Subtotal: 156000, DeliveryFee: 0, Total: 156000,
			TaxRateBPS: 1100, Deliveries: 1,
		},
		Credits: &ordering.CreditQuote{
			Lines: []ordering.CreditLine{{ScheduledMealID: id, MealName: "Test meal",
				ServiceDate: now, SlotTime: "11.30", SlotPeriod: "lunch",
				DietName: "Balanced", Qty: 2}},
			Meals: 2, Balance: 12,
		},
		Payment: &payments.Instructions{OrderID: id, OrderNumber: "EVM-1", Status: "AWAITING_PAYMENT",
			BankName: "BCA", AccountNumber: "123", AccountHolder: "PT Evermore", AmountIDR: 156000},
		Dash: &DashboardView{DateLabel: "19 Oct 2026", MealsToday: 3, Deliveries: 4,
			Capacity: []CapacityRow{{Name: "Dapur Tebet", Code: "KTC-01",
				Cells: []CapacityCell{{SlotLabel: "11.30"}}}}},
	}
}
