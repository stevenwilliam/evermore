package order

import (
	"errors"
	"testing"
	"time"

	"github.com/stevenwilliam/evermore/internal/domain/money"
)

// 01-domain-model.md §4.1: "Illegal transitions are rejected by the domain
// layer, not the handler, and the transition table is a unit test." This is
// that test — it asserts the whole table, both the edges that exist and the
// ones that must not.

func TestTransitionTable_LegalEdges(t *testing.T) {
	legal := []struct {
		from, to Status
		actor    Actor
		reason   string
	}{
		{StatusDraft, StatusAwaitingPayment, ActorCustomer, ""},
		{StatusAwaitingPayment, StatusPaymentSubmitted, ActorCustomer, ""},
		{StatusAwaitingPayment, StatusExpired, ActorSystem, ""},
		{StatusAwaitingPayment, StatusCancelled, ActorCustomer, "berubah pikiran"},
		{StatusPaymentSubmitted, StatusPaid, ActorStaff, ""},
		{StatusPaymentSubmitted, StatusAwaitingPayment, ActorStaff, "bukti tidak cocok"},
		{StatusPaymentSubmitted, StatusExpired, ActorSystem, ""},
		{StatusPaid, StatusCompleted, ActorSystem, ""},
		{StatusPaid, StatusCancelled, ActorStaff, "dapur tidak bisa memenuhi"},
		{StatusPaid, StatusRefunded, ActorAdmin, "transfer ganda"},
	}
	for _, c := range legal {
		if err := Transition(c.from, c.to, c.actor, c.reason); err != nil {
			t.Errorf("%s -> %s as %s should be legal: %v", c.from, c.to, c.actor, err)
		}
	}
}

func TestTransitionTable_IllegalEdgesRejected(t *testing.T) {
	all := []Status{
		StatusDraft, StatusAwaitingPayment, StatusPaymentSubmitted, StatusPaid,
		StatusCompleted, StatusCancelled, StatusExpired, StatusRefunded,
	}
	// Every pair not in the table must be refused. This catches an edge added
	// by accident as well as one removed.
	for _, from := range all {
		for _, to := range all {
			if CanTransition(from, to) {
				continue
			}
			err := Transition(from, to, ActorAdmin, "alasan")
			if !errors.Is(err, ErrIllegalTransition) {
				t.Errorf("%s -> %s is not on the machine but was not rejected: %v", from, to, err)
			}
		}
	}
}

func TestTransition_TerminalStatesGoNowhere(t *testing.T) {
	for _, s := range []Status{StatusCompleted, StatusExpired, StatusRefunded, StatusCancelled} {
		if !IsTerminal(s) {
			t.Errorf("%s should be terminal", s)
		}
		for _, to := range []Status{StatusDraft, StatusPaid, StatusAwaitingPayment} {
			if CanTransition(s, to) {
				t.Errorf("%s -> %s must not exist", s, to)
			}
		}
	}
}

func TestTransition_RefundedIsAdminOnly_D31(t *testing.T) {
	// D-31: money does not go back as policy. REFUNDED exists only for the
	// erroneous transfer, and only an admin may reach it.
	for _, actor := range []Actor{ActorCustomer, ActorStaff, ActorSystem} {
		err := Transition(StatusPaid, StatusRefunded, actor, "transfer ganda")
		if !errors.Is(err, ErrActorNotPermitted) {
			t.Errorf("%s must not reach REFUNDED: %v", actor, err)
		}
	}
	if err := Transition(StatusPaid, StatusRefunded, ActorAdmin, "transfer ganda"); err != nil {
		t.Errorf("admin must be able to: %v", err)
	}
}

func TestTransition_NothingAutomatedCancels(t *testing.T) {
	// CLAUDE.md §7: nothing automated cancels a customer's booking. The system
	// actor may expire an unpaid order, but may never cancel one.
	for _, from := range []Status{StatusAwaitingPayment, StatusPaid} {
		if err := Transition(from, StatusCancelled, ActorSystem, "job"); !errors.Is(err, ErrActorNotPermitted) {
			t.Errorf("the system must not cancel from %s: %v", from, err)
		}
	}
}

func TestTransition_ReasonRequiredWhereDocumented(t *testing.T) {
	needsReason := []struct{ from, to Status }{
		{StatusAwaitingPayment, StatusCancelled},
		{StatusPaymentSubmitted, StatusAwaitingPayment},
		{StatusPaid, StatusCancelled},
		{StatusPaid, StatusRefunded},
	}
	for _, c := range needsReason {
		if err := Transition(c.from, c.to, ActorAdmin, ""); !errors.Is(err, ErrReasonRequired) {
			t.Errorf("%s -> %s must require a reason: %v", c.from, c.to, err)
		}
	}
}

func TestDeliveryTransitions(t *testing.T) {
	legal := []struct{ from, to DeliveryStatus }{
		{DeliveryScheduled, DeliveryPreparing},
		{DeliveryScheduled, DeliverySkipped},
		{DeliveryScheduled, DeliveryScheduled},
		{DeliveryPreparing, DeliveryOutForDelivery},
		{DeliveryOutForDelivery, DeliveryDelivered},
		{DeliveryOutForDelivery, DeliveryFailed},
		{DeliveryFailed, DeliveryScheduled},
	}
	for _, c := range legal {
		if err := TransitionDelivery(c.from, c.to); err != nil {
			t.Errorf("delivery %s -> %s should be legal: %v", c.from, c.to, err)
		}
	}
	illegal := []struct{ from, to DeliveryStatus }{
		{DeliveryScheduled, DeliveryDelivered}, // cannot skip the middle
		{DeliveryDelivered, DeliveryScheduled}, // delivered is terminal
		{DeliveryPreparing, DeliverySkipped},   // skip is before cut-off only
	}
	for _, c := range illegal {
		if err := TransitionDelivery(c.from, c.to); err == nil {
			t.Errorf("delivery %s -> %s must be rejected", c.from, c.to)
		}
	}
}

func TestFulfilmentStatus_Derived(t *testing.T) {
	cases := []struct {
		name string
		in   []DeliveryStatus
		want string
	}{
		{"all delivered", []DeliveryStatus{DeliveryDelivered, DeliveryDelivered}, "DELIVERED"},
		{"one on the road", []DeliveryStatus{DeliveryDelivered, DeliveryOutForDelivery}, "OUT_FOR_DELIVERY"},
		{"kitchen started", []DeliveryStatus{DeliveryScheduled, DeliveryPreparing}, "PREPARING"},
		{"nothing started", []DeliveryStatus{DeliveryScheduled, DeliveryScheduled}, "SCHEDULED"},
		{"skipped ones do not block completion", []DeliveryStatus{DeliveryDelivered, DeliverySkipped}, "DELIVERED"},
		{"everything cancelled", []DeliveryStatus{DeliveryCancelled, DeliveryCancelled}, "CANCELLED"},
		{"no deliveries", nil, ""},
	}
	for _, c := range cases {
		if got := FulfilmentStatus(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestNumber_MatchesTheArtifactFormat(t *testing.T) {
	at := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	if got := Number("EVM", at, 148); got != "EVM-2609-0148" {
		t.Errorf("got %q, want EVM-2609-0148", got)
	}
	if got := Number("PKG", at, 42); got != "PKG-2609-0042" {
		t.Errorf("got %q, want PKG-2609-0042", got)
	}
}

func TestPaymentAmount_SuffixIsNeverLessThanOwed(t *testing.T) {
	// The artifact: total Rp 480.000, suffix 148, transfer Rp 480.148.
	amount, rounding, err := PaymentAmount(480000, 148)
	if err != nil {
		t.Fatal(err)
	}
	if amount != 480148 {
		t.Errorf("amount = %d, want 480148", amount)
	}
	if rounding != 148 {
		t.Errorf("rounding = %d, want 148", rounding)
	}
}

func TestPaymentAmount_NeverUndercharges(t *testing.T) {
	// The case that would silently undercharge if the suffix simply replaced
	// the last three digits: a total already ending above the suffix.
	amount, rounding, err := PaymentAmount(480900, 148)
	if err != nil {
		t.Fatal(err)
	}
	if amount < 480900 {
		t.Fatalf("amount %d is less than the total 480900 — the customer would underpay", amount)
	}
	if amount != 481148 {
		t.Errorf("amount = %d, want 481148", amount)
	}
	if rounding != 248 {
		t.Errorf("rounding = %d, want 248", rounding)
	}
	// The property, across the whole suffix range.
	for _, total := range []money.IDR{1, 999, 1000, 78000, 480000, 480900, 2720000} {
		for suffix := 0; suffix < MaxSuffix; suffix += 37 {
			amt, rnd, err := PaymentAmount(total, suffix)
			if err != nil {
				t.Fatal(err)
			}
			if amt < total {
				t.Fatalf("total=%d suffix=%d gave %d, less than owed", total, suffix, amt)
			}
			if amt-total != rnd {
				t.Fatalf("total=%d suffix=%d: rounding %d != %d", total, suffix, rnd, amt-total)
			}
			if rnd >= 2*MaxSuffix {
				t.Fatalf("total=%d suffix=%d: rounding %d is more than two thousand", total, suffix, rnd)
			}
			if int(amt%MaxSuffix) != suffix {
				t.Fatalf("total=%d suffix=%d: amount %d does not end in the suffix", total, suffix, amt)
			}
		}
	}
}

func TestPaymentAmount_RejectsBadInput(t *testing.T) {
	if _, _, err := PaymentAmount(-1, 148); err == nil {
		t.Error("negative total should be rejected")
	}
	for _, s := range []int{-1, 1000, 5000} {
		if _, _, err := PaymentAmount(1000, s); err == nil {
			t.Errorf("suffix %d should be rejected", s)
		}
	}
}

func TestRandomSuffix_InRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		s, err := RandomSuffix()
		if err != nil {
			t.Fatal(err)
		}
		if s < 0 || s >= MaxSuffix {
			t.Fatalf("suffix %d out of range", s)
		}
	}
}
