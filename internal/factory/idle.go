package factory

// tickKind is what a tick ended with, once started work is accounted for.
type tickKind int

// The kinds a tick can end in. tickBusy is the zero value, so a fresh
// Supervisor's first busy tick stays silent with no separate sentinel.
const (
	tickBusy tickKind = iota
	tickIdle
	tickWaiting
)

// tickState is what the supervisor last reported, so announce can tell
// whether this tick's state is one it already printed.
type tickState struct {
	kind  tickKind
	count int
}

// waitingPhases are the phases Inbox() shows a human (gate halt, commit
// proposal, PR proposal, parked). Keep this in lockstep with inbox.go's
// itemsFor switch — nothing else enforces the two agree.
var waitingPhases = map[Phase]bool{
	PhaseWaiting:   true,
	PhaseCompleted: true,
	PhaseProposed:  true,
	PhaseParked:    true,
}

// waitingCount counts the orders waiting on a human.
func waitingCount(orders map[string]*OrderState) int {
	count := 0
	for _, order := range orders {
		if waitingPhases[order.Phase] {
			count++
		}
	}
	return count
}

// hasUnshippedOrder reports whether any known order has not reached
// PhaseShipped — true for a retrying, stopped, or push-failed order, none
// of which are idle or waiting on a human.
func hasUnshippedOrder(orders map[string]*OrderState) bool {
	for _, order := range orders {
		if order.Phase != PhaseShipped {
			return true
		}
	}
	return false
}

// classify says what a tick ended in: busy (something is running, or an
// order is neither idle nor waiting on a human, such as one retrying before
// its backoff), waiting (nothing running, but a human has something to do),
// or idle (nothing running, nothing waiting, nothing left unshipped).
func classify(orders map[string]*OrderState) tickState {
	if activeCount(orders) > 0 {
		return tickState{kind: tickBusy}
	}
	if waiting := waitingCount(orders); waiting > 0 {
		return tickState{kind: tickWaiting, count: waiting}
	}
	if hasUnshippedOrder(orders) {
		return tickState{kind: tickBusy}
	}
	return tickState{kind: tickIdle}
}

// announce prints the state's line, once per entry: nothing if the state
// matches what was last reported, otherwise the idle or waiting line.
func (s *Supervisor) announce(state tickState) {
	if state == s.lastReported {
		return
	}
	s.lastReported = state
	switch state.kind {
	case tickIdle:
		s.say("idle — no ready orders; commit one to intake/ on %s", s.config.IntakeBranch)
	case tickWaiting:
		s.say("waiting on a human — %d order(s) in the inbox: loom factory inbox", state.count)
	}
}

// reportTickState classifies the ledger's current state and announces it,
// reading only the ledger replay Tick() already produces.
func (s *Supervisor) reportTickState() error {
	orders, err := s.Orders()
	if err != nil {
		return err
	}
	s.announce(classify(orders))
	return nil
}
