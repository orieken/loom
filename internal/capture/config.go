package capture

import "time"

// The retention defaults of the spec: 90 days and 50 MB.
const (
	DefaultRetentionDays = 90
	DefaultStoreCapBytes = int64(50 << 20)
)

// Retention says which day files the purge may delete.
type Retention struct {
	// MaxAgeDays deletes a day file older than this many days.
	MaxAgeDays int
	// MaxBytes deletes the oldest day files while the store is larger.
	MaxBytes int64
}

// Config holds the capture settings. The compiled defaults apply when no
// configuration file exists. Reading the file belongs to an adapter.
type Config struct {
	// Window is the Correction window.
	Window time.Duration
	// Weights are the minutes per correction kind (estimates).
	Weights Weights
	// Signals are the per-signal switches.
	Signals SignalSwitches
	// RedactionGlobs are added to the default redaction list.
	RedactionGlobs []string
	// Retention is the purge policy.
	Retention Retention
}

// DefaultConfig returns the compiled defaults.
func DefaultConfig() Config {
	return Config{
		Window:    DefaultCorrectionWindow,
		Weights:   DefaultWeights(),
		Signals:   DefaultSignalSwitches(),
		Retention: Retention{MaxAgeDays: DefaultRetentionDays, MaxBytes: DefaultStoreCapBytes},
	}
}
