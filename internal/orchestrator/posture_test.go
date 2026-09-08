package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// changingTree returns a different digest on each call after the first,
// standing in for a stage that edited source while it ran.
type changingTree struct {
	digests []string
	calls   int
	err     error
}

func (t *changingTree) Digest() (string, error) {
	if t.err != nil {
		return "", t.err
	}
	index := t.calls
	if index >= len(t.digests) {
		index = len(t.digests) - 1
	}
	t.calls++
	return t.digests[index], nil
}

// A stage that declares no tool which writes files, and changes the working
// tree anyway, is recorded (roadmap L3.30).
//
// Run 4's accessibility-engineer declares Read/Glob/Grep/Bash and modified
// handlers.go through Bash. --allowed-tools is an allowlist over named
// tools, not a write barrier, so "read-only stage" was a property nothing
// enforced and nothing checked.
func TestAStageThatWritesWithoutDeclaringItIsRecorded(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst": {ArtifactContent: "# analysis"},
	})
	executor.WithWorkTree(&changingTree{digests: []string{"before", "after"}})

	if err := executor.Run(context.Background(), oneStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	record := mustLoad(t, store).Stages["analyst"]
	if record.PostureViolation == "" {
		t.Error("a stage changed the tree without declaring write access and nothing recorded it")
	}
	if violations := mustLoad(t, store).PostureViolations(); len(violations) != 1 {
		t.Errorf("PostureViolations() = %v, want the one stage", violations)
	}
}

// A stage that leaves the tree alone is never flagged, whatever it declared.
func TestAStageThatChangedNothingIsNotFlagged(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst": {ArtifactContent: "# analysis"},
	})
	executor.WithWorkTree(&changingTree{digests: []string{"same"}})

	if err := executor.Run(context.Background(), oneStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := mustLoad(t, store).Stages["analyst"].PostureViolation; got != "" {
		t.Errorf("a stage that changed nothing was flagged: %q", got)
	}
}

// The check observes a stage; it must never gate one. A tree that cannot be
// fingerprinted disables the check for that stage rather than failing a run.
func TestAnUnavailableTreeCheckDoesNotFailTheRun(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst": {ArtifactContent: "# analysis"},
	})
	reported := 0
	executor.WithWorkTree(&changingTree{err: errors.New("not a git repository")})
	executor.OnPostureError(func(error) { reported++ })

	if err := executor.Run(context.Background(), oneStagePlan(), input); err != nil {
		t.Fatalf("Run failed because the tree could not be fingerprinted: %v", err)
	}

	if reported == 0 {
		t.Error("the failure was swallowed silently")
	}
	if got := mustLoad(t, store).Stages["analyst"].PostureViolation; got != "" {
		t.Errorf("an unavailable check produced a violation: %q", got)
	}
}

func oneStagePlan() orchestrator.Plan {
	return orchestrator.Plan{Name: "one-stage", Stages: []orchestrator.Stage{
		{ID: "analyst", Agent: "analyst"},
	}}
}
