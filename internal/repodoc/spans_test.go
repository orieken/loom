package repodoc

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// recordingTracer is a test double for orchestrator.Tracer that remembers every
// span and how it ended.
type recordingTracer struct {
	mu    sync.Mutex
	spans []*recordedSpan
}

type recordedSpan struct {
	kind    string
	run     orchestrator.RunSpan
	stage   orchestrator.StageSpan
	invoked orchestrator.ProviderSpan
	outcome orchestrator.SpanOutcome
	ended   bool
}

func (s *recordedSpan) End(outcome orchestrator.SpanOutcome) { s.outcome, s.ended = outcome, true }

func (t *recordingTracer) add(span *recordedSpan) *recordedSpan {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = append(t.spans, span)
	return span
}

func (t *recordingTracer) StartRun(ctx context.Context, run orchestrator.RunSpan) (context.Context, orchestrator.Span) {
	return ctx, t.add(&recordedSpan{kind: "run", run: run})
}

func (t *recordingTracer) StartStage(ctx context.Context, stage orchestrator.StageSpan) (context.Context, orchestrator.Span) {
	return ctx, t.add(&recordedSpan{kind: "stage", stage: stage})
}

func (t *recordingTracer) StartProvider(ctx context.Context, call orchestrator.ProviderSpan) (context.Context, orchestrator.Span) {
	return ctx, t.add(&recordedSpan{kind: "provider", invoked: call})
}

func (t *recordingTracer) ofKind(kind string) []*recordedSpan {
	var out []*recordedSpan
	for _, span := range t.spans {
		if span.kind == kind {
			out = append(out, span)
		}
	}
	return out
}

func tracedRun(t *testing.T, provider orchestrator.Provider, sheets int) (*recordingTracer, RunRecord) {
	t.Helper()
	tracer := &recordingTracer{}
	runner := newRunner(provider, t.TempDir())
	runner.Tracer = tracer
	record, err := runner.Run(context.Background(), tasksFor(tree()[:sheets]))
	if err != nil {
		t.Fatal(err)
	}
	return tracer, record
}

func TestARunEmitsOneRunSpanNamedForThePlanWithItsCounts(t *testing.T) {
	tracer, record := tracedRun(t, scriptedProvider(t, tree()), 4)
	runs := tracer.ofKind("run")
	if len(runs) != 1 || runs[0].run.Plan != PlanName || !runs[0].ended {
		t.Fatalf("%+v", runs)
	}
	if runs[0].outcome.Status != orchestrator.StageStatusCompleted || runs[0].outcome.Reason != "4 verified, 0 unverified, 0 failed" || record.Failed() != 0 {
		t.Errorf("%+v", runs[0].outcome)
	}
}

func TestEveryDocumenterAndAuditorCallIsAStageSpanWithAProviderSpanInside(t *testing.T) {
	tracer, _ := tracedRun(t, scriptedProvider(t, tree()), 4)
	stages, calls := tracer.ofKind("stage"), tracer.ofKind("provider")
	if len(stages) != 8 || len(calls) != 8 {
		t.Fatalf("four directories, each a documenter and an auditor call: %d stages, %d provider spans", len(stages), len(calls))
	}
	for index, stage := range stages {
		if !stage.ended || calls[index].invoked.Stage != stage.stage.ID || calls[index].invoked.Operation != orchestrator.GenAIOperationName {
			t.Errorf("stage %+v and provider %+v must pair up", stage.stage, calls[index].invoked)
		}
	}
}

func TestStageSpansNameTheDocumenterAndTheAuditorSeparately(t *testing.T) {
	tracer, _ := tracedRun(t, scriptedProvider(t, tree()), 4)
	agents := map[string]int{}
	for _, stage := range tracer.ofKind("stage") {
		agents[stage.stage.Agent]++
	}
	if agents[AgentName] != 4 || agents[AuditorName] != 4 {
		t.Errorf("agents = %v", agents)
	}
}

func reportedUsageRun(t *testing.T) *recordingTracer {
	t.Helper()
	provider := scriptedProvider(t, tree())
	usage := &orchestrator.Usage{InputTokens: 120, OutputTokens: 40, CostUSD: 0.07}
	provider.SetScript(stageID("svc/api"), mock.Script{Payload: encode(t, validDoc("svc/api", "h-api")), Usage: usage})
	tracer, _ := tracedRun(t, provider, 4)
	return tracer
}

func TestOnlyTheCallThatReportedUsageCarriesIt(t *testing.T) {
	var carrying []*recordedSpan
	for _, span := range reportedUsageRun(t).ofKind("provider") {
		if span.outcome.Usage != nil {
			carrying = append(carrying, span)
		}
	}
	if len(carrying) != 1 || carrying[0].outcome.Usage.InputTokens != 120 || carrying[0].invoked.Stage != stageID("svc/api") {
		t.Fatalf("exactly the call that reported usage carries it: %+v", carrying)
	}
}

func TestTheRunSpanTotalsWhatWasReported(t *testing.T) {
	run := reportedUsageRun(t).ofKind("run")[0]
	if run.outcome.Usage == nil || run.outcome.Usage.CostUSD != 0.07 {
		t.Errorf("%+v", run.outcome.Usage)
	}
}

func TestSequenceNumbersTheDirectoryAndIterationTheRound(t *testing.T) {
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	provider := scriptedRounds(t, [][]byte{bad, apiDoc(nil)}, []state.RepoDocAuditState{approval("svc/api", "h-api")})
	tracer := &recordingTracer{}
	runner := newRunner(provider, t.TempDir())
	runner.Tracer = tracer
	if _, err := runner.Run(context.Background(), leafOnly()); err != nil {
		t.Fatal(err)
	}
	var seen []orchestrator.StageSpan
	for _, stage := range tracer.ofKind("stage") {
		seen = append(seen, stage.stage)
	}
	want := []orchestrator.StageSpan{
		{ID: stageID("svc/api"), Agent: AgentName, Sequence: 1, Iteration: 1},
		{ID: stageID("svc/api"), Agent: AgentName, Sequence: 1, Iteration: 2},
		{ID: auditStageID("svc/api"), Agent: AuditorName, Sequence: 1, Iteration: 2},
	}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("a revision is another span of the same stage, not a nested one:\n got %+v\nwant %+v", seen, want)
	}
}

func failedCallRun(t *testing.T) *recordingTracer {
	t.Helper()
	provider := scriptedProvider(t, tree())
	provider.SetScript(stageID("svc/api"), mock.Script{Err: errors.New("model unavailable"), Usage: &orchestrator.Usage{InputTokens: 5}})
	tracer, _ := tracedRun(t, provider, 4)
	return tracer
}

func TestAFailedCallEndsItsProviderSpanFailedWithTheErrorAndTheUsageItWasBilled(t *testing.T) {
	var failed []*recordedSpan
	for _, span := range failedCallRun(t).ofKind("provider") {
		if span.outcome.Status == orchestrator.StageStatusFailed {
			failed = append(failed, span)
		}
	}
	if len(failed) != 1 || failed[0].outcome.Err == nil || failed[0].outcome.Usage == nil || failed[0].outcome.Usage.InputTokens != 5 {
		t.Errorf("a failed call was still billed, and says why it failed: %+v", failed)
	}
}

func TestARunWithAFailureIsNotReportedCompleted(t *testing.T) {
	if run := failedCallRun(t).ofKind("run")[0]; run.outcome.Status != orchestrator.StageStatusFailed {
		t.Errorf("%+v", run.outcome)
	}
}

func TestACancelledRunEndsItsSpanInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	provider := scriptedProvider(t, tree())
	calls := 0
	provider.SetHook(func(string, int) *mock.Script {
		if calls++; calls == 1 {
			cancel()
		}
		return nil
	})
	tracer := &recordingTracer{}
	runner := newRunner(provider, t.TempDir())
	runner.Tracer = tracer
	if _, err := runner.Run(ctx, tasksFor(tree())); err != nil {
		t.Fatal(err)
	}
	if run := tracer.ofKind("run")[0]; run.outcome.Status != orchestrator.StageStatusInterrupted {
		t.Errorf("%+v", run.outcome)
	}
}

func TestARunWithNoTracerRecordsNothingAndStillWorks(t *testing.T) {
	record, err := newRunner(scriptedProvider(t, tree()), t.TempDir()).Run(context.Background(), tasksFor(tree()))
	if err != nil || record.Failed() != 0 {
		t.Errorf("%+v %v", record, err)
	}
}

func TestAnExplicitRunIDIsUsedAndOtherwiseItComesFromTheClock(t *testing.T) {
	runner := newRunner(scriptedProvider(t, tree()), t.TempDir())
	runner.RunID = "custom-id"
	if record, _ := runner.Run(context.Background(), tasksFor(tree()[2:3])); record.RunID != "custom-id" {
		t.Errorf("%q", record.RunID)
	}
	if got := RunIDAt(fixedClock()); got != "20261005T120000Z" {
		t.Errorf("%q", got)
	}
}
