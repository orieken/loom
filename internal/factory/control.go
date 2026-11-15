package factory

import (
	"errors"
	"fmt"
	"net/url"
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

// ShippedByHand is one person's record that they finished an order outside
// the factory, in the pull request at URL.
type ShippedByHand struct {
	Order    string
	URL      string
	Approver string
}

// Closer finishes a hand-shipped order the same way the supervisor finishes
// one of its own: archive the run's workspace, then remove its worktree.
type Closer struct {
	Layout     Layout
	Repository Repository
}

// Close records that a human finished request.Order by hand, in the pull
// request at request.URL, as if the supervisor had shipped it: one `shipped`
// ledger entry, then the same archive-then-remove sequence `ship` uses. It
// never calls GitHub; the human running it vouches that the pull request
// merged. The returned string is a warning when archiving or removing the
// worktree fails ("" on full success), matching Supervisor.ship's own
// wording — Close still succeeds (nil error) either way, because the order
// is shipped once its ledger line is recorded.
func (c Closer) Close(ledger *Ledger, orders map[string]*OrderState, request ShippedByHand) (string, error) {
	order, err := closeable(orders, request.Order, request.URL)
	if err != nil {
		return "", err
	}
	entry := Entry{Event: EventShipped, Order: request.Order, URL: request.URL, ApprovedBy: request.Approver}
	if err := ledger.Append(entry); err != nil {
		return "", err
	}
	return archiveAndRemove(order, c.Layout, c.Repository), nil
}

// closeable refuses unless id names a ledger order, rawURL is an http(s)
// URL, and the order is parked or stopped — the only phases a person
// finishes by hand. Every other phase, including one already shipped, is
// refused with RequestRerun's own wording, so a closeable order and a
// rerun-able one are refused identically.
func closeable(orders map[string]*OrderState, id, rawURL string) (*OrderState, error) {
	order, err := orderIn(orders, id)
	if err != nil {
		return nil, err
	}
	if !isHTTPURL(rawURL) {
		return nil, fmt.Errorf("%w: --shipped-by %q is not an http or https URL", ErrNotApplicable, rawURL)
	}
	if order.Phase != PhaseParked && order.Phase != PhaseStopped {
		return nil, fmt.Errorf("%w: %s is %s; stop it first, or it has already shipped", ErrNotApplicable, id, order.Phase)
	}
	return order, nil
}

// isHTTPURL reports whether raw is an http or https URL. It never makes a
// network call: parsing is purely syntactic.
func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
