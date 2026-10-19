package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runBuiltLoom runs the built binary in a project directory with a private HOME.
func runBuiltLoom(t *testing.T, dir, path string, stdin string, args ...string) (string, string) {
	t.Helper()
	return runBuiltProgram(t, captureBinary(t), dir, path, stdin, args...)
}

func runBuiltProgram(t *testing.T, program, dir, path string, stdin string, args ...string) (string, string) {
	t.Helper()
	command := exec.Command(program, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "HOME="+t.TempDir(), "XDG_CACHE_HOME="+t.TempDir(), "PATH="+path)
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("loom %v: %v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

// registeredCommand finds the command that install registered for an event.
func registeredCommand(t *testing.T, settingsPath, event string) string {
	t.Helper()
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &document); err != nil || len(document.Hooks[event]) == 0 {
		t.Fatalf("no entry for %s in %s (err %v)", event, content, err)
	}
	return document.Hooks[event][0].Hooks[0].Command
}

// TestInstallCaptureEndToEndReportsAHumanCorrection runs the BUILT binary
// through the real chain: install --capture, the registered hook commands
// (run through a shell, as the tool would), ingest, and the report.
//
// UNVERIFIED: the payload shape below (session_id, tool_name, tool_input,
// file_path, old_string, new_string) and the way Claude Code runs a hook
// command are assumptions that no document in this repository confirms.
// The test proves the chain is consistent with those assumptions. It does
// not prove them.
func TestInstallCaptureEndToEndReportsAHumanCorrection(t *testing.T) {
	// Build first: newCaptureWorld changes HOME, and a build under a new HOME
	// would fill a read-only module cache inside the test directory.
	binaryDir := filepath.Dir(captureBinary(t))
	world := newCaptureWorld(t)
	path := binaryDir + string(os.PathListSeparator) + os.Getenv("PATH")

	runBuiltLoom(t, world.target, path, "", "install", "--capture", "--level", "1", "--copy", "--target", world.target)

	settingsPath := world.path(settingsLocal)
	file := world.write("app/main.go", "alpha\nbeta\ngamma\n", 0o600)
	runHook := func(event, payload string) {
		t.Helper()
		_, stderr := runBuiltProgram(t, "sh", world.target, path, payload, "-c", registeredCommand(t, settingsPath, event))
		if stderr != "" {
			t.Fatalf("hook %s wrote to stderr: %q", event, stderr)
		}
	}
	// The agent edits line 2; a person then changes it; a later call sees it.
	runHook("PostToolUse", editPayload("s1", file, "old line", "beta"))
	world.write("app/main.go", "alpha\nHUMAN CHANGE\ngamma\n", 0o600)
	runHook("PreToolUse", readPayload("s1", file))

	ingested, ingestErr := runBuiltLoom(t, world.target, path, "", "memory", "ingest")
	if !strings.Contains(ingested+ingestErr, "1 session corrections ingested") {
		t.Fatalf("ingest output: %s %s", ingested, ingestErr)
	}
	// The command prints its report on stderr (the cobra default output).
	reportOut, reportErr := runBuiltLoom(t, world.target, path, "", "memory", "corrections", "--sessions", "--json")

	report := reportOut + reportErr
	var parsed struct {
		Kinds []struct {
			Kind  string `json:"kind"`
			Count int    `json:"count"`
		} `json:"kinds"`
	}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, report)
	}
	for _, kind := range parsed.Kinds {
		if kind.Kind == "hunk-edit" && kind.Count == 1 {
			return
		}
	}
	t.Fatalf("no hunk-edit correction in the report:\n%s", report)
}
