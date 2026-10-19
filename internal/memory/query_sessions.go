package memory

// The Session Corrections report. It is READ-ONLY: it never creates the
// table, never ingests and runs no write statement. A missing table is
// reported, not repaired.
//
// The number of SQL statements is constant: one existence check and one
// grouped aggregate, or one streamed select when --glob is set. It does not
// grow with the number of rows.

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// Report states and the fixed messages that go with the empty ones.
const (
	SessionStateReady       = "ready"
	SessionStateNoData      = "no-data"
	SessionStateNeedsIngest = "needs-ingest"

	SessionNoDataMessage      = "No session data"
	SessionNeedsIngestMessage = "No session data. Run `loom memory ingest`"
	// SessionNewestUnknown is printed when no complete event line is found.
	SessionNewestUnknown = "unknown"
)

// SessionFilter narrows the report. A zero value matches everything.
type SessionFilter struct {
	// Since keeps corrections at or after this time.
	Since time.Time
	// Tool keeps corrections of one tool.
	Tool string
	// Glob keeps corrections whose stored path matches. A correction with no
	// stored path (a redacted path, or no file) never matches.
	Glob string
}

// SessionKindRow is the count and the estimate of one correction kind.
type SessionKindRow struct {
	Kind             capture.CorrectionKind `json:"kind"`
	Count            int                    `json:"count"`
	EstimatedMinutes int                    `json:"estimatedMinutes"`
}

// SessionReport is the Session Corrections section.
type SessionReport struct {
	State  string `json:"state"`
	Method string `json:"method"`
	// NewestEventAt is the time of the newest recorded event, or "unknown".
	// It comes from the day files, so it can be newer than the last ingest.
	NewestEventAt         string           `json:"newestEventAt"`
	Kinds                 []SessionKindRow `json:"kinds"`
	TotalEstimatedMinutes int              `json:"totalEstimatedMinutes"`
	// Unattributed counts the corrections that are not counted in any total.
	Unattributed int `json:"unattributed"`
}

// SessionReport builds the report from the projection and the configuration
// of the weights and the signal switches.
func (s *Store) SessionReport(ctx context.Context, source SessionSource, cfg capture.Config, filter SessionFilter) (SessionReport, error) {
	ctx, cancel := context.WithTimeout(ctx, ReportDeadline)
	defer cancel()
	db, err := openSessionDB(s.path)
	if err != nil {
		return SessionReport{}, err
	}
	defer func() { _ = db.Close() }()
	report, err := buildSessionReport(ctx, db, source, cfg, filter)
	return report, plainSessionError("report", ReportDeadline, err)
}

func buildSessionReport(ctx context.Context, db *sql.DB, source SessionSource, cfg capture.Config, filter SessionFilter) (SessionReport, error) {
	hasTable, err := hasSessionTable(ctx, db)
	if err != nil {
		return SessionReport{}, err
	}
	if !hasTable {
		return absentSessionReport(source)
	}
	counts, err := countSessionKinds(ctx, db, cfg.Signals, filter)
	if err != nil {
		return SessionReport{}, err
	}
	newest, err := newestEventText(ctx, source)
	if err != nil {
		return SessionReport{}, err
	}
	return assembleSessionReport(counts, newest, cfg.Weights), nil
}

func hasSessionTable(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, sessionTableName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("look for the session table: %w", err)
	}
	return count > 0, nil
}

// absentSessionReport covers a store with no session table. Day files on
// disk mean an ingest would fill it.
func absentSessionReport(source SessionSource) (SessionReport, error) {
	hasFiles, err := source.HasDayFiles()
	if err != nil {
		return SessionReport{}, fmt.Errorf("look for day files: %w", err)
	}
	state := SessionStateNoData
	if hasFiles {
		state = SessionStateNeedsIngest
	}
	return SessionReport{State: state, Method: capture.MinutesMethod, NewestEventAt: SessionNewestUnknown,
		Kinds: []SessionKindRow{}}, nil
}

func newestEventText(ctx context.Context, source SessionSource) (string, error) {
	at, isFound, err := source.NewestEventAt(ctx)
	if err != nil {
		return "", fmt.Errorf("read newest event: %w", err)
	}
	if !isFound {
		return SessionNewestUnknown, nil
	}
	return formatSessionTime(at), nil
}

func assembleSessionReport(counts []kindCount, newest string, weights capture.Weights) SessionReport {
	estimator := capture.NewMinutesEstimator(weights)
	report := SessionReport{State: SessionStateReady, Method: estimator.Method(),
		NewestEventAt: newest, Kinds: []SessionKindRow{}}
	for _, row := range counts {
		if row.kind == capture.KindUnknown {
			report.Unattributed += row.count
			continue
		}
		minutes := estimator.EstimateKind(row.kind, row.count)
		report.TotalEstimatedMinutes += minutes
		report.Kinds = append(report.Kinds, SessionKindRow{Kind: row.kind, Count: row.count, EstimatedMinutes: minutes})
	}
	return report
}

type kindCount struct {
	kind  capture.CorrectionKind
	count int
}

// sessionFilterClause is the shared WHERE of both queries. The signal list
// holds one placeholder per signal that is switched on. SQLite accepts an
// empty IN list, which matches nothing.
func sessionFilterClause(signals capture.SignalSwitches, filter SessionFilter) (string, []any) {
	enabled := enabledSignals(signals)
	since := ""
	if !filter.Since.IsZero() {
		since = formatSessionTime(filter.Since)
	}
	args := []any{since, since, filter.Tool, filter.Tool}
	for _, signal := range enabled {
		args = append(args, string(signal))
	}
	clause := "WHERE (? = '' OR at >= ?) AND (? = '' OR tool = ?) AND signal IN (" +
		strings.TrimSuffix(strings.Repeat("?,", len(enabled)), ",") + ")"
	return clause, args
}

// enabledSignals lists the signals that are on, in a fixed order.
func enabledSignals(signals capture.SignalSwitches) []capture.Signal {
	all := []capture.Signal{capture.SignalHunkEdit, capture.SignalRevert,
		capture.SignalDeniedCall, capture.SignalCorrectivePrompt}
	enabled := make([]capture.Signal, 0, len(all))
	for _, signal := range all {
		if signals.IsOn(signal) {
			enabled = append(enabled, signal)
		}
	}
	return enabled
}

func countSessionKinds(ctx context.Context, db *sql.DB, signals capture.SignalSwitches, filter SessionFilter) ([]kindCount, error) {
	if filter.Glob != "" {
		return countKindsByGlob(ctx, db, signals, filter)
	}
	clause, args := sessionFilterClause(signals, filter)
	rows, err := db.QueryContext(ctx,
		`SELECT kind, COUNT(*) FROM session_corrections `+clause+` GROUP BY kind ORDER BY kind`, args...)
	if err != nil {
		return nil, fmt.Errorf("query session corrections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanKindCounts(rows)
}

func scanKindCounts(rows *sql.Rows) ([]kindCount, error) {
	var counts []kindCount
	for rows.Next() {
		var row kindCount
		if err := rows.Scan(&row.kind, &row.count); err != nil {
			return nil, fmt.Errorf("scan session count: %w", err)
		}
		counts = append(counts, row)
	}
	return counts, rows.Err()
}
