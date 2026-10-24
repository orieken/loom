package factory

import (
	"errors"
	"fmt"
)

// ErrNotApplicable reports a request that does not fit the order's phase.
var ErrNotApplicable = errors.New("not applicable")

// RequestStop interrupts a running order at once (ADR-010, 3A). The request
// is recorded before the signal is sent, so the supervisor settling the
// interrupted run reads it as a stop rather than a failure to retry.
func RequestStop(ledger *Ledger, orders map[string]*OrderState, id string) error {
	order, err := orderIn(orders, id)
	if err != nil {
		return err
	}
	if !order.IsActive() || order.PID == 0 {
		return fmt.Errorf("%w: %s is %s, not running", ErrNotApplicable, id, order.Phase)
	}
	if err := ledger.Append(Entry{Event: EventStopRequested, Order: id}); err != nil {
		return err
	}
	return interruptProcess(order.PID)
}

// RequestResume asks the supervisor to continue a stopped order.
func RequestResume(ledger *Ledger, orders map[string]*OrderState, id string) error {
	order, err := orderIn(orders, id)
	if err != nil {
		return err
	}
	if order.Phase != PhaseStopped {
		return fmt.Errorf("%w: %s is %s; only a stopped order is resumed this way", ErrNotApplicable, id, order.Phase)
	}
	return ledger.Append(Entry{Event: EventResumeRequested, Order: id})
}

// RequestRerun asks the supervisor to re-claim an order from its committed
// version, which has changed since the claim (ADR-010, 1C).
func RequestRerun(ledger *Ledger, orders map[string]*OrderState, committed Intake, id string) error {
	order, err := orderIn(orders, id)
	if err != nil {
		return err
	}
	candidate, ok := committed.Orders[id]
	if !ok || candidate.SHA256 == order.Claim.OrderSHA256 {
		return fmt.Errorf("%w: %s has not changed since it was claimed", ErrNotApplicable, id)
	}
	if order.IsActive() || order.Phase == PhaseShipped {
		return fmt.Errorf("%w: %s is %s; stop it first, or it has already shipped", ErrNotApplicable, id, order.Phase)
	}
	return ledger.Append(Entry{Event: EventRerunRequested, Order: id})
}

func orderIn(orders map[string]*OrderState, id string) (*OrderState, error) {
	order, ok := orders[id]
	if !ok {
		return nil, fmt.Errorf("%w: no order %q in the ledger", ErrNotApplicable, id)
	}
	return order, nil
}
