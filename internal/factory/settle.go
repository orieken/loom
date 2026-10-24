package factory

import (
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/orieken/loom/internal/orchestrator"
)

// settleEnded records the outcome of every run that has stopped: the ones
// this supervisor started and saw exit, and the ones a previous supervisor
// started whose process is gone.
func (s *Supervisor) settleEnded() error {
	orders, err := s.Orders()
	if err != nil {
		return err
	}
	for _, order := range Sorted(orders) {
		if order.Phase != PhaseRunning {
			continue
		}
		exitCode, ended := s.hasEnded(order)
		if !ended {
			continue
		}
		if err := s.settle(order, exitCode); err != nil {
			return err
		}
	}
	return nil
}

// hasEnded reports whether an order's process has exited, and its exit code
// when this supervisor was its parent.
func (s *Supervisor) hasEnded(order *OrderState) (*int, bool) {
	process, owned := s.processes[order.ID]
	if !owned {
		return nil, !processAlive(order.PID)
	}
	select {
	case <-process.Done():
		delete(s.processes, order.ID)
		code := process.ExitCode()
		return &code, true
	default:
		return nil, false
	}
}

func (s *Supervisor) settle(order *OrderState, exitCode *int) error {
	state, err := loadRunState(order.Claim.Worktree, order.ID)
	if err != nil {
		return fmt.Errorf("read run state for %s: %w", order.ID, err)
	}
	entry := s.decide(order, judge(exitCode, state))
	entry.Order, entry.Attempt, entry.ExitCode = order.ID, order.Attempt, exitCode
	if err := s.record(entry); err != nil {
		return err
	}
	// A run the supervisor interrupted while shutting down was not stopped
	// by anyone: the next supervisor continues it, without spending one of
	// the order's attempts.
	if entry.Event == EventStopped && !order.StopRequested {
		return s.record(Entry{Event: EventResumeRequested, Order: order.ID})
	}
	return nil
}

// decide turns an outcome into the ledger's next line for the order.
func (s *Supervisor) decide(order *OrderState, result outcome) Entry {
	switch result.kind {
	case outcomeCompleted:
		return Entry{Event: EventCompleted}
	case outcomeWaiting:
		return Entry{Event: EventWaiting, Gate: result.gate}
	case outcomeBudget:
		return Entry{Event: EventParked, Reason: result.reason, Spend: result.spend}
	case outcomeInterrupted:
		return s.decideInterrupted(order, result)
	}
	return s.decideFailure(order, result)
}

// decideInterrupted: a stop a human asked for stays stopped; a supervisor
// shutting down stops its runs to be resumed by the next one; anything
// else interrupted a run nobody meant to stop, which is a transient failure.
func (s *Supervisor) decideInterrupted(order *OrderState, result outcome) Entry {
	if order.StopRequested {
		return Entry{Event: EventStopped}
	}
	if s.draining {
		return Entry{Event: EventStopped, Reason: "the supervisor stopped"}
	}
	return s.decideFailure(order, result)
}

// decideFailure retries a transient failure while the order has attempts
// left (roadmap L3.68) and parks everything else with the reason.
func (s *Supervisor) decideFailure(order *OrderState, result outcome) Entry {
	if result.class != orchestrator.FailureTransient {
		return Entry{Event: EventParked, Reason: "terminal: " + result.reason}
	}
	allowed := order.Claim.Budget.MaxAttempts
	if order.Attempt >= allowed {
		return Entry{Event: EventParked,
			Reason: fmt.Sprintf("attempts exhausted (%d of %d): %s", order.Attempt, allowed, result.reason)}
	}
	notBefore := s.deps.Now().Add(retryDelay(s.config.RetryBase, s.config.RetryMax, order.Attempt))
	return Entry{Event: EventRetryScheduled, NotBefore: &notBefore, Reason: "transient: " + result.reason}
}

// retryDelay is the exponential backoff before retry number attempt, from
// the same library L2.6 adopted for tool calls. Randomization is off: the
// delay is written to the ledger, and a reader should be able to predict it.
func retryDelay(base, ceiling time.Duration, attempt int) time.Duration {
	policy := &backoff.ExponentialBackOff{InitialInterval: base, Multiplier: 2, MaxInterval: ceiling}
	policy.Reset()
	var delay time.Duration
	for range attempt {
		delay = policy.NextBackOff()
	}
	return delay
}
