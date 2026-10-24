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
// mockOptions are the mock provider's test-only knobs.
type mockOptions struct {
	hangStage string
	stageCost float64
	// solution is a directory the mock's developer and QA stages deliver.
	solution string
	code     map[string][]byte
	tests    map[string][]byte
}

func selectProvider(plan orchestrator.Plan, name string, options mockOptions) (orchestrator.Provider, error) {
	switch name {
	case "claude":
		return newClaudeProvider()
	case "mock":
		if err := options.loadSolution(); err != nil {
			return nil, err
		}
		return newMockProvider(plan, options), nil
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
//
// A stage cost makes every stage report spending it, so a budget can be
// exercised end to end. Without one the mock reports nothing, which is the
// honest default: it consumed nothing from a model.
func newMockProvider(plan orchestrator.Plan, options mockOptions) orchestrator.Provider {
	scripts := make(map[string]mock.Script, len(plan.Stages))
	for _, stage := range plan.Stages {
		script := options.scriptFor(stage)
		if options.stageCost > 0 {
			script.Usage = &orchestrator.Usage{Model: "mock", CostUSD: options.stageCost}
		}
		scripts[stage.ID] = script
	}
	if options.hangStage != "" {
		scripts[options.hangStage] = mock.Script{Hang: true}
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
