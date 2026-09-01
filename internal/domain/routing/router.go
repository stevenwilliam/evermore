// Package routing implements the kitchen router from 01-domain-model.md §5.3.
//
// It is pure: the caller supplies the candidate kitchens (already filtered by
// the database's geography predicates) and the router decides which one wins
// and why. Two rules dominate:
//
//   - A polygon overrides a radius. If a kitchen has a service_area, that
//     polygon decides, and its radius is irrelevant.
//   - A manual assignment is never overwritten by a re-route, and a re-route
//     after cut-off is refused.
package routing

import (
	"errors"
	"sort"

	"github.com/google/uuid"
)

// ErrNotServiceable is returned when no kitchen can serve the point. Checkout
// blocks and the attempt is logged to out_of_range_attempt.
var ErrNotServiceable = errors.New("ADDRESS_NOT_SERVICEABLE")

// ErrManualAssignment is returned when a re-route would overwrite a decision a
// human made. §5.3: manual assignment is never overwritten.
var ErrManualAssignment = errors.New("MANUAL_ASSIGNMENT_NOT_OVERWRITTEN")

// ErrAfterCutOff is returned when a re-route is attempted after cut-off.
var ErrAfterCutOff = errors.New("REROUTE_AFTER_CUTOFF")

// Mode records how a delivery came to be assigned to a kitchen.
type Mode string

const (
	ModeAuto   Mode = "AUTO"
	ModeManual Mode = "MANUAL"
)

// Candidate is a kitchen the caller has determined *could* serve the point,
// together with everything the router needs to rank or reject it.
type Candidate struct {
	KitchenID uuid.UUID
	Code      string
	Priority  int // lower is preferred
	DistanceM int

	// Geography verdicts, computed by the database.
	//
	// HasPolygon says the kitchen defines a service_area. InsidePolygon says
	// the point falls in it. InsideRadius says the point is within
	// service_radius_km. The polygon-wins rule is applied here, not by the
	// caller, so it is tested in one place.
	HasPolygon    bool
	InsidePolygon bool
	InsideRadius  bool

	IsActive     bool
	ServesSlot   bool // kitchen_slot covers the requested slot
	OpenThatDay  bool // kitchen_operating_day covers the weekday
	FreePortions int  // max_portions - reserved_portions for date+slot
}

// serviceable applies the polygon-over-radius rule.
func (c Candidate) serviceable() bool {
	if c.HasPolygon {
		return c.InsidePolygon // the polygon decides, radius is ignored
	}
	return c.InsideRadius
}

// eligible applies every non-geographic filter.
func (c Candidate) eligible(portionsNeeded int) bool {
	return c.IsActive &&
		c.ServesSlot &&
		c.OpenThatDay &&
		c.FreePortions >= portionsNeeded &&
		c.serviceable()
}

// Assignment is the routing outcome.
type Assignment struct {
	KitchenID uuid.UUID
	Code      string
	DistanceM int
	Mode      Mode
	Reason    string
}

// Request is a routing question.
type Request struct {
	Candidates     []Candidate
	PortionsNeeded int

	// ExistingMode and ExistingKitchen describe a delivery already assigned,
	// when this is a re-route rather than a first assignment.
	IsReroute       bool
	ExistingMode    Mode
	ExistingKitchen uuid.UUID
	AfterCutOff     bool
}

// Route picks the kitchen.
//
// Ranking, once the ineligible are dropped: priority ascending, then distance
// ascending. Priority first is what "inside two overlapping radii (priority
// decides, then distance)" means in §5.3.
func Route(req Request) (Assignment, error) {
	if req.IsReroute {
		if req.ExistingMode == ModeManual {
			return Assignment{}, ErrManualAssignment
		}
		if req.AfterCutOff {
			return Assignment{}, ErrAfterCutOff
		}
	}

	portions := req.PortionsNeeded
	if portions <= 0 {
		portions = 1
	}

	var eligible []Candidate
	for _, c := range req.Candidates {
		if c.eligible(portions) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return Assignment{}, ErrNotServiceable
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].Priority != eligible[j].Priority {
			return eligible[i].Priority < eligible[j].Priority
		}
		if eligible[i].DistanceM != eligible[j].DistanceM {
			return eligible[i].DistanceM < eligible[j].DistanceM
		}
		// A deterministic tie-break so the same inputs always give the same
		// answer, which matters for reproducing a routing decision later.
		return eligible[i].Code < eligible[j].Code
	})

	win := eligible[0]
	return Assignment{
		KitchenID: win.KitchenID,
		Code:      win.Code,
		DistanceM: win.DistanceM,
		Mode:      ModeAuto,
		Reason:    reasonFor(win, len(eligible)),
	}, nil
}

// Rank returns EVERY eligible kitchen, best first, using the same ordering as
// Route.
//
// Reservation needs the whole list, not just the winner. The free-capacity
// figure a candidate carries was read outside the row lock, so by the time the
// lock is taken the preferred kitchen may be full — and the correct response
// is to try the next kitchen, not to tell the customer there is no capacity
// anywhere. Route is Rank()[0].
func Rank(req Request) ([]Assignment, error) {
	if req.IsReroute {
		if req.ExistingMode == ModeManual {
			return nil, ErrManualAssignment
		}
		if req.AfterCutOff {
			return nil, ErrAfterCutOff
		}
	}

	portions := req.PortionsNeeded
	if portions <= 0 {
		portions = 1
	}

	var eligible []Candidate
	for _, c := range req.Candidates {
		// Capacity is deliberately NOT filtered here: it is re-checked under
		// the lock. Filtering on a stale read would drop a kitchen that has
		// since freed up.
		if c.IsActive && c.ServesSlot && c.OpenThatDay && c.serviceable() {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil, ErrNotServiceable
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].Priority != eligible[j].Priority {
			return eligible[i].Priority < eligible[j].Priority
		}
		if eligible[i].DistanceM != eligible[j].DistanceM {
			return eligible[i].DistanceM < eligible[j].DistanceM
		}
		return eligible[i].Code < eligible[j].Code
	})

	out := make([]Assignment, 0, len(eligible))
	for _, c := range eligible {
		out = append(out, Assignment{
			KitchenID: c.KitchenID, Code: c.Code, DistanceM: c.DistanceM,
			Mode: ModeAuto, Reason: reasonFor(c, len(eligible)),
		})
	}
	return out, nil
}

func reasonFor(c Candidate, n int) string {
	switch {
	case n == 1 && c.HasPolygon:
		return "satu-satunya dapur yang poligonnya mencakup titik ini"
	case n == 1:
		return "satu-satunya dapur dalam radius layanan"
	case c.HasPolygon:
		return "prioritas tertinggi di antara dapur yang poligonnya mencakup titik ini"
	default:
		return "prioritas tertinggi, lalu jarak terdekat"
	}
}
