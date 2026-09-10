package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// typedPlan is the L2.9 slice: a typed analyst whose state the typed
// architect consumes, followed by an untyped stage still on markdown.
func typedPlan() orchestrator.Plan {
	return orchestrator.Plan{
		Name: "typed-plan",
		Stages: []orchestrator.Stage{
			{ID: "analyst", Agent: "analyst", StateKind: string(state.KindAnalysis), Timeout: 5 * time.Second},
			{ID: "architect", Agent: "architect", StateKind: string(state.KindArchitecture),
				Consumes: []string{"analyst"}, Timeout: 5 * time.Second},
			{ID: "developer", Agent: "developer", Timeout: 5 * time.Second},
		},
	}
}

func typedScripts(t *testing.T) map[string]mock.Script {
	t.Helper()
	analysis, ok := mock.TypedScript(string(state.KindAnalysis))
	if !ok {
		t.Fatal("no scripted analysis payload")
	}
	architecture, ok := mock.TypedScript(string(state.KindArchitecture))
	if !ok {
		t.Fatal("no scripted architecture payload")
	}
	return map[string]mock.Script{
		"analyst":   analysis,
		"architect": architecture,
		"developer": {ArtifactContent: "# implementation"},
	}
}

// TestTypedStagesExchangeDataWithNoMarkdownOnThePath is the L2.9 done-when:
// the architect receives the analyst's fields as validated data, and no
// markdown file exists for either stage.
func TestTypedStagesExchangeDataWithNoMarkdownOnThePath(t *testing.T) {
	scripts := typedScripts(t)
	executor, provider, store, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertNoMarkdownFor(t, input, "analyst", "architect")
	received := provider.InputFor("architect")
	if len(received.UpstreamState["analyst"]) == 0 {
		t.Fatalf("architect received no projected analysis: %+v", received)
	}
	assertProjectionContent(t, received.UpstreamState["analyst"])
	assertTypedArtifactRecorded(t, store, input, "analyst")
}

func assertNoMarkdownFor(t *testing.T, input orchestrator.StageInput, stageIDs ...string) {
	t.Helper()
	for _, stageID := range stageIDs {
		if _, err := os.Stat(filepath.Join(input.WorkspaceDir, stageID+".md")); err == nil {
			t.Errorf("stage %q wrote a markdown file; typed stages exchange state, not documents", stageID)
		}
	}
}

// assertProjectionContent checks the architect got the analysis fields it
// declares — and none of the ones it does not. Passing the whole document
// would defeat the point of L2.9.
func assertProjectionContent(t *testing.T, projected []byte) {
	t.Helper()
	var received state.ArchitectInput
	if err := json.Unmarshal(projected, &received); err != nil {
		t.Fatalf("projection does not parse: %v", err)
	}
	if received.Feature == "" || len(received.AcceptanceCriteria) == 0 || received.BoundedContext.Owning == "" {
		t.Errorf("projection is missing fields the architect needs: %+v", received)
	}
	if strings.Contains(string(projected), "definitionOfDone") || strings.Contains(string(projected), "\"qa\"") {
		t.Errorf("projection leaked fields the architect does not read:\n%s", projected)
	}
}

func assertTypedArtifactRecorded(t *testing.T, store *orchestrator.StateStore, input orchestrator.StageInput, stageID string) {
	t.Helper()
	record := mustLoad(t, store).Stages[stageID]
	want := filepath.Join(input.WorkspaceDir, state.TypedStateDir, stageID+".json")
	if record.ArtifactPath != want {
		t.Errorf("stage %q artifact = %q, want its typed state document", stageID, record.ArtifactPath)
	}
	if record.ArtifactSHA256 == "" {
		t.Error("typed state was not digest-recorded; L2.12 integrity must cover it")
	}
}

func TestTypedStageRejectsInvalidPayload(t *testing.T) {
	scripts := typedScripts(t)
	scripts["analyst"] = mock.Script{Payload: fmt.Appendf(nil, `{"schemaVersion":%d,"feature":"x"}`, state.SchemaVersion)}
	executor, provider, store, input := newHarness(t, scripts)

	err := executor.Run(context.Background(), typedPlan(), input)

	if err == nil || !strings.Contains(err.Error(), "summary") {
		t.Fatalf("Run error = %v, want a validation failure naming the missing field", err)
	}
	assertStatus(t, mustLoad(t, store), "analyst", orchestrator.StageStatusFailed)
	assertInvocations(t, provider, []string{"analyst"})
}

func TestTypedStageRejectsInventedFields(t *testing.T) {
	scripts := typedScripts(t)
	valid, _ := mock.TypedScript(string(state.KindAnalysis))
	withExtra := strings.Replace(string(valid.Payload), `{"schemaVersion"`, `{"vibes":"immaculate","schemaVersion"`, 1)
	scripts["analyst"] = mock.Script{Payload: []byte(withExtra)}
	executor, _, _, input := newHarness(t, scripts)

	err := executor.Run(context.Background(), typedPlan(), input)

	if err == nil || !strings.Contains(err.Error(), "vibes") {
		t.Fatalf("Run error = %v, want rejection naming the invented field", err)
	}
}

func TestTypedStageWithNoPayloadFails(t *testing.T) {
	scripts := typedScripts(t)
	scripts["analyst"] = mock.Script{ArtifactContent: "# analysis in markdown"}
	executor, _, _, input := newHarness(t, scripts)

	err := executor.Run(context.Background(), typedPlan(), input)

	if err == nil || !strings.Contains(err.Error(), "no state payload") {
		t.Fatalf("Run error = %v, want a failure about the missing payload", err)
	}
}

func TestEditingTypedStateDemotesTheStage(t *testing.T) {
	executor, provider, store, input := newHarness(t, typedScripts(t))
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("first run: %v", err)
	}

	statePath := filepath.Join(input.WorkspaceDir, state.TypedStateDir, "analyst.json")
	if err := os.WriteFile(statePath, []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit typed state: %v", err)
	}
	var reported []orchestrator.StaleStage
	executor.OnStale(func(stale []orchestrator.StaleStage) { reported = stale })
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if len(reported) == 0 || reported[0].StageID != "analyst" {
		t.Fatalf("stale report = %+v, want the edited analyst", reported)
	}
	assertStatus(t, mustLoad(t, store), "analyst", orchestrator.StageStatusCompleted)
	assertInvocations(t, provider, []string{"analyst", "architect", "developer", "analyst", "architect", "developer"})
}

func TestUntypedStagesAreUnaffected(t *testing.T) {
	executor, _, store, input := newHarness(t, typedScripts(t))
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	record := mustLoad(t, store).Stages["developer"]
	if !strings.HasSuffix(record.ArtifactPath, "developer.md") {
		t.Errorf("untyped stage artifact = %q, want the markdown file it wrote", record.ArtifactPath)
	}
}

func TestDefaultPlanTypedStages(t *testing.T) {
	kinds := typedStagesOf(orchestrator.DefaultDeliverFeaturePlan())
	want := map[string]string{
		// The context manifest is typed so the executor measures its token
		// budget instead of the agent asserting it (L3.25).
		"context-engineer":         string(state.KindContext),
		"analyst":                  string(state.KindAnalysis),
		orchestrator.RouterStageID: string(state.KindRoute),
		"architect":                string(state.KindArchitecture),
		// The review verdict is typed so the loop reads a field (L2.17).
		"code-reviewer": string(state.KindReview),
		// The implementation chain (L2.9 second cut).
		"developer":         string(state.KindImplementation),
		"security-reviewer": string(state.KindSecurity),
		"qa-engineer":       string(state.KindQA),
	}

	if len(kinds) != len(want) {
		t.Fatalf("typed stages = %v, want exactly %v in this cut", kinds, want)
	}
	for stageID, kind := range want {
		if kinds[stageID] != kind {
			t.Errorf("stage %q kind = %q, want %q", stageID, kinds[stageID], kind)
		}
	}
	assertArchitectConsumesAnalyst(t)
}

func typedStagesOf(plan orchestrator.Plan) map[string]string {
	kinds := map[string]string{}
	for _, stage := range plan.Stages {
		if stage.StateKind != "" {
			kinds[stage.ID] = stage.StateKind
		}
	}
	return kinds
}

func assertArchitectConsumesAnalyst(t *testing.T) {
	t.Helper()
	for _, stage := range orchestrator.DefaultDeliverFeaturePlan().Stages {
		if stage.ID == "architect" && strings.Join(stage.Consumes, ",") != "analyst" {
			t.Errorf("architect consumes %q, want analyst", stage.Consumes)
		}
	}
}

// TestTypedStageRendersTheContractsMarkdownView checks the half of L2.9
// that keeps the still-untyped stages working: they were told to read
// analysis.md, so that is where the view lands.
func TestTypedStageRendersTheContractsMarkdownView(t *testing.T) {
	executor, _, store, input := newHarness(t, typedScripts(t))
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, view := range []string{"analysis.md", "architecture-notes.md"} {
		body, err := os.ReadFile(filepath.Join(input.WorkspaceDir, view))
		if err != nil {
			t.Fatalf("rendered view %s missing: %v", view, err)
		}
		if !strings.Contains(string(body), "## ") {
			t.Errorf("%s does not look like a rendered document:\n%s", view, body)
		}
	}
	// The view is derived, so it is not what integrity tracks — editing it
	// must not be able to corrupt a run.
	record := mustLoad(t, store).Stages["analyst"]
	if strings.HasSuffix(record.ArtifactPath, ".md") {
		t.Errorf("the rendered view is being tracked as the artifact: %q", record.ArtifactPath)
	}
}

func TestEditingTheRenderedViewDoesNotDemoteTheStage(t *testing.T) {
	executor, provider, _, input := newHarness(t, typedScripts(t))
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if err := os.WriteFile(filepath.Join(input.WorkspaceDir, "analysis.md"), []byte("# scribbled over\n"), 0o644); err != nil {
		t.Fatalf("edit view: %v", err)
	}
	var reported []orchestrator.StaleStage
	executor.OnStale(func(stale []orchestrator.StaleStage) { reported = stale })
	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if len(reported) != 0 {
		t.Errorf("editing a derived view demoted %+v; only state is tracked", reported)
	}
	assertInvocations(t, provider, []string{"analyst", "architect", "developer"})
}

// TestQAEngineerReadsThreeUpstreams covers the multi-upstream case the
// contracts describe: what was built, what security found, and the criteria
// to test against — each labelled with where it came from.
func TestQAEngineerReadsThreeUpstreams(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	consumed := map[string][]string{}
	for _, stage := range plan.Stages {
		if len(stage.Consumes) > 0 {
			consumed[stage.ID] = stage.Consumes
		}
	}

	qa := consumed["qa-engineer"]
	if len(qa) != 3 {
		t.Fatalf("qa-engineer consumes %v, want its three declared upstreams", qa)
	}
	for _, want := range []string{"developer", "security-reviewer", "analyst"} {
		if !containsString(qa, want) {
			t.Errorf("qa-engineer does not consume %q", want)
		}
	}
	// tech-writer produces markdown in this cut but still reads state.
	if len(consumed["tech-writer"]) != 2 {
		t.Errorf("tech-writer consumes %v, want qa-engineer and analyst", consumed["tech-writer"])
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestMissingUpstreamIsNotAFailure covers a stage the route skipped or the
// run has not reached: its projection is simply absent.
func TestMissingUpstreamIsNotAFailure(t *testing.T) {
	// A stage declared after the architect, so it has not produced state by
	// the time the architect's projections are computed.
	plan := typedPlan()
	plan.Stages[1].Consumes = []string{"analyst", "later-stage"}
	plan.Stages = append(plan.Stages, orchestrator.Stage{
		ID: "later-stage", Agent: "later", Timeout: time.Second,
	})
	scripts := typedScripts(t)
	scripts["later-stage"] = mock.Script{ArtifactContent: "# later"}
	executor, provider, _, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	received := provider.InputFor("architect")
	if len(received.UpstreamState["analyst"]) == 0 {
		t.Error("the upstream that did run was not projected")
	}
	if _, present := received.UpstreamState["later-stage"]; present {
		t.Error("a stage that had not produced state was projected anyway")
	}
}

// The executor measures a context manifest from the files on disk, replacing
// whatever the agent claimed (roadmap L3.25). The agent asserting this number
// is how a pinned set of ~9,100 tokens was reported as 1,350 with an OK
// status — and it is the one number in a run that nothing else checks.
func TestExecutorMeasuresTheContextBudgetFromDisk(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "ARCHITECTURE_RULES.md"), 14949)
	writeSized(t, filepath.Join(root, "DOMAIN_DICTIONARY.md"), 18530)

	manifest := runManifestStage(t, root, `{
		"schemaVersion": 2, "feature": "console-log-filtering", "tier": "analyst",
		"pinnedFiles": [
			{"path": "ARCHITECTURE_RULES.md", "reason": "rules"},
			{"path": "DOMAIN_DICTIONARY.md", "reason": "terms"}
		],
		"budget": {"files": [], "estimatedTokens": 1350, "tierLimitTokens": 120000, "status": "OK"}
	}`)

	const want = (14949 + 18530) / 4
	if manifest.Budget.EstimatedTokens != want {
		t.Errorf("budget = %d, want the measured %d rather than the agent's 1350",
			manifest.Budget.EstimatedTokens, want)
	}
}

// A pinned path that escapes the project is not measured, and not silently
// counted as zero either.
func TestExecutorRefusesToMeasureOutsideTheProject(t *testing.T) {
	root := t.TempDir()

	manifest := runManifestStage(t, root, `{
		"schemaVersion": 2, "feature": "f", "tier": "analyst",
		"pinnedFiles": [{"path": "../../../etc/passwd", "reason": "nope"}]
	}`)

	if manifest.Budget.Unmeasured != 1 {
		t.Fatalf("unmeasured = %d, want the escaping path reported", manifest.Budget.Unmeasured)
	}
	if manifest.Budget.EstimatedTokens != 0 {
		t.Errorf("estimate = %d, want 0 for a path that was refused", manifest.Budget.EstimatedTokens)
	}
}

// runManifestStage runs a one-stage plan whose context-engineer returns the
// given payload, and returns the manifest as it was persisted.
func runManifestStage(t *testing.T, root, payload string) state.ContextState {
	t.Helper()
	workspace := filepath.Join(root, ".claude", "feature-workspace", "f")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	provider := mock.New(map[string]mock.Script{"context-engineer": {Payload: []byte(payload)}})
	plan := orchestrator.Plan{Name: "manifest-only", Stages: []orchestrator.Stage{{
		ID: "context-engineer", Agent: "context-engineer",
		StateKind: string(state.KindContext), Timeout: 5 * time.Second}}}
	input := orchestrator.StageInput{WorkspaceDir: workspace, ProjectRoot: root,
		SpecPath: filepath.Join(workspace, "spec.md")}

	if err := orchestrator.NewExecutor(provider, store).Run(context.Background(), plan, input); err != nil {
		t.Fatalf("run manifest stage: %v", err)
	}
	return readPersistedManifest(t, workspace)
}

func readPersistedManifest(t *testing.T, workspace string) state.ContextState {
	t.Helper()
	path := filepath.Join(workspace, state.TypedStateDir, "context-engineer.json")
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted state: %v", err)
	}
	var manifest state.ContextState
	if err := json.Unmarshal(written, &manifest); err != nil {
		t.Fatalf("decode persisted state: %v", err)
	}
	if manifest.Budget == nil {
		t.Fatal("the persisted manifest carries no measured budget")
	}
	return manifest
}

func writeSized(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
