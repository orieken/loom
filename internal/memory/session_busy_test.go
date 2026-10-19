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

// holdExclusiveLock opens a second connection to the same file and takes
// the exclusive lock, so every other connection waits or fails busy.
func holdExclusiveLock(t *testing.T, store *memory.Store) {
	t.Helper()
	db, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN EXCLUSIVE`); err != nil {
		t.Fatalf("take the lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		_ = conn.Close()
		_ = db.Close()
	})
}

// A locked database must make ingest wait about SessionBusyTimeout and then
// fail with a plain message: not at once (no timeout set) and not unbounded.
func TestIngestOnALockedDatabaseFailsAfterTheBusyTimeout(t *testing.T) {
	t.Parallel()
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	holdExclusiveLock(t, store)
	started := time.Now()

	_, err := store.IngestSessions(context.Background(), mixedSource(), capture.DefaultConfig())

	elapsed := time.Since(started)
	if err == nil || !strings.Contains(err.Error(), "locked by another process") {
		t.Fatalf("error = %v, want the plain locked message", err)
	}
	if elapsed < memory.SessionBusyTimeout-time.Second || elapsed > memory.SessionBusyTimeout+10*time.Second {
		t.Errorf("failed after %s, want about %s", elapsed, memory.SessionBusyTimeout)
	}
}

func TestReportOnALockedDatabaseFailsAfterTheBusyTimeout(t *testing.T) {
	t.Parallel()
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	holdExclusiveLock(t, store)
	started := time.Now()

	_, err := store.SessionReport(context.Background(), mixedSource(), capture.DefaultConfig(), memory.SessionFilter{})

	elapsed := time.Since(started)
	if err == nil || !strings.Contains(err.Error(), "locked by another process") {
		t.Fatalf("error = %v, want the plain locked message", err)
	}
	if elapsed < memory.SessionBusyTimeout-time.Second || elapsed > memory.SessionBusyTimeout+10*time.Second {
		t.Errorf("failed after %s, want about %s", elapsed, memory.SessionBusyTimeout)
	}
}
