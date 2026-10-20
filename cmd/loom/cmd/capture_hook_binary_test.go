package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// These tests run the built loom binary, as a tool hook does.

func TestCaptureHookFailurePathsExitZeroSilentlyAndCountTheFailure(t *testing.T) {
	overCapPayload := `{"prompt":"` + strings.Repeat("x", capture.MaxStdinBytes) + `"}`
	tests := []struct {
		name      string
		event     string
		stdin     string
		prepare   func(*captureProject)
		wantCount string
		rawArgs   []string // when set, the binary runs these arguments in the project root
	}{
		{name: "bad json", event: "PostToolUse", stdin: "{not json", wantCount: "d"},
		{name: "empty stdin", event: "PostToolUse", stdin: "", wantCount: "d"},
		{name: "unknown event", event: "Bogus", stdin: "{}", wantCount: "d"},
		{name: "stdin over the cap", event: "UserPromptSubmit", stdin: overCapPayload, wantCount: "d"},
		{name: "missing salt", event: "UserPromptSubmit", stdin: promptPayload("s", "hi"),
			prepare: func(p *captureProject) { _ = os.Remove(p.layout.SaltPath()) }, wantCount: "s"},
		{name: "salt readable by others", event: "UserPromptSubmit", stdin: promptPayload("s", "hi"),
			prepare: func(p *captureProject) { _ = os.Chmod(p.layout.SaltPath(), 0o644) }, wantCount: "s"},
		{name: "salt of the wrong size", event: "UserPromptSubmit", stdin: promptPayload("s", "hi"),
			prepare: func(p *captureProject) { p.write(".claude/memory/capture.salt", "short", 0o600) }, wantCount: "s"},
		{name: "invalid settings file", event: "UserPromptSubmit", stdin: promptPayload("s", "hi"),
			prepare: func(p *captureProject) { p.write(".claude/capture.yaml", "unknownKey: 1\n", 0o600) }, wantCount: "c"},
		{name: "bad redaction glob", event: "UserPromptSubmit", stdin: promptPayload("s", "hi"),
			prepare: func(p *captureProject) { p.write(".claude/capture.yaml", "redact: [\"[bad\"]\n", 0o600) }, wantCount: "c"},
		{name: "root flag without a value", rawArgs: []string{"capture", "hook", "PreToolUse", "--root"}, wantCount: "d"},
		{name: "help flag of the hook", rawArgs: []string{"capture", "hook", "--help"}},
		{name: "short help flag of the hook", rawArgs: []string{"capture", "hook", "-h"}},
		{name: "capture parent alone", rawArgs: []string{"capture"}},
		{name: "help flag of the parent", rawArgs: []string{"capture", "--help"}},
		{name: "unknown subcommand", rawArgs: []string{"capture", "bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newCaptureProject(t)
			if tt.prepare != nil {
				tt.prepare(project)
			}

			result := runFailurePathCall(project, tt.event, tt.stdin, tt.rawArgs)

			assertSilentSuccess(t, result)
			if got := project.failures(); got != tt.wantCount {
				t.Fatalf("failure counter = %q, want %q", got, tt.wantCount)
			}
			if _, err := os.Stat(project.layout.SessionsDir()); err == nil {
				t.Fatal("a failed call must write no day file")
			}
		})
	}
}

func runFailurePathCall(project *captureProject, event, stdin string, rawArgs []string) hookResult {
	if rawArgs == nil {
		return project.runHook(event, stdin)
	}
	call := binaryCall{binary: captureBinary(project.t), args: rawArgs, stdin: strings.NewReader(""), dir: project.root}
	return runCall(project.t, call)
}

func TestCaptureHookWithoutAnEventOrARootStillExitsZeroSilently(t *testing.T) {
	project := newCaptureProject(t)
	binary := captureBinary(t)

	noEvent := runBinary(t, binary, []string{"capture", "hook", "--root", project.root}, strings.NewReader("{}"))
	noRoot := runBinary(t, binary, []string{"capture", "hook", "Stop"}, strings.NewReader("{}"))

	assertSilentSuccess(t, noEvent)
	assertSilentSuccess(t, noRoot)
	if project.failures() != "d" {
		t.Fatalf("failure counter = %q, want one decode failure for the missing event", project.failures())
	}
}

func TestCaptureHookAcceptsAnUnknownFlag(t *testing.T) {
	project := newCaptureProject(t)

	result := project.runHook("UserPromptSubmit", promptPayload("s", "hello"), "--future-flag", "x")

	assertSilentSuccess(t, result)
	if len(project.dayEvents()) != 1 {
		t.Fatal("the event must be recorded although the flag is unknown")
	}
}

func TestCaptureCommandsAreNotInHelp(t *testing.T) {
	result := runBinary(t, captureBinary(t), []string{"--help"}, nil)

	if result.code != 0 || strings.Contains(strings.ToLower(result.stdout), "capture") {
		t.Fatalf("code %d, help mentions capture: %v", result.code, strings.Contains(result.stdout, "capture"))
	}
}

func TestCaptureHookRecordsAFileOverTheCapAsUnknownAndAFileAtTheCapWithARange(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantRange bool
	}{
		{name: "one byte over the cap", content: strings.Repeat("a", capture.MaxTargetFileBytes+1)},
		{name: "exactly at the cap", content: strings.Repeat("0123456789abcde\n", capture.MaxTargetFileBytes/16-1) + "UNIQUE-LINE-ABC\n", wantRange: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newCaptureProject(t)
			file := project.write("app/big.go", tt.content, 0o600)

			result := project.runHook("PostToolUse", editPayload("s1", file, "old", "UNIQUE-LINE-ABC"))

			assertSilentSuccess(t, result)
			events := project.dayEvents()
			if len(events) != 1 || events[0].HasHunk() != tt.wantRange || events[0].PathRef == "" {
				t.Fatalf("events = %+v, want one event, range = %v", events, tt.wantRange)
			}
			if project.failures() != "" {
				t.Fatalf("failure counter = %q: a large file is recorded, not a failure", project.failures())
			}
		})
	}
}

func TestCaptureHookStallBreakerTripsAndExpires(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantWork  bool
		wantBytes string
	}{
		{name: "marker 30 s old: no work", age: 30 * time.Second, wantWork: false, wantBytes: "t"},
		{name: "marker 61 s old: normal path", age: 61 * time.Second, wantWork: true, wantBytes: "t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newCaptureProject(t)
			project.write(".claude/memory/capture.failures", "t", 0o600)
			stamp := time.Now().Add(-tt.age)
			if err := os.Chtimes(project.layout.FailuresPath(), stamp, stamp); err != nil {
				t.Fatal(err)
			}
			before, _ := os.Stat(project.layout.FailuresPath())

			result := project.runHook("UserPromptSubmit", promptPayload("s", "hello"))

			assertSilentSuccess(t, result)
			after, _ := os.Stat(project.layout.FailuresPath())
			if got := len(project.dayEvents()) == 1; got != tt.wantWork {
				t.Fatalf("recorded = %v, want %v", got, tt.wantWork)
			}
			if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
				t.Fatal("a successful or skipped call must leave the failure file as it was")
			}
		})
	}
}

func TestCaptureHookOneHumanEditGivesOneCorrection(t *testing.T) {
	project := newCaptureProject(t)
	original := "alpha\nbeta\ngamma\n"
	file := project.write("app/main.go", original, 0o600)
	assertSilentSuccess(t, project.runHook("PostToolUse", editPayload("s1", file, "old line", "beta")))
	project.write("app/main.go", "alpha\nHUMAN CHANGE\ngamma\n", 0o600)

	for range 4 {
		assertSilentSuccess(t, project.runHook("PreToolUse", readPayload("s1", file)))
	}

	events := project.dayEvents()
	corrections := detect(events)
	equal(t, "observations", countKind(events, capture.EventHunkObserved), 1)
	equal(t, "corrections", len(corrections), 1)
	equal(t, "correction kind", corrections[0].Kind, capture.KindHunkEdit)
	equal(t, "correction path", *corrections[0].Path, "app/main.go")
}

func TestCaptureHookRevertIsDetectedFromTheOldText(t *testing.T) {
	project := newCaptureProject(t)
	file := project.write("app/main.go", "alpha\nbeta\ngamma\n", 0o600)
	assertSilentSuccess(t, project.runHook("PostToolUse", editPayload("s1", file, "original", "beta")))
	project.write("app/main.go", "alpha\noriginal\ngamma\n", 0o600)

	assertSilentSuccess(t, project.runHook("PreToolUse", readPayload("s1", file)))

	corrections := detect(project.dayEvents())
	if len(corrections) != 1 || corrections[0].Kind != capture.KindRevert {
		t.Fatalf("corrections = %+v, want one revert", corrections)
	}
}

func TestCaptureHookWritesNothingForAnUnchangedHunk(t *testing.T) {
	project := newCaptureProject(t)
	file := project.write("app/main.go", "alpha\nbeta\ngamma\n", 0o600)
	assertSilentSuccess(t, project.runHook("PostToolUse", editPayload("s1", file, "x", "beta")))

	for range 3 {
		assertSilentSuccess(t, project.runHook("PreToolUse", readPayload("s1", file)))
	}

	events := project.dayEvents()
	if len(events) != 4 {
		t.Fatalf("events = %d, want 1 edit + 3 reads and no observation", len(events))
	}
	for _, event := range events {
		if event.Kind == capture.EventHunkObserved {
			t.Fatal("an unchanged hunk must write no observation")
		}
	}
}

func TestCaptureHookStopRunsThePurgeWhenDue(t *testing.T) {
	project := newCaptureProject(t)
	expired := project.layout.DayFile(time.Now().AddDate(0, 0, -120))
	project.write(relativeTo(project, expired), "{}\n", 0o600)

	assertSilentSuccess(t, project.runHook("Stop", marshalPayload(map[string]any{"session_id": "s1"})))
	secondExpired := project.layout.DayFile(time.Now().AddDate(0, 0, -121))
	project.write(relativeTo(project, secondExpired), "{}\n", 0o600)
	assertSilentSuccess(t, project.runHook("Stop", marshalPayload(map[string]any{"session_id": "s1"})))

	if _, err := os.Stat(expired); err == nil {
		t.Fatal("the first Stop must purge the expired day file")
	}
	if _, err := os.Stat(project.layout.PurgeStampPath()); err != nil {
		t.Fatal("the purge must leave its stamp")
	}
	if _, err := os.Stat(secondExpired); err != nil {
		t.Fatal("the second Stop is inside the rate limit and must not purge")
	}
	if events := project.dayEvents(); len(events) != 2 || events[0].Kind != capture.EventStop {
		t.Fatalf("events = %+v, want two stop events", events)
	}
}

func relativeTo(project *captureProject, absolute string) string {
	relative, _ := filepath.Rel(project.root, absolute)
	return relative
}

const (
	canaryPrompt   = "CANARY-PROMPT-3d8f"
	canaryOldText  = "CANARY-OLD-TEXT-7c1e"
	canaryNewText  = "CANARY-NEW-TEXT-9b42"
	canaryContent  = "CANARY-WRITE-CONTENT-2e77"
	canaryCommand  = "CANARY-COMMAND-8a15"
	canaryFileBody = "CANARY-FILE-BODY-5a60"
	canaryRedacted = "CANARYREDACTEDNAME"
)

// runCanaryHookCalls feeds unique strings in the prompt, old_string,
// new_string, content and command through the built binary. It returns every
// byte the calls printed and the source files that hold canaries on purpose.
func runCanaryHookCalls(t *testing.T, project *captureProject) (output string, ownFiles []string) {
	t.Helper()
	source := project.write("app/main.go", "head\n"+canaryNewText+" "+canaryFileBody+"\ntail\n", 0o600)
	written := project.write("app/written.go", canaryContent+"\n", 0o600)
	secret := project.write("secrets/"+canaryRedacted+"/key.txt", "body "+canaryFileBody+"\n", 0o600)
	bash := marshalPayload(map[string]any{"session_id": "s1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "echo " + canaryCommand}})
	write := marshalPayload(map[string]any{"session_id": "s1", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": written, "content": canaryContent}})
	calls := []struct{ event, payload string }{
		{"UserPromptSubmit", promptPayload("s1", "no, "+canaryPrompt)},
		{"PreToolUse", write},
		{"PostToolUse", write},
		{"PostToolUse", editPayload("s1", source, canaryOldText, canaryNewText)},
		{"PostToolUse", editPayload("s1", secret, canaryOldText, "body")},
		{"PostToolUse", bash},
		{"Stop", marshalPayload(map[string]any{"session_id": "s1"})},
	}
	var printed strings.Builder
	for _, call := range calls {
		result := project.runHook(call.event, call.payload)
		assertSilentSuccess(t, result)
		printed.WriteString(result.stdout + result.stderr)
	}
	return printed.String(), []string{source, written, secret}
}

// TestCaptureHookCanaryTextReachesNeitherDiskNorOutput runs the canary calls,
// then searches every file the hook may have written and every byte it
// printed. The database and report side is
// TestCaptureCanaryReachesNeitherDatabaseNorReport.
func TestCaptureHookCanaryTextReachesNeitherDiskNorOutput(t *testing.T) {
	project := newCaptureProject(t)

	output, ownFiles := runCanaryHookCalls(t, project)

	canaries := []string{canaryPrompt, canaryOldText, canaryNewText, canaryContent, canaryCommand, canaryFileBody, canaryRedacted, captureSalt}
	assertNoCanaryOnDisk(t, project, canaries, ownFiles...)
	dayFile := readText(t, project.layout.DayFile(time.Now()))
	if !strings.Contains(dayFile, `"path":"app/main.go"`) || strings.Contains(dayFile, "secrets") {
		t.Fatalf("day file must name the open path and no redacted path:\n%s", dayFile)
	}
	if !strings.Contains(dayFile, `"promptClass":"corrective"`) || len(output) != 0 {
		t.Fatalf("prompt class missing or output not empty (%d bytes)", len(output))
	}
}

func assertNoCanaryOnDisk(t *testing.T, project *captureProject, canaries []string, ownFiles ...string) {
	t.Helper()
	paths := filesToScan(t, project, ownFiles)
	equal(t, "scanned the day file and the stamp", len(paths) >= 2, true)
	for _, path := range paths {
		content := readText(t, path)
		for _, canary := range canaries {
			equal(t, path+" holds "+canary, strings.Contains(content, canary), false)
		}
	}
}

// filesToScan lists every file the hook may have written: all files under
// the root except the source files of the test and the salt.
func filesToScan(t *testing.T, project *captureProject, ownFiles []string) []string {
	t.Helper()
	own := map[string]bool{project.layout.SaltPath(): true}
	for _, file := range ownFiles {
		own[file] = true
	}
	var paths []string
	err := filepath.WalkDir(project.root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !own[path] {
			paths = append(paths, path)
		}
		return err
	})
	equal(t, "walk error", err, nil)
	return paths
}

func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
