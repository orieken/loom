//go:build integration

// Package claude's model-boundary contract test (roadmap L3.35).
//
// # WHY THIS EXISTS
//
// Run 4's audit, §12.6: four of its seven findings were invisible to the test
// suite, which was green before the run and green after it. Neither state
// predicted anything. Three of those defects had one shape — a property the
// framework asserts and verifies against mocks, which does not hold when a
// real model is asked:
//
//   - L3.28  schemaVersion reflected as a bare integer while the validator
//     demanded equality. Every typed stage halted. $2.09, zero stages.
//   - L3.33  the architecture schema does not express a conditional the
//     validator enforces. Halted Experiment A at stage 4.
//   - L3.29  the typed contract told every stage "Do not write files",
//     including the one whose job is writing files. The developer
//     reported success against an empty git diff.
//
// A mock provider cannot find any of these. It builds state in Go, where
// every constant is correct by construction and no instruction is obeyed or
// disobeyed. The gap is not test coverage; it is that the tests were on the
// wrong side of the boundary.
//
// This test puts one on the right side: it asks a real model for each typed
// stage's document and checks the validator accepts it. It is the cheapest
// instrument that would have caught L3.28 and L3.33 — both were a single
// stage invocation away from being visible.
//
// # WHAT IT COSTS
//
// Real model calls. Roughly $0.50–1.50 per stage at run-4 rates, so about
// $5–10 for every kind. Run one kind while iterating:
//
//	LOOM_INTEGRATION=1 go test -tags=integration ./internal/provider/claude/ \
//	  -run 'TestTypedStageContract/analysis' -v
//
// It is excluded from `go test ./...` by the build tag AND gated on the env
// var, because a suite that silently spends money is worse than no suite.
//
// # WHAT IT DOES NOT PROVE
//
// That a stage's output is correct — only that it conforms. A model can
// return a schema-valid analysis that is nonsense, and this passes. It tests
// the contract boundary, not the reasoning, and it is n=1 per run against a
// nondeterministic system: a pass is evidence, not proof.
//
// More important, and measured rather than assumed: it only catches contract
// defects that BIND on the spec below. L3.28 bound on every document, because
// schemaVersion is present in all of them, so this finds it in 23 seconds.
// L3.33 does not: fitness is only required when the architect HAS a decision
// with no fitness function, and against this spec it wrote one for every
// decision. Reintroducing L3.33 and re-running TestTypedStageContract/
// architecture PASSES — verified, not supposed.
//
// So the load-bearing guard for a conditional requirement is the unit test
// asserting it reaches the generated schema
// (TestEveryConditionalRequirementReachesTheSchema), not this. Read this file
// as "the always-binding half of the contract, checked against a real model",
// which is narrower than its name suggests and is the honest scope.
package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// typedStagesUnderContract maps each typed kind to the agent that produces it.
// Every entry in the built-in plan's typed set belongs here; a kind missing
// from this table is a kind nothing checks at the boundary.
func typedStagesUnderContract() map[state.Kind]string {
	return map[state.Kind]string{
		state.KindContext:        "context-engineer",
		state.KindAnalysis:       "analyst",
		state.KindArchitecture:   "architect",
		state.KindImplementation: "developer",
		state.KindReview:         "code-reviewer",
		state.KindSecurity:       "security-reviewer",
		state.KindQA:             "qa-engineer",
	}
}

// TestTypedStageContract asks a real model for each typed document and
// asserts the validator accepts what comes back.
//
// The assertion is deliberately the validator itself, not a hand-written list
// of fields: the defect class is "the schema says something weaker than the
// validator enforces", so anything that re-states the validator's rules here
// would drift from it exactly as the schemas did.
func TestTypedStageContract(t *testing.T) {
	requireIntegration(t)
	for kind, agent := range typedStagesUnderContract() {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			payload := invokeForContract(t, kind, agent)
			if _, err := state.Decode(kind, payload); err != nil {
				t.Fatalf("a real model's %s document was refused by the validator: %v\n\npayload:\n%s",
					kind, err, payload)
			}
		})
	}
}

// TestDeveloperActuallyWritesFiles is L3.29's regression at the boundary that
// found it. The tool grant (L2.22) is verified by unit tests; that the stage
// is not then told to ignore it is only observable here.
func TestDeveloperActuallyWritesFiles(t *testing.T) {
	requireIntegration(t)
	workspace := t.TempDir()
	target := filepath.Join(workspace, "greeting.txt")

	stage := orchestrator.Stage{ID: "developer", Agent: "developer",
		StateKind: string(state.KindImplementation), Timeout: 15 * time.Minute}
	input := orchestrator.StageInput{
		SpecPath:     writeSpec(t, workspace, "# Feature: greeting file\n\nCreate `greeting.txt` in the workspace containing exactly `hello`.\n"),
		WorkspaceDir: workspace,
		ProjectRoot:  workspace,
	}

	if _, err := realProvider(t).Invoke(context.Background(), stage, input); err != nil {
		t.Fatalf("developer stage: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("the developer wrote no file — the tools were granted and then taken away by "+
			"instruction (roadmap L3.29): %v", err)
	}
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("LOOM_INTEGRATION") == "" {
		t.Skip("set LOOM_INTEGRATION=1 to run the model-boundary contract tests (they cost money)")
	}
}

func realProvider(t *testing.T) *Provider {
	t.Helper()
	agents, err := filepath.Abs("../../../shared/agents")
	if err != nil {
		t.Fatalf("resolve agents dir: %v", err)
	}
	if _, err := os.Stat(agents); err != nil {
		t.Fatalf("agent definitions not found at %s: %v", agents, err)
	}
	return New(Config{AgentsDir: agents})
}

// invokeForContract runs one stage against a trivial spec and returns its
// payload. The spec is deliberately minimal: this test is about whether the
// document conforms, and a large feature buys nothing but tokens.
func invokeForContract(t *testing.T, kind state.Kind, agent string) []byte {
	t.Helper()
	workspace := t.TempDir()
	stage := orchestrator.Stage{ID: agent, Agent: agent,
		StateKind: string(kind), Timeout: 15 * time.Minute}
	input := orchestrator.StageInput{
		SpecPath:     writeSpec(t, workspace, contractSpec),
		WorkspaceDir: workspace,
		ProjectRoot:  workspace,
	}

	output, err := realProvider(t).Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("%s stage failed before returning a document: %v", agent, err)
	}
	if len(output.Payload) == 0 {
		t.Fatalf("%s returned no state payload", agent)
	}
	return output.Payload
}

// contractSpec is small enough to keep the bill down and complete enough that
// every stage has something to say about it.
const contractSpec = `# Feature: trim whitespace from a greeting

Add a ` + "`trim()`" + ` helper that removes leading and trailing whitespace from a
greeting string before it is displayed.

## Acceptance criteria
- ` + "`trim(\"  hi  \")`" + ` returns ` + "`\"hi\"`" + `
- An already-trimmed string is returned unchanged
- An empty string returns an empty string, never an error
`

func writeSpec(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}
