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

// A deletion-only implementation that renamed seven files and reported them
// as "old -> new" in filesModified parked saturday-monorepo's
// golden-site-file-names order as work it did not do, although the work was
// done. These cases are the fix: filesDeleted checked the opposite way from
// filesCreated/filesModified, and "old -> new" rejected outright.

// A path absent now and seen as changed by the posture check is a true
// deletion claim.
func TestATrueDeletionClaimIsAccepted(t *testing.T) {
	root := t.TempDir()
	tree := &changingTree{digests: []string{"same"}, changed: []string{"internal/site/old-name.go"}}

	err := runImplementationWithPayload(t, root, deletionOnlyPayload, tree)

	if err != nil {
		t.Fatalf("a true deletion claim was rejected: %v", err)
	}
}

// A path still on disk contradicts the claim, regardless of what the
// posture check saw.
func TestAFalseDeletionClaimIsRejected(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "internal/site/old-name.go"), "package site\n")
	tree := &changingTree{digests: []string{"same"}, changed: []string{"internal/site/old-name.go"}}

	err := runImplementationWithPayload(t, root, deletionOnlyPayload, tree)

	if err == nil {
		t.Fatal("a deletion claim for a file that still exists was accepted")
	}
	if !strings.Contains(err.Error(), "the file still exists") {
		t.Errorf("error %q does not give the still-exists reason", err)
	}
	if !strings.Contains(err.Error(), "filesDeleted") {
		t.Errorf("error %q does not name the field that made the claim", err)
	}
}

// A path absent now, and absent from a posture check that genuinely
// observed the tree, was never there to delete.
func TestANeverExistedDeletionClaimIsRejected(t *testing.T) {
	root := t.TempDir()
	// No changed path at all: the tree was observed (not nil), and it saw
	// nothing — a non-nil empty set, not "unknown".
	tree := &changingTree{digests: []string{"same"}}

	err := runImplementationWithPayload(t, root, deletionOnlyPayload, tree)

	if err == nil {
		t.Fatal("a claim to have deleted a path that never existed was accepted")
	}
	if !strings.Contains(err.Error(), "no such file was there to delete") {
		t.Errorf("error %q does not give the never-existed reason", err)
	}
}

// A filesDeleted path that escapes the project root cannot be resolved, so
// existingPaths treats it the same as "not found on disk" rather than
// "still exists" — consistent with the filesCreated/filesModified direction
// covered by TestAClaimEscapingTheProjectIsRejected. It is still not a path
// the posture check ever saw change, so the claim fails as never-existed
// rather than being silently accepted.
func TestAnEscapingDeletionClaimIsRejected(t *testing.T) {
	root := t.TempDir()
	tree := &changingTree{digests: []string{"same"}}
	payload := strings.Replace(deletionOnlyPayload,
		`"internal/site/old-name.go"`, `"../../../etc/passwd"`, 1)

	err := runImplementationWithPayload(t, root, payload, tree)

	if err == nil {
		t.Fatal("a deletion claim naming a path outside the project was accepted")
	}
	if !strings.Contains(err.Error(), "no such file was there to delete") {
		t.Errorf("error %q does not give the never-existed reason", err)
	}
}

// A rename reported correctly — the new path in filesCreated, the old path
// in filesDeleted — is the correct encoding and must pass both halves.
func TestACorrectlyEncodedRenamePasses(t *testing.T) {
	root := t.TempDir()
	tree := &changingTree{digests: []string{"same"}, changed: []string{"internal/site/old-name.go"}}

	err := runImplementationWithPayload(t, root, renamePayload, tree)

	if err != nil {
		t.Fatalf("a correctly-encoded rename was rejected: %v", err)
	}
}

// "old -> new" written into one field never reaches claim-checking: it
// fails at validation, before the filesystem or the posture check is
// consulted.
func TestARenameArrowInOneFieldFailsBeforeClaimChecking(t *testing.T) {
	root := t.TempDir()

	err := runImplementationWithPayload(t, root, renameArrowPayload, nil)

	if err == nil {
		t.Fatal("an \"old -> new\" entry in one field was accepted")
	}
	if !strings.Contains(err.Error(), "filesModified") {
		t.Errorf("error %q does not name the offending field", err)
	}
	if !strings.Contains(err.Error(), "filesCreated") || !strings.Contains(err.Error(), "filesDeleted") {
		t.Errorf("error %q does not describe the two-field rename encoding", err)
	}
}

// Naming the same path in both filesCreated and filesDeleted is an
// incoherent claim: filesCreated requires the path to exist and filesDeleted
// requires it to be absent, so each field is checked independently and the
// claim can never satisfy both at once (analysis.md's own edge case).
func TestTheSamePathInBothFilesCreatedAndFilesDeletedIsRejected(t *testing.T) {
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "internal/site/same-name.go"), "package site\n")
	tree := &changingTree{digests: []string{"same"}, changed: []string{"internal/site/same-name.go"}}
	payload := strings.Replace(deletionOnlyPayload,
		`"filesDeleted": ["internal/site/old-name.go"]`,
		`"filesCreated": ["internal/site/same-name.go"], "filesDeleted": ["internal/site/same-name.go"]`, 1)

	err := runImplementationWithPayload(t, root, payload, tree)

	if err == nil {
		t.Fatal("naming the same path in filesCreated and filesDeleted was accepted")
	}
	if !strings.Contains(err.Error(), "the file still exists") {
		t.Errorf("error %q does not give the still-exists reason for the deletion half", err)
	}
}

const deletionOnlyPayload = `{
	"schemaVersion": 2,
	"feature": "golden-site-file-names",
	"filesDeleted": ["internal/site/old-name.go"],
	"selfReview": [{"item": "Every public method has an intention-revealing name", "checked": true}],
	"simpleDesign": [{"item": "Passes the tests", "checked": true}]
}`

const renamePayload = `{
	"schemaVersion": 2,
	"feature": "golden-site-file-names",
	"filesCreated": ["internal/site/new-name.go"],
	"filesDeleted": ["internal/site/old-name.go"],
	"selfReview": [{"item": "Every public method has an intention-revealing name", "checked": true}],
	"simpleDesign": [{"item": "Passes the tests", "checked": true}]
}`

const renameArrowPayload = `{
	"schemaVersion": 2,
	"feature": "golden-site-file-names",
	"filesModified": ["internal/site/old-name.go -> internal/site/new-name.go"],
	"selfReview": [{"item": "Every public method has an intention-revealing name", "checked": true}],
	"simpleDesign": [{"item": "Passes the tests", "checked": true}]
}`

// runImplementationWithPayload runs a one-stage developer plan whose
// provider returns payload, optionally wiring tree so the deletion claim's
// "did the posture check see this change" half has something to read. A
// nil tree leaves posture checking off, which is fine for a payload that
// must fail before claim-checking is ever reached.
func runImplementationWithPayload(t *testing.T, root, payload string, tree orchestrator.WorkTree) error {
	t.Helper()
	workspace := filepath.Join(root, ".claude", "feature-workspace", "golden-site-file-names")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	provider := mock.New(map[string]mock.Script{
		"developer": {Payload: []byte(payload)}})
	plan := orchestrator.Plan{Name: "developer-only", Stages: []orchestrator.Stage{{
		ID: "developer", Agent: "developer", StateKind: string(state.KindImplementation)}}}
	input := orchestrator.StageInput{WorkspaceDir: workspace, ProjectRoot: root,
		SpecPath: filepath.Join(workspace, "spec.md")}

	executor := orchestrator.NewExecutor(provider, store)
	if tree != nil {
		executor.WithWorkTree(tree)
	}
	return executor.Run(context.Background(), plan, input)
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
