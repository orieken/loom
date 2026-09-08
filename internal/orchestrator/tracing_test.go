package orchestrator_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// recordingTracer captures the span tree without any OpenTelemetry
// involvement, which is the point of the seam: the executor's tracing
// behaviour is testable without an SDK, an exporter, or a collector.
type recordingTracer struct {
	mutex     sync.Mutex
	runs      []orchestrator.RunSpan
	stages    []orchestrator.StageSpan
	providers []orchestrator.ProviderSpan
	ended     []orchestrator.SpanOutcome
	open      int
}

func (r *recordingTracer) StartRun(ctx context.Context, run orchestrator.RunSpan) (context.Context, orchestrator.Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.runs = append(r.runs, run)
	r.open++
	return ctx, &recordingSpan{tracer: r}
}

func (r *recordingTracer) StartStage(ctx context.Context, stage orchestrator.StageSpan) (context.Context, orchestrator.Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stages = append(r.stages, stage)
	r.open++
	return ctx, &recordingSpan{tracer: r}
}

func (r *recordingTracer) StartProvider(ctx context.Context, invocation orchestrator.ProviderSpan) (context.Context, orchestrator.Span) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.providers = append(r.providers, invocation)
	r.open++
	return ctx, &recordingSpan{tracer: r}
}

type recordingSpan struct {
	tracer *recordingTracer
}

func (s *recordingSpan) End(outcome orchestrator.SpanOutcome) {
	s.tracer.mutex.Lock()
	defer s.tracer.mutex.Unlock()
	s.tracer.ended = append(s.tracer.ended, outcome)
	s.tracer.open--
}

func (r *recordingTracer) stageIDs() []string {
	ids := make([]string, 0, len(r.stages))
	for _, stage := range r.stages {
		ids = append(ids, stage.ID)
	}
	return ids
}

func (r *recordingTracer) statuses() []orchestrator.StageStatus {
	statuses := make([]orchestrator.StageStatus, 0, len(r.ended))
	for _, outcome := range r.ended {
		statuses = append(statuses, outcome.Status)
	}
	return statuses
}

func assertStrings(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

func TestRunTracesOneSpanPerStageUnderOneRunSpan(t *testing.T) {
	scripts := map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	}
	executor, _, _, input := newHarness(t, scripts)
	tracer := &recordingTracer{}
	executor.WithTracer(tracer)

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(tracer.runs) != 1 {
		t.Fatalf("run spans = %d, want exactly 1", len(tracer.runs))
	}
	if tracer.runs[0].Plan != "test-plan" {
		t.Errorf("run span plan = %q, want %q", tracer.runs[0].Plan, "test-plan")
	}
	assertStrings(t, tracer.stageIDs(), []string{"analyst", "developer", "qa-engineer"}, "traced stages")
	if tracer.open != 0 {
		t.Errorf("%d spans left open — every span must be closed on every path", tracer.open)
	}
}

// A span must report the status run state records, not a status inferred
// from whether an error came back. The two describing the same stage
// differently is the defect the shared vocabulary exists to prevent.
func TestStageSpanReportsThePersistedStatus(t *testing.T) {
	boom := errors.New("agent exploded")
	scripts := map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {Err: boom},
	}
	executor, _, store, input := newHarness(t, scripts)
	tracer := &recordingTracer{}
	executor.WithTracer(tracer)

	if err := executor.Run(context.Background(), threeStagePlan(), input); !errors.Is(err, boom) {
		t.Fatalf("Run error = %v, want %v", err, boom)
	}

	state := mustLoad(t, store)
	// Spans close innermost-first, so each stage contributes its provider
	// span and then itself.
	want := []orchestrator.StageStatus{
		orchestrator.StageStatusCompleted, // analyst provider span
		orchestrator.StageStatusCompleted, // analyst stage span
		orchestrator.StageStatusFailed,    // developer provider span
		orchestrator.StageStatusFailed,    // developer stage span
		orchestrator.StageStatusFailed,    // run span
	}
	got := tracer.statuses()
	if len(got) != len(want) {
		t.Fatalf("span statuses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("span statuses = %v, want %v", got, want)
		}
	}
	if state.Stages["developer"].Status != orchestrator.StageStatusFailed {
		t.Error("precondition: run state should record developer as FAILED")
	}
}

// A run that halts at a gate is waiting on a human, not failing. It must
// still close its root span, or everything it recorded before the barrier
// is lost when the process exits.
func TestGateHaltClosesTheRunSpanAsWaiting(t *testing.T) {
	plan := orchestrator.Plan{
		Name: "gated-plan",
		Stages: []orchestrator.Stage{
			{ID: "analyst", Agent: "analyst", Timeout: 5 * time.Second},
			{ID: "developer", Agent: "developer", Gate: "confirm-design", Timeout: 5 * time.Second},
		},
	}
	executor, _, _, input := newHarness(t, map[string]mock.Script{"analyst": {ArtifactContent: "# analysis"}})
	tracer := &recordingTracer{}
	executor.WithTracer(tracer)

	err := executor.Run(context.Background(), plan, input)
	if !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want ErrWaitingApproval", err)
	}
	if tracer.open != 0 {
		t.Errorf("%d spans left open after a gate halt", tracer.open)
	}
	last := tracer.ended[len(tracer.ended)-1]
	if last.Status != orchestrator.StageStatusWaitingApproval {
		t.Errorf("run span status = %q, want %q", last.Status, orchestrator.StageStatusWaitingApproval)
	}
}

// With no tracer registered the run loop must behave identically, and cost
// nothing. Tracing is off by default and must never be load-bearing.
func TestRunWithoutATracerIsUnchanged(t *testing.T) {
	executor, provider, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertInvocations(t, provider, []string{"analyst", "developer", "qa-engineer"})
	assertStatus(t, mustLoad(t, store), "qa-engineer", orchestrator.StageStatusCompleted)
}

// WithTracer(nil) must restore the no-op rather than panic on the next span.
func TestWithNilTracerDisablesTracing(t *testing.T) {
	executor, _, _, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})
	executor.WithTracer(&recordingTracer{})
	executor.WithTracer(nil)

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// The model call gets its own span beneath the stage. A stage is not only
// its invocation — projection, validation and persistence happen around it
// — so attaching cost to the stage span would silently claim that time too.
func TestProviderInvocationGetsItsOwnSpanUnderTheStage(t *testing.T) {
	executor, _, _, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})
	tracer := &recordingTracer{}
	executor.WithTracer(tracer)

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(tracer.providers) != 3 {
		t.Fatalf("provider spans = %d, want one per invoked stage (3)", len(tracer.providers))
	}
	for _, invocation := range tracer.providers {
		if invocation.Operation != orchestrator.GenAIOperationName {
			t.Errorf("operation = %q, want the GenAI semconv name %q", invocation.Operation, orchestrator.GenAIOperationName)
		}
	}
	if tracer.open != 0 {
		t.Errorf("%d spans left open", tracer.open)
	}
}

// Usage the provider reported must reach run state, so the cost of a run is
// answerable without a collector configured.
func TestReportedUsageIsRecordedInRunState(t *testing.T) {
	usage := &orchestrator.Usage{Model: "claude-opus-5", InputTokens: 100, OutputTokens: 20, CostUSD: 0.5}
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis", Usage: usage},
		"developer":   {ArtifactContent: "# implementation", Usage: usage},
		"qa-engineer": {ArtifactContent: "# qa report", Usage: usage},
	})

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	state := mustLoad(t, store)
	if got := state.Stages["developer"].Usage; got == nil || *got != *usage {
		t.Errorf("developer usage = %+v, want %+v", got, usage)
	}
	total := state.TotalUsage()
	if total.InputTokens != 300 || total.CostUSD != 1.5 {
		t.Errorf("total usage = %+v, want 300 input tokens and 1.5 USD", total)
	}
}

// A provider that reports nothing must total to nothing, not to zeros
// presented as measurements. Absence and zero are different facts.
func TestUsageIsAbsentNotZeroWhenNothingIsReported(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := mustLoad(t, store).Stages["analyst"].Usage; got != nil {
		t.Errorf("usage = %+v, want nil when the provider reported nothing", got)
	}
}

// A call that produced tokens and then failed still cost money. Dropping
// its usage would understate the run, and failures are often the expensive
// part.
func TestFailedStageKeepsTheUsageItAlreadyConsumed(t *testing.T) {
	usage := &orchestrator.Usage{InputTokens: 900, OutputTokens: 10, CostUSD: 0.25}
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {Err: errors.New("agent exploded"), Usage: usage},
	})

	if err := executor.Run(context.Background(), threeStagePlan(), input); err == nil {
		t.Fatal("Run succeeded; the developer stage was scripted to fail")
	}

	record := mustLoad(t, store).Stages["developer"]
	if record.Status != orchestrator.StageStatusFailed {
		t.Fatalf("developer status = %q, want FAILED", record.Status)
	}
	if record.Usage == nil || record.Usage.CostUSD != 0.25 {
		t.Errorf("failed stage usage = %+v, want the 0.25 USD it consumed before failing", record.Usage)
	}
}

// A stage that failed, cost money, and then succeeded on a retry must total
// BOTH attempts (roadmap L3.22).
//
// The third real end-to-end run reported $8.7978 against a true $9.4923.
// The difference was exactly the qa-engineer attempt that failed to parse:
// the record's usage was assigned rather than accumulated, so the retry
// overwrote the failed attempt and the run's own summary lost it. The error
// was always in the same direction and grew with how badly a run went.
func TestRetriedStageTotalsEveryAttemptNotJustTheLast(t *testing.T) {
	failed := &orchestrator.Usage{OutputTokens: 4159, CacheReadTokens: 517665, CostUSD: 0.69}
	succeeded := &orchestrator.Usage{OutputTokens: 4185, CacheReadTokens: 621621, CostUSD: 0.74}
	executor, provider, store, input := newHarness(t, map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {ArtifactContent: "# implementation"},
	})
	provider.SetHook(failThenSucceed("qa-engineer", failed, succeeded))

	if err := executor.Run(context.Background(), threeStagePlan(), input); err == nil {
		t.Fatal("Run succeeded; qa-engineer was scripted to fail its first attempt")
	}
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume run: %v", err)
	}

	record := mustLoad(t, store).Stages["qa-engineer"]
	if record.Usage == nil {
		t.Fatal("qa-engineer reported no usage at all")
	}
	assertBothAttempts(t, *record.Usage, *failed, *succeeded)
}

// failThenSucceed scripts one stage to fail its first attempt and succeed on
// the retry, which is the shape a parse failure plus --resume produces.
func failThenSucceed(stageID string, failed, succeeded *orchestrator.Usage) func(string, int) *mock.Script {
	return func(invoked string, invocation int) *mock.Script {
		if invoked != stageID {
			return nil
		}
		if invocation == 1 {
			return &mock.Script{Err: errors.New("agent did not return a JSON state document"), Usage: failed}
		}
		return &mock.Script{ArtifactContent: "# qa report", Usage: succeeded}
	}
}

func assertBothAttempts(t *testing.T, got, failed, succeeded orchestrator.Usage) {
	t.Helper()
	if want := failed.CostUSD + succeeded.CostUSD; !isNear(got.CostUSD, want) {
		t.Errorf("cost = %v, want both attempts (%v)", got.CostUSD, want)
	}
	if want := failed.CacheReadTokens + succeeded.CacheReadTokens; got.CacheReadTokens != want {
		t.Errorf("cache reads = %d, want both attempts (%d)", got.CacheReadTokens, want)
	}
}

// The run total is what an auditor compares against the trace spans, so it
// must agree with the sum of every provider call the run actually made.
func TestRunTotalAgreesWithEveryProviderCall(t *testing.T) {
	perCall := &orchestrator.Usage{OutputTokens: 100, CostUSD: 0.25}
	executor, provider, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis", Usage: perCall},
		"developer":   {ArtifactContent: "# implementation", Usage: perCall},
		"qa-engineer": {ArtifactContent: "# qa report", Usage: perCall},
	})
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		if stageID == "developer" && invocation == 1 {
			return &mock.Script{Err: errors.New("agent exploded"), Usage: perCall}
		}
		return nil
	})

	_ = executor.Run(context.Background(), threeStagePlan(), input)
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume run: %v", err)
	}

	calls := len(provider.Invocations())
	total := mustLoad(t, store).TotalUsage()
	if want := float64(calls) * perCall.CostUSD; !isNear(total.CostUSD, want) {
		t.Errorf("run total = %v across %d provider calls, want %v", total.CostUSD, calls, want)
	}
}

// Float sums are compared with a tolerance because currency in float64 does
// not associate; the assertion is about the missing line item, not the ulp.
func isNear(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// Deleting a stage record must not delete what it cost (roadmap L3.22,
// second occurrence).
//
// Re-running one stage requires deleting its record by hand, because loom
// has no rollback command (run 4 §9.1). Run 4 did exactly that and the
// executor then reported $20.2718 for a run whose spans total $21.5106 —
// short by the $1.2389 of the discarded attempt. Money spent is a fact
// about the run, not a property of a record someone may remove.
func TestDeletingAStageRecordDoesNotDeleteWhatItCost(t *testing.T) {
	perCall := &orchestrator.Usage{OutputTokens: 100, CostUSD: 1.2389}
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis", Usage: perCall},
		"developer":   {ArtifactContent: "# implementation", Usage: perCall},
		"qa-engineer": {ArtifactContent: "# qa report", Usage: perCall},
	})
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	state := mustLoad(t, store)
	before := state.TotalUsage().CostUSD
	// The hand-rollback run 4 performed, reproduced.
	delete(state.Stages, "developer")
	if err := store.Save(state); err != nil {
		t.Fatalf("save rolled-back state: %v", err)
	}

	after := mustLoad(t, store).TotalUsage().CostUSD
	if !isNear(after, before) {
		t.Errorf("total after deleting a record = %v, want the unchanged %v — "+
			"the deleted attempt was still billed", after, before)
	}
}

// A state written before the accumulator existed still totals correctly
// from its records, so the fix does not silently zero old runs.
func TestAStateWithNoAccumulatorStillTotalsItsRecords(t *testing.T) {
	state := &orchestrator.RunState{Stages: map[string]orchestrator.StageRecord{
		"analyst":   {Usage: &orchestrator.Usage{CostUSD: 0.25}},
		"developer": {Usage: &orchestrator.Usage{CostUSD: 0.75}},
	}}

	if got := state.TotalUsage().CostUSD; !isNear(got, 1.0) {
		t.Errorf("total = %v, want 1.0 derived from the records", got)
	}
}
