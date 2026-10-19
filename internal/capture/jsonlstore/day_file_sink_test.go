package jsonlstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

func TestDaySinkRoutesByTheUTCDateInAnyLocalZone(t *testing.T) {
	zone := time.FixedZone("UTC-6", -6*60*60)
	tests := []struct {
		name     string
		at       time.Time
		wantFile string
	}{
		{name: "23:59:59.9 UTC", at: time.Date(2026, 3, 4, 17, 59, 59, 900_000_000, zone), wantFile: "2026-03-04.jsonl"},
		{name: "00:00:00.1 UTC", at: time.Date(2026, 3, 4, 18, 0, 0, 100_000_000, zone), wantFile: "2026-03-05.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			// The literal keeps the local zone: NewSessionEvent would convert it.
			local := capture.SessionEvent{V: capture.EventVersion, ID: "e1", At: tt.at, Kind: capture.EventStop}

			err := NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{local})

			if err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(layout.SessionsDir())
			if len(entries) != 1 || entries[0].Name() != tt.wantFile {
				t.Fatalf("files = %v, want only %s", entries, tt.wantFile)
			}
		})
	}
}

func TestDaySinkTwoInstantsAcrossMidnightLandInTwoFiles(t *testing.T) {
	layout := newLayout(t)
	sink := NewDaySink(layout)
	before := time.Date(2026, 3, 4, 23, 59, 59, 900_000_000, time.UTC)
	after := before.Add(200 * time.Millisecond)

	_ = sink.Append(t.Context(), []capture.SessionEvent{event("a", before, capture.EventStop, 0)})
	_ = sink.Append(t.Context(), []capture.SessionEvent{event("b", after, capture.EventStop, 0)})

	if !strings.Contains(readFile(t, layout.DayFile(before)), `"id":"a"`) || !strings.Contains(readFile(t, layout.DayFile(after)), `"id":"b"`) {
		t.Fatal("each event must be in the file of its own UTC date")
	}
}

func TestDaySinkCreatesPrivateFilesAndDirectory(t *testing.T) {
	layout := newLayout(t)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	if err := NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{event("a", at, capture.EventStop, 0)}); err != nil {
		t.Fatal(err)
	}

	assertMode(t, layout.SessionsDir(), 0o700)
	assertMode(t, layout.DayFile(at), 0o600)
}

func TestDaySinkDoesNotCreateTheParentDirectories(t *testing.T) {
	layout := capture.NewLayout(t.TempDir())

	err := NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{event("a", time.Now(), capture.EventStop, 0)})

	if err == nil {
		t.Fatal("want an error for a root without .claude/memory")
	}
	if _, statErr := os.Stat(layout.SessionsDir()); statErr == nil {
		t.Fatal("the sink must not build the capture directories")
	}
}

func TestDaySinkBatchStaysWithinTheCapAndKeepsTheMainEventFirst(t *testing.T) {
	layout := newLayout(t)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	var events []capture.SessionEvent
	for i := range 10 {
		events = append(events, event(fmt.Sprintf("e%d", i), at, capture.EventHunkObserved, 3000))
	}

	if err := NewDaySink(layout).Append(t.Context(), events); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, layout.DayFile(at))
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	equal(t, "batch within the cap", len(content) <= capture.MaxBatchBytes, true)
	equal(t, "some line written", len(lines) > 0, true)
	equal(t, "fewer lines than events", len(lines) < len(events), true)
	for i, line := range lines {
		equal(t, fmt.Sprintf("line %d is event e%d", i, i), strings.Contains(line, fmt.Sprintf(`"id":"e%d"`, i)), true)
	}
}

func TestDaySinkDropsALaterLineThatDoesNotFitButKeepsTheMainEvent(t *testing.T) {
	layout := newLayout(t)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	tooLong := event("long", at, capture.EventHunkObserved, 5000)
	events := []capture.SessionEvent{event("main", at, capture.EventToolUse, 0), tooLong, event("late", at, capture.EventStop, 0)}

	err := NewDaySink(layout).Append(t.Context(), events)

	content := readFile(t, layout.DayFile(at))
	if err != nil || !strings.Contains(content, `"id":"main"`) || strings.Contains(content, `"id":"long"`) || strings.Contains(content, `"id":"late"`) {
		t.Fatalf("err = %v, content = %.200s, want only the main event", err, content)
	}
}

func TestDaySinkRefusesAMainEventThatCannotFit(t *testing.T) {
	layout := newLayout(t)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	err := NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{event("long", at, capture.EventToolUse, 5000)})

	if !errors.Is(err, capture.ErrLineTooLong) {
		t.Fatalf("err = %v, want ErrLineTooLong", err)
	}
	if _, statErr := os.Stat(layout.DayFile(at)); statErr == nil {
		t.Fatal("no file may be written for a refused event")
	}
}

func TestDaySinkTrimsAnOversizedEventToTheLineCapAndItStillParses(t *testing.T) {
	layout := newLayout(t)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	big := event("big", at, capture.EventToolUse, 0)
	var keys, types []string
	for range 200 {
		keys = append(keys, strings.Repeat("k", 64))
		types = append(types, "string")
	}
	big = big.WithArgShape(keys, types)

	err := NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{big})

	line := strings.TrimSuffix(readFile(t, layout.DayFile(at)), "\n")
	stored, decodeErr := capture.DecodeLine([]byte(line))
	if err != nil || len(line)+1 > capture.MaxLineBytes || decodeErr != nil || !stored.ArgsTruncated {
		t.Fatalf("err = %v, decode = %v, bytes = %d, truncated = %v, want a parsed line within the cap", err, decodeErr, len(line)+1, stored.ArgsTruncated)
	}
}

func TestDaySinkWritesNothingForNoEventsOrACancelledContext(t *testing.T) {
	layout := newLayout(t)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	one := []capture.SessionEvent{event("a", time.Now(), capture.EventStop, 0)}

	if err := NewDaySink(layout).Append(t.Context(), nil); err != nil {
		t.Fatalf("empty append: %v", err)
	}
	if err := NewDaySink(layout).Append(cancelled, one); err == nil {
		t.Fatal("a cancelled context must stop the append")
	}
	if _, err := os.Stat(layout.SessionsDir()); err == nil {
		t.Fatal("nothing may be created")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

// The concurrency test re-runs this test binary as two real processes.
const (
	appenderEnv      = "JSONLSTORE_APPENDER_TAG"
	appenderRootEnv  = "JSONLSTORE_APPENDER_ROOT"
	singleAppends    = 1000
	batchAppends     = 250
	batchSize        = 4
	singlePathLength = 3700
	batchPathLength  = 3600
)

var concurrentDay = time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

// runAppender is the body of one appender process.
func runAppender() {
	tag := os.Getenv(appenderEnv)
	sink := NewDaySink(capture.NewLayout(os.Getenv(appenderRootEnv)))
	for i := range singleAppends {
		single := event(fmt.Sprintf("%s-single-%04d", tag, i), concurrentDay, capture.EventToolUse, singlePathLength)
		must(sink.Append(context.Background(), []capture.SessionEvent{single}))
	}
	for b := range batchAppends {
		var batch []capture.SessionEvent
		for k := range batchSize {
			batch = append(batch, event(fmt.Sprintf("%s-batch-%04d-%d", tag, b, k), concurrentDay, capture.EventToolUse, batchPathLength))
		}
		must(sink.Append(context.Background(), batch))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func TestDaySinkTwoRealProcessesAppendWithoutLoss(t *testing.T) {
	layout := newLayout(t)
	var processes []*exec.Cmd
	for _, tag := range []string{"p1", "p2"} {
		process := exec.Command(os.Args[0], "-test.run=^$")
		process.Env = append(os.Environ(), appenderEnv+"="+tag, appenderRootEnv+"="+layout.Root())
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		processes = append(processes, process)
	}
	for _, process := range processes {
		if err := process.Wait(); err != nil {
			t.Fatalf("appender failed: %v", err)
		}
	}

	assertConcurrentFile(t, readFile(t, layout.DayFile(concurrentDay)))
}

func assertConcurrentFile(t *testing.T, content string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	want := 2 * (singleAppends + batchAppends*batchSize)
	if len(lines) != want {
		t.Fatalf("lines = %d, want %d: a line was lost or glued", len(lines), want)
	}
	seen := make(map[string]bool, want)
	previousID := ""
	for index, line := range lines {
		var stored capture.SessionEvent
		if len(line)+1 > capture.MaxLineBytes || json.Unmarshal([]byte(line), &stored) != nil || seen[stored.ID] {
			t.Fatalf("line %d is torn, too long or a duplicate: %.80s", index, line)
		}
		seen[stored.ID] = true
		assertBatchIsContiguous(t, stored.ID, previousID)
		previousID = stored.ID
	}
}

// assertBatchIsContiguous checks that the lines of one batch follow each
// other in the file, which only a single Write call guarantees. The id has
// the form <process>-batch-<batch>-<position>.
func assertBatchIsContiguous(t *testing.T, id, previousID string) {
	t.Helper()
	parts := strings.Split(id, "-")
	if parts[1] != "batch" || parts[3] == "0" {
		return
	}
	var position int
	_, _ = fmt.Sscanf(parts[3], "%d", &position)
	want := fmt.Sprintf("%s-batch-%s-%d", parts[0], parts[2], position-1)
	if previousID != want {
		t.Fatalf("batch line %s follows %s, want %s: another write interleaved", id, previousID, want)
	}
}
