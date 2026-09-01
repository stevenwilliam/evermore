package nutrition

import "testing"

// Matrix from 01-domain-model.md §5.2b.

func panel(kcal, protein int) *Panel {
	return &Panel{CaloriesKcal: kcal, ProteinMG: protein}
}

func TestAggregate_OneFood(t *testing.T) {
	got := Aggregate([]Component{{FoodName: "Ayam", Panel: panel(300, 30000)}})
	if got.CaloriesKcal != 300 || got.ProteinMG != 30000 {
		t.Errorf("got %+v", got)
	}
	if !got.Complete {
		t.Error("a single complete component makes a complete panel")
	}
}

func TestAggregate_FourFoods_ExactIntegerSums(t *testing.T) {
	// The artifact's meal: 520 kkal, 38,2 g protein across four components.
	comps := []Component{
		{FoodName: "Ayam panggang lemon", Panel: &Panel{CaloriesKcal: 245, ProteinMG: 31200, FatMG: 9100, CarbohydrateMG: 2100, FibreMG: 400, SodiumMG: 380}},
		{FoodName: "Quinoa herba", Panel: &Panel{CaloriesKcal: 180, ProteinMG: 5300, FatMG: 5600, CarbohydrateMG: 31200, FibreMG: 3800, SodiumMG: 140}},
		{FoodName: "Brokoli kukus", Panel: &Panel{CaloriesKcal: 55, ProteinMG: 1600, FatMG: 3200, CarbohydrateMG: 8300, FibreMG: 4600, SodiumMG: 90}},
		{FoodName: "Infused water timun", Panel: &Panel{CaloriesKcal: 40, ProteinMG: 100, FatMG: 200, CarbohydrateMG: 3000, FibreMG: 600, SodiumMG: 30}},
	}
	got := Aggregate(comps)
	if got.CaloriesKcal != 520 {
		t.Errorf("kcal = %d, want 520", got.CaloriesKcal)
	}
	if got.ProteinMG != 38200 {
		t.Errorf("protein = %d mg, want 38200 (38,2 g)", got.ProteinMG)
	}
	if got.SodiumMG != 640 {
		t.Errorf("sodium = %d mg, want 640", got.SodiumMG)
	}
	if got.FibreMG != 9400 {
		t.Errorf("fibre = %d mg, want 9400 (9,4 g)", got.FibreMG)
	}
	if !got.Complete {
		t.Error("all four panels present, so the meal panel is complete")
	}
}

func TestAggregate_MissingPanelMarksIncomplete_NeverUnderReports(t *testing.T) {
	got := Aggregate([]Component{
		{FoodName: "Ayam", Panel: panel(300, 30000)},
		{FoodName: "Quinoa", Panel: nil},
	})
	if got.Complete {
		t.Fatal("a missing component panel must mark the meal incomplete")
	}
	if len(got.MissingFoods) != 1 || got.MissingFoods[0] != "Quinoa" {
		t.Errorf("must name the missing food, got %v", got.MissingFoods)
	}
	// The totals still carry what is known, but Complete=false is the signal
	// that stops this being published as if it were the whole panel.
	if got.CaloriesKcal != 300 {
		t.Errorf("kcal = %d, want the known 300", got.CaloriesKcal)
	}
}

func TestAggregate_EmptyMealIsNotComplete(t *testing.T) {
	if Aggregate(nil).Complete {
		t.Error("a meal with no items is not a complete panel")
	}
}

func TestAggregate_ExtrasSummedByKey(t *testing.T) {
	got := Aggregate([]Component{
		{FoodName: "A", Panel: &Panel{Extras: map[string]int{"kalium_mg": 300, "besi_mg": 2}}},
		{FoodName: "B", Panel: &Panel{Extras: map[string]int{"kalium_mg": 150}}},
	})
	if got.Extras["kalium_mg"] != 450 {
		t.Errorf("kalium = %d, want 450", got.Extras["kalium_mg"])
	}
	if got.Extras["besi_mg"] != 2 {
		t.Errorf("a key on only one component still carries: got %d", got.Extras["besi_mg"])
	}
}
