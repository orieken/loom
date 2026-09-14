package orchestrator

// Trace-shape recording (roadmap L3.44).
//
// Nothing here asserts that a known input produces a known trajectory. A
// router change that adds two stages, a loop that terminates by its bound
// instead of converging, a stage that stops being invoked at all — each
// passes every other test in this repository as long as the artifacts
// validate. `agent-eval` grades one agent's output and
// `pipeline-retrospective` trends timings across deliveries; neither of them
// is looking at the run as a whole.
//
// A shape is an EXTRACTION, not a recording. Durations, trace IDs, span IDs
// and timestamps are excluded by construction: they change on every run and
// are not the thing being protected. What survives is what the executor
// decided.
//
// One honest limit: a shape records what RAN. A stage the router skips is
// settled before its span opens, so it appears nowhere. That still catches a
// routing change — the executed set changes, and the sequence with it — but
// a stage skipped for a *different reason* with the set otherwise identical
// is invisible here. The route is recorded in run state, which is where that
// question belongs.

import (
	"context"
	"sort"
	"sync"
)

// Shape is the structural record of one run.
type Shape struct {
	Plan string `json:"plan"`
	// Stages is every stage span in start order, repeats included — a loop
	// that ran three rounds appears three times, which is the trajectory.
	Stages []StageStep `json:"stages"`
	// Loops is how each loop settled, keyed by loop ID.
	Loops map[string]string `json:"loops,omitempty"`
	// ProviderCalls counts model invocations. Skipped and internal stages
	// make this differ from the stage count, and that difference is the
	// number a routing regression moves.
	ProviderCalls int `json:"providerCalls"`
}

// StageStep is one stage span: which stage, which round, how it ended.
type StageStep struct {
	ID        string `json:"id"`
	Iteration int    `json:"iteration,omitempty"`
	Status    string `json:"status"`
}

// ShapeRecorder is a Tracer that builds a Shape instead of exporting spans.
// It is exported so anything that can drive a run can capture one — a test
// today, a `loom` command later — without this logic moving.
type ShapeRecorder struct {
	mutex         sync.Mutex
	plan          string
	steps         []StageStep
	loops         map[string]string
	providerCalls int
}

// StartRun records the plan under test.
func (r *ShapeRecorder) StartRun(ctx context.Context, run RunSpan) (context.Context, Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.plan = run.Plan
	return ctx, &shapeSpan{recorder: r, index: -1}
}

// StartStage appends a step, whose status is filled in when the span ends.
func (r *ShapeRecorder) StartStage(ctx context.Context, stage StageSpan) (context.Context, Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.steps = append(r.steps, StageStep{ID: stage.ID, Iteration: stage.Iteration})
	return ctx, &shapeSpan{recorder: r, index: len(r.steps) - 1}
}

// StartProvider counts one model invocation.
func (r *ShapeRecorder) StartProvider(ctx context.Context, _ ProviderSpan) (context.Context, Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.providerCalls++
	return ctx, &shapeSpan{recorder: r, index: -1}
}

// Shape returns what was recorded. Safe to call once the run has ended.
func (r *ShapeRecorder) Shape() Shape {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	shape := Shape{Plan: r.plan, ProviderCalls: r.providerCalls,
		Stages: append([]StageStep(nil), r.steps...)}
	if len(r.loops) > 0 {
		shape.Loops = map[string]string{}
		for _, id := range sortedLoopIDs(r.loops) {
			shape.Loops[id] = r.loops[id]
		}
	}
	return shape
}

func sortedLoopIDs(loops map[string]string) []string {
	ids := make([]string, 0, len(loops))
	for id := range loops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type shapeSpan struct {
	recorder *ShapeRecorder
	// index is the step this span fills in, or -1 for a run or provider span
	// that contributes no step of its own.
	index int
}

func (s *shapeSpan) End(outcome SpanOutcome) {
	s.recorder.mutex.Lock()
	defer s.recorder.mutex.Unlock()
	if len(outcome.LoopOutcomes) > 0 {
		s.recorder.loops = outcome.LoopOutcomes
	}
	if s.index < 0 {
		return
	}
	s.recorder.steps[s.index].Status = string(outcome.Status)
}
