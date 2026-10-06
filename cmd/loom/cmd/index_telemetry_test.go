package cmd

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
)

type spanLog struct {
	mu    sync.Mutex
	runs  []orchestrator.RunSpan
	stage []orchestrator.StageSpan
	ends  []orchestrator.SpanOutcome
}

type logSpan struct{ log *spanLog }

func (s logSpan) End(outcome orchestrator.SpanOutcome) {
	s.log.mu.Lock()
	defer s.log.mu.Unlock()
	s.log.ends = append(s.log.ends, outcome)
}

func (l *spanLog) StartRun(ctx context.Context, run orchestrator.RunSpan) (context.Context, orchestrator.Span) {
	l.runs = append(l.runs, run)
	return ctx, logSpan{l}
}

func (l *spanLog) StartStage(ctx context.Context, stage orchestrator.StageSpan) (context.Context, orchestrator.Span) {
	l.stage = append(l.stage, stage)
	return ctx, logSpan{l}
}

func (l *spanLog) StartProvider(ctx context.Context, _ orchestrator.ProviderSpan) (context.Context, orchestrator.Span) {
	return ctx, logSpan{l}
}

func tracedGoRepoIndex(t *testing.T) *spanLog {
	t.Helper()
	log := &spanLog{}
	if _, err := tracedIndex(context.Background(), log, committedGoRepo(t)); err != nil {
		t.Fatal(err)
	}
	return log
}

func TestIndexingEmitsARunSpanAndAnInternalStage(t *testing.T) {
	log := tracedGoRepoIndex(t)
	if len(log.runs) != 1 || log.runs[0].Plan != "repo-context-index" {
		t.Errorf("%+v", log.runs)
	}
	if len(log.stage) != 1 || log.stage[0].ID != indexStageID || !log.stage[0].Internal {
		t.Errorf("%+v", log.stage)
	}
}

func TestIndexingSpansCarryWhatItFoundAndNoTokens(t *testing.T) {
	log := tracedGoRepoIndex(t)
	if len(log.ends) != 2 || log.ends[0].Status != orchestrator.StageStatusCompleted || log.ends[0].Usage != nil {
		t.Fatalf("an indexing run has no tokens: %+v", log.ends)
	}
	if !strings.Contains(log.ends[0].Reason, "2 nodes, 1 edges, 3 fact sheets, 3 index entries") {
		t.Errorf("%q", log.ends[0].Reason)
	}
}

func TestAFailedIndexingRunEndsItsSpansFailedWithTheError(t *testing.T) {
	log := &spanLog{}
	if _, err := tracedIndex(context.Background(), log, t.TempDir()); err == nil {
		t.Fatal("not a git repository")
	}
	if len(log.ends) != 2 || log.ends[0].Status != orchestrator.StageStatusFailed || log.ends[0].Err == nil {
		t.Errorf("%+v", log.ends)
	}
}

func TestIndexingWithNoTracerStillWorks(t *testing.T) {
	if _, err := tracedIndex(context.Background(), nil, committedGoRepo(t)); err != nil {
		t.Fatal(err)
	}
}
