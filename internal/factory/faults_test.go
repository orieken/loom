package factory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/worktree"
)

// Every failure the supervisor can meet outside itself is reported, never
// swallowed into a wrong phase.

var errInjected = errors.New("injected failure")

// faultyRepository fails the named operation and delegates the rest to git.
type faultyRepository struct {
	*worktree.Git
	failing map[string]bool
}

func (r faultyRepository) ListFiles(commit, dir string) ([]string, error) {
	if r.failing["list:"+dir] {
		return nil, errInjected
	}
	return r.Git.ListFiles(commit, dir)
}

func (r faultyRepository) ShowFile(commit, path string) ([]byte, error) {
	if r.failing["show"] {
		return nil, errInjected
	}
	return r.Git.ShowFile(commit, path)
}

func (r faultyRepository) AddWorktree(path, branch, base string) error {
	if r.failing["worktree"] {
		return errInjected
	}
	return r.Git.AddWorktree(path, branch, base)
}

func (r faultyRepository) RemoveWorktree(path string) error {
	if r.failing["remove"] {
		return errInjected
	}
	return r.Git.RemoveWorktree(path)
}

func failing(factory *testFactory, operations ...string) {
	set := map[string]bool{}
	for _, operation := range operations {
		set[operation] = true
	}
	factory.supervisor.deps.Repository = faultyRepository{Git: worktree.New(factory.root), failing: set}
}

func TestReadIntakeReportsEachGitFailure(t *testing.T) {
	for _, operation := range []string{"list:intake", "list:features", "show"} {
		t.Run(operation, func(t *testing.T) {
			factory := newTestFactory(t, orderSpec{id: "alpha"})
			failing(factory, operation)
			if err := factory.supervisor.Tick(); !errors.Is(err, errInjected) {
				t.Errorf("tick err = %v, want the injected failure", err)
			}
		})
	}
}

func TestAClaimThatCannotMakeItsWorktreeFails(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	failing(factory, "worktree")
	if err := factory.supervisor.Tick(); !errors.Is(err, errInjected) {
		t.Errorf("tick err = %v, want the worktree failure", err)
	}
	if launches := factory.launcher.launchesOf("alpha"); len(launches) != 0 {
		t.Error("a run launched without its worktree")
	}
}

// A worktree that cannot be removed is reported; the order is still shipped.
func TestAShippedOrderWhoseWorktreeCannotBeRemovedStaysShipped(t *testing.T) {
	factory := newFinishingFactory(t, "branch")
	failing(factory, "remove")
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.assertPhase("alpha", PhaseShipped)
}

func TestAShippedOrderWhoseRunCannotBeArchivedKeepsItsWorktree(t *testing.T) {
	factory := newFinishingFactory(t, "branch")
	blocker := filepath.Join(factory.root, StateDir, "runs")
	writeTestFile(t, blocker, "a file where the archive directory belongs")
	factory.approve("alpha", GateCommit)
	factory.tick()

	factory.assertPhase("alpha", PhaseShipped)
	if _, err := os.Stat(factory.orders()["alpha"].Claim.Worktree); err != nil {
		t.Error("the worktree was removed although its run was never archived")
	}
}

func TestTheInboxReportsAnUnreadableRunState(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{code: exitWaitingApproval, state: waitingState})
	factory.tick()
	factory.tick()
	writeTestFile(t, RunStatePath(factory.orders()["alpha"].Claim.Worktree, "alpha"), "{not json")

	committed, _ := ReadIntake(worktree.New(factory.root), "main", []string{"deliver-feature"})
	if _, err := Inbox(factory.orders(), committed, openRealTree); err == nil || !strings.Contains(err.Error(), "inbox for alpha") {
		t.Errorf("inbox err = %v, want the unreadable state reported", err)
	}
	if err := factory.supervisor.Tick(); err != nil {
		t.Fatalf("tick: %v", err)
	}
	factory.approveRaw("alpha", "confirm-design")
	if err := factory.supervisor.Tick(); err == nil {
		t.Error("acting on an approval over unreadable state succeeded")
	}
}

func (f *testFactory) approveRaw(id, gate string) {
	f.t.Helper()
	if err := f.supervisor.ledger.Append(Entry{Event: EventApproved, Order: id, Gate: gate, Digest: "x"}); err != nil {
		f.t.Fatal(err)
	}
}

func openRealTree(dir string) Tree { return worktree.New(dir) }

// faultyTree fails the named operation.
type faultyTree struct {
	Tree
	failing string
}

func (t faultyTree) Digest() (string, error) {
	if t.failing == "digest" {
		return "", errInjected
	}
	return t.Tree.Digest()
}

func (t faultyTree) ChangedPaths() ([]string, error) {
	if t.failing == "changed" {
		return nil, errInjected
	}
	return t.Tree.ChangedPaths()
}

func (t faultyTree) CommitAll(message string, exclude []string) (string, error) {
	if t.failing == "commit" {
		return "", errInjected
	}
	return t.Tree.CommitAll(message, exclude)
}

func faultyTrees(operation string) TreeOpener {
	return func(dir string) Tree { return faultyTree{Tree: worktree.New(dir), failing: operation} }
}

func TestTheInboxReportsATreeItCannotRead(t *testing.T) {
	for _, operation := range []string{"digest", "changed"} {
		t.Run(operation, func(t *testing.T) {
			factory := newFinishingFactory(t, "pr")
			committed, _ := ReadIntake(worktree.New(factory.root), "main", []string{"deliver-feature"})
			if _, err := Inbox(factory.orders(), committed, faultyTrees(operation)); !errors.Is(err, errInjected) {
				t.Errorf("inbox err = %v, want the tree failure", err)
			}
		})
	}
}

func TestACommitThatFailsIsReported(t *testing.T) {
	for _, operation := range []string{"digest", "commit"} {
		t.Run(operation, func(t *testing.T) {
			factory := newFinishingFactory(t, "pr")
			factory.approve("alpha", GateCommit)
			factory.supervisor.deps.Trees = faultyTrees(operation)
			if err := factory.supervisor.Tick(); !errors.Is(err, errInjected) {
				t.Errorf("tick err = %v, want the %s failure", err, operation)
			}
			factory.assertPhase("alpha", PhaseCompleted)
		})
	}
}

// A completed `finish: none` order and a running one ask nothing of anyone.
func TestTheInboxIsEmptyForWorkThatNeedsNobody(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"}, orderSpec{id: "bravo"})
	factory.launcher.script("bravo", run{hold: true, state: completedState})
	factory.tick()
	if items := factory.inbox(); len(items) != 0 {
		t.Errorf("inbox = %+v, want nothing for running work", items)
	}
	order := &OrderState{ID: "x", Phase: PhaseCompleted, Claim: Claim{Finish: "none"}}
	if items, err := commitItem(order, openRealTree); err != nil || items != nil {
		t.Errorf("commitItem(finish none) = %+v, %v", items, err)
	}
}

func TestStopAndResumeReportALedgerTheyCannotWrite(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, blocker, "x")
	ledger := OpenLedger(filepath.Join(blocker, "ledger.jsonl"))
	running := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseRunning, PID: 1 << 22}}
	if err := RequestStop(ledger, running, "alpha"); err == nil {
		t.Error("a stop was accepted although it could not be recorded")
	}
	stopped := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseStopped}}
	if err := RequestResume(ledger, stopped, "nobody"); err == nil {
		t.Error("a resume for an unknown order was accepted")
	}
}

func TestTheLauncherReportsALogItCannotOpen(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, blocker, "x")
	if _, err := (ExecLauncher{Executable: "/bin/true"}).Launch(LaunchRequest{LogPath: filepath.Join(blocker, "logs", "a.log")}); err == nil {
		t.Error("a launch with an uncreatable log directory succeeded")
	}
	directory := t.TempDir()
	if _, err := (ExecLauncher{Executable: "/bin/true"}).Launch(LaunchRequest{LogPath: directory}); err == nil {
		t.Error("a launch whose log is a directory succeeded")
	}
}

func TestPrepareReportsAnUnwritableIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ignoredDirectory(dir); err == nil {
		t.Error("writing .gitignore over a directory succeeded")
	}
}
