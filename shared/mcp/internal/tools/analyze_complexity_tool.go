package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// AnalyzeComplexityTool analyzes code files for cyclomatic complexity and LOC function length.
type AnalyzeComplexityTool struct {
	logger   *logging.Logger
	analyzer *analyzers.ComplexityAnalyzer
	root     WorkspaceRoot
}

// NewAnalyzeComplexityTool wires the tool with its dependencies.
func NewAnalyzeComplexityTool(logger *logging.Logger, analyzer *analyzers.ComplexityAnalyzer, root WorkspaceRoot) *AnalyzeComplexityTool {
	return &AnalyzeComplexityTool{logger: logger, analyzer: analyzer, root: root}
}

func (t *AnalyzeComplexityTool) Name() string { return "analyze_complexity" }

func (t *AnalyzeComplexityTool) Description() string {
	return "Analyze cyclomatic complexity and function length against framework thresholds (complexity < 7, LOC < 30)"
}

func (t *AnalyzeComplexityTool) InputSchema() json.RawMessage {
	return objectSchema([]string{"projectPath"}, map[string]any{
		"projectPath": projectPathProperty(),
		"maxComplexity": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Maximum allowed cyclomatic complexity (default 7)",
		},
		"maxLines": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Maximum allowed lines of code per function (default 30)",
		},
	})
}

func (t *AnalyzeComplexityTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.ComplexityAnalysisResult{})
}

func (t *AnalyzeComplexityTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling analyze_complexity request")
	projectPath, maxComplexity, maxLines := parseComplexityArgs(request.Args)
	if projectPath == "" {
		return missingArgument("projectPath", "projectPath is required"), nil
	}
	projectPath, err := t.root.Resolve(projectPath)
	if err != nil {
		return pathFailure("projectPath", err), nil
	}
	result, err := t.analyzer.Analyze(ctx, projectPath, maxComplexity, maxLines)
	if err != nil {
		t.logger.Error("Complexity analysis failed", "error", err)
		return operationFailure("complexity analysis", err), nil
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal complexity result", "error", err)
		return operationFailure("formatting the result", err), nil
	}
	t.logger.Info("Complexity analysis completed", "path", projectPath, "violations", result.ViolationsCount)
	return domain.NewTextResult(string(resultJSON)), nil
}

func parseComplexityArgs(args map[string]any) (projectPath string, maxComplexity, maxLines int) {
	projectPath, _ = args["projectPath"].(string)
	maxComplexity = 7
	if mc, ok := args["maxComplexity"].(float64); ok && mc > 0 {
		maxComplexity = int(mc)
	}
	maxLines = 30
	if ml, ok := args["maxLines"].(float64); ok && ml > 0 {
		maxLines = int(ml)
	}
	return
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *AnalyzeComplexityTool) SafeArgumentNames() []string {
	return []string{"projectPath", "maxComplexity", "maxLines"}
}
