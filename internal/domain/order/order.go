// Package order holds the commercial lifecycle from 01-domain-model.md §4.1
// and §4.2, plus order numbering and the payment matching suffix (D-16).
//
// The order owns commercial states; a delivery owns fulfilment states (D-15).
// An order with twenty deliveries cannot itself be OUT_FOR_DELIVERY, so the
// brief's fulfilment names are exposed as a derived read-only value instead.
package order

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// Status is the commercial state of an order.
type Status string

const (
	StatusDraft            Status = "DRAFT"
	StatusAwaitingPayment  Status = "AWAITING_PAYMENT"
	StatusPaymentSubmitted Status = "PAYMENT_SUBMITTED"
	StatusPaid             Status = "PAID"
	StatusCompleted        Status = "COMPLETED"
	StatusCancelled        Status = "CANCELLED"
	StatusExpired          Status = "EXPIRED"
	StatusRefunded         Status = "REFUNDED"
)

// DeliveryStatus is the fulfilment state of one delivery.
type DeliveryStatus string

const (
	DeliveryScheduled      DeliveryStatus = "SCHEDULED"
	DeliveryPreparing      DeliveryStatus = "PREPARING"
	DeliveryOutForDelivery DeliveryStatus = "OUT_FOR_DELIVERY"
	DeliveryDelivered      DeliveryStatus = "DELIVERED"
	DeliveryFailed         DeliveryStatus = "FAILED"
	DeliverySkipped        DeliveryStatus = "SKIPPED"
	DeliveryCancelled      DeliveryStatus = "CANCELLED"
)

// Actor is who is attempting a transition. Some edges are role-gated: only an
// admin may reach REFUNDED (D-31).
type Actor string

const (
	ActorCustomer Actor = "customer"
	ActorStaff    Actor = "staff"
	ActorAdmin    Actor = "admin"
	ActorSystem   Actor = "system" // scheduled jobs
)

// ErrIllegalTransition is returned for any edge not on the machine. The domain
// layer rejects it, not the handler.
var ErrIllegalTransition = errors.New("ILLEGAL_ORDER_TRANSITION")

// ErrActorNotPermitted is returned when the edge exists but this actor may not
// take it.
var ErrActorNotPermitted = errors.New("TRANSITION_ACTOR_NOT_PERMITTED")

// ErrReasonRequired is returned for edges that must record why.
var ErrReasonRequired = errors.New("TRANSITION_REASON_REQUIRED")

type edge struct {
	from, to Status
}

// transitions is the machine from §4.1, as data. The transition table is a
// unit test, so this map is the single place an edge can be added.
var transitions = map[edge]struct {
	actors         []Actor
	requiresReason bool
}{
	{StatusDraft, StatusAwaitingPayment}:            {actors: []Actor{ActorCustomer, ActorStaff}},
	{StatusAwaitingPayment, StatusPaymentSubmitted}: {actors: []Actor{ActorCustomer, ActorStaff}},
	{StatusAwaitingPayment, StatusExpired}:          {actors: []Actor{ActorSystem}},
	{StatusAwaitingPayment, StatusCancelled}:        {actors: []Actor{ActorCustomer, ActorStaff}, requiresReason: true},
	{StatusPaymentSubmitted, StatusPaid}:            {actors: []Actor{ActorStaff, ActorAdmin}},
	{StatusPaymentSubmitted, StatusAwaitingPayment}: {actors: []Actor{ActorStaff, ActorAdmin}, requiresReason: true},
	{StatusPaymentSubmitted, StatusExpired}:         {actors: []Actor{ActorSystem}},
	{StatusPaid, StatusCompleted}:                   {actors: []Actor{ActorSystem, ActorStaff}},
	{StatusPaid, StatusCancelled}:                   {actors: []Actor{ActorStaff, ActorAdmin}, requiresReason: true},
	{StatusPaid, StatusRefunded}:                    {actors: []Actor{ActorAdmin}, requiresReason: true},
}

// CanTransition reports whether an edge exists at all, ignoring the actor.
func CanTransition(from, to Status) bool {
	_, ok := transitions[edge{from, to}]
	return ok
}

// Transition validates a state change. Nothing automated cancels a customer's
// booking: every edge into CANCELLED requires a human actor and a reason, and
// the only ActorSystem edges are EXPIRED (the payment deadline) and COMPLETED.
func Transition(from, to Status, actor Actor, reason string) error {
	rule, ok := transitions[edge{from, to}]
	if !ok {
		return fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, from, to)
	}
	// An admin can take any edge staff can, plus the admin-only ones. Listing
	// admin on every staff edge instead would make it easy to forget one, and
	// a forgotten one reads as "the admin is locked out of their own system".
	permitted := slices.Contains(rule.actors, actor) ||
		(actor == ActorAdmin && slices.Contains(rule.actors, ActorStaff))
	if !permitted {
		return fmt.Errorf("%w: %s may not take %s -> %s", ErrActorNotPermitted, actor, from, to)
	}
	if rule.requiresReason && reason == "" {
		return fmt.Errorf("%w: %s -> %s", ErrReasonRequired, from, to)
	}
	return nil
}

// IsTerminal reports whether an order can move no further.
func IsTerminal(s Status) bool {
	switch s {
	case StatusCompleted, StatusExpired, StatusRefunded, StatusCancelled:
		return true
	}
	return false
}

// FulfilmentStatus derives the brief's §6.3 fulfilment name from the order's
// deliveries (D-15). It is read-only and never stored.
func FulfilmentStatus(deliveries []DeliveryStatus) string {
	if len(deliveries) == 0 {
		return ""
	}
	counts := map[DeliveryStatus]int{}
	for _, d := range deliveries {
		counts[d]++
	}
	live := len(deliveries) - counts[DeliveryCancelled] - counts[DeliverySkipped]
	switch {
	case live == 0:
		return string(DeliveryCancelled)
	case counts[DeliveryDelivered] == live:
		return string(DeliveryDelivered)
	case counts[DeliveryOutForDelivery] > 0:
		return string(DeliveryOutForDelivery)
	case counts[DeliveryPreparing] > 0:
		return string(DeliveryPreparing)
	default:
		return string(DeliveryScheduled)
	}
}

// deliveryTransitions is the machine from §4.2.
var deliveryTransitions = map[struct{ from, to DeliveryStatus }]bool{
	{DeliveryScheduled, DeliveryPreparing}:      true,
	{DeliveryScheduled, DeliverySkipped}:        true,
	{DeliveryScheduled, DeliveryCancelled}:      true,
	{DeliveryScheduled, DeliveryScheduled}:      true, // reschedule / re-route
	{DeliveryPreparing, DeliveryOutForDelivery}: true,
	{DeliveryOutForDelivery, DeliveryDelivered}: true,
	{DeliveryOutForDelivery, DeliveryFailed}:    true,
	{DeliveryFailed, DeliveryScheduled}:         true, // staff reschedules
}

// TransitionDelivery validates a fulfilment state change.
func TransitionDelivery(from, to DeliveryStatus) error {
	if !deliveryTransitions[struct{ from, to DeliveryStatus }{from, to}] {
		return fmt.Errorf("%w: delivery %s -> %s", ErrIllegalTransition, from, to)
	}
	return nil
}

// Number builds an order number of the form EVM-YYMM-NNNN, as the artifact
// shows (EVM-2609-0148). seq is the monthly sequence, allocated by the
// database so it cannot collide.
func Number(prefix string, at time.Time, seq int) string {
	return fmt.Sprintf("%s-%02d%02d-%04d", prefix, at.Year()%100, int(at.Month()), seq)
}

// MaxSuffix is the exclusive upper bound of the payment matching suffix: a
// three-digit number, 000-999.
const MaxSuffix = 1000

// PaymentAmount attaches a unique three-digit suffix to the total so that an
// incoming bank transfer can be matched to one order (D-16).
//
// The suffix is a matching device, not consideration: it is excluded from the
// tax base and lands in payment_rounding_idr so that reports reconcile
// (01-domain-model.md §3.11).
//
// The suffix replaces the last three digits rather than being added to them,
// so the amount a customer is asked to transfer never drifts upward by more
// than the rounding the suffix itself represents.
func PaymentAmount(total money.IDR, suffix int) (amount money.IDR, rounding money.IDR, err error) {
	if total < 0 {
		return 0, 0, money.ErrNegative
	}
	if suffix < 0 || suffix >= MaxSuffix {
		return 0, 0, fmt.Errorf("order: suffix %d out of range 0-999", suffix)
	}
	base := (total / MaxSuffix) * MaxSuffix
	amount = base + money.IDR(suffix)
	if amount < total {
		// The suffix landed below the total's own last three digits; carry to
		// the next thousand so the customer is never asked for less than owed.
		amount += MaxSuffix
	}
	return amount, amount - total, nil
}

// RandomSuffix draws a matching suffix from a CSPRNG. Callers retry on
// collision against the open orders for the same bank account and day.
func RandomSuffix() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(MaxSuffix))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
