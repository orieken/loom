package memory_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

var sessionBase = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

const (
	hashBefore = "hash-before"
	hashAfter  = "hash-after"
	hashHuman  = "hash-human"
)

// fakeSessionSource feeds a fixed event list to ingest. It is a test double
// for the SessionSource port, so the memory package is tested without files.
type fakeSessionSource struct {
	events     []capture.SessionEvent
	stats      memory.SessionReadStats
	hasFiles   bool
	newest     time.Time
	hasNewest  bool
	streamErr  error
	streamHook func(ctx context.Context)
}

func (f *fakeSessionSource) Stream(ctx context.Context, visit func(capture.SessionEvent) error) (memory.SessionReadStats, error) {
	if f.streamHook != nil {
		f.streamHook(ctx)
	}
	for i, event := range f.events {
		if f.streamErr != nil && i == len(f.events)-1 {
			return f.stats, f.streamErr
		}
		if err := visit(event); err != nil {
			return f.stats, err
		}
	}
	return f.stats, nil
}

func (f *fakeSessionSource) HasDayFiles() (bool, error) { return f.hasFiles, nil }

func (f *fakeSessionSource) NewestEventAt(context.Context) (time.Time, bool, error) {
	return f.newest, f.hasNewest, nil
}

func textPtr(text string) *string { return &text }

// hunkEditEvents is an agent write followed by a person's change: one
// hunk-edit correction at write time plus 30 seconds.
func hunkEditEvents(path string, offset time.Duration) []capture.SessionEvent {
	ref := "ref-" + path
	write := capture.NewSessionEvent("w-"+path, sessionBase.Add(offset), capture.EventToolResult).
		WithTool(capture.ToolEdit).WithPath(ref, textPtr(path)).
		WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 3, PreHash: hashBefore, PostHash: hashAfter})
	seen := capture.NewSessionEvent("o-"+path, sessionBase.Add(offset+30*time.Second), capture.EventHunkObserved).
		WithPath(ref, nil).WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 3, PostHash: hashHuman})
	return []capture.SessionEvent{write, seen}
}

// redactedEditEvents is a hunk-edit whose path was redacted: no display path.
func redactedEditEvents(offset time.Duration) []capture.SessionEvent {
	events := hunkEditEvents("secret.pem", offset)
	events[0] = events[0].WithPath("ref-secret", nil)
	events[1] = events[1].WithPath("ref-secret", nil)
	return events
}

func revertEvents(path string, offset time.Duration) []capture.SessionEvent {
	events := hunkEditEvents(path, offset)
	events[1] = events[1].WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 3, PostHash: hashBefore})
	return events
}

func promptEvent(id string, offset time.Duration) capture.SessionEvent {
	return capture.NewSessionEvent(id, sessionBase.Add(offset), capture.EventPrompt).
		WithPromptClass(capture.PromptCorrective)
}

func deniedEvent(id string, offset time.Duration) capture.SessionEvent {
	return capture.NewSessionEvent(id, sessionBase.Add(offset), capture.EventToolDenied).WithTool(capture.ToolEdit)
}

// unknownEvents is a change after a Bash call: the author cannot be known.
func unknownEvents(path string, offset time.Duration) []capture.SessionEvent {
	events := hunkEditEvents(path, offset)
	bash := capture.NewSessionEvent("b-"+path, sessionBase.Add(offset+10*time.Second), capture.EventToolResult).
		WithTool(capture.ToolBash)
	return []capture.SessionEvent{events[0], bash, events[1]}
}

func joinEvents(parts ...[]capture.SessionEvent) []capture.SessionEvent {
	var joined []capture.SessionEvent
	for _, part := range parts {
		joined = append(joined, part...)
	}
	return joined
}

func newSessionStore(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.Open(memory.DefaultPath(t.TempDir()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func ingestSessions(t *testing.T, store *memory.Store, source memory.SessionSource) memory.SessionIngest {
	t.Helper()
	result, err := store.IngestSessions(context.Background(), source, capture.DefaultConfig())
	if err != nil {
		t.Fatalf("IngestSessions: %v", err)
	}
	return result
}

func sessionReport(t *testing.T, store *memory.Store, source memory.SessionSource, cfg capture.Config, filter memory.SessionFilter) memory.SessionReport {
	t.Helper()
	report, err := store.SessionReport(context.Background(), source, cfg, filter)
	if err != nil {
		t.Fatalf("SessionReport: %v", err)
	}
	return report
}

type sessionRow struct {
	Seq                 int
	Kind, Signal, At    string
	Tool, Path, PathRef sql.NullString
}

// storedRows reads the table directly, so a test checks what is stored and
// not what the writer believes it stored.
func storedRows(t *testing.T, store *memory.Store) []sessionRow {
	t.Helper()
	rows, err := openReadOnly(t, store).Query(
		`SELECT seq, kind, signal, at, tool, path, path_ref FROM session_corrections ORDER BY seq`)
	if err != nil {
		t.Fatalf("read session rows: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var stored []sessionRow
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(&row.Seq, &row.Kind, &row.Signal, &row.At, &row.Tool, &row.Path, &row.PathRef); err != nil {
			t.Fatalf("scan: %v", err)
		}
		stored = append(stored, row)
	}
	return stored
}

func tableNames(t *testing.T, store *memory.Store) []string {
	t.Helper()
	rows, err := openReadOnly(t, store).Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name)
	}
	return names
}

func hasTable(t *testing.T, store *memory.Store, name string) bool {
	t.Helper()
	for _, candidate := range tableNames(t, store) {
		if candidate == name {
			return true
		}
	}
	return false
}

func manyHunkEdits(count int) []capture.SessionEvent {
	var events []capture.SessionEvent
	for i := 0; i < count; i++ {
		events = append(events, hunkEditEvents(fmt.Sprintf("src/f%d.go", i), time.Duration(i)*time.Hour)...)
	}
	return events
}

func fileMode(t *testing.T, path string) (os.FileMode, bool) {
	t.Helper()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm(), true
}

func sidecars(path string) []string {
	return []string{path + "-wal", path + "-shm", path + "-journal"}
}
