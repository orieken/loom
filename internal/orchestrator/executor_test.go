package orchestrator_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

func threeStagePlan() orchestrator.Plan {
	return orchestrator.Plan{
		Name: "test-plan",
		Stages: []orchestrator.Stage{
			{ID: "analyst", Agent: "analyst", Timeout: 5 * time.Second},
			{ID: "developer", Agent: "developer", Timeout: 5 * time.Second},
			{ID: "qa-engineer", Agent: "qa-engineer", Timeout: 5 * time.Second},
		},
	}
}

func newHarness(t *testing.T, scripts map[string]mock.Script) (*orchestrator.Executor, *mock.Provider, *orchestrator.StateStore, orchestrator.StageInput) {
	t.Helper()
	dir := t.TempDir()
	provider := mock.New(scripts)
	store := orchestrator.NewStateStore(filepath.Join(dir, orchestrator.RunStateFileName))
	input := orchestrator.StageInput{SpecPath: filepath.Join(dir, "spec.md"), WorkspaceDir: dir}
	return orchestrator.NewExecutor(provider, store), provider, store, input
}

func mustLoad(t *testing.T, store *orchestrator.StateStore) *orchestrator.RunState {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state == nil {
		t.Fatal("no state file was persisted")
	}
	return state
}

func assertStatus(t *testing.T, state *orchestrator.RunState, stageID string, want orchestrator.StageStatus) {
	t.Helper()
	got := state.Stages[stageID].Status
	if got != want {
		t.Errorf("stage %q status = %q, want %q", stageID, got, want)
	}
}

func TestRunHappyPathOverThreeStages(t *testing.T) {
	scripts := map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	}
	executor, provider, store, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	state := mustLoad(t, store)
	for stageID, script := range scripts {
		assertStatus(t, state, stageID, orchestrator.StageStatusCompleted)
		wantSum := sha256.Sum256([]byte(script.ArtifactContent))
		if got := state.Stages[stageID].ArtifactSHA256; got != hex.EncodeToString(wantSum[:]) {
			t.Errorf("stage %q artifact SHA-256 = %q, want hash of scripted content", stageID, got)
		}
	}
	wantOrder := []string{"analyst", "developer", "qa-engineer"}
	assertInvocations(t, provider, wantOrder)
}

func assertInvocations(t *testing.T, provider *mock.Provider, want []string) {
	t.Helper()
	got := provider.Invocations()
	if len(got) != len(want) {
		t.Fatalf("invocations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("invocations = %v, want %v", got, want)
		}
	}
}

func TestRunStopsOnFirstFailureAndPersistsFailed(t *testing.T) {
	boom := errors.New("agent exploded")
	executor, provider, store, input := newHarness(t, map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {Err: boom},
	})

	err := executor.Run(context.Background(), threeStagePlan(), input)
	if !errors.Is(err, boom) {
		t.Fatalf("Run error = %v, want wrapped %v", err, boom)
	}

	state := mustLoad(t, store)
	assertStatus(t, state, "analyst", orchestrator.StageStatusCompleted)
	assertStatus(t, state, "developer", orchestrator.StageStatusFailed)
	if state.Stages["developer"].Error == "" {
		t.Error("failed stage did not persist its error message")
	}
	if _, ran := state.Stages["qa-engineer"]; ran {
		t.Error("stage after the failure was started; run should stop on first failure")
	}
	assertInvocations(t, provider, []string{"analyst", "developer"})
}

func TestStageTimeoutFailsTheStage(t *testing.T) {
	plan := threeStagePlan()
	plan.Stages[1].Timeout = 20 * time.Millisecond
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {Hang: true},
	})

	err := executor.Run(context.Background(), plan, input)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context.DeadlineExceeded", err)
	}
	state := mustLoad(t, store)
	assertStatus(t, state, "developer", orchestrator.StageStatusFailed)
}

func TestCancelMidStagePersistsResumableCheckpoint(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":   {ArtifactContent: "# analysis"},
		"developer": {Hang: true},
	})
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(20*time.Millisecond, cancel)
	defer timer.Stop()
	defer cancel()

	err := executor.Run(ctx, threeStagePlan(), input)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}

	state := mustLoad(t, store)
	assertStatus(t, state, "analyst", orchestrator.StageStatusCompleted)
	assertStatus(t, state, "developer", orchestrator.StageStatusInterrupted)
	if state.IsStageCompleted("developer") {
		t.Error("interrupted stage must not count as completed for resume")
	}
}

func TestResumeSkipsCompletedAndRerunsInterruptedStage(t *testing.T) {
	executor, provider, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {Hang: true},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(20*time.Millisecond, cancel)
	defer timer.Stop()
	if err := executor.Run(ctx, threeStagePlan(), input); !errors.Is(err, context.Canceled) {
		t.Fatalf("first run error = %v, want context.Canceled", err)
	}
	cancel()

	provider.SetScript("developer", mock.Script{ArtifactContent: "# implementation"})
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume run: %v", err)
	}

	state := mustLoad(t, store)
	for _, stageID := range []string{"analyst", "developer", "qa-engineer"} {
		assertStatus(t, state, stageID, orchestrator.StageStatusCompleted)
	}
	// First run invoked analyst + developer; resume must skip the completed
	// analyst and re-run only the interrupted developer, then qa-engineer.
	assertInvocations(t, provider, []string{"analyst", "developer", "developer", "qa-engineer"})
}

func TestRunRefusesStateFromDifferentPlan(t *testing.T) {
	executor, _, store, input := newHarness(t, nil)
	if err := store.Save(orchestrator.NewRunState("some-other-plan", orchestrator.CreatedByExecutor)); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	err := executor.Run(context.Background(), threeStagePlan(), input)
	if err == nil {
		t.Fatal("Run accepted state belonging to a different plan")
	}
}

func TestDefaultDeliverFeaturePlanEncodesTheLinearAgentSequence(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	if err := plan.Validate(); err != nil {
		t.Fatalf("default plan invalid: %v", err)
	}
	// Fifteen agent stages plus the executor-internal router (L3.0); the
	// fifteenth is acceptance-scenarios, qa-engineer before the build (L3.62).
	if len(plan.Stages) != 16 {
		t.Fatalf("default plan has %d stages, want 16", len(plan.Stages))
	}
	first, last := plan.Stages[0], plan.Stages[len(plan.Stages)-1]
	if first.ID != "context-engineer" || last.ID != "devops-engineer" {
		t.Errorf("default plan order wrong: first %q, last %q", first.ID, last.ID)
	}
	for _, stage := range plan.Stages {
		if stage.Timeout <= 0 {
			t.Errorf("stage %q has no timeout; every invocation must be bounded", stage.ID)
		}
	}
}

func TestPlanValidateRejectsDefects(t *testing.T) {
	cases := []struct {
		name string
		plan orchestrator.Plan
	}{
		{"empty name", orchestrator.Plan{Stages: []orchestrator.Stage{{ID: "a"}}}},
		{"no stages", orchestrator.Plan{Name: "p"}},
		{"empty stage ID", orchestrator.Plan{Name: "p", Stages: []orchestrator.Stage{{ID: ""}}}},
		{"duplicate stage ID", orchestrator.Plan{Name: "p", Stages: []orchestrator.Stage{{ID: "a"}, {ID: "a"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.plan.Validate(); err == nil {
				t.Errorf("Validate accepted a defective plan (%s)", tc.name)
			}
		})
	}
}

func TestStateSurvivesCrashBetweenTempWriteAndRename(t *testing.T) {
	dir := t.TempDir()
	store := orchestrator.NewStateStore(filepath.Join(dir, orchestrator.RunStateFileName))
	state := orchestrator.NewRunState("test-plan", orchestrator.CreatedByExecutor)
	state.Stages["analyst"] = orchestrator.StageRecord{Status: orchestrator.StageStatusCompleted, StartedAt: time.Now().UTC()}
	if err := store.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Simulate a crash after the temp write but before os.Rename: a stray
	// temp file with garbage sits beside the committed state file.
	crashLeftover := filepath.Join(dir, orchestrator.RunStateFileName+".tmp-crashed")
	if err := os.WriteFile(crashLeftover, []byte("{ this is not valid json"), 0o644); err != nil {
		t.Fatalf("write crash leftover: %v", err)
	}

	loaded := mustLoad(t, store)
	if loaded.PlanName != "test-plan" || !loaded.IsStageCompleted("analyst") {
		t.Error("crash leftover corrupted the loaded state; rename must be the only commit point")
	}
}

func TestLoadRejectsUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, orchestrator.RunStateFileName)
	if err := os.WriteFile(path, []byte(`{"schemaVersion": 99, "planName": "p"}`), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := orchestrator.NewStateStore(path).Load(); err == nil {
		t.Fatal("Load accepted a run state with an unsupported schema version")
	}
}

func TestLoadReturnsNilForFreshRun(t *testing.T) {
	store := orchestrator.NewStateStore(filepath.Join(t.TempDir(), orchestrator.RunStateFileName))
	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state != nil {
		t.Fatal("Load of a missing file must mean a fresh run (nil state), not an error or empty state")
	}
}
