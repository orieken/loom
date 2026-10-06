package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

// A context manifest's blast radius is computed by the executor from the repo
// graph, never taken from the agent.
func TestMeasureTypedOutputResolvesAContextManifestsBlastRadius(t *testing.T) {
	payload := []byte(`{"schemaVersion":2,"feature":"f","tier":"developer","pinnedFiles":[{"path":"a.go","reason":"r"}],` +
		`"blastRadius":{"seeds":["internal/state"],"impacted":[{"path":"forged","distance":1,"confidence":"exact"}],"total":9}}`)
	decoded, err := state.Decode(state.KindContext, payload)
	if err != nil {
		t.Fatal(err)
	}
	resolver := state.BlastRadiusResolver(func(_ []string, _ int) (state.BlastResolution, error) {
		return state.BlastResolution{GraphSHA: "sha", Fresh: true, Impacted: []state.BlastImpacted{{Path: "real", Distance: 1, Confidence: "exact"}}}, nil
	})
	measured, err := measureTypedOutput(decoded, StageInput{ProjectRoot: t.TempDir()}, payload, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(measured), `"path":"real"`) || strings.Contains(string(measured), "forged") || !strings.Contains(string(measured), `"total":1`) {
		t.Errorf("the agent's answer must be replaced by the graph's: %s", measured)
	}
}

func TestAnExecutorWithoutAResolverMarksTheRadiusUnresolved(t *testing.T) {
	payload := []byte(`{"schemaVersion":2,"feature":"f","tier":"developer","pinnedFiles":[{"path":"a.go","reason":"r"}],"blastRadius":{"seeds":["x"],"total":4}}`)
	decoded, _ := state.Decode(state.KindContext, payload)
	measured, err := measureTypedOutput(decoded, StageInput{ProjectRoot: t.TempDir()}, payload, nil)
	if err != nil || !strings.Contains(string(measured), "unresolved") || strings.Contains(string(measured), `"total":4`) {
		t.Errorf("%v %s", err, measured)
	}
}

func TestExecutorWiresItsResolverIntoValidation(t *testing.T) {
	executor := &Executor{}
	called := false
	executor.WithBlastRadiusResolver(func([]string, int) (state.BlastResolution, error) { called = true; return state.BlastResolution{}, nil })
	if executor.blastResolver == nil {
		t.Fatal("not set")
	}
	_, _ = executor.blastResolver([]string{"x"}, 1)
	if !called {
		t.Error("the resolver given is the resolver kept")
	}
}

func TestOrNoopGivesANonNilTracerSoCallersNeverBranch(t *testing.T) {
	tracer := OrNoop(nil)
	if tracer == nil {
		t.Fatal("OrNoop(nil) must return a usable tracer")
	}
	ctx, span := tracer.StartRun(context.Background(), RunSpan{})
	span.End(SpanOutcome{})
	if ctx == nil {
		t.Error("a context must come back")
	}
	if given := (noopTracer{}); OrNoop(given) != Tracer(given) {
		t.Error("a real tracer is returned unchanged")
	}
}
