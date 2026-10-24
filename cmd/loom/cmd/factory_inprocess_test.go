package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

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
	status, err := runFactoryCommand(t, runFactoryStatus)
	if err != nil || !strings.Contains(status, "gate confirm-design") {
		t.Fatalf("status = %q, %v", status, err)
	}
	inbox, err := runFactoryCommand(t, runFactoryInbox)
	if err != nil || !strings.Contains(inbox, "loom factory approve alpha confirm-design") {
		t.Fatalf("inbox = %q, %v", inbox, err)
	}
	if output, err := runFactoryCommand(t, runFactoryApprove, "alpha", "confirm-design"); err != nil || !strings.Contains(output, "Approved confirm-design") {
		t.Fatalf("approve = %q, %v", output, err)
	}
}

func TestFactoryControlsReportTheirRefusalsInProcess(t *testing.T) {
	root := newFactoryProject(t, "alpha")
	t.Chdir(root)
	t.Setenv(actAsLoomEnv, "1")
	useFactoryFlags(t, onceAgainstTheMock())
	if _, err := runFactoryCommand(t, runFactoryRun); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"stop": runFactoryStop, "resume": runFactoryResume, "rerun": runFactoryRerun,
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
