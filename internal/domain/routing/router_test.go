package routing

import (
	"testing"

	"github.com/google/uuid"
)

// Matrix from 01-domain-model.md §5.3, item by item.

func base() Candidate {
	return Candidate{
		KitchenID: uuid.New(), Code: "KTC-01", Priority: 1, DistanceM: 1000,
		InsideRadius: true, IsActive: true, ServesSlot: true,
		OpenThatDay: true, FreePortions: 40,
	}
}

func TestRoute_InsideOneRadius(t *testing.T) {
	c := base()
	got, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got.KitchenID != c.KitchenID || got.Mode != ModeAuto {
		t.Errorf("got %+v", got)
	}
}

func TestRoute_TwoOverlappingRadii_PriorityThenDistance(t *testing.T) {
	near := base()
	near.Code, near.Priority, near.DistanceM = "KBY-02", 2, 500 // closer but lower priority
	far := base()
	far.Code, far.Priority, far.DistanceM = "KTC-01", 1, 3000

	got, err := Route(Request{Candidates: []Candidate{near, far}, PortionsNeeded: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "KTC-01" {
		t.Errorf("priority must beat distance: got %s, want KTC-01", got.Code)
	}

	// Same priority: distance decides.
	near.Priority = 1
	got, err = Route(Request{Candidates: []Candidate{near, far}, PortionsNeeded: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "KBY-02" {
		t.Errorf("at equal priority distance decides: got %s, want KBY-02", got.Code)
	}
}

func TestRoute_InsidePolygonOutsideRadius_PolygonWins(t *testing.T) {
	c := base()
	c.HasPolygon, c.InsidePolygon, c.InsideRadius = true, true, false
	if _, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 1}); err != nil {
		t.Errorf("a polygon hit must serve even outside the radius: %v", err)
	}
}

func TestRoute_InsideRadiusOutsidePolygon_PolygonWins(t *testing.T) {
	c := base()
	c.HasPolygon, c.InsidePolygon, c.InsideRadius = true, false, true
	if _, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 1}); err != ErrNotServiceable {
		t.Errorf("a polygon miss must block even inside the radius: got %v", err)
	}
}

func TestRoute_OutsideEverything_Blocks(t *testing.T) {
	c := base()
	c.InsideRadius = false
	if _, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 1}); err != ErrNotServiceable {
		t.Errorf("got %v, want ErrNotServiceable", err)
	}
	if _, err := Route(Request{Candidates: nil, PortionsNeeded: 1}); err != ErrNotServiceable {
		t.Errorf("no candidates at all: got %v", err)
	}
}

func TestRoute_DroppedCandidates(t *testing.T) {
	cases := []struct {
		name  string
		mutex func(*Candidate)
	}{
		{"at capacity", func(c *Candidate) { c.FreePortions = 3 }}, // needs 4
		{"inactive kitchen", func(c *Candidate) { c.IsActive = false }},
		{"does not serve that slot", func(c *Candidate) { c.ServesSlot = false }},
		{"closed that weekday", func(c *Candidate) { c.OpenThatDay = false }},
	}
	for _, tc := range cases {
		c := base()
		tc.mutex(&c)
		if _, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 4}); err != ErrNotServiceable {
			t.Errorf("%s: got %v, want ErrNotServiceable", tc.name, err)
		}
	}
}

func TestRoute_CapacityExactlyEnough(t *testing.T) {
	c := base()
	c.FreePortions = 4
	if _, err := Route(Request{Candidates: []Candidate{c}, PortionsNeeded: 4}); err != nil {
		t.Errorf("exactly enough capacity must be accepted: %v", err)
	}
}

func TestRoute_ManualAssignmentNeverOverwritten(t *testing.T) {
	c := base()
	_, err := Route(Request{
		Candidates: []Candidate{c}, PortionsNeeded: 1,
		IsReroute: true, ExistingMode: ModeManual,
	})
	if err != ErrManualAssignment {
		t.Errorf("got %v, want ErrManualAssignment", err)
	}
}

func TestRoute_RerouteAfterCutOffRefused(t *testing.T) {
	c := base()
	_, err := Route(Request{
		Candidates: []Candidate{c}, PortionsNeeded: 1,
		IsReroute: true, ExistingMode: ModeAuto, AfterCutOff: true,
	})
	if err != ErrAfterCutOff {
		t.Errorf("got %v, want ErrAfterCutOff", err)
	}
}

func TestRoute_RerouteBeforeCutOffAllowed(t *testing.T) {
	c := base()
	if _, err := Route(Request{
		Candidates: []Candidate{c}, PortionsNeeded: 1,
		IsReroute: true, ExistingMode: ModeAuto, AfterCutOff: false,
	}); err != nil {
		t.Errorf("an auto re-route before cut-off is allowed: %v", err)
	}
}

func TestRoute_IsDeterministic(t *testing.T) {
	// Identical priority and distance: the tie-break must be stable so a
	// routing decision can be reproduced from the record.
	a, b := base(), base()
	a.Code, b.Code = "KLG-03", "KBY-02"
	for i := 0; i < 50; i++ {
		got, err := Route(Request{Candidates: []Candidate{a, b}, PortionsNeeded: 1})
		if err != nil {
			t.Fatal(err)
		}
		if got.Code != "KBY-02" {
			t.Fatalf("iteration %d picked %s, want the stable KBY-02", i, got.Code)
		}
	}
}
