package capture

import "time"

// CorrectionKind is the reported category of a Session Correction.
type CorrectionKind string

// The correction kinds. KindUnknown marks a change that cannot be
// attributed to a person (a Bash call may explain it). It is never counted.
const (
	KindHunkEdit         CorrectionKind = "hunk-edit"
	KindRevert           CorrectionKind = "revert"
	KindDeniedCall       CorrectionKind = "denied-call"
	KindCorrectivePrompt CorrectionKind = "corrective-prompt"
	KindUnknown          CorrectionKind = "unknown"
)

// Signal is the detector that found a correction. A per-signal switch reads
// it.
type Signal string

// The four signals.
const (
	SignalHunkEdit         Signal = "hunk-edit"
	SignalRevert           Signal = "revert"
	SignalDeniedCall       Signal = "denied-call"
	SignalCorrectivePrompt Signal = "corrective-prompt"
)

// SignalSwitches turns signals on and off. A signal that is not listed is
// off.
type SignalSwitches map[Signal]bool

// DefaultSignalSwitches turns on every signal except denied-call. Denied-call
// stays off until a person verifies how the host tool reports a denial
// (analysis criterion 12).
func DefaultSignalSwitches() SignalSwitches {
	return SignalSwitches{
		SignalHunkEdit:         true,
		SignalRevert:           true,
		SignalDeniedCall:       false,
		SignalCorrectivePrompt: true,
	}
}

// IsOn reports whether a signal is switched on.
func (s SignalSwitches) IsOn(signal Signal) bool {
	return s[signal]
}

// Correction is one detected Session Correction. It is derived data: a pure
// result of the ordered event stream and the Config.
type Correction struct {
	Kind    CorrectionKind
	Signal  Signal
	At      time.Time
	Tool    string
	Path    *string
	PathRef string
}

// IsCountable reports whether the correction counts in any total. An
// unattributed change does not (analysis criterion 13).
func (c Correction) IsCountable() bool {
	return c.Kind != KindUnknown
}
