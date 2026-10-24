package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

func TestTheMockDeliversASolutionThroughTheStagesThatWriteCode(t *testing.T) {
	solution := t.TempDir()
	writeTestFile(t, filepath.Join(solution, "calc", "divide.go"), "package calc\n")
	writeTestFile(t, filepath.Join(solution, "calc", "divide_test.go"), "package calc\n")
	plan := orchestrator.Plan{Stages: []orchestrator.Stage{
		{ID: "developer", StateKind: string(state.KindImplementation)},
		{ID: "qa-engineer", StateKind: string(state.KindQA)},
	}}
	provider, err := selectProvider(plan, "mock", mockOptions{solution: solution})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	root := t.TempDir()
	for _, stage := range plan.Stages {
		if _, err := provider.Invoke(context.Background(), stage, orchestrator.StageInput{ProjectRoot: root, WorkspaceDir: t.TempDir()}); err != nil {
			t.Fatalf("invoke %s: %v", stage.ID, err)
		}
	}
	for _, path := range []string{"calc/divide.go", "calc/divide_test.go"} {
		if content, err := os.ReadFile(filepath.Join(root, path)); err != nil || string(content) != "package calc\n" {
			t.Errorf("%s = %q, %v; want the solution's file", path, content, err)
		}
	}
}

func TestAMissingSolutionDirectoryIsReported(t *testing.T) {
	plan := orchestrator.Plan{Stages: []orchestrator.Stage{{ID: "developer"}}}
	if _, err := selectProvider(plan, "mock", mockOptions{solution: filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Error("a missing --mock-solution directory was accepted")
	}
}
