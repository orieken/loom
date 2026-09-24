package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// VerifyDependenciesTool exposes DependencyBoundaryAnalyzer as an MCP tool.
type VerifyDependenciesTool struct {
	logger   *logging.Logger
	analyzer *analyzers.DependencyBoundaryAnalyzer
	root     WorkspaceRoot
}

// NewVerifyDependenciesTool wires the tool with its dependencies.
func NewVerifyDependenciesTool(logger *logging.Logger, analyzer *analyzers.DependencyBoundaryAnalyzer, root WorkspaceRoot) *VerifyDependenciesTool {
	return &VerifyDependenciesTool{logger: logger, analyzer: analyzer, root: root}
}

func (t *VerifyDependenciesTool) Name() string { return "verify_dependencies" }

func (t *VerifyDependenciesTool) Description() string {
	return "Verify Clean Architecture layer boundaries by scanning Go and TypeScript imports, flagging any inner-to-outer layer dependency"
}

func (t *VerifyDependenciesTool) InputSchema() json.RawMessage {
	return projectPathOnlySchema()
}

func (t *VerifyDependenciesTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.DependencyVerificationResult{})
}

func (t *VerifyDependenciesTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling verify_dependencies request")

	projectPath := request.StringArg("projectPath")
	if projectPath == "" {
		return missingArgument("projectPath", "projectPath is required"), nil
	}
	projectPath, err := t.root.Resolve(projectPath)
	if err != nil {
		return pathFailure("projectPath", err), nil
	}

	result, err := t.analyzer.Analyze(ctx, projectPath)
	if err != nil {
		t.logger.Error("Dependency verification failed", "error", err)
		return operationFailure("dependency verification", err), nil
	}

	body, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal dependency verification result", "error", err)
		return operationFailure("formatting the result", err), nil
	}

	t.logger.Info("Dependency verification completed", "path", projectPath, "violations", result.ViolationsCount)
	return domain.NewTextResult(string(body)), nil
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *VerifyDependenciesTool) SafeArgumentNames() []string {
	return []string{"projectPath"}
}
