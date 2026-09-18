// Package memory is the episodic store: what runs actually did, kept
// beyond the life of a feature workspace (roadmap L3.5).
//
// The framework's memory has been semantic only — markdown KIs and ADRs,
// which record what someone concluded and never what happened. This records
// what happened: which stages ran, how long they took, what they cost, how
// many rounds a review loop needed, which gates a human stopped at, and
// whose output a human had to correct.
//
// It collects nothing. Four epics already produce all of this per run —
// L3.8 (usage), L4.5 (corrections), L2.16 (policy decisions), L2.17
// (iterations), L3.0 (routing) — and it dies with the workspace. This
// package reads the records the executor already writes and makes them
// queryable. If a query needs a fact nothing records, that is a finding to
// report, not a licence to add an emitter.
//
// The archived JSONL is the durable record and this store is a projection
// of it, which is why re-ingest is idempotent: deleting episodes.db is a
// recoverable event, not a loss.
package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Register the pure-Go sqlite driver, as the BM25 retriever does.
	_ "modernc.org/sqlite"
)

// DefaultDir is where a project keeps its episodic store, relative to the
// project root. Project-local, like every other framework record: a global
// store would pool data from every repository loom runs in, including
// client code, into one file outside those repositories.
const DefaultDir = ".claude/memory"

// DefaultFileName is the store itself.
const DefaultFileName = "episodes.db"

// DefaultPath is the store's location under a project root.
func DefaultPath(projectRoot string) string {
	return filepath.Join(projectRoot, DefaultDir, DefaultFileName)
}

// schema is applied on every open. Every statement is idempotent, so
// opening an existing store is the same operation as creating one.
//
// Cost and correction counts are columns rather than fields inside a JSON
// blob, because they are what anyone actually asks about and a blob is not
// queryable without unpacking every row.
const schema = `
CREATE TABLE IF NOT EXISTS runs (
	run_id        TEXT PRIMARY KEY,
	feature       TEXT NOT NULL,
	plan          TEXT NOT NULL,
	created_by    TEXT NOT NULL,
	spec_path     TEXT,
	started_at    TEXT NOT NULL,
	updated_at    TEXT,
	completed     INTEGER NOT NULL DEFAULT 0,
	waiting_gate  TEXT,
	input_tokens  INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd      REAL NOT NULL DEFAULT 0,
	ingested_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stages (
	run_id        TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
	stage_id      TEXT NOT NULL,
	agent         TEXT,
	status        TEXT NOT NULL,
	sequence      INTEGER NOT NULL DEFAULT 0,
	iterations    INTEGER NOT NULL DEFAULT 0,
	gate          TEXT,
	skip_reason   TEXT,
	started_at    TEXT,
	finished_at   TEXT,
	duration_ms   INTEGER,
	input_tokens  INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd      REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (run_id, stage_id)
);

CREATE TABLE IF NOT EXISTS events (
	run_id   TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
	seq      INTEGER NOT NULL,
	at       TEXT NOT NULL,
	kind     TEXT NOT NULL,
	stage_id TEXT,
	gate     TEXT,
	agent    TEXT,
	detail   TEXT,
	PRIMARY KEY (run_id, seq)
);

CREATE TABLE IF NOT EXISTS corrections (
	run_id    TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
	seq       INTEGER NOT NULL,
	stage_id  TEXT NOT NULL,
	agent     TEXT,
	gate      TEXT,
	added     INTEGER NOT NULL DEFAULT 0,
	removed   INTEGER NOT NULL DEFAULT 0,
	diff_path TEXT,
	at        TEXT,
	PRIMARY KEY (run_id, seq)
);

CREATE TABLE IF NOT EXISTS policy_decisions (
	run_id   TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
	seq      INTEGER NOT NULL,
	gate     TEXT NOT NULL,
	effect   TEXT NOT NULL,
	honoured INTEGER NOT NULL DEFAULT 0,
	at       TEXT,
	PRIMARY KEY (run_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_stages_agent ON stages(agent);
CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind);
CREATE INDEX IF NOT EXISTS idx_corrections_agent ON corrections(agent);
`

// SchemaVersion is bumped whenever the tables change shape.
//
// A mismatch rebuilds the store from scratch rather than migrating it. That
// is only safe because the store is a PROJECTION: run-state.json,
// run-events.jsonl and the typed stage documents under state/ are archived
// in git, and `loom memory ingest` rebuilds everything from them. Migrations
// are for records; this is a cache.
const SchemaVersion = 2

const versionTable = `CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`

// dropAll is applied when the version does not match. Order matters only in
// that children go before parents.
const dropAll = `
DROP TABLE IF EXISTS policy_decisions;
DROP TABLE IF EXISTS corrections;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS stages;
DROP TABLE IF EXISTS runs;
`

// Store is an open episodic database.
type Store struct {
	db   *sql.DB
	path string
}

// Open creates or opens the store at path, applying the schema. Foreign
// keys are enabled explicitly — sqlite defaults them off, and the cascade
// deletes are what make re-ingest a replace rather than a duplicate.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open episodic store %s: %w", path, err)
	}
	if err := initialise(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("episodic store %s: %w", path, err)
	}
	return &Store{db: db, path: path}, nil
}

func initialise(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := reconcileVersion(db); err != nil {
		return err
	}
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// reconcileVersion drops a store built by an older schema. Nothing is lost
// that `loom memory ingest` cannot restore from the archived records, which
// is the whole reason the store is allowed to be disposable.
func reconcileVersion(db *sql.DB) error {
	if _, err := db.Exec(versionTable); err != nil {
		return fmt.Errorf("create schema_meta: %w", err)
	}
	var found int
	err := db.QueryRow(`SELECT version FROM schema_meta LIMIT 1`).Scan(&found)
	if err == nil && found == SchemaVersion {
		return nil
	}
	if _, err := db.Exec(dropAll); err != nil {
		return fmt.Errorf("rebuild store for schema %d: %w", SchemaVersion, err)
	}
	if _, err := db.Exec(`DELETE FROM schema_meta`); err != nil {
		return fmt.Errorf("reset schema version: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_meta (version) VALUES (?)`, SchemaVersion); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	return nil
}

// Path returns the file the store lives in.
func (s *Store) Path() string { return s.path }

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }
