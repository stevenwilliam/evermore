// Package nutrition aggregates a meal's nutrition panel from the foods that
// compose it (D-33, 01-domain-model.md §5.2b).
//
// A meal is what is sold; its panel is the sum of its components. Every field
// is an integer in its base unit (kcal, mg, ml) so the sum is exact.
package nutrition

// Panel is one nutrition panel, per portion. Every quantity is an integer in
// its base unit — CLAUDE.md §4 forbids floating point on money, and the same
// exactness is wanted here so that four components sum without drift.
type Panel struct {
	CaloriesKcal   int            `json:"calories_kcal"`
	ProteinMG      int            `json:"protein_mg"`
	FatMG          int            `json:"fat_mg"`
	SaturatedFatMG int            `json:"saturated_fat_mg"`
	CarbohydrateMG int            `json:"carbohydrate_mg"`
	SugarMG        int            `json:"sugar_mg"`
	FibreMG        int            `json:"fibre_mg"`
	SodiumMG       int            `json:"sodium_mg"`
	CholesterolMG  int            `json:"cholesterol_mg"`
	Extras         map[string]int `json:"extras,omitempty"`

	// Complete is false when at least one component had no panel. The meal
	// then reports an incomplete panel rather than silently under-reporting
	// the totals — §5.2b names this explicitly.
	Complete bool `json:"complete"`

	// MissingFoods names the components that had no panel, so the back office
	// can say which one to fill in rather than just refusing.
	MissingFoods []string `json:"missing_foods,omitempty"`
}

// Component is one food in a meal, with its panel if it has one.
type Component struct {
	FoodID   string
	FoodName string
	Panel    *Panel // nil when the food has no nutrition row
}

// Aggregate sums the components into the meal's panel.
//
// A missing component panel marks the result incomplete. Numeric Extras keys
// are summed by key; a key present on one component and absent on another is
// summed as if the absent one contributed zero, which is the only reading that
// keeps the sum associative.
func Aggregate(components []Component) Panel {
	out := Panel{Complete: true}
	if len(components) == 0 {
		// An empty meal is not "complete" in any useful sense; saying so keeps
		// a meal with no items out of the published calendar.
		out.Complete = false
		return out
	}
	for _, c := range components {
		if c.Panel == nil {
			out.Complete = false
			out.MissingFoods = append(out.MissingFoods, c.FoodName)
			continue
		}
		p := c.Panel
		out.CaloriesKcal += p.CaloriesKcal
		out.ProteinMG += p.ProteinMG
		out.FatMG += p.FatMG
		out.SaturatedFatMG += p.SaturatedFatMG
		out.CarbohydrateMG += p.CarbohydrateMG
		out.SugarMG += p.SugarMG
		out.FibreMG += p.FibreMG
		out.SodiumMG += p.SodiumMG
		out.CholesterolMG += p.CholesterolMG
		for k, v := range p.Extras {
			if out.Extras == nil {
				out.Extras = make(map[string]int, len(p.Extras))
			}
			out.Extras[k] += v
		}
	}
	return out
}
