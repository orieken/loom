package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/claude"
	"github.com/orieken/loom/internal/provider/mock"
)

// selectProvider builds the stage provider named by --provider. The claude
// provider fails each stage loudly when the binary is absent — there is no
// silent fallback to mock.
func selectProvider(plan orchestrator.Plan, name, mockHangStage string) (orchestrator.Provider, error) {
	switch name {
	case "claude":
		return newClaudeProvider()
	case "mock":
		return newMockProvider(plan, mockHangStage), nil
	default:
		return nil, fmt.Errorf("unknown provider %q — use claude or mock", name)
	}
}

func newClaudeProvider() (orchestrator.Provider, error) {
	agentsDir, err := findAgentsDir()
	if err != nil {
		return nil, err
	}
	return claude.New(claude.Config{AgentsDir: agentsDir}), nil
}

// findAgentsDir locates the agent definitions in the current project:
// shared/agents/ in a framework checkout, .claude/agents/ in an installed
// project (a symlink to the same content).
func findAgentsDir() (string, error) { return findAgentsDirIn(".") }

// findAgentsDirIn is findAgentsDir for a project root other than the cwd.
func findAgentsDirIn(root string) (string, error) {
	for _, candidate := range []string{filepath.Join(root, "shared", "agents"), filepath.Join(root, ".claude", "agents")} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("no agent definitions found (looked for shared/agents/ and .claude/agents/) — run `loom install` first")
}

// newMockProvider scripts a deterministic run: every stage writes a canned
// artifact; the optional hang stage blocks until interrupted, so integration
// tests can exercise SIGINT checkpointing through the real CLI.
func newMockProvider(plan orchestrator.Plan, hangStage string) orchestrator.Provider {
	scripts := make(map[string]mock.Script, len(plan.Stages))
	for _, stage := range plan.Stages {
		scripts[stage.ID] = scriptFor(stage)
	}
	if hangStage != "" {
		scripts[hangStage] = mock.Script{Hang: true}
	}
	return mock.New(scripts)
}

// scriptFor gives typed stages a valid state document and every other stage
// a canned markdown artifact.
func scriptFor(stage orchestrator.Stage) mock.Script {
	if script, typed := mock.TypedScript(stage.StateKind); typed {
		return script
	}
	return mock.Script{ArtifactContent: fmt.Sprintf("# mock artifact for %s\n", stage.ID)}
}
