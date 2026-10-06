package state_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func contextWithBlast(radius *state.BlastRadius) state.ContextState {
	return state.ContextState{
		SchemaVersion: state.SchemaVersion, Feature: "f", Tier: "developer",
		PinnedFiles: []state.PinnedFile{{Path: "a.go", Reason: "r"}}, BlastRadius: radius,
	}
}

func resolver(impacted ...state.BlastImpacted) state.BlastRadiusResolver {
	return func([]string, int) (state.BlastResolution, error) {
		return state.BlastResolution{GraphSHA: "abcdef0123456789", Fresh: true, Impacted: impacted}, nil
	}
}

func TestAContextWithoutABlastRadiusIsUnchanged(t *testing.T) {
	context := contextWithBlast(nil)
	if err := context.Validate(); err != nil {
		t.Fatal(err)
	}
	context.ResolveBlastRadius(resolver())
	if context.BlastRadius != nil {
		t.Error("nothing to resolve, nothing to invent")
	}
	if _, body, err := state.RenderView(state.KindContext, mustJSON(t, context)); err != nil || strings.Contains(body, "Blast Radius") {
		t.Errorf("the section appears only when there is one: %v", err)
	}
}

func TestBlastRadiusValidationNamesTheBrokenField(t *testing.T) {
	cases := map[string]*state.BlastRadius{
		"blastRadius.seeds":    {Seeds: nil},
		"blastRadius.seeds[1]": {Seeds: []string{"a", ""}},
		"blastRadius.depth":    {Seeds: []string{"a"}, Depth: 11},
	}
	for field, radius := range cases {
		context := contextWithBlast(radius)
		assertValidationNames(t, context.Validate(), field)
	}
	negative := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}, Depth: -1})
	assertValidationNames(t, negative.Validate(), "blastRadius.depth")
	for _, depth := range []int{0, 1, 10} {
		ok := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}, Depth: depth})
		if err := ok.Validate(); err != nil {
			t.Errorf("depth %d is valid: %v", depth, err)
		}
	}
}

func forgedRadius() *state.BlastRadius {
	return &state.BlastRadius{
		Seeds: []string{"internal/state"}, Depth: 2,
		Impacted: []state.BlastImpacted{{Path: "made/up", Distance: 1, Confidence: "exact"}}, Total: 1, Fresh: true, GraphSHA: "forged", Truncated: true, Unresolved: "lie",
	}
}

// The agent supplies the question. Whatever it wrote as the answer is replaced.
func TestTheExecutorReplacesAnythingTheAgentWroteAsTheAnswer(t *testing.T) {
	context := contextWithBlast(forgedRadius())
	context.ResolveBlastRadius(resolver(state.BlastImpacted{Path: "internal/orchestrator", Distance: 1, Confidence: "exact"}))
	got := context.BlastRadius
	if got.Total != 1 || got.Impacted[0].Path != "internal/orchestrator" || got.GraphSHA != "abcdef0123456789" {
		t.Errorf("a model-written radius must not survive: %+v", got)
	}
	if got.Truncated || got.Unresolved != "" {
		t.Errorf("forged flags must not survive: %+v", got)
	}
}

func TestTheQuestionTheAgentAskedIsKept(t *testing.T) {
	context := contextWithBlast(forgedRadius())
	context.ResolveBlastRadius(resolver())
	if got := context.BlastRadius; got.Seeds[0] != "internal/state" || got.Depth != 2 {
		t.Errorf("%+v", got)
	}
}

func TestAnUnresolvableRadiusSaysWhyInsteadOfReadingAsEmpty(t *testing.T) {
	failing := state.BlastRadiusResolver(func([]string, int) (state.BlastResolution, error) {
		return state.BlastResolution{}, errors.New("not in the repo graph: typo")
	})
	context := contextWithBlast(&state.BlastRadius{Seeds: []string{"typo"}})
	context.ResolveBlastRadius(failing)
	if got := context.BlastRadius; got.Unresolved != "not in the repo graph: typo" || len(got.Impacted) != 0 || got.Total != 0 {
		t.Errorf("%+v", got)
	}
	none := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	none.ResolveBlastRadius(nil)
	if !strings.Contains(none.BlastRadius.Unresolved, "loom index") {
		t.Errorf("no resolver at all: %+v", none.BlastRadius)
	}
}

func TestALongRadiusIsCappedButStillReportsItsTrueTotal(t *testing.T) {
	var many []state.BlastImpacted
	for i := 0; i < state.MaxBlastRadiusEntries+7; i++ {
		many = append(many, state.BlastImpacted{Path: "d", Distance: 1, Confidence: "exact"})
	}
	context := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	context.ResolveBlastRadius(resolver(many...))
	got := context.BlastRadius
	if len(got.Impacted) != state.MaxBlastRadiusEntries || got.Total != state.MaxBlastRadiusEntries+7 || !got.Truncated {
		t.Errorf("len=%d total=%d truncated=%v", len(got.Impacted), got.Total, got.Truncated)
	}
	exact := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	exact.ResolveBlastRadius(resolver(many[:state.MaxBlastRadiusEntries]...))
	if exact.BlastRadius.Truncated {
		t.Error("exactly the cap is not a truncation")
	}
}

func TestBlastRadiusRendersTheQuestionAndTheComputedAnswer(t *testing.T) {
	context := contextWithBlast(&state.BlastRadius{Seeds: []string{"internal/state", "cmd/x"}})
	context.ResolveBlastRadius(resolver(
		state.BlastImpacted{Path: "internal/orchestrator", Distance: 1, Confidence: "exact"},
		state.BlastImpacted{Path: "cmd/loom", Distance: 2, Confidence: "approximate"},
	))
	_, body, err := state.RenderView(state.KindContext, mustJSON(t, context))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Blast Radius", "`internal/state`, `cmd/x`", "graph at `abcdef01` (fresh)", "2 directories depend on it", "`internal/orchestrator` — 1 import away, exact", "`cmd/loom` — 2 imports away, approximate"} {
		if !strings.Contains(body, want) {
			t.Errorf("view lacks %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "## Blast Radius") > strings.Index(body, "## Token Budget") {
		t.Error("the blast radius belongs before the budget it informs")
	}
}

func TestBlastRadiusRenderingSaysWhenItIsStaleUnresolvedOrTruncated(t *testing.T) {
	stale := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	stale.ResolveBlastRadius(func([]string, int) (state.BlastResolution, error) {
		return state.BlastResolution{GraphSHA: "", Fresh: false, Impacted: []state.BlastImpacted{{Path: "b", Distance: 1, Confidence: "exact"}}}, nil
	})
	_, body, _ := state.RenderView(state.KindContext, mustJSON(t, stale))
	if !strings.Contains(body, "STALE") || !strings.Contains(body, "`unknown`") || !strings.Contains(body, "1 directory depend") {
		t.Errorf("%s", body)
	}
	unresolved := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	unresolved.ResolveBlastRadius(nil)
	if _, body, _ := state.RenderView(state.KindContext, mustJSON(t, unresolved)); !strings.Contains(body, "**Not resolved**") {
		t.Errorf("%s", body)
	}
	var many []state.BlastImpacted
	for i := 0; i < state.MaxBlastRadiusEntries+3; i++ {
		many = append(many, state.BlastImpacted{Path: "d", Distance: 1, Confidence: "exact"})
	}
	capped := contextWithBlast(&state.BlastRadius{Seeds: []string{"a"}})
	capped.ResolveBlastRadius(resolver(many...))
	if _, body, _ := state.RenderView(state.KindContext, mustJSON(t, capped)); !strings.Contains(body, "… and 3 more") {
		t.Errorf("%s", body)
	}
}

func TestTheContextSchemaTellsTheAgentWhichBlastFieldsAreItsToSupply(t *testing.T) {
	schema, ok := state.SchemaForKind(state.KindContext)
	if !ok || !strings.Contains(string(schema), "blastRadius") || strings.Count(string(schema), "Computed by loom. Do not supply") < 6 {
		t.Error("the agent must be told everything but seeds and depth is replaced")
	}
}

// The rendered manifest for a change with a blast radius is a golden file: a
// reader's view of what context-engineer pinned and why. Regenerate
// deliberately with UPDATE_GOLDEN=1 and read the diff.
func TestContextManifestWithABlastRadiusMatchesItsGoldenFile(t *testing.T) {
	context := state.ContextState{
		SchemaVersion: state.SchemaVersion, Feature: "rename-ledger-digest", Tier: "developer", TargetStage: "developer",
		PinnedFiles: []state.PinnedFile{
			{Path: "internal/ledger/digest.go", Reason: "the change itself"},
			{Path: "internal/orchestrator/integrity.go", Reason: "distance-1 exact dependent of internal/ledger (impact)"},
			{Path: "cmd/loom/cmd/verify.go", Reason: "distance-2 dependent; the signature it calls changes"},
		},
		BlastRadius: &state.BlastRadius{Seeds: []string{"internal/ledger"}, Depth: 3},
	}
	context.ResolveBlastRadius(resolver(
		state.BlastImpacted{Path: "internal/orchestrator", Distance: 1, Confidence: "exact"},
		state.BlastImpacted{Path: "internal/policy", Distance: 1, Confidence: "approximate"},
		state.BlastImpacted{Path: "cmd/loom/cmd", Distance: 2, Confidence: "exact"},
	))
	_, actual, err := state.RenderView(state.KindContext, mustJSON(t, context))
	if err != nil {
		t.Fatal(err)
	}
	golden := "testdata/context-manifest-blast-radius.golden.md"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(expected) != actual {
		t.Errorf("rendered manifest differs from %s; run with UPDATE_GOLDEN=1 and review the diff\n%s", golden, actual)
	}
}
