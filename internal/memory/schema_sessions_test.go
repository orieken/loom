package memory_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

func sessionSchemaText(t *testing.T, store *memory.Store) string {
	t.Helper()
	rows, err := openReadOnly(t, store).Query(
		`SELECT name, sql FROM sqlite_master WHERE name LIKE '%session%' ORDER BY name`)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var text strings.Builder
	for rows.Next() {
		var name, statement string
		if err := rows.Scan(&name, &statement); err != nil {
			t.Fatalf("scan: %v", err)
		}
		text.WriteString(name + ": " + statement + "\n")
	}
	return text.String()
}

func TestIngestingTwiceLeavesTheSameSessionSchema(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, &fakeSessionSource{})
	first := sessionSchemaText(t, store)

	ingestSessions(t, store, &fakeSessionSource{})

	if second := sessionSchemaText(t, store); second != first {
		t.Errorf("schema changed on the second run:\n%s\nvs\n%s", first, second)
	}
	for _, want := range []string{"session_corrections", "idx_session_corrections_at", "idx_session_corrections_tool"} {
		if !strings.Contains(first, want) {
			t.Errorf("schema lacks %s:\n%s", want, first)
		}
	}
	if strings.Contains(first, "session_events") {
		t.Errorf("session_events must not exist:\n%s", first)
	}
}

// Open runs for every `loom memory` command. A session table made there
// would come back after uninstall.
func TestOpenCreatesNoSessionTable(t *testing.T) {
	store := newSessionStore(t)

	for _, name := range tableNames(t, store) {
		if strings.HasPrefix(name, "session_") {
			t.Errorf("Open created %s", name)
		}
	}
}

func TestSchemaVersionStaysAtThree(t *testing.T) {
	if memory.SchemaVersion != 3 {
		t.Errorf("SchemaVersion = %d, want 3: a bump would drop every session row", memory.SchemaVersion)
	}
}

func TestReopeningAVersionThreeStoreKeepsSessionRows(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, &fakeSessionSource{events: hunkEditEvents("src/a.go", 0)})
	path := store.Path()
	_ = store.Close()

	reopened, err := memory.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if rows := storedRows(t, reopened); len(rows) != 1 {
		t.Errorf("reopen kept %d session rows, want 1", len(rows))
	}
}

func TestVersionMismatchDropsTheTableAndTheReportSaysNoData(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: hunkEditEvents("src/a.go", 0)}
	ingestSessions(t, store, source)
	path := store.Path()
	_ = store.Close()
	forceVersion(t, path, 2)

	reopened, err := memory.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if hasTable(t, reopened, "session_corrections") {
		t.Fatal("a version mismatch left session_corrections behind")
	}
	report := sessionReport(t, reopened, source, capture.DefaultConfig(), memory.SessionFilter{})
	if report.State != memory.SessionStateNoData {
		t.Errorf("state = %q, want %q", report.State, memory.SessionStateNoData)
	}
	ingestSessions(t, reopened, source)
	if rows := storedRows(t, reopened); len(rows) != 1 {
		t.Errorf("ingest refilled %d rows, want 1", len(rows))
	}
}

func forceVersion(t *testing.T, path string, version int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`UPDATE schema_meta SET version = ?`, version); err != nil {
		t.Fatalf("force version: %v", err)
	}
}

func TestIngestLimitsTheDatabaseToItsOwner(t *testing.T) {
	store := newSessionStore(t)

	ingestSessions(t, store, &fakeSessionSource{})

	mode, _ := fileMode(t, store.Path())
	if mode != 0o600 {
		t.Errorf("episodes.db mode = %o, want 600", mode)
	}
}

// The sidecars of modernc.org/sqlite were unverified. This test records what
// the driver does during an open write transaction and fails if any sidecar
// is open to group or others.
func TestSidecarsOfAWriteTransactionAreLimitedToTheOwner(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, &fakeSessionSource{})
	db := openReadOnly(t, store)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO session_corrections (seq, kind, signal, at) VALUES (1, 'revert', 'revert', 'x')`); err != nil {
		t.Fatal(err)
	}

	for _, sidecar := range sidecars(store.Path()) {
		mode, isPresent := fileMode(t, sidecar)
		t.Logf("sidecar %s present=%v mode=%o", sidecar[len(store.Path()):], isPresent, mode)
		if isPresent && mode&0o077 != 0 {
			t.Errorf("%s is open to group or others: %o", sidecar, mode)
		}
	}
}

// "…:00.000Z" must sort before "…:00.500Z" as text, and --since must use the
// same format. RFC3339Nano would write "…:00Z", which sorts after both.
func TestSessionTimesHaveFixedWidthAndSortAsText(t *testing.T) {
	store := newSessionStore(t)
	events := []capture.SessionEvent{
		promptEvent("p1", 0),
		promptEvent("p2", 500*time.Millisecond),
	}
	ingestSessions(t, store, &fakeSessionSource{events: events})

	rows := storedRows(t, store)

	if len(rows) != 2 || rows[0].At != "2026-10-18T12:00:00.000Z" || rows[1].At != "2026-10-18T12:00:00.500Z" {
		t.Fatalf("stored times = %+v", rows)
	}
	cutoff := sessionBase.Add(250 * time.Millisecond)
	report := sessionReport(t, store, &fakeSessionSource{}, capture.DefaultConfig(), memory.SessionFilter{Since: cutoff})
	if len(report.Kinds) != 1 || report.Kinds[0].Count != 1 {
		t.Errorf("--since 12:00:00.250 kept %+v, want the .500 row only", report.Kinds)
	}
}
