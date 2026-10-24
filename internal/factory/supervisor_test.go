package factory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
)

// Roadmap L3.65's Done-when: with three ready orders and two slots, two runs
// start, and the third starts when one finishes.
func TestTwoRunsStartAndTheThirdWaitsForASlot(t *testing.T) {
	factory := newTestFactory(t,
		orderSpec{id: "alpha", priority: 1}, orderSpec{id: "bravo", priority: 2}, orderSpec{id: "charlie", priority: 3})
	for _, id := range []string{"alpha", "bravo", "charlie"} {
		factory.launcher.script(id, run{code: exitCompleted, state: completedState, hold: true})
	}

	factory.tick()
	factory.assertPhase("alpha", PhaseRunning)
	factory.assertPhase("bravo", PhaseRunning)
	factory.assertPhase("charlie", "")

	factory.tick()
	factory.assertPhase("charlie", "")

	factory.launcher.release("alpha")
	factory.tick()
	factory.assertPhase("alpha", PhaseShipped)
	factory.assertPhase("charlie", PhaseRunning)
}

// Each run gets its own worktree on its own branch, and its spec is the
// claim snapshot inside it (L3.66, ADR-010 1C).
func TestAClaimRunsInItsOwnWorktreeFromASnapshot(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.tick()

	launches := factory.launcher.launchesOf("alpha")
	if len(launches) != 1 {
		t.Fatalf("launches = %d, want 1", len(launches))
	}
	launch := launches[0]
	assertWorktreeOnBranch(t, factory, launch.Dir, "alpha")
	for _, want := range [][]string{
		{"--spec", ClaimPath("alpha")}, {"--plan", "deliver-feature"}, {"--provider", "mock"},
		{"--max-usd", "10"}, {"--max-hours", "4"},
	} {
		if !hasArg(launch.Args, want...) {
			t.Errorf("run args %v lack %v", launch.Args, want)
		}
	}
	if status := runGit(t, factory.root, "status", "--porcelain"); status != "" {
		t.Errorf("the root checkout is dirty after a claim:\n%s", status)
	}
}

func assertWorktreeOnBranch(t *testing.T, factory *testFactory, dir, id string) {
	t.Helper()
	if dir != factory.supervisor.config.Layout.WorktreePath(id) {
		t.Errorf("run directory = %s, want the order's worktree", dir)
	}
	if branch := runGit(t, dir, "branch", "--show-current"); branch != BranchFor(id) {
		t.Errorf("worktree branch = %q, want %s", branch, BranchFor(id))
	}
	snapshot, err := os.ReadFile(filepath.Join(dir, ClaimPath(id)))
	if err != nil || !strings.Contains(string(snapshot), "id: "+id) {
		t.Fatalf("claim snapshot = %q, %v", snapshot, err)
	}
}

// ADR-010, 2B: an order the factory has not seen committed does not exist.
func TestAnUncommittedReadyOrderIsNotClaimed(t *testing.T) {
	factory := newTestFactory(t)
	writeTestFile(t, filepath.Join(factory.root, IntakeDir, "alpha.md"), orderSpec{id: "alpha"}.text())

	factory.tick()

	factory.assertPhase("alpha", "")
}

func TestOnlyValidReadyOrdersAreClaimed(t *testing.T) {
	invalid := orderSpec{id: "broken", attempts: -1}
	factory := newTestFactory(t, orderSpec{id: "draft", status: "draft"}, orderSpec{id: "held", status: "hold"}, invalid)
	factory.tick()
	for _, id := range []string{"draft", "held", "broken"} {
		factory.assertPhase(id, "")
	}
}

// Review item I-10: an order sharing a feature spec's stem is invalid.
func TestAnOrderNamedLikeAFeatureSpecIsNotClaimed(t *testing.T) {
	factory := newTestFactory(t)
	writeTestFile(t, filepath.Join(factory.root, FeaturesDir, "alpha.md"), "# Feature: alpha\n")
	factory.commitOrder(orderSpec{id: "alpha"})

	factory.tick()

	factory.assertPhase("alpha", "")
}

// Review item I-8: same priority, same day — id decides, not git's listing.
func TestTiesBreakOnCreatedThenID(t *testing.T) {
	factory := newTestFactory(t,
		orderSpec{id: "zulu", created: "2026-10-24"}, orderSpec{id: "yankee", created: "2026-10-23"},
		orderSpec{id: "alpha", created: "2026-10-24"})
	factory.supervisor.config.MaxConcurrent = 1
	for _, id := range []string{"zulu", "yankee", "alpha"} {
		factory.launcher.script(id, run{code: exitCompleted, state: completedState, hold: true})
	}

	var started []string
	for _, id := range []string{"yankee", "alpha", "zulu"} {
		factory.tick()
		started = append(started, firstRunning(factory))
		factory.launcher.release(id)
	}
	if strings.Join(started, ",") != "yankee,alpha,zulu" {
		t.Errorf("start order = %v, want yankee (older), then alpha before zulu", started)
	}
}

func firstRunning(factory *testFactory) string {
	for _, order := range Sorted(factory.orders()) {
		if order.Phase == PhaseRunning {
			return order.ID
		}
	}
	return ""
}

func TestADependentWaitsUntilItsDependencyShips(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "base"}, orderSpec{id: "follow", dependsOn: []string{"base"}})
	factory.launcher.script("base", run{code: exitCompleted, state: completedState, hold: true})

	factory.tick()
	factory.assertPhase("follow", "")

	factory.launcher.release("base")
	factory.tick()
	factory.assertPhase("base", PhaseShipped)
	factory.assertPhase("follow", PhaseRunning)
}

// The ledger replays to the same state after a supervisor dies mid-tick: a
// claim recorded but never launched is launched exactly once by the next
// supervisor, and a half-written line is dropped.
func TestASupervisorKilledBetweenClaimAndLaunchIsRecovered(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	committed, err := ReadIntake(factory.supervisor.deps.Repository, "main", factory.supervisor.config.Plans)
	if err != nil {
		t.Fatal(err)
	}
	claim := factory.supervisor.claimFor(committed.Orders["alpha"], committed.Commit)
	if err := factory.supervisor.ledger.Append(Entry{Event: EventClaimed, Order: "alpha", Claim: &claim}); err != nil {
		t.Fatal(err)
	}
	appendRaw(t, factory.supervisor.config.Layout.LedgerPath(), `{"event":"launched","order":"alpha","pi`)
	before := Replay(mustEntries(t, factory.supervisor.ledger))

	factory.supervisor = factory.newSupervisor()
	if again := Replay(mustEntries(t, factory.supervisor.ledger)); again["alpha"].Phase != before["alpha"].Phase {
		t.Fatalf("replay differs across supervisors: %v vs %v", again["alpha"].Phase, before["alpha"].Phase)
	}
	factory.tick()

	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Fatalf("launches after recovery = %d, want exactly 1", len(launches))
	}
	if order := factory.orders()["alpha"]; order.Phase != PhaseRunning || order.Attempt != 1 {
		t.Errorf("recovered order = %s attempt %d, want running attempt 1", order.Phase, order.Attempt)
	}
}

// A run a previous supervisor started is settled from its state once its
// process is gone — the new supervisor was never its parent.
func TestAnOrphanedRunIsSettledFromItsState(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()

	factory.supervisor = factory.newSupervisor()
	factory.tick()

	factory.assertPhase("alpha", PhaseWaiting)
}

func appendRaw(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func mustEntries(t *testing.T, ledger *Ledger) []Entry {
	t.Helper()
	entries, err := ledger.Entries()
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	return entries
}

// ADR-010, 1C: editing an order after its claim never changes the run; the
// inbox says it changed, and `rerun` is the human's choice.
func TestEditingAClaimedOrderLeavesTheRunAloneAndTellsTheInbox(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	worktree := factory.orders()["alpha"].Claim.Worktree
	before, _ := os.ReadFile(filepath.Join(worktree, ClaimPath("alpha")))

	factory.commitOrder(orderSpec{id: "alpha", priority: 1})
	factory.tick()

	after, _ := os.ReadFile(filepath.Join(worktree, ClaimPath("alpha")))
	if string(before) != string(after) {
		t.Error("the claim snapshot changed when the intake file was edited")
	}
	if !hasItem(factory.inbox(), "alpha", ItemChanged) {
		t.Errorf("inbox = %+v, want a changed-since-claim item", factory.inbox())
	}
}

func hasItem(items []InboxItem, order string, kind ItemKind) bool {
	for _, item := range items {
		if item.Order == order && item.Kind == kind {
			return true
		}
	}
	return false
}

func TestRerunReclaimsTheEditedOrder(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.commitOrder(orderSpec{id: "alpha", priority: 1})
	committed, err := ReadIntake(factory.supervisor.deps.Repository, "main", factory.supervisor.config.Plans)
	if err != nil {
		t.Fatal(err)
	}

	if err := RequestRerun(factory.supervisor.ledger, factory.orders(), committed, "alpha"); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	factory.tick()

	launches := factory.launcher.launchesOf("alpha")
	snapshot, _ := os.ReadFile(filepath.Join(launches[len(launches)-1].Dir, ClaimPath("alpha")))
	if len(launches) != 2 || !hasArg(launches[1].Args, "--resume") || !strings.Contains(string(snapshot), "priority: 1") {
		t.Errorf("rerun launches = %d (%v), want a resume from the new snapshot", len(launches), launches)
	}
}

func TestRerunRefusesAnUnchangedOrRunningOrder(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	factory.tick()
	committed, _ := ReadIntake(factory.supervisor.deps.Repository, "main", factory.supervisor.config.Plans)
	if err := RequestRerun(factory.supervisor.ledger, factory.orders(), committed, "alpha"); err == nil {
		t.Error("rerun accepted an order that has not changed")
	}
	factory.commitOrder(orderSpec{id: "alpha", priority: 1})
	committed, _ = ReadIntake(factory.supervisor.deps.Repository, "main", factory.supervisor.config.Plans)
	if err := RequestRerun(factory.supervisor.ledger, factory.orders(), committed, "alpha"); err == nil {
		t.Error("rerun accepted a running order")
	}
	if err := RequestRerun(factory.supervisor.ledger, factory.orders(), committed, "nobody"); err == nil {
		t.Error("rerun accepted an order the ledger never claimed")
	}
}

// ADR-010, 3A: a held order finishes its leg and is not resumed past it.
func TestAHeldOrderIsNotResumedPastItsHalt(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.approve("alpha", "confirm-design")
	factory.commitOrder(orderSpec{id: "alpha", status: "hold"})

	factory.tick()

	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 1 {
		t.Errorf("a held order was resumed (%d launches)", len(launches))
	}
	factory.assertPhase("alpha", PhaseWaiting)
}

func TestTheRunStateBelongsToTheRun(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.tick()
	factory.tick()
	state, err := loadRunState(factory.orders()["alpha"].Claim.Worktree, "alpha")
	if err != nil || state == nil || !state.IsStageCompleted("analyst") {
		t.Errorf("run state = %+v, %v", state, err)
	}
	_ = orchestrator.RunStateFileName
}
