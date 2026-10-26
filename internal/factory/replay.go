package factory

import (
	"sort"
	"strings"
	"time"
)

// Phase is where an order is, as the ledger tells it.
type Phase string

// The phases an order moves through.
const (
	// PhaseClaimed: taken, but no process has been recorded starting yet —
	// the supervisor died between the two, and the next tick launches it.
	PhaseClaimed   Phase = "claimed"
	PhaseRunning   Phase = "running"
	PhaseWaiting   Phase = "waiting"
	PhaseRetrying  Phase = "retrying"
	PhaseParked    Phase = "parked"
	PhaseStopped   Phase = "stopped"
	PhaseCompleted Phase = "completed"
	PhaseCommitted Phase = "committed"
	PhasePushed    Phase = "pushed"
	PhaseProposed  Phase = "proposed"
	PhaseShipped   Phase = "shipped"
)

// OrderState is one order's current state, folded from its ledger lines.
type OrderState struct {
	ID      string
	Phase   Phase
	Attempt int
	Claim   Claim
	PID     int
	Gate    string
	Reason  string
	// NotBefore is when a scheduled retry may start.
	NotBefore time.Time
	// KeepsAttempt marks a scheduled retry that spends no attempt.
	KeepsAttempt bool
	// Unmeasured names the stages whose unknown spend parked the order.
	Unmeasured []string
	// StopRequested and ResumeRequested are a human's asks not yet acted on.
	StopRequested   bool
	ResumeRequested bool
	RerunRequested  bool
	// Approval is a human approval the supervisor has not yet acted on.
	Approval *Entry
	Spend    *Spend
	Commit   string
	Proposal *Proposal
	URL      string
	// Since is when the order entered its current phase.
	Since time.Time
}

// Replay folds the ledger into each order's state. It is a pure function of
// the entries, which is what makes a supervisor killed mid-tick safe: the
// next one reads the same ledger and arrives at the same state.
func Replay(entries []Entry) map[string]*OrderState {
	orders := map[string]*OrderState{}
	for _, entry := range entries {
		order := orders[entry.Order]
		if entry.Event == EventClaimed {
			order = &OrderState{ID: entry.Order}
			orders[entry.Order] = order
		}
		if order == nil {
			continue // a line for an order never claimed is noise, not state
		}
		order.apply(entry)
	}
	return orders
}

// Sorted returns the orders by id, for stable output.
func Sorted(orders map[string]*OrderState) []*OrderState {
	list := make([]*OrderState, 0, len(orders))
	for _, order := range orders {
		list = append(list, order)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

func (o *OrderState) apply(entry Entry) {
	if phase, ok := phaseOf[entry.Event]; ok {
		o.Phase, o.Since, o.Reason = phase, entry.At, entry.Reason
	}
	if update, ok := entryUpdates[entry.Event]; ok {
		update(o, entry)
	}
}

// phaseOf maps the events that move an order to the phase they move it to.
var phaseOf = map[Event]Phase{
	EventClaimed:        PhaseClaimed,
	EventLaunched:       PhaseRunning,
	EventWaiting:        PhaseWaiting,
	EventCompleted:      PhaseCompleted,
	EventRetryScheduled: PhaseRetrying,
	EventParked:         PhaseParked,
	EventStopped:        PhaseStopped,
	EventCommitted:      PhaseCommitted,
	EventPushed:         PhasePushed,
	EventProposed:       PhaseProposed,
	EventShipped:        PhaseShipped,
}

// entryUpdates carry each event's facts onto the order.
var entryUpdates = map[Event]func(*OrderState, Entry){
	EventClaimed:         applyClaim,
	EventLaunched:        applyLaunch,
	EventWaiting:         func(o *OrderState, e Entry) { o.Gate, o.PID = e.Gate, 0 },
	EventCompleted:       clearProcess,
	EventRetryScheduled:  applyRetry,
	EventParked:          applyPark,
	EventStopped:         func(o *OrderState, e Entry) { clearProcess(o, e); o.StopRequested = false },
	EventStopRequested:   func(o *OrderState, _ Entry) { o.StopRequested = true },
	EventResumeRequested: func(o *OrderState, _ Entry) { o.ResumeRequested = true },
	EventRerunRequested:  func(o *OrderState, _ Entry) { o.RerunRequested = true },
	EventApproved:        func(o *OrderState, e Entry) { approval := e; o.Approval = &approval },
	EventApprovalDropped: func(o *OrderState, e Entry) { o.Approval, o.Reason = nil, e.Reason },
	EventCommitted:       func(o *OrderState, e Entry) { o.Commit, o.Approval = e.Commit, nil },
	EventProposed:        func(o *OrderState, e Entry) { o.Proposal = e.Proposal },
	EventPROpened:        func(o *OrderState, e Entry) { o.URL, o.Approval = e.URL, nil },
	EventFinishFailed:    func(o *OrderState, e Entry) { o.Reason = e.Reason },
}

func applyClaim(o *OrderState, entry Entry) {
	if entry.Claim != nil {
		o.Claim = *entry.Claim
	}
	o.Attempt = 0
}

func applyLaunch(o *OrderState, entry Entry) {
	o.PID, o.Attempt, o.ResumeRequested = entry.PID, entry.Attempt, false
	o.KeepsAttempt, o.Unmeasured = false, nil
	if entry.Approving != "" {
		o.Approval = nil
	}
}

func applyPark(o *OrderState, entry Entry) {
	clearProcess(o, entry)
	o.Spend, o.Unmeasured = entry.Spend, entry.Unmeasured
	if len(o.Unmeasured) == 0 {
		o.Unmeasured = unmeasuredInReason(entry.Reason)
	}
}

// unmeasuredInReason recovers the unmeasured stages from a park recorded
// before the ledger carried them, so an order parked by an older supervisor
// can still be approved. It reads the one reason budgetOutcome writes for
// this case: "... cannot be verified — [a b] reported no usage".
func unmeasuredInReason(reason string) []string {
	_, rest, found := strings.Cut(reason, "cannot be verified — [")
	if !found {
		return nil
	}
	stages, _, closed := strings.Cut(rest, "] reported no usage")
	if !closed {
		return nil
	}
	return strings.Fields(stages)
}

func applyRetry(o *OrderState, entry Entry) {
	clearProcess(o, entry)
	o.KeepsAttempt = entry.KeepsAttempt
	if entry.NotBefore != nil {
		o.NotBefore = *entry.NotBefore
	}
}

func clearProcess(o *OrderState, _ Entry) { o.PID = 0 }

// IsActive reports whether the order holds a run slot.
func (o *OrderState) IsActive() bool {
	return o.Phase == PhaseRunning
}
