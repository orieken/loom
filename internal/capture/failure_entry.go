package capture

import (
	"strings"
	"time"
)

// FailureCause is the code of one hook failure (architecture-notes.md
// decision 3). The set is closed: it holds a short fixed word per failure
// path and never free text, a path, an error message or payload
// (architecture-guardrails.md #9).
type FailureCause string

// The failure causes. A new cause is added here and in FailureCauses on
// purpose; a test pins the set.
const (
	// CauseDecode is a payload, event name or flag that cannot be read.
	CauseDecode FailureCause = "decode"
	// CauseSalt is a salt the hook cannot use.
	CauseSalt FailureCause = "salt"
	// CauseConfig is a capture.yaml the hook cannot use.
	CauseConfig FailureCause = "config"
	// CauseWrite is a day file the hook cannot append to, and any error that
	// has no cause of its own.
	CauseWrite FailureCause = "write"
	// CauseStall is a call that the watchdog stopped. It is also the marker
	// of the fast-fail breaker (decision 15).
	CauseStall FailureCause = "stall"
	// CausePanic is a panic that the hook recovered.
	CausePanic FailureCause = "panic"
	// CauseFlag is a command-line flag error.
	CauseFlag FailureCause = "flag"
)

// FailureCauses lists the closed set, in the order health breaks a tie.
func FailureCauses() []FailureCause {
	return []FailureCause{CauseDecode, CauseSalt, CauseConfig, CauseWrite, CauseStall, CausePanic, CauseFlag}
}

// IsKnown reports whether the cause is a member of the closed set.
func (c FailureCause) IsKnown() bool {
	for _, known := range FailureCauses() {
		if c == known {
			return true
		}
	}
	return false
}

// FailureTimeLayout has a fixed width and is always UTC, so a log line has a
// fixed offset for its cause and sorts as text.
const FailureTimeLayout = "2006-01-02T15:04:05.000Z"

// FailureEntry is one line of the failure log: an instant and a cause.
type FailureEntry struct {
	At    time.Time
	Cause FailureCause
}

// NewFailureEntry builds an entry. A cause outside the closed set becomes
// CauseWrite, so a caller can never put free text into the log.
func NewFailureEntry(at time.Time, cause FailureCause) FailureEntry {
	if !cause.IsKnown() {
		cause = CauseWrite
	}
	return FailureEntry{At: at.UTC(), Cause: cause}
}

// Line is the text of the entry: the UTC time, one space, the cause and a
// newline. The hook appends it with one Write.
func (e FailureEntry) Line() string {
	cause := e.Cause
	if !cause.IsKnown() {
		cause = CauseWrite
	}
	return e.At.Format(FailureTimeLayout) + " " + string(cause) + "\n"
}

// ParseFailureLine reads one log line without its newline. It reports false
// for any text that Line could not have written, such as a byte of the old
// counter format, a torn line or a foreign line.
func ParseFailureLine(line string) (FailureEntry, bool) {
	stamp, cause, hasSeparator := strings.Cut(line, " ")
	at, err := time.Parse(FailureTimeLayout, stamp)
	entry := FailureEntry{At: at, Cause: FailureCause(cause)}
	return entry, hasSeparator && err == nil && entry.Cause.IsKnown()
}
