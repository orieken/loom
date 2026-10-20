package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/jsonlstore"
)

// captureSalt is a printable salt, so a test can search output for it.
// runBinaryLimit fails a hook that hangs instead of waiting for the test timeout.
const runBinaryLimit = 15 * time.Second

// startupSlack covers process start-up and a loaded machine. The test
// asserts the limit plus this slack, and reports the measured time.
const startupSlack = 700 * time.Millisecond

const captureSalt = "0123456789abcdef0123456789abcdef"

var (
	builtBinaryOnce sync.Once
	builtBinaryPath string
	builtBinaryErr  error
	builtBinaryDir  string
)

// TestMain removes the binary that the capture tests build once.
func TestMain(m *testing.M) {
	code := m.Run()
	if builtBinaryDir != "" {
		os.RemoveAll(builtBinaryDir)
	}
	os.Exit(code)
}

// captureBinary builds the loom binary once for all capture tests. The tests
// run the built binary, never `go run`, so process start is part of what
// they see.
func captureBinary(t *testing.T) string {
	t.Helper()
	builtBinaryOnce.Do(func() {
		builtBinaryDir, builtBinaryErr = os.MkdirTemp("", "loom-capture-test")
		if builtBinaryErr != nil {
			return
		}
		builtBinaryPath = filepath.Join(builtBinaryDir, "loom")
		build := exec.Command("go", "build", "-o", builtBinaryPath, "github.com/orieken/loom/cmd/loom")
		if output, err := build.CombinedOutput(); err != nil {
			builtBinaryErr = &buildError{output: string(output), err: err}
			return
		}
		// The first run of a new binary pays a one-time system scan on macOS
		// (about 400 ms measured). Pay it here, outside any timed call.
		_ = exec.Command(builtBinaryPath, "--version").Run()
	})
	if builtBinaryErr != nil {
		t.Fatalf("build loom binary: %v", builtBinaryErr)
	}
	return builtBinaryPath
}

type buildError struct {
	output string
	err    error
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.output }

// captureProject is an installed project: salt, memory directory, target files.
type captureProject struct {
	t      *testing.T
	root   string
	layout capture.Layout
	hasher *capture.Hasher
}

func newCaptureProject(t *testing.T) *captureProject {
	t.Helper()
	root := t.TempDir()
	layout := capture.NewLayout(root)
	project := &captureProject{t: t, root: root, layout: layout}
	project.write(filepath.Join(".claude", "memory", "capture.salt"), captureSalt, 0o600)
	salt, err := capture.NewSalt([]byte(captureSalt))
	if err != nil {
		t.Fatal(err)
	}
	project.hasher = capture.NewHasher(salt)
	return project
}

func (p *captureProject) write(relative, content string, mode os.FileMode) string {
	p.t.Helper()
	path := filepath.Join(p.root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		p.t.Fatal(err)
	}
	return path
}

// hookResult is what one hook process did.
type hookResult struct {
	code    int
	stdout  string
	stderr  string
	elapsed time.Duration
}

// runHook runs the built binary once, as the tool would.
func (p *captureProject) runHook(event, stdin string, extraArgs ...string) hookResult {
	p.t.Helper()
	args := append([]string{"capture", "hook", event, "--root", p.root}, extraArgs...)
	return runBinary(p.t, captureBinary(p.t), args, strings.NewReader(stdin))
}

func runBinary(t *testing.T, binary string, args []string, stdin *strings.Reader) hookResult {
	t.Helper()
	return runCall(t, binaryCall{binary: binary, args: args, stdin: stdin})
}

// binaryCall is one run of the built binary. The directory is the working
// directory of the run; the memory commands read the project from it.
type binaryCall struct {
	binary string
	args   []string
	stdin  *strings.Reader
	dir    string
}

func runCall(t *testing.T, call binaryCall) hookResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runBinaryLimit)
	defer cancel()
	command := exec.CommandContext(ctx, call.binary, call.args...)
	command.Dir = call.dir
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if call.stdin != nil {
		command.Stdin = call.stdin
	}
	start := time.Now()
	err := command.Run()
	code := 0
	if exitErr, isExit := err.(*exec.ExitError); isExit {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", call.args, err)
	}
	return hookResult{code: code, stdout: stdout.String(), stderr: stderr.String(), elapsed: time.Since(start)}
}

// assertSilentSuccess checks the contract that holds on every path.
func assertSilentSuccess(t *testing.T, result hookResult) {
	t.Helper()
	if result.code != 0 || result.stdout != "" || result.stderr != "" {
		t.Fatalf("code %d, stdout %q, stderr %q; want exit 0 and no output", result.code, result.stdout, result.stderr)
	}
}

func (p *captureProject) failures() string {
	content, err := os.ReadFile(p.layout.FailuresPath())
	if err != nil {
		return ""
	}
	return string(content)
}

// dayEvents reads every event of today's day file with the ingest reader.
func (p *captureProject) dayEvents() []capture.SessionEvent {
	p.t.Helper()
	paths, err := jsonlstore.DayFilePaths(p.layout)
	if err != nil {
		p.t.Fatal(err)
	}
	var events []capture.SessionEvent
	for _, path := range paths {
		_, err := jsonlstore.ReadDayFile(p.t.Context(), path, func(e capture.SessionEvent) error {
			events = append(events, e)
			return nil
		})
		if err != nil {
			p.t.Fatal(err)
		}
	}
	return events
}

// equal fails the test when two values differ. Moving the comparison here
// keeps each test free of the branches that a failure message needs.
func equal[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

func countKind(events []capture.SessionEvent, kind capture.EventKind) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

// payloads of the hook, in the shape of the tool (UNVERIFIED field names).
func editPayload(sessionID, file, oldText, newText string) string {
	return marshalPayload(map[string]any{
		"session_id": sessionID, "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": file, "old_string": oldText, "new_string": newText},
	})
}

func readPayload(sessionID, file string) string {
	return marshalPayload(map[string]any{
		"session_id": sessionID, "tool_name": "Read", "tool_input": map[string]any{"file_path": file},
	})
}

func promptPayload(sessionID, text string) string {
	return marshalPayload(map[string]any{"session_id": sessionID, "prompt": text})
}

func marshalPayload(value map[string]any) string {
	body, _ := json.Marshal(value)
	return string(body)
}

// detect runs the stored events through the detector, as ingest does.
func detect(events []capture.SessionEvent) []capture.Correction {
	detector := capture.NewDetector(capture.DefaultConfig())
	var found []capture.Correction
	for _, event := range events {
		found = append(found, detector.Observe(event)...)
	}
	return found
}
