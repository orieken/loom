package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// contractByArtifact maps a pipeline artifact's basename to its contract file
// in shared/contracts/ — the same mapping the validate-artifact skill
// documents. The three frontmatter contracts are deliberately absent: they
// have no "## Required Sections" list for this structural tool to check.
var contractByArtifact = map[string]string{
	"context-manifest.md":       "context-manifest-contract.md",
	"analysis.md":               "analysis-contract.md",
	"architecture-notes.md":     "architecture-contract.md",
	"performance-report.md":     "performance-contract.md",
	"data-engineering-notes.md": "data-engineering-contract.md",
	"implementation-notes.md":   "implementation-contract.md",
	"code-review-report.md":     "review-contract.md",
	"accessibility-report.md":   "accessibility-contract.md",
	"security-report.md":        "security-contract.md",
	"qa-report.md":              "qa-contract.md",
	"visual-qa-report.md":       "visual-qa-report-contract.md",
	"observability-report.md":   "observability-contract.md",
	"docs-report.md":            "docs-contract.md",
	"devops-report.md":          "devops-contract.md",
	"refactoring-notes.md":      "refactoring-contract.md",
}

// ValidateArtifactTool exposes ArtifactContractAnalyzer as the read-only
// validate_artifact MCP tool (roadmap D.5). Structural checks only — the
// contract-specific prose content rules stay with the validate-artifact
// skill until L2.11 lands typed semantic validation.
type ValidateArtifactTool struct {
	logger       *logging.Logger
	analyzer     *analyzers.ArtifactContractAnalyzer
	contractsDir string
	root         WorkspaceRoot
}

// NewValidateArtifactTool wires the tool. contractsDir points at the
// framework's shared/contracts/ directory and may be empty when the install
// root is unknown — callers must then pass contractPath explicitly.
func NewValidateArtifactTool(logger *logging.Logger, analyzer *analyzers.ArtifactContractAnalyzer, contractsDir string, root WorkspaceRoot) *ValidateArtifactTool {
	return &ValidateArtifactTool{logger: logger, analyzer: analyzer, contractsDir: contractsDir, root: root}
}

func (t *ValidateArtifactTool) Name() string { return "validate_artifact" }

func (t *ValidateArtifactTool) Description() string {
	return "Validate a pipeline artifact against its inter-agent contract in shared/contracts/: required heading presence (exact text and level) plus WARN-level retrieval frontmatter checks. Structural only — returns typed violations, never judges content quality"
}

func (t *ValidateArtifactTool) InputSchema() json.RawMessage {
	return objectSchema([]string{"artifactPath"}, map[string]any{
		"artifactPath": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Path to the pipeline artifact markdown file (e.g. analysis.md)" + pathNote,
		},
		"contractPath": map[string]any{
			"type":        "string",
			"description": "Path to the contract markdown file (inside the workspace or the framework's contracts directory); omit to infer it from the artifact filename against the framework's shared/contracts/ directory",
		},
	})
}

func (t *ValidateArtifactTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.ArtifactValidationResult{})
}

func (t *ValidateArtifactTool) Execute(_ context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling validate_artifact request")

	artifactPath := request.StringArg("artifactPath")
	if artifactPath == "" {
		return domain.NewErrorResult("artifactPath is required"), nil
	}
	artifactPath, err := t.root.Resolve(artifactPath)
	if err != nil {
		return domain.NewErrorResult(err.Error()), nil
	}
	contractPath, err := t.resolveContractPath(artifactPath, request.StringArg("contractPath"))
	if err != nil {
		return domain.NewErrorResult(err.Error()), nil
	}
	return t.validate(artifactPath, contractPath)
}

func (t *ValidateArtifactTool) validate(artifactPath, contractPath string) (*domain.ToolResult, error) {
	result, err := t.analyzer.Validate(artifactPath, contractPath)
	if err != nil {
		t.logger.Error("Artifact validation failed", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Artifact validation failed: %v", err)), nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal artifact validation result", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Failed to format result: %v", err)), nil
	}
	t.logger.Info("Artifact validation completed", "artifact", artifactPath, "status", result.Status, "violations", len(result.Violations))
	return domain.NewTextResult(string(body)), nil
}

// resolveContractPath prefers an explicit contractPath, then falls back to
// the filename mapping rooted at the framework's contracts directory.
func (t *ValidateArtifactTool) resolveContractPath(artifactPath, explicit string) (string, error) {
	if explicit != "" {
		return t.resolveExplicitContract(explicit)
	}
	contractFile, ok := contractByArtifact[filepath.Base(artifactPath)]
	if !ok {
		return "", fmt.Errorf("no known contract for artifact %q — pass contractPath explicitly", filepath.Base(artifactPath))
	}
	if t.contractsDir == "" {
		return "", fmt.Errorf("framework contracts directory is unknown (set AI_ASSISTANT_DOTFILES_PATH) — pass contractPath explicitly")
	}
	return filepath.Join(t.contractsDir, contractFile), nil
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *ValidateArtifactTool) SafeArgumentNames() []string {
	return []string{"artifactPath", "contractPath"}
}

// resolveExplicitContract accepts a contract inside the workspace, or inside
// the framework's own contracts directory, and nothing else: a contract is
// read and echoed back as headings, so it is a path argument like any other.
func (t *ValidateArtifactTool) resolveExplicitContract(explicit string) (string, error) {
	inWorkspace, err := t.root.Resolve(explicit)
	if err == nil || t.contractsDir == "" {
		return inWorkspace, err
	}
	contracts, contractsErr := NewWorkspaceRoot(t.contractsDir)
	if contractsErr != nil {
		return "", err
	}
	if inContracts, contractsErr := contracts.Resolve(explicit); contractsErr == nil {
		return inContracts, nil
	}
	return "", err
}
