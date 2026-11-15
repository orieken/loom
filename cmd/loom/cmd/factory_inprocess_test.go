package cmd

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/worktree"
	"github.com/spf13/cobra"
)

// In-process: the same commands the binary tests drive, run here so the
// coverage gate can measure them. Runs are launched as children of this
// test binary acting as loom (see main_child_test.go).

func useFactoryFlags(t *testing.T, flags factoryFlags) {
	t.Helper()
	saved := factoryArgs
	factoryArgs = flags
	t.Cleanup(func() { factoryArgs = saved })
}

func onceAgainstTheMock() factoryFlags {
	return factoryFlags{once: true, maxConcurrent: 2, branch: "main", provider: "mock",
		runArgs: []string{"--mock-stage-cost=0.01"}}
}

func runFactoryCommand(t *testing.T, run func(*cobra.Command, []string) error, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetContext(context.Background())
	err := run(command, args)
	return out.String(), err
}

func TestFactoryCommandsInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	t.Setenv(actAsLoomEnv, "1")
	useFactoryFlags(t, onceAgainstTheMock())

	if output, err := runFactoryCommand(t, runFactoryRun); err != nil {
		t.Fatalf("factory run: %v\n%s", err, output)
	}
	expectFactoryOutput(t, runFactoryStatus, nil, "gate confirm-design")
	expectFactoryOutput(t, runFactoryInbox, nil, "loom factory approve alpha confirm-design")
	expectFactoryOutput(t, runFactoryApprove, []string{"alpha", "confirm-design"}, "Approved confirm-design")
}

func expectFactoryOutput(t *testing.T, run func(*cobra.Command, []string) error, args []string, want string) {
	t.Helper()
	output, err := runFactoryCommand(t, run, args...)
	if err != nil || !strings.Contains(output, want) {
		t.Fatalf("output = %q, err = %v; want %q", output, err, want)
	}
}

func TestFactoryControlsReportTheirRefusalsInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	t.Setenv(actAsLoomEnv, "1")
	useFactoryFlags(t, onceAgainstTheMock())
	useFactoryCloseFlags(t, "https://example.com/pr/1")
	if _, err := runFactoryCommand(t, runFactoryRun); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"stop": runFactoryStop, "resume": runFactoryResume, "rerun": runFactoryRerun, "close": runFactoryClose,
	} {
		if _, err := runFactoryCommand(t, run, "alpha"); err == nil {
			t.Errorf("%s accepted an order waiting at a gate", name)
		}
	}
	if _, err := runFactoryCommand(t, runFactoryApprove, "alpha", "confirm-ship"); err == nil {
		t.Error("approve accepted a gate the order is not waiting on")
	}
}

func TestFactoryRunSupervisesUntilItsContextEnds(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	t.Setenv(actAsLoomEnv, "1")
	flags := onceAgainstTheMock()
	flags.once = false
	useFactoryFlags(t, flags)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	command.SetContext(ctx)

	if err := runFactoryRun(command, nil); err != nil {
		t.Fatalf("factory run: %v", err)
	}
	status, _ := runFactoryCommand(t, runFactoryStatus)
	if !strings.Contains(status, "alpha") {
		t.Errorf("status = %q, want alpha claimed", status)
	}
}

func TestFactoryCommandsRefuseOutsideARepositoryInProcess(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"run": runFactoryRun, "status": runFactoryStatus, "inbox": runFactoryInbox,
	} {
		if _, err := runFactoryCommand(t, run); err == nil || !strings.Contains(err.Error(), "git repository") {
			t.Errorf("%s outside git: err = %v", name, err)
		}
	}
}

func TestFactoryStatusAndInboxStartEmptyInProcess(t *testing.T) {
	t.Chdir(newFactoryProject(t))
	useFactoryFlags(t, onceAgainstTheMock())
	if status, _ := runFactoryCommand(t, runFactoryStatus); !strings.Contains(status, "No orders claimed yet") {
		t.Errorf("status = %q", status)
	}
	if inbox, _ := runFactoryCommand(t, runFactoryInbox); !strings.Contains(inbox, "Nothing is waiting on you") {
		t.Errorf("inbox = %q", inbox)
	}
}

func TestShortDigestAndApprover(t *testing.T) {
	if shortDigest("0123456789abcdef") != "0123456789ab" || shortDigest("abc") != "abc" {
		t.Error("shortDigest does not keep twelve characters")
	}
	if approverName() == "" {
		t.Error("no approver name")
	}
}

// writeLedger puts entries in the factory's ledger directly, to stand up an
// order in a given phase without running anything.
func writeLedger(t *testing.T, root string, entries ...factory.Entry) {
	t.Helper()
	ledger := factory.OpenLedger(factory.Layout{Root: root}.LedgerPath())
	for _, entry := range entries {
		if err := ledger.Append(entry); err != nil {
			t.Fatal(err)
		}
	}
}

func claimedEntry(id string) factory.Entry {
	return factory.Entry{Event: factory.EventClaimed, Order: id, Claim: &factory.Claim{OrderSHA256: "an-older-version"}}
}

func TestFactoryResumeContinuesAStoppedOrderInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	writeLedger(t, root, claimedEntry("alpha"), factory.Entry{Event: factory.EventStopped, Order: "alpha"})
	expectFactoryOutput(t, runFactoryResume, []string{"alpha"}, "Resuming alpha")
}

func TestFactoryRerunReclaimsAnEditedOrderInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	useFactoryFlags(t, onceAgainstTheMock())
	writeLedger(t, root, claimedEntry("alpha"), factory.Entry{Event: factory.EventWaiting, Order: "alpha", Gate: "confirm-design"})
	expectFactoryOutput(t, runFactoryRerun, []string{"alpha"}, "Re-running alpha")
}

func useFactoryCloseFlags(t *testing.T, shippedBy string) {
	t.Helper()
	saved := factoryCloseFlags.shippedBy
	factoryCloseFlags.shippedBy = shippedBy
	t.Cleanup(func() { factoryCloseFlags.shippedBy = saved })
}

// worktreeForClose stands up a real git worktree with a run state file in
// it, the same shape a claimed order's own worktree has, so Close's
// archive-then-remove sequence can fully succeed in this in-process test.
func worktreeForClose(t *testing.T, root, id string) string {
	t.Helper()
	layout := factory.Layout{Root: root}
	path := layout.WorktreePath(id)
	if err := worktree.New(root).AddWorktree(path, factory.BranchFor(id), "main"); err != nil {
		t.Fatal(err)
	}
	state := orchestrator.NewRunState(orchestrator.DefaultDeliverFeaturePlanName, orchestrator.CreatedByExecutor)
	if err := orchestrator.NewStateStore(factory.RunStatePath(path, id)).Save(state); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFactoryCloseShipsAParkedOrderInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	path := worktreeForClose(t, root, "alpha")
	writeLedger(t, root, factory.Entry{Event: factory.EventClaimed, Order: "alpha",
		Claim: &factory.Claim{OrderSHA256: "x", Worktree: path, Branch: factory.BranchFor("alpha")}},
		factory.Entry{Event: factory.EventParked, Order: "alpha"})
	useFactoryCloseFlags(t, "https://github.com/orieken/saturday-monorepo/pull/23")

	expectFactoryOutput(t, runFactoryClose, []string{"alpha"},
		"Closed alpha as shipped, by https://github.com/orieken/saturday-monorepo/pull/23")

	status, _ := runFactoryCommand(t, runFactoryStatus)
	if !strings.Contains(status, string(factory.PhaseShipped)) || !strings.Contains(status, "pull/23") {
		t.Errorf("status = %q, want alpha shipped with its url", status)
	}
	inbox, _ := runFactoryCommand(t, runFactoryInbox)
	if strings.Contains(inbox, "alpha") {
		t.Errorf("inbox = %q, want a closed order gone from the inbox", inbox)
	}
}

func TestFactoryClosePrintsAWarningWhenFinishingFailsInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	writeLedger(t, root, claimedEntry("alpha"), factory.Entry{Event: factory.EventStopped, Order: "alpha"})
	useFactoryCloseFlags(t, "https://example.com/pr/1")

	output, err := runFactoryCommand(t, runFactoryClose, "alpha")
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if !strings.Contains(output, "was not archived") {
		t.Errorf("output = %q, want a warning naming the archiving failure", output)
	}
	status, _ := runFactoryCommand(t, runFactoryStatus)
	if !strings.Contains(status, string(factory.PhaseShipped)) {
		t.Errorf("status = %q, want alpha recorded shipped even though finishing warned", status)
	}
}

func TestFactoryCloseRefusesAWrongPhaseInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	writeLedger(t, root, claimedEntry("alpha"))
	useFactoryCloseFlags(t, "https://example.com/pr/1")
	if _, err := runFactoryCommand(t, runFactoryClose, "alpha"); err == nil {
		t.Error("close accepted an order that is neither parked nor stopped")
	}
}

func TestFactoryCloseRefusesAnInvalidURLInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	writeLedger(t, root, claimedEntry("alpha"), factory.Entry{Event: factory.EventStopped, Order: "alpha"})
	useFactoryCloseFlags(t, "not-a-url")
	if _, err := runFactoryCommand(t, runFactoryClose, "alpha"); err == nil {
		t.Error("close accepted a --shipped-by value that is not an http or https URL")
	}
}

func TestFactoryCloseRefusesAnUnknownOrderInProcess(t *testing.T) {
	root := newFactoryProject(t)
	t.Chdir(root)
	useFactoryCloseFlags(t, "https://example.com/pr/1")
	if _, err := runFactoryCommand(t, runFactoryClose, "nobody"); err == nil {
		t.Error("close accepted an order the ledger does not hold")
	}
}

func TestFactoryStopInterruptsARunningOrderInProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT is POSIX-only")
	}
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	sleeper := exec.Command("sleep", "30")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill(); _, _ = sleeper.Process.Wait() })
	writeLedger(t, root, claimedEntry("alpha"), factory.Entry{Event: factory.EventLaunched, Order: "alpha", PID: sleeper.Process.Pid})
	expectFactoryOutput(t, runFactoryStop, []string{"alpha"}, "Stopping alpha")
}

// A corrupt ledger is reported by every command that reads it.
func TestFactoryCommandsReportACorruptLedgerInProcess(t *testing.T) {
	root := newFactoryProject(t)
	t.Chdir(root)
	useFactoryFlags(t, onceAgainstTheMock())
	useFactoryCloseFlags(t, "https://example.com/pr/1")
	writeTestFile(t, factory.Layout{Root: root}.LedgerPath(), "not json\n{}\n")
	for name, call := range map[string]struct {
		run  func(*cobra.Command, []string) error
		args []string
	}{
		"status": {runFactoryStatus, nil}, "inbox": {runFactoryInbox, nil},
		"approve": {runFactoryApprove, []string{"alpha", "commit"}}, "stop": {runFactoryStop, []string{"alpha"}},
		"resume": {runFactoryResume, []string{"alpha"}}, "rerun": {runFactoryRerun, []string{"alpha"}},
		"close": {runFactoryClose, []string{"alpha"}},
	} {
		if _, err := runFactoryCommand(t, call.run, call.args...); err == nil {
			t.Errorf("%s accepted a corrupt ledger", name)
		}
	}
}

func TestFactoryCommandsReportAMissingBranchInProcess(t *testing.T) {
	t.Chdir(newFactoryProject(t))
	flags := onceAgainstTheMock()
	flags.branch = "no-such-branch"
	useFactoryFlags(t, flags)
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"inbox": runFactoryInbox, "rerun": runFactoryRerun, "run": runFactoryRun,
	} {
		if _, err := runFactoryCommand(t, run, "alpha"); err == nil {
			t.Errorf("%s accepted a branch that does not exist", name)
		}
	}
}

func TestASecondSupervisorIsRefusedInProcess(t *testing.T) {
	root := newFactoryProject(t)
	t.Chdir(root)
	layout := factory.Layout{Root: root}
	if err := layout.Prepare(); err != nil {
		t.Fatal(err)
	}
	lock, err := factory.AcquireLock(layout.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Release() }()
	useFactoryFlags(t, onceAgainstTheMock())
	if _, err := runFactoryCommand(t, runFactoryRun); !errors.Is(err, factory.ErrSupervisorRunning) {
		t.Errorf("err = %v, want ErrSupervisorRunning", err)
	}
}

func TestStatusDetailNamesTheGateTheURLOrTheReason(t *testing.T) {
	for want, order := range map[string]*factory.OrderState{
		"gate confirm-design": {Phase: factory.PhaseWaiting, Gate: "confirm-design"},
		"https://example/1":   {Phase: factory.PhaseShipped, URL: "https://example/1"},
		"budget: usd":         {Phase: factory.PhaseParked, Reason: "budget: usd"},
	} {
		if got := statusDetail(order); got != want {
			t.Errorf("statusDetail = %q, want %q", got, want)
		}
	}
	if openTree(t.TempDir()) == nil {
		t.Error("openTree returned nothing")
	}
}
