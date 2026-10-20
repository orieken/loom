package memory

// The session-capture projection: one table, session_corrections.
//
// The table is NOT part of the `schema` constant. `Open` runs `schema` for
// every `loom memory` command, so a table in `schema` would come back, empty,
// after `loom uninstall`. Only session ingest creates this table.
//
// SchemaVersion does not change. A bump would run dropAll and rebuild from
// archived runs only, which would delete every session row.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Limits of session ingest and the session report.
const (
	// SessionBusyTimeout is how long a connection waits for a lock held by
	// another process.
	SessionBusyTimeout = 5 * time.Second
	// IngestDeadline bounds one session ingest.
	IngestDeadline = 120 * time.Second
	// ReportDeadline bounds one session report.
	ReportDeadline = 30 * time.Second
	// HealthTailBytes is the most that is read from the end of the newest
	// day file to find the newest event time.
	HealthTailBytes = 4 << 10

	sessionTableName = "session_corrections"
	// sessionTimeLayout has a fixed width, always in UTC. RFC3339Nano drops
	// trailing zeros, so "…:00Z" would sort after "…:00.5Z" as text.
	sessionTimeLayout = "2006-01-02T15:04:05.000Z"
	sessionFileMode   = 0o600
	// defaultSessionDriver is the SQL driver of the session handles.
	defaultSessionDriver = "sqlite"
)

// sessionDriverName is a variable so a test can swap in a driver that counts
// statements.
var sessionDriverName = defaultSessionDriver

const sessionSchema = `
CREATE TABLE IF NOT EXISTS session_corrections (
	seq      INTEGER PRIMARY KEY,
	kind     TEXT NOT NULL,
	signal   TEXT NOT NULL,
	at       TEXT NOT NULL,
	tool     TEXT,
	path     TEXT,
	path_ref TEXT
);

CREATE INDEX IF NOT EXISTS idx_session_corrections_at   ON session_corrections(at);
CREATE INDEX IF NOT EXISTS idx_session_corrections_tool ON session_corrections(tool);
`

// formatSessionTime renders a time in the fixed-width UTC form that the
// session table stores and compares as text.
func formatSessionTime(at time.Time) string {
	return at.UTC().Format(sessionTimeLayout)
}

// sessionDSN names the database file and sets the busy timeout on every
// connection that the pool opens.
func sessionDSN(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	query := url.Values{}
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", SessionBusyTimeout.Milliseconds()))
	address := url.URL{Scheme: "file", Path: absolute, RawQuery: query.Encode()}
	return address.String(), nil
}

// openSessionDB opens a handle with the session busy timeout. The caller
// closes it. Opening does not touch the file.
func openSessionDB(path string) (*sql.DB, error) {
	dsn, err := sessionDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(sessionDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open episodic store %s: %w", path, err)
	}
	return db, nil
}

// ensureSessionSchema creates the session table and its indexes. It is safe
// to run twice. Only session ingest calls it (IngestSessions): the report and
// `loom health` never do. The file is limited to its owner (mode 0600).
func ensureSessionSchema(ctx context.Context, db *sql.DB, path string) error {
	if err := restrictToOwner(path); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, sessionSchema); err != nil {
		return fmt.Errorf("apply session schema: %w", err)
	}
	return restrictToOwner(path)
}

// restrictToOwner creates the file when it is absent and limits it to the
// owner. A rollback journal takes the mode of the database file.
func restrictToOwner(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, sessionFileMode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, sessionFileMode); err != nil {
		return fmt.Errorf("limit %s to its owner: %w", path, err)
	}
	return nil
}
