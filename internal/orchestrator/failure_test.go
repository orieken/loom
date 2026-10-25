package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// Roadmap L3.68 classifies a failed run from its recorded state, so every
// failure says whether retrying it could plausibly help.

func TestAProviderErrorIsTransient(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"developer": {Err: errors.New("provider unavailable")},
	})
	_ = executor.Run(context.Background(), threeStagePlan(), input)

	if got := mustLoad(t, store).Stages["developer"].FailureClass; got != orchestrator.FailureTransient {
		t.Errorf("failure class = %q, want transient", got)
	}
}

func TestATimeoutIsTransient(t *testing.T) {
	plan := threeStagePlan()
	plan.Stages[0].Timeout = 10 * time.Millisecond
	executor, _, store, input := newHarness(t, map[string]mock.Script{"analyst": {Hang: true}})
	_ = executor.Run(context.Background(), plan, input)

	record := mustLoad(t, store).Stages["analyst"]
	if record.Status != orchestrator.StageStatusFailed || record.FailureClass != orchestrator.FailureTransient {
		t.Errorf("record = %s/%q, want FAILED/transient", record.Status, record.FailureClass)
	}
}

func TestAnInterruptionIsTransient(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{"analyst": {Hang: true}})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_ = executor.Run(ctx, threeStagePlan(), input)

	record := mustLoad(t, store).Stages["analyst"]
	if record.Status != orchestrator.StageStatusInterrupted || record.FailureClass != orchestrator.FailureTransient {
		t.Errorf("record = %s/%q, want INTERRUPTED/transient", record.Status, record.FailureClass)
	}
}

// A model that broke its schema usually does not break it the same way
// twice, so a supervisor may retry it (bounded by the order's attempts).
// This was terminal until the second factory dogfood run parked an order
// over one invented field.
func TestAProviderStagesInvalidArtifactIsTransient(t *testing.T) {
	plan := threeStagePlan()
	plan.Stages[0].StateKind = "analysis"
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst": {Payload: []byte(`{"not":"an analysis"}`)},
	})
	err := executor.Run(context.Background(), plan, input)

	record := mustLoad(t, store).Stages["analyst"]
	if record.Status != orchestrator.StageStatusFailed || record.FailureClass != orchestrator.FailureTransient {
		t.Errorf("record = %s/%q, want FAILED/transient", record.Status, record.FailureClass)
	}
	if !errors.Is(err, orchestrator.ErrInvalidState) || !strings.Contains(record.Error, "returned invalid state") {
		t.Errorf("err = %v, record error = %q; want an invalid-state failure", err, record.Error)
	}
}

func TestARerunClearsThePreviousFailureClass(t *testing.T) {
	executor, provider, store, input := newHarness(t, map[string]mock.Script{})
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		if stageID == "developer" && invocation == 1 {
			return &mock.Script{Err: errors.New("provider unavailable")}
		}
		return nil
	})
	_ = executor.Run(context.Background(), threeStagePlan(), input)
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := mustLoad(t, store).Stages["developer"].FailureClass; got != "" {
		t.Errorf("a completed stage still carries failure class %q", got)
	}
}
