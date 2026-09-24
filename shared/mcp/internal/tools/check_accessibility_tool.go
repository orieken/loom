package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// CheckAccessibilityTool exposes the AccessibilityAnalyzer as an MCP tool.
type CheckAccessibilityTool struct {
	logger   *logging.Logger
	analyzer *analyzers.AccessibilityAnalyzer
	root     WorkspaceRoot
}

// NewCheckAccessibilityTool wires the tool with its dependencies.
func NewCheckAccessibilityTool(logger *logging.Logger, analyzer *analyzers.AccessibilityAnalyzer, root WorkspaceRoot) *CheckAccessibilityTool {
	return &CheckAccessibilityTool{logger: logger, analyzer: analyzer, root: root}
}

func (t *CheckAccessibilityTool) Name() string { return "check_accessibility" }

func (t *CheckAccessibilityTool) Description() string {
	return "Scan UI template files (HTML, Vue, JSX, TSX, Svelte) for semantic-HTML and ARIA accessibility violations"
}

// InputSchema requires one of filePath or projectPath — declared here, so the
// server's validation (L2.1) reports it, rather than only Execute's check.
func (t *CheckAccessibilityTool) InputSchema() json.RawMessage {
	return eitherOfSchema([]string{"filePath", "projectPath"}, map[string]any{
		"filePath": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Path to a single UI template file to scan" + pathNote,
		},
		"projectPath": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Path to a project root; the walker scans every .html/.htm/.vue/.jsx/.tsx/.svelte file underneath" + pathNote,
		},
	})
}

func (t *CheckAccessibilityTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.AccessibilityReportResult{})
}

func (t *CheckAccessibilityTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling check_accessibility request")

	target := resolveAccessibilityTarget(request)
	if target == "" {
		return domain.NewErrorResult("either filePath or projectPath is required"), nil
	}
	target, err := t.root.Resolve(target)
	if err != nil {
		return domain.NewErrorResult(err.Error()), nil
	}

	result, err := t.analyzer.Analyze(ctx, target)
	if err != nil {
		t.logger.Error("Accessibility analysis failed", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Accessibility analysis failed: %v", err)), nil
	}

	body, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal accessibility result", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Failed to format result: %v", err)), nil
	}

	t.logger.Info("Accessibility analysis completed", "path", target, "violations", result.ViolationsCount)
	return domain.NewTextResult(string(body)), nil
}

func resolveAccessibilityTarget(request domain.ToolRequest) string {
	if filePath := request.StringArg("filePath"); filePath != "" {
		return filePath
	}
	return request.StringArg("projectPath")
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *CheckAccessibilityTool) SafeArgumentNames() []string {
	return []string{"filePath", "projectPath"}
}
