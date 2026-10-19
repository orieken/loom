package capture

import (
	"slices"
	"time"
)

// EventVersion is the day-file format version of this package. A change to
// the field list of SessionEvent needs a new version and a reader for both
// (architecture-notes.md decision 13; RFC, status Accepted).
const EventVersion = 1

// EventKind says what a Session Event records.
type EventKind string

// The event kinds. The first four map to the hook events of the tool; the
// last one is written by the hook when a hunk hash changed.
const (
	EventToolUse      EventKind = "tool-use"
	EventToolResult   EventKind = "tool-result"
	EventToolDenied   EventKind = "tool-denied"
	EventPrompt       EventKind = "prompt"
	EventStop         EventKind = "stop"
	EventHunkObserved EventKind = "hunk-observed"
)

// PromptClass is the only thing kept of a prompt.
type PromptClass string

// The prompt classes.
const (
	PromptCorrective PromptClass = "corrective"
	PromptOther      PromptClass = "other"
)

// SessionEvent is one day-file line (format v:1). It holds properties of an
// event and never a payload: no prompt text, no argument value, no code
// (architecture-guardrails.md #9). The field list is frozen; a test fails on
// any change.
//
// The fields are exported only so encoding/json can read them. Treat a
// SessionEvent as a value: build it with NewSessionEvent and the With
// methods, which return a changed copy and never mutate the receiver.
type SessionEvent struct {
	V             int         `json:"v"`
	ID            string      `json:"id"`
	At            time.Time   `json:"at"`
	Kind          EventKind   `json:"kind"`
	Tool          string      `json:"tool,omitempty"`
	SessionRef    string      `json:"sessionRef,omitempty"`
	ArgKeys       []string    `json:"argKeys,omitempty"`
	ArgTypes      []string    `json:"argTypes,omitempty"`
	PathRef       string      `json:"pathRef,omitempty"`
	Path          *string     `json:"path,omitempty"`
	RangeStart    int         `json:"rangeStart,omitempty"`
	RangeEnd      int         `json:"rangeEnd,omitempty"`
	PreHash       string      `json:"preHash,omitempty"`
	PostHash      string      `json:"postHash,omitempty"`
	PromptClass   PromptClass `json:"promptClass,omitempty"`
	ArgsTruncated bool        `json:"argsTruncated,omitempty"`
}

// NewSessionEvent builds a minimal event. The time is stored in UTC.
func NewSessionEvent(id string, at time.Time, kind EventKind) SessionEvent {
	return SessionEvent{V: EventVersion, ID: id, At: at.UTC(), Kind: kind}
}

// WithTool returns a copy that names the tool.
func (e SessionEvent) WithTool(tool string) SessionEvent {
	e.Tool = tool
	return e
}

// WithSessionRef returns a copy that holds the salted hash of the session id.
func (e SessionEvent) WithSessionRef(ref string) SessionEvent {
	e.SessionRef = ref
	return e
}

// WithArgShape returns a copy that holds argument keys and types. The slices
// are copied so the caller cannot change the event afterwards.
func (e SessionEvent) WithArgShape(keys, types []string) SessionEvent {
	e.ArgKeys = slices.Clone(keys)
	e.ArgTypes = slices.Clone(types)
	return e
}

// WithPath returns a copy that holds the salted path hash. The display path
// is nil for a redacted path. The pointer target is copied.
func (e SessionEvent) WithPath(ref string, display *string) SessionEvent {
	e.PathRef = ref
	e.Path = nil
	if display != nil {
		copied := *display
		e.Path = &copied
	}
	return e
}

// WithHunk returns a copy that holds a line range and its hashes.
func (e SessionEvent) WithHunk(facts HunkFacts) SessionEvent {
	e.RangeStart = facts.RangeStart
	e.RangeEnd = facts.RangeEnd
	e.PreHash = facts.PreHash
	e.PostHash = facts.PostHash
	return e
}

// WithPromptClass returns a copy that holds the class label of a prompt.
func (e SessionEvent) WithPromptClass(class PromptClass) SessionEvent {
	e.PromptClass = class
	return e
}

// HasHunk reports whether the event holds a usable line range.
func (e SessionEvent) HasHunk() bool {
	return e.RangeStart >= 1 && e.RangeEnd >= e.RangeStart
}
