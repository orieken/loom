package factory

import (
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// The second factory dogfood run hit a Claude session limit overnight. One
// order spent both its attempts a minute apart on a limit that reset four
// hours later; another parked on a budget it could never verify again,
// because the stage the limit cut short reported no usage.

func unmeasuredBudgetState(string) *orchestrator.RunState {
	finished := time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)
	state := budgetState("")
	state.Stages["developer"] = orchestrator.StageRecord{Status: orchestrator.StageStatusFailed, Sequence: 2,
		FinishedAt: &finished, FailureClass: orchestrator.FailureUnavailable}
	state.BudgetHalt = &orchestrator.BudgetHalt{Limit: orchestrator.BudgetLimitUSD, Allowed: 10, Consumed: 1,
		BeforeStage: "developer", Unmeasured: []string{"developer"}}
	return state
}

func TestAnUnavailableProviderIsWaitedOutWithoutSpendingAnAttempt(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 1})
	factory.launcher.script("alpha",
		run{code: 1, state: failedState(orchestrator.FailureUnavailable)},
		run{code: 1, state: failedState(orchestrator.FailureUnavailable)})

	factory.tick() // claim and launch; the provider is unavailable
	factory.tick() // settle: a retry after the wait, not a park
	assertWaitingOutTheLimit(t, factory)

	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Fatalf("relaunched before the wait was over (%d launches)", len(launches))
	}

	factory.now = factory.now.Add(DefaultUnavailableWait)
	factory.tick() // relaunched on the same attempt; unavailable again
	factory.tick() // settled again: still waiting, never parked on attempts
	if order := factory.orders()["alpha"]; order.Phase != PhaseRetrying || order.Attempt != 1 {
		t.Errorf("order = %s attempt %d, want still retrying on attempt 1 of 1", order.Phase, order.Attempt)
	}
}

func assertWaitingOutTheLimit(t *testing.T, factory *testFactory) {
	t.Helper()
	order := factory.orders()["alpha"]
	if order.Phase != PhaseRetrying || !order.KeepsAttempt || !order.NotBefore.Equal(factory.now.Add(DefaultUnavailableWait)) {
		t.Fatalf("order = %s keeps=%v notBefore=%v, want a retry in %v that keeps its attempt",
			order.Phase, order.KeepsAttempt, order.NotBefore, DefaultUnavailableWait)
	}
	if !strings.HasPrefix(order.Reason, "unavailable: ") {
		t.Errorf("reason = %q", order.Reason)
	}
}

func TestAnOrdinaryRetryStillSpendsAnAttemptAfterAnUnavailableOne(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 3})
	factory.launcher.script("alpha",
		run{code: 1, state: failedState(orchestrator.FailureUnavailable)},
		run{code: 1, state: failedState(orchestrator.FailureTransient)})
	factory.tick()
	factory.tick()
	factory.now = factory.now.Add(DefaultUnavailableWait)
	factory.tick() // attempt 1 again, fails transiently
	factory.tick() // settle and relaunch: this retry spends one

	launches := factory.launcher.launchesOf("alpha")
	if order := factory.orders()["alpha"]; len(launches) != 3 || order.Attempt != 2 {
		t.Errorf("launches = %d, attempt = %d; want three launches, the last on attempt 2", len(launches), order.Attempt)
	}
}

func TestABudgetTheRunCannotVerifyIsApprovable(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: unmeasuredBudgetState})
	factory.tick()
	factory.tick()

	assertApprovableBudgetPark(t, factory)

	factory.approve("alpha", GateBudget)
	factory.tick()

	assertResumedAcceptingTheSpend(t, factory)
	if order := factory.orders()["alpha"]; order.Attempt != 1 {
		t.Errorf("attempt = %d; accepting a budget spends no attempt", order.Attempt)
	}
}

func assertApprovableBudgetPark(t *testing.T, factory *testFactory) {
	t.Helper()
	order := factory.orders()["alpha"]
	if order.Phase != PhaseParked || strings.Join(order.Unmeasured, ",") != "developer" {
		t.Fatalf("order = %s unmeasured %v, want parked on developer's unknown spend", order.Phase, order.Unmeasured)
	}
	item := findItem(factory.inbox(), "alpha", ItemParked)
	if item.Gate != GateBudget || item.Digest == "" || !strings.Contains(item.Summary, "loom factory approve alpha budget") {
		t.Fatalf("item = %+v, want an approvable budget item", item)
	}
}

func assertResumedAcceptingTheSpend(t *testing.T, factory *testFactory) {
	t.Helper()
	launches := factory.launcher.launchesOf("alpha")
	last := launches[len(launches)-1]
	if len(launches) != 2 || !hasArg(last.Args, "--accept-unmeasured", "oscar") || !hasArg(last.Args, "--resume") {
		t.Fatalf("launches = %v, want a resume accepting the unmeasured spend on oscar's behalf", launches)
	}
	if hasArg(last.Args, "--approve") {
		t.Error("an accepted budget was passed as a gate approval")
	}
}

// A ceiling actually reached is not a human's to wave through here: it is
// raised by editing the order, which re-claims it.
func TestABudgetCeilingReachedIsNotApprovable(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})
	factory.tick()
	factory.tick()

	if item := findItem(factory.inbox(), "alpha", ItemParked); item.IsApprovable() {
		t.Errorf("item = %+v, want an informational park", item)
	}
	if _, err := Approve(factory.supervisor.ledger, factory.inbox(), "alpha", GateBudget, "oscar"); err == nil {
		t.Error("a reached ceiling was approved as unmeasured spend")
	}
}

func TestABudgetApprovalForAnotherHaltIsDropped(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: unmeasuredBudgetState})
	factory.tick()
	factory.tick()
	factory.approve("alpha", GateBudget)
	if err := factory.supervisor.record(Entry{Event: EventParked, Order: "alpha", Reason: "budget again",
		Spend: &Spend{TotalUSD: 2}, Unmeasured: []string{"developer", "qa-engineer"}}); err != nil {
		t.Fatal(err)
	}

	factory.tick()

	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Errorf("an approval of a different halt relaunched the order (%d launches)", len(launches))
	}
	if order := factory.orders()["alpha"]; order.Approval != nil || !strings.Contains(order.Reason, "budget halt changed") {
		t.Errorf("order = %+v, want the approval dropped", order)
	}
}

func TestAParkedOrdersSpendLeavesOutStagesRoutedAround(t *testing.T) {
	state := unmeasuredBudgetState("")
	state.Stages["data-engineer"] = orchestrator.StageRecord{Status: orchestrator.StageStatusSkipped, Sequence: 3}
	var stages []string
	for _, stage := range spendOf(state).PerStage {
		stages = append(stages, stage.Stage)
	}
	if strings.Join(stages, ",") != "analyst,developer" {
		t.Errorf("spend lists %v; a skipped stage spent nothing and is not unmeasured", stages)
	}
}

func findItem(items []InboxItem, order string, kind ItemKind) InboxItem {
	for _, item := range items {
		if item.Order == order && item.Kind == kind {
			return item
		}
	}
	return InboxItem{}
}

// An order parked before the ledger recorded its unmeasured stages — the
// dogfood run's pre-ship-checks — is approvable from its reason.
func TestAParkRecordedBeforeUnmeasuredWasLoggedIsStillApprovable(t *testing.T) {
	cases := map[string][]string{
		"budget: usd ceiling cannot be verified — [developer] reported no usage":         {"developer"},
		"budget: usd ceiling cannot be verified — [analyst developer] reported no usage": {"analyst", "developer"},
		"budget: usd ceiling reached before stage developer (41 of 40)":                  nil,
		"terminal: stage analyst failed: boom":                                           nil,
		"budget: usd ceiling cannot be verified — [developer":                            nil,
	}
	for reason, want := range cases {
		order := &OrderState{}
		order.apply(Entry{Event: EventParked, Reason: reason})
		if strings.Join(order.Unmeasured, ",") != strings.Join(want, ",") {
			t.Errorf("%q: unmeasured = %v, want %v", reason, order.Unmeasured, want)
		}
	}
}
