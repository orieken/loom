package memory

// Converting the executor's types into columns.
//
// The recurring concern here is the same one epics 84–87 kept meeting: a
// zero that means "not measured" must not be stored as a zero that means
// "measured as none". Durations and finish times are nullable for exactly
// that reason — a stage that never finished has no duration, and writing 0
// would put it top of any "fastest stage" query.

import (
	"database/sql"
	"strings"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// timestamp renders a time for storage, or the empty string for a zero
// time. RFC3339 with nanoseconds sorts lexicographically, which is what
// lets the queries order by it without parsing.
func timestamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func boolean(value bool) int {
	if value {
		return 1
	}
	return 0
}

// finishedAt is NULL rather than empty for a stage still in flight, so a
// query can distinguish "did not finish" from "finished at an unknown
// time".
func finishedAt(record orchestrator.StageRecord) sql.NullString {
	if record.FinishedAt == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: timestamp(*record.FinishedAt), Valid: true}
}

// durationMillis is NULL unless both ends are known. A stage that was
// skipped or never ran has no duration, and storing 0 would make it the
// fastest stage in every report.
func durationMillis(record orchestrator.StageRecord) sql.NullInt64 {
	if record.FinishedAt == nil || record.StartedAt.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: record.FinishedAt.Sub(record.StartedAt).Milliseconds(), Valid: true}
}

// eventDetail flattens the fields that vary by event kind into one
// searchable column. The kind-specific fields stay in their own columns
// where they exist; this is for the rest, so a reader is not left guessing
// why a stage went stale or what a correction changed.
func eventDetail(event orchestrator.Event) string {
	parts := []string{
		string(event.StaleReason), event.Reason, event.Loop,
		string(event.ApprovalMethod), event.Correction, event.Error,
	}
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " | ")
}

// joined flattens a string slice for storage. Empty becomes NULL rather
// than "": a decision with no conflict and one with an unrecorded conflict
// are different facts, and "" would merge them.
func joined(values []string) sql.NullString {
	if len(values) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.Join(values, ","), Valid: true}
}
