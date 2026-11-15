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

func TestCloseableAcceptsParkedAndStopped(t *testing.T) {
	for _, phase := range []Phase{PhaseParked, PhaseStopped} {
		orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: phase}}
		order, err := closeable(orders, "alpha", "https://example.com/pr/1")
		if err != nil || order.ID != "alpha" {
			t.Errorf("closeable(%s) = %v, %v, want alpha accepted", phase, order, err)
		}
	}
}

func TestCloseableRefusesEveryOtherPhase(t *testing.T) {
	for _, phase := range []Phase{
		PhaseClaimed, PhaseRunning, PhaseWaiting, PhaseRetrying,
		PhaseCompleted, PhaseCommitted, PhasePushed, PhaseProposed, PhaseShipped,
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
		PhaseClaimed, PhaseRunning, PhaseWaiting, PhaseRetrying,
		PhaseCompleted, PhaseCommitted, PhasePushed, PhaseProposed,
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

func TestCloseRefusesAnOrderThatIsNotParkedOrStopped(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	factory.assertPhase("alpha", PhaseWaiting)

	closer := Closer{Layout: Layout{Root: factory.root}, Repository: factory.supervisor.deps.Repository}
	before, err := factory.supervisor.ledger.Entries()
	if err != nil {
		t.Fatal(err)
	}
	_, err = closer.Close(factory.supervisor.ledger, factory.orders(),
		ShippedByHand{Order: "alpha", URL: "https://example.com/pr/1", Approver: "oscar"})
	if !errors.Is(err, ErrNotApplicable) {
		t.Errorf("err = %v, want ErrNotApplicable for a waiting order", err)
	}
	after, err := factory.supervisor.ledger.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Error("the ledger changed although close was refused")
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
