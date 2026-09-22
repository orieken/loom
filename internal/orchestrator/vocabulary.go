package orchestrator

// The event vocabulary (roadmap L3.9).
//
// One Go enum is the source of truth. The JSON Schema and the documentation
// table under shared/schemas/telemetry/ are GENERATED from what is here, by
// `go run ./cmd/gen-schemas`, and a test fails when the committed copies
// drift — the same arrangement typed state uses, for the same reason.
//
// This exists because the framework previously kept two hand-maintained
// lists: six event types documented in a schema, nine more specified across
// other spec files, and a recorder instructed to refuse any type not
// documented. Nothing checked the prose against anything, so 60% of the
// specified surface sat outside the list the recorder was told to enforce.
// A generated table cannot drift from the constants it is generated from.
//
// The vocabulary holds ONLY what the executor actually emits. Types that
// are specified but have no emitter live in the roadmap, not here: an enum
// where entries fire from nowhere is the same trap in a new location, and a
// consumer would have to learn which members are real.

import "sort"

// EventDoc describes one event kind for the generated schema and table.
type EventDoc struct {
	Kind EventKind
	// Summary says what happened, in one line.
	Summary string
	// Fields names the Event fields this kind populates beyond At and Kind,
	// so a consumer knows what to expect rather than probing for it.
	Fields []string
	// Roadmap names the item that introduced the kind, so a reader can find
	// out why it exists.
	Roadmap string
}

// EventVocabulary returns every emitted event kind, sorted by kind so the
// generated output is deterministic.
func EventVocabulary() []EventDoc {
	docs := eventDocs()
	sort.Slice(docs, func(i, j int) bool { return docs[i].Kind < docs[j].Kind })
	return docs
}

func eventDocs() []EventDoc {
	return append(runAndStageDocs(), loopAndGateDocs()...)
}

func runAndStageDocs() []EventDoc {
	return []EventDoc{
		{EventRunStarted, "A run began. Emitted once per run, not once per invocation.", nil, "M0.4"},
		{EventRunResumed, "An invocation continued a run already on disk — after a gate, an interrupt, or a failure.", nil, "L3.16"},
		{EventRunCompleted, "Every stage of the plan settled.", nil, "M0.4"},
		{EventStageStarted, "A stage began executing.", []string{"stage", "sequence"}, "M0.4"},
		{EventStageCompleted, "A stage finished and its artifact was digested.", []string{"stage", "sequence"}, "M0.4"},
		{EventStageFailed, "A stage returned an error.", []string{"stage", "sequence", "error"}, "M0.4"},
		{EventStageInterrupted, "A stage was cancelled, leaving a resumable checkpoint.", []string{"stage", "sequence", "error"}, "M0.4"},
		{EventStageStale, "A completed stage was demoted because its artifact or an input changed.", []string{"stage", "sequence", "staleReason"}, "L2.12"},
		{EventStageSkipped, "The router routed around a stage the run does not need.", []string{"stage", "reason"}, "L3.0"},
		{EventPostureViolation, "A stage changed the working tree without declaring a tool that writes files.", []string{"stage", "reason"}, "L3.30"},
	}
}

func loopAndGateDocs() []EventDoc {
	return []EventDoc{
		{EventLoopIterated, "A bounded loop sent its span round again.", []string{"loop", "stage", "iteration"}, "L2.17"},
		{EventLoopExhausted, "A bounded loop reached its iteration limit.", []string{"loop", "stage", "iteration"}, "L2.17"},
		{EventGateWaiting, "The run halted at a gate and is waiting on a human.", []string{"stage", "gate"}, "L2.13"},
		{EventGateApproved, "A human approved a gate through a named channel.", []string{"gate", "approvalMethod"}, "L2.13"},
		{EventGateInvalidated, "An approval was reset because a bound artifact changed.", []string{"gate", "stage"}, "L2.14"},
		{EventArtifactCorrected, "A human edited a stage's output at a gate.", []string{"stage", "agent", "gate", "correction", "diffPath"}, "L4.5"},
		{EventPolicyEvaluated, "The policies watching a gate were evaluated; the run halts regardless.", []string{"gate", "reason", "correction"}, "L2.16"},
	}
}

// EventKindStrings returns the vocabulary's kinds as plain strings, for the
// generated schema's enum constraint.
func EventKindStrings() []string {
	docs := EventVocabulary()
	kinds := make([]string, 0, len(docs))
	for _, doc := range docs {
		kinds = append(kinds, string(doc.Kind))
	}
	return kinds
}
