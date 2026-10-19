package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

var sessionDay = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func resetMemoryArgs(t *testing.T) {
	t.Helper()
	memoryArgs = memoryFlags{limit: 20, moreThan: 2}
	t.Cleanup(func() { memoryArgs = memoryFlags{} })
}

// sessionProject is a project root with no docs/features at all.
func sessionProject(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	resetMemoryArgs(t)
}

func textOf(text string) *string { return &text }

func encodeEvents(t *testing.T, events ...capture.SessionEvent) string {
	t.Helper()
	var text strings.Builder
	for _, event := range events {
		line, err := capture.MarshalLine(event)
		if err != nil {
			t.Fatal(err)
		}
		text.Write(line)
	}
	return text.String()
}

func hunkEditPair(path string, at time.Time) []capture.SessionEvent {
	ref := "ref-" + path
	write := capture.NewSessionEvent("w"+path, at, capture.EventToolResult).WithTool(capture.ToolEdit).
		WithPath(ref, textOf(path)).
		WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 3, PreHash: "pre", PostHash: "post"})
	seen := capture.NewSessionEvent("o"+path, at.Add(30*time.Second), capture.EventHunkObserved).
		WithPath(ref, nil).WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 3, PostHash: "human"})
	return []capture.SessionEvent{write, seen}
}

func writeDayFile(t *testing.T, name, content string) {
	t.Helper()
	dir := sessionLayout().SessionsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runCorrections(t *testing.T) string {
	t.Helper()
	return runMemoryCommand(t, runMemoryCorrections)
}

func TestIngestRebuildsFromDayFilesWithNoArchivedRuns(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))

	output := runMemoryCommand(t, runMemoryIngest)

	if strings.Contains(output, "nothing to ingest") {
		t.Errorf("a project with day files must not print nothing to ingest:\n%s", output)
	}
	for _, want := range []string{"1 session corrections ingested", "torn lines skipped: 0",
		"unknown-version lines skipped: 0", "clock anomalies: 0", "unattributed corrections (not counted): 0"} {
		if !strings.Contains(output, want) {
			t.Errorf("ingest output lacks %q:\n%s", want, output)
		}
	}
}

func TestIngestWithRunsAndDayFilesIngestsBoth(t *testing.T) {
	projectWithStore(t)
	resetMemoryArgs(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))

	output := runMemoryCommand(t, runMemoryIngest)

	if !strings.Contains(output, "1 of 1 archived runs ingested") || !strings.Contains(output, "1 session corrections ingested") {
		t.Errorf("both sources must be ingested:\n%s", output)
	}
}

func TestIngestWithADirSkipsSessions(t *testing.T) {
	projectWithStore(t)
	resetMemoryArgs(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	memoryArgs.dir = filepath.Join("docs", "features", "user-auth")

	output := runMemoryCommand(t, runMemoryIngest)

	if strings.Contains(output, "session corrections") {
		t.Errorf("--dir must skip sessions:\n%s", output)
	}
}

func TestIngestWithNeitherSourcePrintsNothingToIngest(t *testing.T) {
	sessionProject(t)

	output := runMemoryCommand(t, runMemoryIngest)

	if !strings.Contains(output, "No archived runs found under docs/features — nothing to ingest.") {
		t.Errorf("output = %q", output)
	}
}

// A torn last line, a glued line, an over-long line and a v:2 line are
// skipped and counted. The good lines still commit.
func TestIngestSkipsAndCountsBadLinesAndStillCommits(t *testing.T) {
	sessionProject(t)
	good := encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...)
	glued := strings.TrimSuffix(encodeEvents(t, hunkEditPair("src/g.go", sessionDay.Add(time.Hour))[0]), "\n") +
		strings.TrimSuffix(encodeEvents(t, hunkEditPair("src/g.go", sessionDay.Add(time.Hour))[1]), "\n") + "\n"
	overlong := `{"v":1,"id":"` + strings.Repeat("x", capture.MaxLineBytes+10) + "\"}\n"
	versionTwo := `{"v":2,"id":"future","at":"2026-10-18T12:00:00Z","kind":"prompt"}` + "\n"
	torn := `{"v":1,"id":"torn","at":"2026-10-18`
	writeDayFile(t, "2026-10-18.jsonl", good+glued+overlong+versionTwo+torn)

	output := runMemoryCommand(t, runMemoryIngest)

	for _, want := range []string{"1 session corrections ingested", "torn lines skipped: 3", "unknown-version lines skipped: 1"} {
		if !strings.Contains(output, want) {
			t.Errorf("output lacks %q:\n%s", want, output)
		}
	}
}

func TestIngestCountsClockAnomalies(t *testing.T) {
	sessionProject(t)
	later := capture.NewSessionEvent("a", sessionDay.Add(time.Minute), capture.EventStop)
	earlier := capture.NewSessionEvent("b", sessionDay, capture.EventStop)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, later, earlier))

	output := runMemoryCommand(t, runMemoryIngest)

	if !strings.Contains(output, "clock anomalies: 1") {
		t.Errorf("output = %s", output)
	}
}

// The detector runs across day files in date order.
func TestDetectionSpansDayFilesInDateOrder(t *testing.T) {
	sessionProject(t)
	pair := hunkEditPair("src/a.go", sessionDay.Add(23*time.Hour+59*time.Minute+45*time.Second))
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, pair[0]))
	writeDayFile(t, "2026-10-19.jsonl", encodeEvents(t, pair[1]))

	output := runMemoryCommand(t, runMemoryIngest)

	if !strings.Contains(output, "1 session corrections ingested") {
		t.Errorf("a correction across midnight was missed:\n%s", output)
	}
}

func TestIngestLimitsTheDatabaseToItsOwner(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))

	_ = runMemoryCommand(t, runMemoryIngest)

	info, err := os.Stat(memory.DefaultPath("."))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("episodes.db mode = %o, want 600", info.Mode().Perm())
	}
}

// Delete episodes.db and its sidecars, ingest again, and the report is the
// same bytes. The project has day files and no docs/features.
func TestRebuildEqualityOfTheCorrectionsReport(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-17.jsonl", encodeEvents(t, hunkEditPair("docs/b.md", sessionDay.Add(-24*time.Hour))...))
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)
	first := runCorrections(t)

	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err := os.Remove(memory.DefaultPath(".") + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	_ = runMemoryCommand(t, runMemoryIngest)
	second := runCorrections(t)

	if first != second {
		t.Errorf("report changed after a rebuild:\n--- first\n%s\n--- second\n%s", first, second)
	}
	if !strings.Contains(first, "Newest recorded event: 2026-10-18T12:00:30.000Z") {
		t.Errorf("header does not show the newest event time:\n%s", first)
	}
}

func TestPurgedDayFileLosesItsRowsAfterReingest(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-17.jsonl", encodeEvents(t, hunkEditPair("docs/b.md", sessionDay.Add(-24*time.Hour))...))
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)

	if err := os.Remove(filepath.Join(sessionLayout().SessionsDir(), "2026-10-17.jsonl")); err != nil {
		t.Fatal(err)
	}
	output := runMemoryCommand(t, runMemoryIngest)

	if !strings.Contains(output, "1 session corrections ingested") {
		t.Errorf("the purged file kept its rows:\n%s", output)
	}
}

func TestCorrectionsTextShowsSessionSectionSeparateFromGateSection(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)

	output := runCorrections(t)

	gate := strings.Index(output, "No human corrections recorded.")
	session := strings.Index(output, "Session Corrections")
	if gate < 0 || session < gate {
		t.Fatalf("the gate-level section must come first, then Session Corrections:\n%s", output)
	}
	for _, want := range []string{"Method: fixed weight per kind (estimate)", "Newest recorded event:",
		"hunk-edit", "Total estimated minutes: 2", "unattributed (not counted): 0",
		"Weights (minutes per correction, estimate): hunk-edit 2, revert 3, denied-call 1, corrective-prompt 1"} {
		if !strings.Contains(output[session:], want) {
			t.Errorf("session section lacks %q:\n%s", want, output)
		}
	}
}

func TestCorrectionsSaysNoSessionDataAndWhenToIngest(t *testing.T) {
	sessionProject(t)

	none := runCorrections(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	needsIngest := runCorrections(t)

	if !strings.Contains(none, "No session data\n") || strings.Contains(none, "Run `loom memory ingest`") {
		t.Errorf("no data and no files: %q", none)
	}
	if !strings.Contains(needsIngest, "No session data. Run `loom memory ingest`") {
		t.Errorf("day files but no table: %q", needsIngest)
	}
}

func TestCorrectionsReportDoesNotCreateTheSessionTable(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))

	_ = runCorrections(t)

	store, err := memory.Open(memory.DefaultPath("."))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	planned, err := memory.PlanSessionTableDrop(store.Path())
	if err != nil || len(planned) != 0 {
		t.Errorf("the report created the session table: %v %v", planned, err)
	}
}

func TestCorrectionsFiltersApplyToSessionCorrectionsOnly(t *testing.T) {
	sessionProject(t)
	pair := hunkEditPair("src/a.go", sessionDay)
	other := hunkEditPair("docs/b.md", sessionDay.Add(time.Hour))
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, append(pair, other...)...))
	_ = runMemoryCommand(t, runMemoryIngest)
	memoryArgs.glob = "docs/**"
	memoryArgs.since = "2026-10-18"
	memoryArgs.tool = "Edit"

	output := runCorrections(t)

	if !strings.Contains(output, "Total estimated minutes: 2") || !strings.Contains(output, "apply to Session Corrections only") {
		t.Errorf("output = %s", output)
	}
}

func TestSessionsJSONEmitsTheObjectAndPlainJSONStaysABareArray(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)
	memoryArgs.asJSON = true

	plain := runCorrections(t)
	memoryArgs.sessions = true
	object := runCorrections(t)

	var array []map[string]any
	if err := json.Unmarshal([]byte(plain), &array); err != nil {
		t.Errorf("--json is no longer a bare array: %v\n%s", err, plain)
	}
	var report memory.SessionReport
	if err := json.Unmarshal([]byte(object), &report); err != nil {
		t.Fatalf("--sessions --json: %v\n%s", err, object)
	}
	if report.State != "ready" || report.TotalEstimatedMinutes != 2 || len(report.Kinds) != 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestSessionFilterFlagsWithPlainJSONAreRefused(t *testing.T) {
	sessionProject(t)
	memoryArgs.asJSON = true
	memoryArgs.tool = "Edit"

	err := runMemoryCorrections(&cobra.Command{}, nil)

	if err == nil || !strings.Contains(err.Error(), "--sessions") {
		t.Errorf("error = %v", err)
	}
}

func TestAFilterThatMatchesNothingSaysSo(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)
	memoryArgs.tool = "NoSuchTool"

	output := runCorrections(t)

	if !strings.Contains(output, "No session corrections match.") || strings.Contains(output, "Total estimated minutes") {
		t.Errorf("output = %s", output)
	}
}

func TestAnInvalidCaptureConfigurationStopsTheReport(t *testing.T) {
	sessionProject(t)
	if err := os.MkdirAll(".claude", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionLayout().ConfigPath(), []byte("window: [not valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runMemoryCorrections(&cobra.Command{}, nil)

	if err == nil {
		t.Error("an unreadable capture.yaml must not be ignored")
	}
}

func TestInvalidSinceIsRejected(t *testing.T) {
	sessionProject(t)
	memoryArgs.since = "yesterday"

	err := runMemoryCorrections(&cobra.Command{}, nil)

	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Errorf("error = %v", err)
	}
}

func TestParseSinceAcceptsADateAndAnRFC3339Time(t *testing.T) {
	date, errDate := parseSince("2026-10-18")
	instant, errTime := parseSince("2026-10-18T12:00:30.5Z")

	if errDate != nil || !date.Equal(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v, %v", date, errDate)
	}
	if errTime != nil || instant.Nanosecond() != 500_000_000 {
		t.Errorf("time = %v, %v", instant, errTime)
	}
}

// --- the header value: newest event time from a 4 KB tail ---

func newestFrom(t *testing.T, files map[string]string) (time.Time, bool) {
	t.Helper()
	sessionProject(t)
	for name, content := range files {
		writeDayFile(t, name, content)
	}
	at, isFound, err := newDaySessionSource(sessionLayout()).NewestEventAt(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return at, isFound
}

func stopAt(t *testing.T, at time.Time) string {
	t.Helper()
	return encodeEvents(t, capture.NewSessionEvent("s", at, capture.EventStop))
}

func TestNewestEventIsTheLastCompleteValidLineOfTheNewestFile(t *testing.T) {
	old := stopAt(t, sessionDay.Add(-time.Hour))
	last := stopAt(t, sessionDay.Add(time.Minute))

	at, isFound := newestFrom(t, map[string]string{
		"2026-10-17.jsonl": stopAt(t, sessionDay.Add(48*time.Hour)),
		"2026-10-18.jsonl": old + last,
	})

	if !isFound || !at.Equal(sessionDay.Add(time.Minute)) {
		t.Errorf("newest = %v %v, want 12:01", at, isFound)
	}
}

func TestNewestEventSkipsAFinalLineWithoutANewline(t *testing.T) {
	complete := stopAt(t, sessionDay)
	unterminated := strings.TrimSuffix(stopAt(t, sessionDay.Add(time.Hour)), "\n")

	at, isFound := newestFrom(t, map[string]string{"2026-10-18.jsonl": complete + unterminated})

	if !isFound || !at.Equal(sessionDay) {
		t.Errorf("newest = %v %v, want the complete line at 12:00", at, isFound)
	}
}

func TestNewestEventSkipsAnInvalidLastCompleteLine(t *testing.T) {
	complete := stopAt(t, sessionDay)

	at, isFound := newestFrom(t, map[string]string{"2026-10-18.jsonl": complete + "not json\n"})

	if !isFound || !at.Equal(sessionDay) {
		t.Errorf("newest = %v %v", at, isFound)
	}
}

func TestNewestEventReadsOnlyTheTailOfALargeFile(t *testing.T) {
	filler := strings.Repeat(stopAt(t, sessionDay.Add(-time.Hour)), 500)
	last := stopAt(t, sessionDay.Add(time.Minute))
	if len(filler) < 4*memory.HealthTailBytes {
		t.Fatalf("filler of %d bytes is too small", len(filler))
	}

	at, isFound := newestFrom(t, map[string]string{"2026-10-18.jsonl": filler + last})

	if !isFound || !at.Equal(sessionDay.Add(time.Minute)) {
		t.Errorf("newest = %v %v", at, isFound)
	}
}

func TestNewestEventIsUnknownWithoutACompleteLineOrAFile(t *testing.T) {
	_, hasTornOnly := newestFrom(t, map[string]string{"2026-10-18.jsonl": `{"v":1,"id":"torn"`})
	_, hasEmptyFile := newestFrom(t, map[string]string{"2026-10-18.jsonl": ""})
	_, hasHugeLineOnly := newestFrom(t, map[string]string{"2026-10-18.jsonl": strings.Repeat("x", 3*memory.HealthTailBytes) + "\n"})
	_, hasFile := newestFrom(t, map[string]string{})

	if hasTornOnly || hasEmptyFile || hasHugeLineOnly || hasFile {
		t.Errorf("found: torn=%v empty=%v huge=%v none=%v, want all false", hasTornOnly, hasEmptyFile, hasHugeLineOnly, hasFile)
	}
}

func TestHeaderPrintsUnknownWhenNoCompleteLineIsLeft(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...))
	_ = runMemoryCommand(t, runMemoryIngest)
	writeDayFile(t, "2026-10-18.jsonl", `{"v":1,"id":"torn"`)

	output := runCorrections(t)

	if !strings.Contains(output, "Newest recorded event: unknown\n") {
		t.Errorf("output = %s", output)
	}
}

func TestATornLastLineDoesNotChangeTheHeader(t *testing.T) {
	sessionProject(t)
	writeDayFile(t, "2026-10-18.jsonl", encodeEvents(t, hunkEditPair("src/a.go", sessionDay)...)+`{"v":1,"id":"torn"`)
	_ = runMemoryCommand(t, runMemoryIngest)

	output := runCorrections(t)

	if !strings.Contains(output, "Newest recorded event: 2026-10-18T12:00:30.000Z") {
		t.Errorf("the torn last line must be skipped:\n%s", output)
	}
}
