package capture

import "time"

// Per-call limits of the capture hook. Each value is a named constant so a
// test can pin it and a reviewer can find it in one place.
//
// Source of the values: architecture-notes.md revision 2, decision 3 (hook
// runtime budget), decision 2 (line and batch caps), decision 15 (fast-fail),
// decision 9 (clock anomaly) and performance-report.md mandates A, D and G.
const (
	// HookStopAfter is the watchdog time, counted from main().
	HookStopAfter = 500 * time.Millisecond
	// CounterWriteWait is the longest wait for the failure-counter write
	// before the hook exits (mandate A).
	CounterWriteWait = 50 * time.Millisecond
	// StallWindow is the fast-fail window after a stall (decision 15).
	StallWindow = 60 * time.Second
	// StallCause is the failure-counter byte that marks a stall.
	StallCause byte = 't'

	// MaxStdinBytes caps the hook payload read from standard input.
	MaxStdinBytes = 4 << 20
	// MaxReadBudgetBytes caps all bytes read in one call (mandate D).
	MaxReadBudgetBytes = 4 << 20
	// MaxTargetFileBytes caps one file; a larger file is never read.
	MaxTargetFileBytes = 1 << 20
	// MaxTailBytes caps the backward read of the day-file tail.
	MaxTailBytes = 256 << 10
	// TailChunkBytes is the chunk size of the backward tail read.
	TailChunkBytes = 32 << 10
	// MaxRehashedHunks caps the open hunks re-hashed in one call.
	MaxRehashedHunks = 20
	// MaxBatchBytes caps the single Write of one call.
	MaxBatchBytes = 16 << 10

	// MaxLineBytes caps one day-file line, newline included (mandate G).
	MaxLineBytes = 4096
	// MaxPathBytes caps the stored project-relative path.
	MaxPathBytes = 512
	// MaxArgKeys caps the argument keys kept in one line.
	MaxArgKeys = 16
	// MaxNameBytes caps an argument key name or a tool name.
	MaxNameBytes = 64

	// BytesPerMegabyte converts the megabyte figures of the settings and the
	// health report to bytes.
	BytesPerMegabyte = 1 << 20

	// PurgeInterval is the retention purge rate limit.
	PurgeInterval = time.Hour
	// ClockAnomalyThreshold is the backward step that ingest reports.
	ClockAnomalyThreshold = 5 * time.Second
	// DefaultCorrectionWindow is the default Correction window.
	DefaultCorrectionWindow = 10 * time.Minute
)
