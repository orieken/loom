package capture

import "time"

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
}

// DefaultConfig returns the compiled defaults.
func DefaultConfig() Config {
	return Config{
		Window:  DefaultCorrectionWindow,
		Weights: DefaultWeights(),
		Signals: DefaultSignalSwitches(),
	}
}
