package orchestrator

// The timeline is the audit half of L2.12's premise: state says where a run
// is, and this says what happened and when. Every timestamp here is taken
// from the clock by the process doing the work, so durations are measured
// by subtraction rather than recalled by a model. It is deliberately NOT
// OpenTelemetry (roadmap L3.8) and not a replacement for the model-written
// pipeline-trace.json — it is a local, append-only audit log.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunEventsFileName is the append-only timeline beside run-state.json.
const RunEventsFileName = "run-events.jsonl"

// EventKind is what happened. The set is deliberately small: every kind
// here is a transition some human or tool asks about after the fact.
type EventKind string

// The recorded transitions.
const (
	EventRunStarted       EventKind = "run.started"
	EventRunCompleted     EventKind = "run.completed"
	EventStageStarted     EventKind = "stage.started"
	EventStageCompleted   EventKind = "stage.completed"
	EventStageFailed      EventKind = "stage.failed"
	EventStageInterrupted EventKind = "stage.interrupted"
	EventStageStale       EventKind = "stage.stale"
	// EventStageSkipped records a stage the router routed around, with the
	// reason (roadmap L3.0).
	EventStageSkipped EventKind = "stage.skipped"
	// EventPostureViolation records a stage that changed the working tree
	// without declaring a tool that writes files (roadmap L3.30).
	EventPostureViolation EventKind = "posture.violation"
	// EventLoopIterated records the review loop sending the developer back
	// for another round; EventLoopExhausted records it hitting its bound.
	EventLoopIterated  EventKind = "loop.iterated"
	EventLoopExhausted EventKind = "loop.exhausted"
	EventGateWaiting   EventKind = "gate.waiting"
	EventGateApproved  EventKind = "gate.approved"
	// EventGateInvalidated records an approval reset by an edit to a bound
	// artifact (roadmap L2.14). Stage names the artifact that changed.
	EventGateInvalidated EventKind = "gate.invalidated"
	// EventArtifactCorrected records a human editing a stage's output at a
	// gate (roadmap L4.5) — the corrective signal the telemetry schema has
	// always specified and nothing has ever emitted. Attributed to the
	// producing stage and agent rather than to the gate, because whose
	// output needed fixing is the part worth learning from.
	EventArtifactCorrected EventKind = "artifact.corrected"
	// EventPolicyEvaluated records what the policies watching a gate
	// decided (roadmap L2.16). approval-gates.md asserted this event for
	// every decision since v3.3 and nothing emitted it; something does now,
	// which is what earns it a place in the vocabulary.
	EventPolicyEvaluated EventKind = "policy.evaluated"
)

// Event is one line of the timeline.
type Event struct {
	At             time.Time      `json:"at"`
	Kind           EventKind      `json:"kind"`
	Stage          string         `json:"stage,omitempty"`
	Sequence       int            `json:"sequence,omitempty"`
	Gate           string         `json:"gate,omitempty"`
	StaleReason    StaleReason    `json:"staleReason,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Loop           string         `json:"loop,omitempty"`
	Iteration      int            `json:"iteration,omitempty"`
	ApprovalMethod ApprovalMethod `json:"approvalMethod,omitempty"`
	// Agent and Correction describe a human correction (roadmap L4.5):
	// which agent produced what was corrected, and a +added/-removed
	// summary. The diff itself is on disk, named by Reason, so one event
	// stays one greppable line.
	Agent      string `json:"agent,omitempty"`
	Correction string `json:"correction,omitempty"`
	// DiffPath names the retained diff on disk. It is a separate field from
	// Reason because Reason means "why a stage was skipped" — overloading it
	// would make the timeline's JSON ambiguous to anything reading it.
	DiffPath string `json:"diffPath,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Timeline appends events to one run's log.
type Timeline struct {
	path string
}

// NewTimeline returns the timeline stored beside the given state file.
func NewTimeline(statePath string) *Timeline {
	return &Timeline{path: filepath.Join(filepath.Dir(statePath), RunEventsFileName)}
}

// Path returns the file the timeline appends to.
func (t *Timeline) Path() string { return t.path }

// Append writes one event as a single line. Append-only, one write per
// line, is the whole atomicity model: the file is never read back, rewritten,
// or truncated, so a concurrent writer or a crash can at worst leave one
// torn trailing line, which readers skip.
func (t *Timeline) Append(event Event) error {
	event.At = time.Now().UTC()
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode run event: %w", err)
	}
	return t.appendLine(append(line, '\n'))
}

func (t *Timeline) appendLine(line []byte) error {
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return fmt.Errorf("create timeline directory: %w", err)
	}
	file, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open run events: %w", err)
	}
	if _, err := file.Write(line); err != nil {
		_ = file.Close()
		return fmt.Errorf("append run event: %w", err)
	}
	return file.Close()
}

// Read returns the recorded events in order. A line that does not parse is
// skipped rather than failing the read: a torn final line from a killed
// process must not hide the history before it.
func (t *Timeline) Read() ([]Event, error) {
	file, err := os.Open(t.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open run events: %w", err)
	}
	defer func() { _ = file.Close() }()
	return decodeEvents(file), nil
}

func decodeEvents(file *os.File) []Event {
	events := make([]Event, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		events = append(events, event)
	}
	return events
}
