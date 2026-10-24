package factory

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// Roadmap L3.68's Done-when: a transient failure is resumed and completes;
// a terminal one is parked on the first attempt.
func TestATransientFailureIsRetriedAndCompletes(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTransient)})

	factory.tick() // claim and launch; the run fails
	factory.tick() // settle: retry scheduled with no delay, then relaunched
	factory.tick() // settle the retry: completed and shipped

	launches := factory.launcher.launchesOf("alpha")
	if len(launches) != 2 || !hasArg(launches[1].Args, "--resume") {
		t.Fatalf("launches = %v, want a fresh run then a resume", launches)
	}
	factory.assertPhase("alpha", PhaseShipped)
	if !hasEvent(factory, "alpha", EventRetryScheduled) {
		t.Error("the ledger does not record the retry")
	}
}

func TestATerminalFailureIsParkedOnTheFirstAttempt(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 3})
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTerminal)})

	factory.tick()
	factory.tick()

	factory.assertPhase("alpha", PhaseParked)
	if order := factory.orders()["alpha"]; !strings.Contains(order.Reason, "terminal: stage analyst failed: boom") {
		t.Errorf("park reason = %q", order.Reason)
	}
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Errorf("a terminal failure was retried (%d launches)", len(launches))
	}
}

// State written before failure classes existed is treated as terminal.
func TestAnUnclassifiedFailureIsParked(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: 1, state: failedState("")})
	factory.tick()
	factory.tick()
	factory.assertPhase("alpha", PhaseParked)
}

func TestRetriesStopAtMaxAttempts(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 2})
	transient := run{code: 1, state: failedState(orchestrator.FailureTransient)}
	factory.launcher.script("alpha", transient, transient)

	for range 4 {
		factory.tick()
	}

	factory.assertPhase("alpha", PhaseParked)
	if order := factory.orders()["alpha"]; !strings.Contains(order.Reason, "attempts exhausted (2 of 2)") {
		t.Errorf("park reason = %q", order.Reason)
	}
}

func TestARetryWaitsForItsBackoff(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 3})
	factory.supervisor.config.RetryBase, factory.supervisor.config.RetryMax = time.Minute, time.Hour
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTransient)})

	factory.tick()
	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Fatalf("relaunched before the backoff elapsed (%d launches)", len(launches))
	}
	factory.now = factory.now.Add(time.Minute)
	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 2 {
		t.Errorf("not relaunched once the backoff elapsed (%d launches)", len(launches))
	}
}

func TestRetryDelayDoublesUpToItsCeiling(t *testing.T) {
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{{0, 0}, {1, time.Minute}, {2, 2 * time.Minute}, {3, 4 * time.Minute}, {10, 5 * time.Minute}} {
		if got := retryDelay(time.Minute, 5*time.Minute, tc.attempt); got != tc.want {
			t.Errorf("retryDelay(attempt %d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// Roadmap L3.70's Done-when: the order's budget seeds the run's governor, and
// a budget halt parks the order with per-stage spend in the ledger.
func TestABudgetHaltParksTheOrderWithItsSpend(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})

	factory.tick()
	factory.tick()

	order := factory.orders()["alpha"]
	if order.Phase != PhaseParked || !strings.Contains(order.Reason, "budget: usd ceiling") {
		t.Fatalf("order = %s %q, want parked on its budget", order.Phase, order.Reason)
	}
	want := &Spend{TotalUSD: 1, PerStage: []orchestrator.StageSpend{{Stage: "analyst", CostUSD: 1, Measured: true}}}
	if !reflect.DeepEqual(order.Spend, want) {
		t.Errorf("spend = %+v, want $1 attributed to analyst", order.Spend)
	}
	if !hasItem(factory.inbox(), "alpha", ItemParked) {
		t.Error("a parked order is missing from the inbox")
	}
}

// Editing a parked order is how a human clears it: the edited order is
// re-claimed and its run resumed with the new budget.
func TestEditingAParkedOrderReclaimsIt(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})
	factory.tick()
	factory.tick()

	raised := strings.Replace(orderSpec{id: "alpha"}.text(), "max-usd: 10", "max-usd: 25", 1)
	writeTestFile(t, filepath.Join(factory.root, IntakeDir, "alpha.md"), raised)
	runGit(t, factory.root, "commit", "-qam", "raise the budget")
	factory.tick()

	launches := factory.launcher.launchesOf("alpha")
	if len(launches) != 2 || !hasArg(launches[1].Args, "--max-usd", "25") || !hasArg(launches[1].Args, "--resume") {
		t.Errorf("launches = %v, want a resume with the raised budget", launches)
	}
}

func TestAnUneditedParkedOrderStaysParked(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: 1, state: failedState(orchestrator.FailureTerminal)})
	factory.tick()
	factory.tick()
	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Errorf("a parked order was relaunched without an edit (%d launches)", len(launches))
	}
}

// ADR-010, 3A: `loom factory stop` interrupts at once, the run is recorded as
// stopped rather than failed, and a resume continues it without spending an
// attempt.
func TestStopInterruptsAndResumeContinues(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	factory.tick()
	order := factory.orders()["alpha"]

	// RequestStop signals the PID; a fake PID has no process, so interrupt
	// the fake as the signal would.
	if err := factory.supervisor.ledger.Append(Entry{Event: EventStopRequested, Order: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := factory.launcher.running["alpha"].Interrupt(); err != nil {
		t.Fatal(err)
	}
	factory.tick()
	factory.assertPhase("alpha", PhaseStopped)

	if err := RequestResume(factory.supervisor.ledger, factory.orders(), "alpha"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	factory.tick()
	if resumed := factory.orders()["alpha"]; resumed.Phase != PhaseRunning || resumed.Attempt != order.Attempt {
		t.Errorf("resumed order = %s attempt %d, want running, still attempt %d", resumed.Phase, resumed.Attempt, order.Attempt)
	}
}

func TestStopAndResumeRefuseTheWrongPhase(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	if err := RequestStop(factory.supervisor.ledger, factory.orders(), "alpha"); err == nil {
		t.Error("stop accepted an order that is not running")
	}
	if err := RequestResume(factory.supervisor.ledger, factory.orders(), "alpha"); err == nil {
		t.Error("resume accepted an order that is not stopped")
	}
	if err := RequestStop(factory.supervisor.ledger, factory.orders(), "nobody"); err == nil {
		t.Error("stop accepted an unknown order")
	}
}

// An interruption nobody asked for is a transient failure, and is retried.
func TestAnUnrequestedInterruptionIsRetried(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitInterrupted, state: interruptedState})
	factory.tick()
	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 2 {
		t.Errorf("an unrequested interruption was not retried (%d launches)", len(launches))
	}
}

// A supervisor shutting down stops its runs, and the next one resumes them
// without spending an attempt.
func TestDrainingStopsRunsForTheNextSupervisor(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha", attempts: 1})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	factory.tick()

	if err := factory.supervisor.Drain(true); err != nil {
		t.Fatalf("drain: %v", err)
	}
	factory.assertPhase("alpha", PhaseStopped)

	factory.supervisor = factory.newSupervisor()
	factory.tick()
	factory.assertPhase("alpha", PhaseRunning)
	if attempt := factory.orders()["alpha"].Attempt; attempt != 1 {
		t.Errorf("attempt after resuming a drained run = %d, want 1", attempt)
	}
}

func hasEvent(factory *testFactory, order string, event Event) bool {
	for _, entry := range mustEntries(factory.t, factory.supervisor.ledger) {
		if entry.Order == order && entry.Event == event {
			return true
		}
	}
	return false
}

// --- the approval inbox (L3.67) ---

func TestAGateWaitsInTheInboxAndIsApprovedThroughLoomRun(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()

	assertOneGateItem(t, factory.inbox(), "confirm-design")
	factory.tick()
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Fatalf("resumed before anyone approved (%d launches)", len(launches))
	}

	factory.approve("alpha", "confirm-design")
	factory.tick()

	launches := factory.launcher.launchesOf("alpha")
	if len(launches) != 2 || !hasArg(launches[1].Args, "--resume", "--approve", "confirm-design") {
		t.Errorf("launches = %v, want `loom run --resume --approve confirm-design`", launches)
	}
}

func assertOneGateItem(t *testing.T, items []InboxItem, gate string) {
	t.Helper()
	if len(items) != 1 || items[0].Gate != gate || items[0].Digest == "" || len(items[0].Details) != 1 {
		t.Fatalf("inbox = %+v, want one %s item bound to analyst's digest", items, gate)
	}
}

func TestAnApprovalIsDroppedWhenAnArtifactChangedAfterIt(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.approve("alpha", "confirm-design")

	worktree := factory.orders()["alpha"].Claim.Worktree
	edited := waitingState("alpha")
	record := edited.Stages["analyst"]
	record.ArtifactSHA256 = "edited"
	edited.Stages["analyst"] = record
	if err := orchestrator.NewStateStore(RunStatePath(worktree, "alpha")).Save(edited); err != nil {
		t.Fatal(err)
	}
	factory.tick()

	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Errorf("a stale approval was acted on (%d launches)", len(launches))
	}
	if order := factory.orders()["alpha"]; order.Approval != nil || !strings.Contains(order.Reason, "changed after the approval") {
		t.Errorf("order = %+v, want the approval dropped with the reason", order)
	}
}

func TestApproveRefusesWhatTheInboxIsNotAsking(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()

	_, err := Approve(factory.supervisor.ledger, factory.inbox(), "alpha", "confirm-ship", "oscar")
	if err == nil || !strings.Contains(err.Error(), "waiting on confirm-design") {
		t.Errorf("err = %v, want a refusal naming the gate it is waiting on", err)
	}
	if _, err := Approve(factory.supervisor.ledger, factory.inbox(), "bravo", "commit", "oscar"); err == nil ||
		!strings.Contains(err.Error(), "waiting on nothing") {
		t.Errorf("err = %v, want a refusal for an order with nothing pending", err)
	}
}

// --- the finish line (L3.69) ---

func newFinishingFactory(t *testing.T, finish string, scope ...string) *testFactory {
	t.Helper()
	factory := newTestFactory(t, orderSpec{id: "alpha", finish: finish, scope: scope})
	origin := t.TempDir()
	runGit(t, origin, "init", "-q", "--bare")
	runGit(t, factory.root, "remote", "add", "origin", origin)
	factory.launcher.script("alpha", run{code: exitCompleted, state: func(id string) *orchestrator.RunState {
		writeTestFile(t, filepath.Join(factory.root, StateDir, "worktrees", id, "src", "feature.go"), "package src\n")
		return completedState(id)
	}})
	factory.tick()
	factory.tick()
	return factory
}

func TestAFinishedRunIsCommittedOnlyAfterApprovalThenPushedAndProposed(t *testing.T) {
	factory := newFinishingFactory(t, "pr", "src/**")
	factory.assertPhase("alpha", PhaseCompleted)
	items := factory.inbox()
	if len(items) != 1 || items[0].Gate != GateCommit || strings.Join(items[0].Details, ",") != "src/feature.go" {
		t.Fatalf("inbox = %+v, want a commit item listing src/feature.go", items)
	}

	factory.approve("alpha", GateCommit)
	factory.tick()

	order := factory.orders()["alpha"]
	if order.Phase != PhaseProposed || order.Proposal.Title != "feat(alpha): Deliver alpha" {
		t.Fatalf("order = %s %+v, want a proposal after the push", order.Phase, order.Proposal)
	}
	assertPushedCommit(t, factory, order, "src/feature.go")
	if len(factory.pulls.opened) != 0 {
		t.Error("a pull request was opened without a human approving it")
	}
}

func assertPushedCommit(t *testing.T, factory *testFactory, order *OrderState, want string) {
	t.Helper()
	if origin := runGit(t, factory.root, "ls-remote", "origin", order.Claim.Branch); origin == "" {
		t.Errorf("%s was not pushed", order.Claim.Branch)
	}
	if committed := runGit(t, order.Claim.Worktree, "show", "--name-only", "--format=", "HEAD"); committed != want {
		t.Errorf("commit contains %q, want only %s", committed, want)
	}
}

// L3.69's Done-when: nothing opens a pull request without a recorded human
// approval of that exact proposal, however many times the supervisor ticks.
func TestAPullRequestOpensOnlyForAnApprovedProposal(t *testing.T) {
	factory := newFinishingFactory(t, "pr")
	factory.approve("alpha", GateCommit)
	for range 3 {
		factory.tick()
	}
	if len(factory.pulls.opened) != 0 {
		t.Fatal("a pull request opened before its proposal was approved")
	}
	forged := Entry{Event: EventApproved, Order: "alpha", Gate: GateOpenPR, Digest: "not-the-proposal"}
	if err := factory.supervisor.ledger.Append(forged); err != nil {
		t.Fatal(err)
	}
	factory.tick()
	if len(factory.pulls.opened) != 0 {
		t.Fatal("a pull request opened for an approval bound to a different proposal")
	}

	factory.approve("alpha", GateOpenPR)
	factory.tick()

	if len(factory.pulls.opened) != 1 {
		t.Fatalf("pull requests opened = %d, want 1", len(factory.pulls.opened))
	}
	assertShippedAndCleanedUp(t, factory, factory.orders()["alpha"])
}

func assertShippedAndCleanedUp(t *testing.T, factory *testFactory, order *OrderState) {
	t.Helper()
	if order.Phase != PhaseShipped || order.URL == "" {
		t.Errorf("order = %s %q, want shipped with the PR's URL", order.Phase, order.URL)
	}
	if _, err := os.Stat(order.Claim.Worktree); !os.IsNotExist(err) {
		t.Error("a shipped order's worktree was not removed")
	}
	archived := filepath.Join(factory.root, StateDir, "runs", order.ID, "run-state.json")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("the shipped run's state was not archived: %v", err)
	}
}

func TestAFinishBranchOrderShipsAfterThePush(t *testing.T) {
	factory := newFinishingFactory(t, "branch")
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.assertPhase("alpha", PhaseShipped)
	if len(factory.pulls.opened) != 0 {
		t.Error("finish: branch opened a pull request")
	}
}

func TestACommitApprovalIsDroppedWhenTheTreeChangesAfterIt(t *testing.T) {
	factory := newFinishingFactory(t, "pr")
	factory.approve("alpha", GateCommit)
	worktree := factory.orders()["alpha"].Claim.Worktree
	writeTestFile(t, filepath.Join(worktree, "src", "late.go"), "package src\n")

	factory.tick()

	factory.assertPhase("alpha", PhaseCompleted)
	if order := factory.orders()["alpha"]; order.Approval != nil {
		t.Error("a commit approval survived an edit to the tree it approved")
	}
}

func TestAPushFailureIsRecordedOnceAndRetried(t *testing.T) {
	factory := newFinishingFactory(t, "branch")
	runGit(t, factory.root, "remote", "remove", "origin")
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.tick()

	factory.assertPhase("alpha", PhaseCommitted)
	failures := 0
	for _, entry := range mustEntries(t, factory.supervisor.ledger) {
		if entry.Event == EventFinishFailed {
			failures++
		}
	}
	if failures != 1 {
		t.Errorf("finish failures recorded = %d, want 1 for one repeated reason", failures)
	}
}

func TestOutOfScopeChangesAreMarkedInTheInbox(t *testing.T) {
	factory := newFinishingFactory(t, "pr", "docs/**")
	items := factory.inbox()
	if len(items) != 1 || !strings.Contains(items[0].Details[0], "outside scope.paths") {
		t.Errorf("inbox = %+v, want src/feature.go marked outside scope", items)
	}
}

func TestFinishNoneShipsAndKeepsTheWorktree(t *testing.T) {
	factory := newFinishingFactory(t, "none")
	factory.assertPhase("alpha", PhaseShipped)
	if _, err := os.Stat(factory.orders()["alpha"].Claim.Worktree); err != nil {
		t.Errorf("finish: none removed the worktree a human was meant to take over: %v", err)
	}
}
