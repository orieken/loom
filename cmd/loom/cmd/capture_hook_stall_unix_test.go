//go:build unix

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// A named pipe with no reader blocks every open for writing. It models a
// disk that stalls: the day file and the failure log can both be one.

func makePipe(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureHookExitsZeroInsideTheLimitWhenTheSinkAndTheCounterBothBlock(t *testing.T) {
	project := newCaptureProject(t)
	makePipe(t, project.layout.DayFile(time.Now().UTC()))
	makePipe(t, project.layout.FailureLogPath())

	result := project.runHook("UserPromptSubmit", promptPayload("s", "hello"))

	assertSilentSuccess(t, result)
	limit := capture.HookStopAfter + capture.CounterWriteWait
	if result.elapsed < capture.HookStopAfter || result.elapsed > limit+startupSlack {
		t.Fatalf("exited after %v, want between %v and %v", result.elapsed, capture.HookStopAfter, limit+startupSlack)
	}
	t.Logf("hard stop with sink and counter blocked: %v (limit %v)", result.elapsed, limit)
}

func TestCaptureHookStallThenBreakerThenExpiry(t *testing.T) {
	project := newCaptureProject(t)
	makePipe(t, project.layout.DayFile(time.Now().UTC()))

	stalled := project.runHook("UserPromptSubmit", promptPayload("s", "first"))
	tripped := project.runHook("UserPromptSubmit", promptPayload("s", "second"))

	assertSilentSuccess(t, stalled)
	assertSilentSuccess(t, tripped)
	if project.failures() != "stall" {
		t.Fatalf("failure log = %q, want one stall line", project.failures())
	}
	if tripped.elapsed >= capture.HookStopAfter {
		t.Fatalf("the call with the breaker open took %v, want far less than %v", tripped.elapsed, capture.HookStopAfter)
	}

	old := time.Now().Add(-capture.StallWindow - time.Second)
	project.write(".claude/memory/capture.failures.log", capture.NewFailureEntry(old, capture.CauseStall).Line(), 0o600)
	probe := project.runHook("UserPromptSubmit", promptPayload("s", "third"))
	assertSilentSuccess(t, probe)
	if probe.elapsed < capture.HookStopAfter {
		t.Fatalf("after the window the hook takes the normal path again and stalls on the pipe; took %v", probe.elapsed)
	}
	if project.failures() != "stall stall" {
		t.Fatalf("failure log = %q, want a second stall line from the half-open probe", project.failures())
	}
}

func TestCaptureHookStopsAfterTheLimitWhenStdinNeverCloses(t *testing.T) {
	project := newCaptureProject(t)
	command := exec.Command(captureBinary(t), "capture", "hook", "UserPromptSubmit", "--root", project.root)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	start := time.Now()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	waitForExit(t, command, capture.HookStopAfter+capture.CounterWriteWait+startupSlack)

	equal(t, "exited after the watchdog time", time.Since(start) >= capture.HookStopAfter, true)
	equal(t, "failure log", project.failures(), "stall")
}

// waitForExit fails the test when the process does not exit with code 0
// inside the limit.
func waitForExit(t *testing.T, command *exec.Cmd, limit time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		equal(t, "exit error", err, nil)
	case <-time.After(limit):
		_ = command.Process.Kill()
		t.Fatal("the hook hung on a stdin that never closes")
	}
}
