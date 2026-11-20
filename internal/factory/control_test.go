package factory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/intake"
)

func TestIsHTTPURL(t *testing.T) {
	for url, want := range map[string]bool{
		"https://github.com/orieken/saturday-monorepo/pull/23": true,
		"http://example.com/pr/1":                              true,
		"":                                                     false,
		"ftp://example.com/pr/1":                               false,
		"github.com/orieken/pull/1":                            false,
		"https://":                                             false,
		"not a url at all \x7f":                                false,
		// An ANSI escape sequence is a control character; url.Parse already
		// refuses it, so a --shipped-by value carrying one never reaches
		// status's detail column or close's own success line.
		"https://example.com/pr/1\x1b[31m": false,
	} {
		if got := isHTTPURL(url); got != want {
			t.Errorf("isHTTPURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestCloseableRefusesAnUnknownOrder(t *testing.T) {
	orders := map[string]*OrderState{}
	if _, err := closeable(orders, "nobody", "https://example.com/pr/1"); !errors.Is(err, ErrNotApplicable) {
		t.Errorf("err = %v, want ErrNotApplicable for an unknown order", err)
	}
}

func TestCloseableRefusesAnInvalidURL(t *testing.T) {
	orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseParked}}
	for _, bad := range []string{"", "not-a-url", "ftp://example.com/1"} {
		if _, err := closeable(orders, "alpha", bad); !errors.Is(err, ErrNotApplicable) {
			t.Errorf("closeable(%q) err = %v, want ErrNotApplicable", bad, err)
		}
	}
}

// TestCloseableAcceptsPhasesWaitingOnAHuman covers every phase closeable
// accepts: parked, stopped, waiting at a run gate, completed and awaiting
// its commit approval, and proposed and awaiting its pull-request approval
// — the phases where an order is waiting on a human with no process of its
// own running.
func TestCloseableAcceptsPhasesWaitingOnAHuman(t *testing.T) {
	for _, phase := range []Phase{PhaseParked, PhaseStopped, PhaseWaiting, PhaseCompleted, PhaseProposed} {
		orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: phase}}
		order, err := closeable(orders, "alpha", "https://example.com/pr/1")
		if err != nil || order.ID != "alpha" {
			t.Errorf("closeable(%s) = %v, %v, want alpha accepted", phase, order, err)
		}
	}
}

func TestCloseableRefusesEveryOtherPhase(t *testing.T) {
	for _, phase := range []Phase{
		PhaseClaimed, PhaseRunning, PhaseRetrying, PhaseCommitted, PhasePushed, PhaseShipped,
	} {
		orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: phase}}
		if _, err := closeable(orders, "alpha", "https://example.com/pr/1"); !errors.Is(err, ErrNotApplicable) {
			t.Errorf("closeable(%s) err = %v, want ErrNotApplicable", phase, err)
		}
	}
}

// TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst covers the
// per-phase refusal scenarios: the message must name the order's current
// phase, and tell the operator to stop it first (or, for an order already
// shipped, say so instead of the stop instruction).
func TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst(t *testing.T) {
	for _, phase := range []Phase{
		PhaseClaimed, PhaseRunning, PhaseRetrying, PhaseCommitted, PhasePushed,
	} {
		orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: phase}}
		_, err := closeable(orders, "alpha", "https://example.com/pr/1")
		message := err.Error()
		if !strings.Contains(message, string(phase)) {
			t.Errorf("closeable(%s) err = %q, want the phase named", phase, message)
		}
		if !strings.Contains(message, "stop it first") {
			t.Errorf("closeable(%s) err = %q, want an instruction to stop it first", phase, message)
		}
	}

	orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseShipped}}
	_, err := closeable(orders, "alpha", "https://example.com/pr/1")
	message := err.Error()
	if !strings.Contains(message, string(PhaseShipped)) || !strings.Contains(message, "already shipped") {
		t.Errorf("closeable(shipped) err = %q, want the phase named and noted as already shipped", message)
	}
}

// TestCloseShipsAHandFinishedOrderAndUnblocksItsDependent exercises the
// golden path: a budget-parked order, closed by hand, is folded to shipped
// with its URL, its worktree is archived then removed, and a dependent
// order's depends-on is satisfied with no edit to either order.
func TestCloseShipsAHandFinishedOrderAndUnblocksItsDependent(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})
	factory.tick()
	factory.tick()
	factory.assertPhase("alpha", PhaseParked)
	worktree := factory.orders()["alpha"].Claim.Worktree
	entriesBefore := len(mustEntries(t, factory.supervisor.ledger))

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	request := ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(), request)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning != "" {
		t.Fatalf("close warning = %q, want full success", warning)
	}

	orders := factory.orders()
	assertShippedWithURL(t, orders["alpha"], request.URL)
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Error("the worktree was not removed")
	}
	if !dependenciesShipped(intake.Order{DependsOn: []string{"alpha"}}, orders) {
		t.Error("a dependent order's depends-on was not satisfied by the closed order")
	}
	assertExactlyOneNewShippedEntry(t, factory.supervisor.ledger, entriesBefore, request)
}

func assertShippedWithURL(t *testing.T, order *OrderState, url string) {
	t.Helper()
	if order.Phase != PhaseShipped {
		t.Errorf("phase = %s, want shipped", order.Phase)
	}
	if order.URL != url {
		t.Errorf("url = %q, want %q", order.URL, url)
	}
}

// assertExactlyOneNewShippedEntry checks that close appended exactly one
// ledger entry, that it carries the requested url and approver, and that its
// event is "shipped" — the full shape the golden-path scenarios require.
func assertExactlyOneNewShippedEntry(t *testing.T, ledger *Ledger, before int, request ShippedByHand) {
	t.Helper()
	entries, err := ledger.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != before+1 {
		t.Fatalf("entries = %d, want exactly %d (one new entry)", len(entries), before+1)
	}
	last := entries[len(entries)-1]
	if last.Event != EventShipped || last.URL != request.URL || last.ApprovedBy != request.Approver {
		t.Errorf("last entry = %+v, want a shipped entry carrying the url and approver", last)
	}
}

// TestCloseShipsAHandFinishedStoppedOrder covers the second close-golden-path
// scenario: a *stopped* order (rather than a parked one) is also closeable,
// and its ledger entry carries the url and approver.
func TestCloseShipsAHandFinishedStoppedOrder(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	factory.tick()
	if err := factory.supervisor.ledger.Append(Entry{Event: EventStopRequested, Order: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := factory.launcher.running["alpha"].Interrupt(); err != nil {
		t.Fatal(err)
	}
	factory.tick()
	factory.assertPhase("alpha", PhaseStopped)
	entriesBefore := len(mustEntries(t, factory.supervisor.ledger))

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	request := ShippedByHand{Order: "alpha", URL: "https://github.com/example/repo/pull/1", Approver: "oscar"}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(), request)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning != "" {
		t.Fatalf("close warning = %q, want full success", warning)
	}
	assertShippedWithURL(t, factory.orders()["alpha"], request.URL)
	assertExactlyOneNewShippedEntry(t, factory.supervisor.ledger, entriesBefore, request)
}

// TestCloseRefusesARunningOrder covers AC3: an order whose run process is
// still alive is refused, and the ledger is unchanged.
func TestCloseRefusesARunningOrder(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitCompleted, state: completedState, hold: true})
	factory.tick()
	factory.assertPhase("alpha", PhaseRunning)

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	before, err := factory.supervisor.ledger.Entries()
	if err != nil {
		t.Fatal(err)
	}
	_, err = closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"})
	if !errors.Is(err, ErrNotApplicable) {
		t.Errorf("err = %v, want ErrNotApplicable for a running order", err)
	}
	after, err := factory.supervisor.ledger.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Error("the ledger changed although close was refused")
	}
}

// TestCloseShipsAWaitingOrderAndUnblocksItsDependent covers AC1 for an order
// waiting at a run gate: closing it ships it exactly as a parked order does.
func TestCloseShipsAWaitingOrderAndUnblocksItsDependent(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.assertPhase("alpha", PhaseWaiting)
	worktree := factory.orders()["alpha"].Claim.Worktree
	entriesBefore := len(mustEntries(t, factory.supervisor.ledger))

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	request := ShippedByHand{Order: "alpha", URL: "https://example.com/pr/2", Approver: "oscar"}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(), request)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning != "" {
		t.Fatalf("close warning = %q, want full success", warning)
	}

	orders := factory.orders()
	assertShippedWithURL(t, orders["alpha"], request.URL)
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Error("the worktree was not removed")
	}
	if !dependenciesShipped(intake.Order{DependsOn: []string{"alpha"}}, orders) {
		t.Error("a dependent order's depends-on was not satisfied by the closed order")
	}
	assertExactlyOneNewShippedEntry(t, factory.supervisor.ledger, entriesBefore, request)
}

// TestCloseShipsACompletedOrderAwaitingCommitApproval covers AC1 for an order
// completed and waiting for its commit to be approved.
func TestCloseShipsACompletedOrderAwaitingCommitApproval(t *testing.T) {
	factory := newFinishingFactory(t, "pr", "src/**")
	factory.assertPhase("alpha", PhaseCompleted)
	worktree := factory.orders()["alpha"].Claim.Worktree
	entriesBefore := len(mustEntries(t, factory.supervisor.ledger))

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	request := ShippedByHand{Order: "alpha", URL: "https://example.com/pr/3", Approver: "oscar"}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(), request)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning != "" {
		t.Fatalf("close warning = %q, want full success", warning)
	}

	orders := factory.orders()
	assertShippedWithURL(t, orders["alpha"], request.URL)
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Error("the worktree was not removed")
	}
	assertExactlyOneNewShippedEntry(t, factory.supervisor.ledger, entriesBefore, request)
}

// TestCloseShipsAProposedOrderAwaitingPullRequestApproval covers AC1 for an
// order proposed and waiting for its pull request to be approved. close
// never opens the pull request itself.
func TestCloseShipsAProposedOrderAwaitingPullRequestApproval(t *testing.T) {
	factory := newFinishingFactory(t, "pr")
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.assertPhase("alpha", PhaseProposed)
	worktree := factory.orders()["alpha"].Claim.Worktree
	entriesBefore := len(mustEntries(t, factory.supervisor.ledger))

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	request := ShippedByHand{Order: "alpha", URL: "https://example.com/pr/4", Approver: "oscar"}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(), request)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning != "" {
		t.Fatalf("close warning = %q, want full success", warning)
	}
	if len(factory.pulls.opened) != 0 {
		t.Error("close opened a pull request itself")
	}

	orders := factory.orders()
	assertShippedWithURL(t, orders["alpha"], request.URL)
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Error("the worktree was not removed")
	}
	assertExactlyOneNewShippedEntry(t, factory.supervisor.ledger, entriesBefore, request)
}

// TestSupervisorTickAfterCloseDoesNotRelaunch covers AC4: closing a waiting
// order and running one more supervisor tick never relaunches, commits,
// pushes, or proposes it again.
func TestSupervisorTickAfterCloseDoesNotRelaunch(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.assertPhase("alpha", PhaseWaiting)

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	if _, err := closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/5", Approver: "oscar"}); err != nil {
		t.Fatalf("close: %v", err)
	}
	entriesAfterClose := len(mustEntries(t, factory.supervisor.ledger))
	launchesAfterClose := len(factory.launcher.launchesOf("alpha"))

	factory.tick()

	factory.assertPhase("alpha", PhaseShipped)
	if got := len(mustEntries(t, factory.supervisor.ledger)); got != entriesAfterClose {
		t.Errorf("ledger entries after one more tick = %d, want %d (no new entry)", got, entriesAfterClose)
	}
	if got := len(factory.launcher.launchesOf("alpha")); got != launchesAfterClose {
		t.Errorf("launches after one more tick = %d, want %d (no relaunch)", got, launchesAfterClose)
	}
}

// TestSupervisorTickAfterCloseIgnoresAStaleCommitApproval covers AC2: an
// order closed while a commit-gate approval is already in the ledger and
// not yet acted on is never committed on a later tick.
func TestSupervisorTickAfterCloseIgnoresAStaleCommitApproval(t *testing.T) {
	factory := newFinishingFactory(t, "pr", "src/**")
	factory.approve("alpha", GateCommit)
	factory.assertPhase("alpha", PhaseCompleted)

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	if _, err := closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/6", Approver: "oscar"}); err != nil {
		t.Fatalf("close: %v", err)
	}

	factory.tick()

	factory.assertPhase("alpha", PhaseShipped)
	if hasEvent(factory, "alpha", EventCommitted) {
		t.Error("the supervisor committed a closed order's stale approval")
	}
}

// TestSupervisorTickAfterCloseIgnoresAStaleOpenPRApproval covers AC2: an
// order closed while an open-pr approval is already in the ledger and not
// yet acted on never has a pull request opened for it on a later tick.
func TestSupervisorTickAfterCloseIgnoresAStaleOpenPRApproval(t *testing.T) {
	factory := newFinishingFactory(t, "pr")
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.approve("alpha", GateOpenPR)
	factory.assertPhase("alpha", PhaseProposed)

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	if _, err := closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/7", Approver: "oscar"}); err != nil {
		t.Fatalf("close: %v", err)
	}

	factory.tick()

	factory.assertPhase("alpha", PhaseShipped)
	if len(factory.pulls.opened) != 0 {
		t.Error("the supervisor opened a pull request for a closed order's stale approval")
	}
}

// TestCloseKeepsTheWorktreeWhenArchivingFails matches
// TestAShippedOrderWhoseRunCannotBeArchivedKeepsItsWorktree's own scenario,
// through Close rather than the supervisor's own ship.
func TestCloseKeepsTheWorktreeWhenArchivingFails(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})
	factory.tick()
	factory.tick()
	blocker := filepath.Join(factory.root, StateDir, "runs")
	writeTestFile(t, blocker, "a file where the archive directory belongs")

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning == "" {
		t.Fatal("close reported full success although archiving failed")
	}
	order := factory.orders()["alpha"]
	if order.Phase != PhaseShipped {
		t.Errorf("phase = %s, want shipped even though archiving failed", order.Phase)
	}
	if _, err := os.Stat(order.Claim.Worktree); err != nil {
		t.Error("the worktree was removed although its run was never archived")
	}
}

// TestCloseKeepsTheLedgerEntryWhenRemovingTheWorktreeFails matches
// TestAShippedOrderWhoseWorktreeCannotBeRemovedStaysShipped's own scenario,
// through Close.
func TestCloseKeepsTheLedgerEntryWhenRemovingTheWorktreeFails(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitBudget, state: budgetState})
	factory.tick()
	factory.tick()
	failing(factory, "remove")

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	warning, err := closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if warning == "" {
		t.Fatal("close reported full success although removing the worktree failed")
	}
	if factory.orders()["alpha"].Phase != PhaseShipped {
		t.Error("the order is not recorded shipped although only the worktree removal failed")
	}
}

func TestCloseReportsALedgerItCannotWrite(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, blocker, "x")
	ledger := OpenLedger(filepath.Join(blocker, "ledger.jsonl"))
	orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseParked}}
	closer := Closer{Layout: Layout{Root: t.TempDir()}, Repository: nil}
	if _, err := closer.Close(ledger, orders, ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"}); err == nil {
		t.Error("close was accepted although it could not be recorded")
	}
}

func TestApplyShippedIgnoresAnEmptyURL(t *testing.T) {
	order := &OrderState{ID: "alpha", URL: "https://example.com/already-there"}
	applyShipped(order, Entry{Event: EventShipped, Order: "alpha"})
	if order.URL != "https://example.com/already-there" {
		t.Errorf("url = %q, want the supervisor's own shipped line to leave it alone", order.URL)
	}
	applyShipped(order, Entry{Event: EventShipped, Order: "alpha", URL: "https://example.com/pr/9"})
	if order.URL != "https://example.com/pr/9" {
		t.Errorf("url = %q, want the hand-closed url folded on", order.URL)
	}
}
