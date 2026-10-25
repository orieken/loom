package factory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX processes and signals")
	}
}

func TestExecLauncherRunsInTheDirectoryAndLogs(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "logs", "alpha.log")
	process, err := ExecLauncher{Executable: "/bin/sh"}.Launch(LaunchRequest{
		Dir: dir, Args: []string{"-c", "pwd; exit 3"}, LogPath: log,
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	<-process.Done()
	if process.ExitCode() != 3 || process.PID() <= 0 {
		t.Errorf("exit code %d, pid %d; want 3 and a real pid", process.ExitCode(), process.PID())
	}
	output, _ := os.ReadFile(log)
	resolved, _ := filepath.EvalSymlinks(dir)
	if !strings.Contains(string(output), resolved) && !strings.Contains(string(output), dir) {
		t.Errorf("log = %q, want the run's working directory", output)
	}
}

func TestExecLauncherReportsAMissingExecutable(t *testing.T) {
	_, err := ExecLauncher{Executable: filepath.Join(t.TempDir(), "no-such-loom")}.Launch(LaunchRequest{
		Dir: t.TempDir(), LogPath: filepath.Join(t.TempDir(), "x.log"),
	})
	if err == nil {
		t.Error("launching a missing binary succeeded")
	}
}

// `loom factory stop` records the request first, then interrupts the real
// process, which exits on SIGINT.
func TestRequestStopRecordsThenInterrupts(t *testing.T) {
	skipOnWindows(t)
	process := startInterruptibleProcess(t)
	ledger := OpenLedger(filepath.Join(t.TempDir(), "ledger.jsonl"))
	orders := map[string]*OrderState{"alpha": {ID: "alpha", Phase: PhaseRunning, PID: process.PID()}}

	if err := RequestStop(ledger, orders, "alpha"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	waitForExit(t, process)
	if process.ExitCode() != exitInterrupted {
		t.Errorf("exit code = %d, want %d", process.ExitCode(), exitInterrupted)
	}
	if entries, _ := ledger.Entries(); len(entries) != 1 || entries[0].Event != EventStopRequested {
		t.Errorf("ledger = %+v, want the stop request recorded", entries)
	}
}

func TestProcessAliveTellsALiveProcessFromAnExitedOne(t *testing.T) {
	skipOnWindows(t)
	process := startInterruptibleProcess(t)
	if !processAlive(process.PID()) {
		t.Fatal("a running process reads as dead")
	}
	if err := process.Interrupt(); err != nil {
		t.Fatal(err)
	}
	waitForExit(t, process)
	if processAlive(process.PID()) || processAlive(0) {
		t.Error("an exited process, or pid 0, reads as alive")
	}
}

// startInterruptibleProcess runs a shell that exits 130 on SIGINT, as `loom
// run` does after checkpointing.
func startInterruptibleProcess(t *testing.T) Process {
	t.Helper()
	process, err := ExecLauncher{Executable: "/bin/sh"}.Launch(LaunchRequest{
		Dir: t.TempDir(), Args: []string{"-c", "trap 'exit 130' INT; while :; do sleep 0.05; done"},
		LogPath: filepath.Join(t.TempDir(), "x.log"),
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let the shell install its trap
	return process
}

func waitForExit(t *testing.T, process Process) {
	t.Helper()
	select {
	case <-process.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the process ignored the interrupt")
	}
}

func TestASecondSupervisorCannotTakeTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supervisor.lock")
	first, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := AcquireLock(path); !errors.Is(err, ErrSupervisorRunning) {
		t.Errorf("second lock err = %v, want ErrSupervisorRunning", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	_ = again.Release()
	if _, err := AcquireLock(filepath.Join(t.TempDir(), "missing", "x.lock")); err == nil {
		t.Error("a lock in a missing directory was taken")
	}
}

func TestRunTicksUntilCancelledThenDrains(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- factory.supervisor.Run(ctx, time.Hour) }()
	waitFor(t, func() bool { return len(factory.launcher.launchesOf("alpha")) == 1 })

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
	factory.assertPhase("alpha", PhaseStopped)
}

// Run wakes as soon as a run it started ends, rather than at the interval.
func TestRunWakesWhenARunEnds(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error)
	go func() { done <- factory.supervisor.Run(ctx, time.Hour) }()
	waitFor(t, func() bool { return len(factory.launcher.launchesOf("alpha")) == 1 })

	factory.launcher.release("alpha")
	waitFor(t, func() bool { return factory.phase("alpha") == PhaseShipped })
	cancel()
	<-done
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestATickFailsWhenTheIntakeBranchIsMissing(t *testing.T) {
	factory := newTestFactory(t)
	factory.supervisor.config.IntakeBranch = "no-such-branch"
	if err := factory.supervisor.Tick(); err == nil {
		t.Error("a tick against a missing branch succeeded")
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	supervisor := New(Config{RetryBase: time.Minute}, Dependencies{})
	config := supervisor.config
	if config.MaxConcurrent != DefaultMaxConcurrent || config.Provider != DefaultProvider || config.RetryMax != time.Minute {
		t.Errorf("config = %+v, want the defaults", config)
	}
	supervisor.say("output goes nowhere by default")
}

func TestJudgeReadsTheOutcomeFromStateThenExitCode(t *testing.T) {
	code := func(value int) *int { return &value }
	cases := []struct {
		name  string
		exit  *int
		state *orchestrator.RunState
		want  outcomeKind
		class orchestrator.FailureClass
	}{
		{"no state", code(1), nil, outcomeFailed, orchestrator.FailureTerminal},
		{"no state, orphaned", nil, nil, outcomeFailed, orchestrator.FailureTerminal},
		{"completed", code(0), completedState("x"), outcomeCompleted, ""},
		{"completed, orphaned", nil, completedState("x"), outcomeCompleted, ""},
		{"waiting", code(3), waitingState("x"), outcomeWaiting, ""},
		{"budget", code(4), budgetState("x"), outcomeBudget, ""},
		{"interrupted by exit code", code(130), completedState("x"), outcomeInterrupted, orchestrator.FailureTransient},
		{"interrupted stage", nil, interruptedState("x"), outcomeInterrupted, orchestrator.FailureTransient},
		{"failed without a failed stage", code(1), completedState("x"), outcomeFailed, orchestrator.FailureTerminal},
		{"transient stage failure", code(1), failedState(orchestrator.FailureTransient)("x"), outcomeFailed, orchestrator.FailureTransient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(tc.exit, tc.state)
			if got.kind != tc.want || got.class != tc.class {
				t.Errorf("judge = %s/%q, want %s/%q", got.kind, got.class, tc.want, tc.class)
			}
		})
	}
}

func TestAnUnverifiableBudgetSaysWhichStagesReportedNothing(t *testing.T) {
	state := budgetState("x")
	state.BudgetHalt.Unmeasured = []string{"analyst"}
	if got := judge(nil, state); !strings.Contains(got.reason, "cannot be verified") {
		t.Errorf("reason = %q", got.reason)
	}
}

func TestLedgerRefusesACorruptMiddleLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	writeTestFile(t, path, "{\"event\":\"claimed\",\"order\":\"a\"}\nnot json\n{\"event\":\"launched\",\"order\":\"a\"}\n")
	if _, err := OpenLedger(path).Entries(); !errors.Is(err, ErrCorruptLedger) {
		t.Errorf("err = %v, want ErrCorruptLedger", err)
	}
	if entries, err := OpenLedger(filepath.Join(t.TempDir(), "absent.jsonl")).Entries(); err != nil || entries != nil {
		t.Errorf("an absent ledger = %v, %v; want empty", entries, err)
	}
}

func TestLedgerAppendFailsWhereItCannotWrite(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, blocker, "x")
	if err := OpenLedger(filepath.Join(blocker, "ledger.jsonl")).Append(Entry{Event: EventClaimed}); err == nil {
		t.Error("appending under a regular file succeeded")
	}
}

func TestReplayIgnoresLinesForUnclaimedOrders(t *testing.T) {
	orders := Replay([]Entry{{Event: EventLaunched, Order: "ghost"}, {Event: "from-the-future", Order: "ghost"}})
	if len(orders) != 0 {
		t.Errorf("orders = %v, want none", orders)
	}
}

func TestTitleIsTheFirstHeading(t *testing.T) {
	if got := Title("intro\n\n# Lock accounts\n## Detail\n"); got != "Lock accounts" {
		t.Errorf("Title = %q", got)
	}
	if got := Title("no heading here"); got != "" {
		t.Errorf("Title = %q, want empty", got)
	}
}

func TestSubjectFallsBackForAnUnknownTypeAndNoTitle(t *testing.T) {
	order := &OrderState{ID: "alpha", Claim: Claim{Type: "unknown"}}
	if got := subject(order); got != "chore(alpha): deliver alpha" {
		t.Errorf("subject = %q", got)
	}
}

func TestSpendLinesMarkUnmeasuredStages(t *testing.T) {
	lines := spendLines(&Spend{TotalUSD: 2, PerStage: []orchestrator.StageSpend{
		{Stage: "analyst", CostUSD: 2, Measured: true}, {Stage: "developer"},
	}})
	if strings.Join(lines, "|") != "total $2.0000|analyst $2.0000|developer unmeasured" {
		t.Errorf("lines = %v", lines)
	}
}

func TestANilProposalHasNoDigest(t *testing.T) {
	var proposal *Proposal
	if proposal.Digest() != "" {
		t.Error("a nil proposal has a digest")
	}
	if digest, stages := BindingDigest(nil); digest != "" || stages != nil {
		t.Error("nil state has a binding digest")
	}
}

func TestInScopeMatchesGlobsAndDirectoryTrees(t *testing.T) {
	scope := []string{"src/**", "*.md"}
	for path, want := range map[string]bool{"src/a/b.go": true, "README.md": true, "cmd/main.go": false} {
		if got := inScope(path, scope); got != want {
			t.Errorf("inScope(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestGHPullRequestsRunsGhAndReturnsTheURL(t *testing.T) {
	skipOnWindows(t)
	bin := t.TempDir()
	writeTestFile(t, filepath.Join(bin, "gh"), "#!/bin/sh\necho \"$@\" > args.txt\necho Creating\necho https://github.com/o/r/pull/7\n")
	if err := os.Chmod(filepath.Join(bin, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()

	url, err := GHPullRequests{}.Open(dir, Proposal{Title: "feat(a): x", Body: "body", Base: "main", Head: "factory/a"})
	if err != nil || url != "https://github.com/o/r/pull/7" {
		t.Fatalf("Open = %q, %v", url, err)
	}
	args, _ := os.ReadFile(filepath.Join(dir, "args.txt"))
	if !strings.Contains(string(args), "pr create --base main --head factory/a --title feat(a): x") {
		t.Errorf("gh args = %q", args)
	}

	writeTestFile(t, filepath.Join(bin, "gh"), "#!/bin/sh\necho no auth >&2\nexit 1\n")
	if _, err := (GHPullRequests{Timeout: time.Second}).Open(dir, Proposal{}); err == nil || !strings.Contains(err.Error(), "no auth") {
		t.Errorf("err = %v, want gh's failure reported", err)
	}
}

type failingPulls struct{ calls int }

func (p *failingPulls) Open(string, Proposal) (string, error) {
	p.calls++
	return "", errors.New("rate limited")
}

func TestAFailedPullRequestIsRecordedAndRetried(t *testing.T) {
	factory := newFinishingFactory(t, "pr")
	pulls := &failingPulls{}
	factory.supervisor.deps.Pulls = pulls
	factory.approve("alpha", GateCommit)
	factory.tick()
	factory.approve("alpha", GateOpenPR)
	factory.tick()
	factory.tick()

	factory.assertPhase("alpha", PhaseProposed)
	if pulls.calls != 2 || !strings.Contains(factory.orders()["alpha"].Reason, "rate limited") {
		t.Errorf("calls = %d, reason = %q; want a retry each tick and the reason kept", pulls.calls, factory.orders()["alpha"].Reason)
	}
}

func TestLayoutPaths(t *testing.T) {
	layout := Layout{Root: "/repo"}
	if layout.LockPath() != filepath.Join("/repo", StateDir, "supervisor.lock") {
		t.Errorf("lock path = %s", layout.LockPath())
	}
	if err := (Layout{Root: filepath.Join(t.TempDir(), "file")}).Prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, blocker, "x")
	if err := (Layout{Root: blocker}).Prepare(); err == nil {
		t.Error("preparing under a regular file succeeded")
	}
}

// One pass for cron: claim, wait for the runs to halt, settle and finish.
func TestOnceClaimsWaitsAndFinishes(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"}, orderSpec{id: "bravo"}, orderSpec{id: "charlie"})
	if err := factory.supervisor.Once(); err != nil {
		t.Fatalf("once: %v", err)
	}
	factory.assertPhase("alpha", PhaseShipped)
	factory.assertPhase("bravo", PhaseShipped)
	factory.assertPhase("charlie", "")
}

// Every run writes its delivery record under docs/features/<id>/, which is
// part of the order's work wherever its scope points.
func TestTheOrdersOwnDeliveryRecordIsInScope(t *testing.T) {
	scope := []string{"calc/**"}
	for path, want := range map[string]bool{
		"docs/features/divide/run-state.json": true, "docs/features/other/run-state.json": false,
		"calc/divide.go": true, "docs/features/divide-two/x": false,
	} {
		if got := InOrderScope(path, "divide", scope); got != want {
			t.Errorf("InOrderScope(%s) = %v, want %v", path, got, want)
		}
	}
}

// Claude Code refuses to write any path with a .claude segment, so a
// worktree there leaves every agent unable to write (found by the first
// real-provider run). The worktree must sit outside .claude/, and stay
// ignored by the root checkout without ignoring the rest of .loom/.
func TestWorktreesLiveOutsideClaudeAndIgnoreOnlyThemselves(t *testing.T) {
	root := t.TempDir()
	layout := Layout{Root: root}
	for _, segment := range strings.Split(filepath.ToSlash(layout.WorktreePath("alpha")), "/") {
		if segment == ".claude" {
			t.Fatalf("worktree path %s has a .claude segment; agents cannot write there", layout.WorktreePath("alpha"))
		}
	}
	if err := layout.Prepare(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".loom", "worktrees", ".gitignore")); err != nil {
		t.Errorf("the worktree directory does not ignore itself: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".loom", ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("Prepare wrote .loom/.gitignore, which would hide loom index's files: %v", err)
	}
}
