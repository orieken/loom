package memory

// Session ingest: rebuild session_corrections from the day files.
//
// The rows are a pure function of the day files and the capture
// configuration. Ingest replaces all rows in one transaction, so a second run
// gives the same rows, a purged day file loses its rows, and a failed run
// leaves the old rows in place.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// SessionReadStats counts the lines that a SessionSource skipped. A source
// never fails on these.
type SessionReadStats struct {
	// TornLines counts lines that do not parse, lines over the line cap and
	// a final line with no newline.
	TornLines int
	// UnknownVersionLines counts lines with a day-file version that the
	// reader does not know.
	UnknownVersionLines int
	// ClockAnomalies counts events earlier than the event before them by
	// more than capture.ClockAnomalyThreshold.
	ClockAnomalies int
}

// SessionSource reads the day files. The memory package defines it, and an
// adapter in the command layer implements it, so this package never imports
// the file adapter.
type SessionSource interface {
	// Stream calls visit for each valid event, in day-file date order and
	// then line order. A visit error stops the read and is returned.
	Stream(ctx context.Context, visit func(capture.SessionEvent) error) (SessionReadStats, error)
	// HasDayFiles reports whether any day file exists.
	HasDayFiles() (bool, error)
	// NewestEventAt reads at most HealthTailBytes from the end of the newest
	// day file and returns the time of the last complete valid line. The bool
	// is false when no such line exists.
	NewestEventAt(ctx context.Context) (time.Time, bool, error)
}

// SessionIngest is what one session ingest stored and skipped.
type SessionIngest struct {
	Corrections int
	// Unattributed counts the corrections of kind unknown.
	Unattributed int
	Stats        SessionReadStats
}

// IngestSessions replaces every session correction with the corrections that
// the day files hold now. All four signals are detected and stored, whatever
// the per-signal switches say: the report applies the switches, so a switch
// change needs no re-ingest.
func (s *Store) IngestSessions(ctx context.Context, source SessionSource, cfg capture.Config) (SessionIngest, error) {
	ctx, cancel := context.WithTimeout(ctx, IngestDeadline)
	defer cancel()
	db, err := openSessionDB(s.path)
	if err != nil {
		return SessionIngest{}, err
	}
	defer func() { _ = db.Close() }()
	if err := ensureSessionSchema(ctx, db, s.path); err != nil {
		return SessionIngest{}, plainSessionError("ingest", IngestDeadline, err)
	}
	result, err := replaceSessionRows(ctx, db, source, cfg)
	return result, plainSessionError("ingest", IngestDeadline, err)
}

func replaceSessionRows(ctx context.Context, db *sql.DB, source SessionSource, cfg capture.Config) (SessionIngest, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SessionIngest{}, fmt.Errorf("begin session ingest: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := streamIntoTable(ctx, tx, source, cfg)
	if err != nil {
		return SessionIngest{}, err
	}
	if err := tx.Commit(); err != nil {
		return SessionIngest{}, fmt.Errorf("commit session ingest: %w", err)
	}
	return result, nil
}

func streamIntoTable(ctx context.Context, tx *sql.Tx, source SessionSource, cfg capture.Config) (SessionIngest, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_corrections`); err != nil {
		return SessionIngest{}, fmt.Errorf("clear session corrections: %w", err)
	}
	insert, err := tx.PrepareContext(ctx, insertSessionCorrection)
	if err != nil {
		return SessionIngest{}, fmt.Errorf("prepare session insert: %w", err)
	}
	defer func() { _ = insert.Close() }()
	writer := &correctionWriter{ctx: ctx, insert: insert, detector: capture.NewDetector(detectAllSignals(cfg))}
	stats, err := source.Stream(ctx, writer.observe)
	if err != nil {
		return SessionIngest{}, fmt.Errorf("read day files: %w", err)
	}
	if err := writer.store(writer.detector.Close()); err != nil {
		return SessionIngest{}, err
	}
	return SessionIngest{Corrections: writer.seq, Unattributed: writer.unattributed, Stats: stats}, nil
}

const insertSessionCorrection = `INSERT INTO session_corrections (seq, kind, signal, at, tool, path, path_ref)
VALUES (?, ?, ?, ?, ?, ?, ?)`

// detectAllSignals returns the configuration with every signal on.
func detectAllSignals(cfg capture.Config) capture.Config {
	cfg.Signals = capture.SignalSwitches{
		capture.SignalHunkEdit: true, capture.SignalRevert: true,
		capture.SignalDeniedCall: true, capture.SignalCorrectivePrompt: true,
	}
	return cfg
}

// correctionWriter numbers the detector output and inserts it. The number
// follows the detector output order, which depends only on the day files and
// the configuration.
type correctionWriter struct {
	ctx          context.Context
	insert       *sql.Stmt
	detector     *capture.Detector
	seq          int
	unattributed int
}

func (w *correctionWriter) observe(event capture.SessionEvent) error {
	return w.store(w.detector.Observe(event))
}

func (w *correctionWriter) store(corrections []capture.Correction) error {
	for _, correction := range corrections {
		w.seq++
		if correction.Kind == capture.KindUnknown {
			w.unattributed++
		}
		if err := w.write(correction); err != nil {
			return err
		}
	}
	return nil
}

func (w *correctionWriter) write(correction capture.Correction) error {
	_, err := w.insert.ExecContext(w.ctx, w.seq, string(correction.Kind), string(correction.Signal),
		formatSessionTime(correction.At), optionalText(correction.Tool),
		optionalPath(correction.Path), optionalText(correction.PathRef))
	if err != nil {
		return fmt.Errorf("insert session correction: %w", err)
	}
	return nil
}

func optionalText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func optionalPath(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return optionalText(*value)
}

// plainSessionError turns a lock or a deadline into a message that says what
// happened and what to do. Any other error passes through. There is no retry
// loop: the busy timeout is the one wait.
func plainSessionError(action string, deadline time.Duration, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("session %s stopped after %s: %w", action, deadline, err)
	case strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked"):
		return fmt.Errorf("session %s: episodes.db is locked by another process (waited %s); try again later: %w",
			action, SessionBusyTimeout, err)
	}
	return err
}
