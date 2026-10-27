package factory

import (
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// Dogfood report, case 1: every order shipped and intake has nothing new —
// the supervisor says so, once.
func TestAFreshSupervisorWithNoOrdersReportsIdle(t *testing.T) {
	factory := newTestFactory(t)

	factory.tick()

	if count := factory.outputCount("idle — no ready orders; commit one to intake/ on main"); count != 1 {
		t.Fatalf("idle lines = %d, want exactly 1 in %v", count, factory.outputLines())
	}
}

// AC3: consecutive ticks in the same state print the line once, not on
// every tick.
func TestIdlePrintsOnceAcrossConsecutiveTicks(t *testing.T) {
	factory := newTestFactory(t)

	factory.tick()
	factory.tick()

	if count := factory.outputCount("idle —"); count != 1 {
		t.Errorf("idle lines across two idle ticks = %d, want 1", count)
	}
}

// AC2: a run halted at a gate is one of the four waiting kinds.
func TestAGateHaltCountsAsWaiting(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})

	factory.tick() // claim and launch; running, so silent
	factory.tick() // settles to waiting

	if count := factory.outputCount("waiting on a human — 1 order(s) in the inbox: loom factory inbox"); count != 1 {
		t.Fatalf("waiting lines = %d, want exactly 1 in %v", count, factory.outputLines())
	}
}

// AC2: a parked order is also counted as waiting on a human.
func TestAParkedOrderCountsAsWaiting(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 1})
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTerminal)})

	factory.tick()
	factory.tick()

	factory.assertPhase("alpha", PhaseParked)
	if count := factory.outputCount("waiting on a human — 1 order(s) in the inbox: loom factory inbox"); count != 1 {
		t.Fatalf("waiting lines = %d, want exactly 1 in %v", count, factory.outputLines())
	}
}

// AC4: while waiting, a second order entering a waiting phase re-prints the
// line with the incremented count.
func TestASecondWaitingOrderReprintsWithTheIncrementedCount(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	if count := factory.outputCount("waiting on a human — 1 order(s) in the inbox: loom factory inbox"); count != 1 {
		t.Fatalf("waiting-at-1 lines = %d, want 1 in %v", count, factory.outputLines())
	}

	factory.commitOrder(orderSpec{id: "bravo", finish: "pr"})
	factory.tick() // claims and launches bravo; running, so silent
	factory.tick() // bravo settles to completed, awaiting a commit approval

	factory.assertPhase("bravo", PhaseCompleted)
	if count := factory.outputCount("waiting on a human — 2 order(s) in the inbox: loom factory inbox"); count != 1 {
		t.Fatalf("waiting-at-2 lines = %d, want 1 in %v", count, factory.outputLines())
	}
}

// AC4: leaving waiting (an approval resumes the run) is silent, and
// re-entering idle once the order ships prints the idle line again.
func TestLeavingWaitingThenReturningToIdleReprints(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	if count := factory.outputCount("waiting on a human"); count != 1 {
		t.Fatalf("waiting lines before approval = %d, want 1", count)
	}

	factory.approve("alpha", "confirm-design")
	factory.tick() // relaunches; running, so silent
	factory.tick() // the relaunch completes and ships (finish: none)

	factory.assertPhase("alpha", PhaseShipped)
	if count := factory.outputCount("waiting on a human"); count != 1 {
		t.Errorf("waiting lines stayed at 1 while leaving the state, got %d", count)
	}
	if count := factory.outputCount("idle —"); count != 1 {
		t.Errorf("idle lines after re-entering idle = %d, want 1", count)
	}
}

// Spec's resolved Open Question: an order retrying before its backoff is
// neither idle nor waiting on a human, so it prints nothing.
func TestARetryingOrderPrintsNothing(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 3})
	factory.supervisor.config.RetryBase, factory.supervisor.config.RetryMax = time.Minute, time.Hour
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTransient)})

	factory.tick() // claim and launch; the run fails
	factory.tick() // retry scheduled, waiting on its own backoff

	factory.assertPhase("alpha", PhaseRetrying)
	for _, line := range factory.outputLines() {
		if strings.Contains(line, "idle —") || strings.Contains(line, "waiting on a human") {
			t.Errorf("unexpected idle/waiting output for a retrying order: %q", line)
		}
	}
}

// AC5: `loom factory run --once` applies the same once-per-entry rule to
// its single pass.
func TestOnceAppliesTheOncePerEntryRuleToItsSinglePass(t *testing.T) {
	factory := newTestFactory(t)

	if err := factory.supervisor.Once(); err != nil {
		t.Fatalf("once: %v", err)
	}
	if count := factory.outputCount("idle —"); count != 1 {
		t.Fatalf("idle lines after the first Once() = %d, want 1", count)
	}

	if err := factory.supervisor.Once(); err != nil {
		t.Fatalf("once: %v", err)
	}
	if count := factory.outputCount("idle —"); count != 1 {
		t.Errorf("idle lines after a second, unchanged Once() = %d, want still 1", count)
	}
}
