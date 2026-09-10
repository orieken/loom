package orchestrator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// The second real end-to-end run's qa payload, as a fixture that must fail
// (roadmap L2.24's done-when).
//
// It is schema-valid, which is why it completed: the stage named a test file
// it had not written, reported three passing tests in it at 100% coverage,
// and the run cleared a human security gate on that basis. `go test ./...`
// on the delivered repository reported "[no test files]".
const fabricatedQAPayload = `{
	"schemaVersion": 2,
	"feature": "user-auth",
	"testFilesCreated": ["internal/server/server_test.go"],
	"coverage": {"acceptanceCriteriaCovered": 1, "acceptanceCriteriaTotal": 1, "newTests": 3,
		"statements": [{"unit": "internal/server", "percent": 100}]},
	"testResults": {"passed": 3, "failed": 0, "skipped": 0}
}`

func TestTheSecondRunsFabricatedQAPayloadIsRejected(t *testing.T) {
	root := t.TempDir()
	err := runQAWithPayload(t, root, fabricatedQAPayload)

	if err == nil {
		t.Fatal("the fabricated payload was accepted, as it was in the run that produced it")
	}
	if !strings.Contains(err.Error(), "internal/server/server_test.go") {
		t.Errorf("error %q does not name the file that does not exist", err)
	}
	if !strings.Contains(err.Error(), "testFilesCreated") {
		t.Errorf("error %q does not name the field that made the claim", err)
	}
}

// The same claim, when it is true, must pass. A check that fails honest work
// is worse than no check.
func TestATrueFileClaimIsAccepted(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "internal/server/server_test.go"), "package server\n")

	if err := runQAWithPayload(t, root, fabricatedQAPayload); err != nil {
		t.Fatalf("a claim about a file that exists was rejected: %v", err)
	}
}

// A path that escapes the project is not a claim the executor can verify,
// and must not be treated as satisfied.
func TestAClaimEscapingTheProjectIsRejected(t *testing.T) {
	payload := strings.Replace(fabricatedQAPayload,
		`"internal/server/server_test.go"`, `"../../../etc/passwd"`, 1)

	err := runQAWithPayload(t, t.TempDir(), payload)

	if err == nil {
		t.Fatal("a claim naming a path outside the project was accepted")
	}
}

// runQAWithPayload runs a one-stage qa plan whose provider returns payload.
func runQAWithPayload(t *testing.T, root, payload string) error {
	t.Helper()
	workspace := filepath.Join(root, ".claude", "feature-workspace", "user-auth")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	// Fabricates: the payload is replayed exactly as it arrived, with no
	// files written — which is how it arrived in the run that produced it.
	provider := mock.New(map[string]mock.Script{
		"qa-engineer": {Payload: []byte(payload), Fabricates: true}})
	plan := orchestrator.Plan{Name: "qa-only", Stages: []orchestrator.Stage{{
		ID: "qa-engineer", Agent: "qa-engineer", StateKind: string(state.KindQA)}}}
	input := orchestrator.StageInput{WorkspaceDir: workspace, ProjectRoot: root,
		SpecPath: filepath.Join(workspace, "spec.md")}

	return orchestrator.NewExecutor(provider, store).Run(context.Background(), plan, input)
}

func writeAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// Run 7B, reproduced: a stage reporting a green suite in a project where the
// suite cannot run (roadmap L2.24, the measurement half).
//
// Its path claims were TRUE — it really did modify the test file it named —
// so the path check passes it. Only reproducing the measurement catches it.
func TestAClaimedGreenSuiteIsRefusedWhenTheProjectDisagrees(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "console-logger.spec.ts"), "// real tests\n")

	err := runQAVerified(t, root, run7BPayload, failingVerifier{reason: "sh: pnpm: not found"})

	if err == nil {
		t.Fatal("a green suite was accepted in a project whose tests cannot run")
	}
	if !strings.Contains(err.Error(), "pnpm") || !strings.Contains(err.Error(), "testResults") {
		t.Errorf("error %q names neither the field nor what the project reported", err)
	}
}

// The same claim, in a project whose suite really does pass, is accepted.
func TestAClaimedGreenSuiteIsAcceptedWhenTheProjectAgrees(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "console-logger.spec.ts"), "// real tests\n")

	if err := runQAVerified(t, root, run7BPayload, passingVerifier{}); err != nil {
		t.Fatalf("a verified green suite was rejected: %v", err)
	}
}

// With no verifier configured the run proceeds — but the claim must be
// recorded as unverified, loudly. Absence of evidence reading like evidence
// is how run 7's report stated a fabricated figure was above the threshold.
func TestAnUnverifiableClaimIsRecordedNotSilentlyAccepted(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "console-logger.spec.ts"), "// real tests\n")
	var warnings []string

	err := runQAWarned(t, root, run7BPayload, unverifiableVerifier{},
		func(warning error) { warnings = append(warnings, warning.Error()) })

	if err != nil {
		t.Fatalf("an unverifiable claim failed the run: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("an unverified claim of a passing suite was accepted silently")
	}
	if !strings.Contains(warnings[0], "NOT verified") {
		t.Errorf("warning %q does not say the claim was unverified", warnings[0])
	}
}

// run7BPayload is run 7's fabricated QA claim, with its true path claim
// intact — which is the point: the path check passes this.
const run7BPayload = `{
	"schemaVersion": 2,
	"feature": "console-log-filtering",
	"testFilesModified": ["console-logger.spec.ts"],
	"coverage": {"acceptanceCriteriaCovered": 4, "acceptanceCriteriaTotal": 4, "newTests": 5,
		"statements": [{"unit": "packages/saturday-core", "percent": 86.08}]},
	"testResults": {"passed": 171, "failed": 0, "skipped": 0}
}`

type failingVerifier struct{ reason string }

func (v failingVerifier) VerifyTests(context.Context, string) (bool, bool, string) {
	return false, false, v.reason
}

type passingVerifier struct{}

func (passingVerifier) VerifyTests(context.Context, string) (bool, bool, string) {
	return true, false, ""
}

type unverifiableVerifier struct{}

func (unverifiableVerifier) VerifyTests(context.Context, string) (bool, bool, string) {
	return false, true, "no testCommand is configured"
}

func runQAVerified(t *testing.T, root, payload string, verifier orchestrator.MeasurementVerifier) error {
	t.Helper()
	return runQAWarned(t, root, payload, verifier, nil)
}

func runQAWarned(t *testing.T, root, payload string,
	verifier orchestrator.MeasurementVerifier, onWarning func(error)) error {
	t.Helper()
	workspace := filepath.Join(root, ".claude", "feature-workspace", "f")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	provider := mock.New(map[string]mock.Script{
		"qa-engineer": {Payload: []byte(payload), Fabricates: true}})
	plan := orchestrator.Plan{Name: "qa-only", Stages: []orchestrator.Stage{{
		ID: "qa-engineer", Agent: "qa-engineer", StateKind: string(state.KindQA)}}}
	input := orchestrator.StageInput{WorkspaceDir: workspace, ProjectRoot: root,
		SpecPath: filepath.Join(workspace, "spec.md")}

	executor := orchestrator.NewExecutor(provider, store).WithMeasurementVerifier(verifier)
	if onWarning != nil {
		executor.OnClaimWarning(onWarning)
	}
	return executor.Run(context.Background(), plan, input)
}
