package capture

import (
	"fmt"
	"io"
)

// EditText holds the old and new text of the edits in one Edit or MultiEdit
// call. The hook needs it to find the edited lines in the file after the
// write. It exists to be used once and to be forgotten.
//
// PRECONDITION: no code path stores, logs, serializes or prints the text.
// ENFORCED-BY: TestCaptureHookCanaryTextReachesNeitherDiskNorOutput (the day
// file, the failure file and the hook output), TestCaptureCanaryReachesNeitherDatabaseNorReport
// (the database files and the report output), and
// TestRecorderCanaryTextNeverReachesAStoredLine (in-process). The type has no
// exported field and no accessor, and every print verb gives "[redacted]".
type EditText struct {
	edits []editPair
}

type editPair struct {
	oldText string
	newText string
}

// With returns a copy that also holds one edit.
func (t EditText) With(oldText, newText string) EditText {
	edits := append(append([]editPair(nil), t.edits...), editPair{oldText, newText})
	return EditText{edits: edits}
}

// Len counts the edits.
func (t EditText) Len() int { return len(t.edits) }

// String prints a fixed marker, never the text.
func (EditText) String() string { return redactedText }

// GoString prints a fixed marker for %#v.
func (EditText) GoString() string { return redactedText }

// Format prints a fixed marker for every verb.
func (EditText) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, redactedText)
}

// MarshalText keeps the text out of JSON and other text encoders.
func (EditText) MarshalText() ([]byte, error) { return []byte(redactedText), nil }
