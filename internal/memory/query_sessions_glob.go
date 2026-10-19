package memory

// The --glob variant of the session report. A SQL index cannot serve it: the
// Go matcher defines `**`, and the SQLite GLOB operator reads `**` another
// way. The query narrows by time, tool and signal in SQL. The Go matcher
// then runs on the stored path of each streamed row.

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/orieken/loom/internal/capture"
)

// countKindsByGlob streams one select and counts in Go. It never holds all
// rows, and it never runs a query per row. A NULL path never matches.
func countKindsByGlob(ctx context.Context, db *sql.DB, signals capture.SignalSwitches, filter SessionFilter) ([]kindCount, error) {
	if err := capture.ValidateGlob(filter.Glob); err != nil {
		return nil, err
	}
	clause, args := sessionFilterClause(signals, filter)
	rows, err := db.QueryContext(ctx,
		`SELECT kind, path FROM session_corrections `+clause+` ORDER BY at, seq`, args...)
	if err != nil {
		return nil, fmt.Errorf("query session corrections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	counts, err := tallyMatchingPaths(rows, filter.Glob)
	if err != nil {
		return nil, err
	}
	return sortedKindCounts(counts), nil
}

func tallyMatchingPaths(rows *sql.Rows, glob string) (map[capture.CorrectionKind]int, error) {
	counts := map[capture.CorrectionKind]int{}
	for rows.Next() {
		var kind capture.CorrectionKind
		var path sql.NullString
		if err := rows.Scan(&kind, &path); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		if path.Valid && capture.MatchGlob(glob, path.String) {
			counts[kind]++
		}
	}
	return counts, rows.Err()
}

func sortedKindCounts(counts map[capture.CorrectionKind]int) []kindCount {
	sorted := make([]kindCount, 0, len(counts))
	for kind, count := range counts {
		sorted = append(sorted, kindCount{kind: kind, count: count})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].kind < sorted[j].kind })
	return sorted
}
