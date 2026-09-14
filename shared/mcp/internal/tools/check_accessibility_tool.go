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
}

// NewCheckAccessibilityTool wires the tool with its dependencies.
func NewCheckAccessibilityTool(logger *logging.Logger, analyzer *analyzers.AccessibilityAnalyzer) *CheckAccessibilityTool {
	return &CheckAccessibilityTool{logger: logger, analyzer: analyzer}
}

func (t *CheckAccessibilityTool) Name() string { return "check_accessibility" }

func (t *CheckAccessibilityTool) Description() string {
	return "Scan UI template files (HTML, Vue, JSX, TSX, Svelte) for semantic-HTML and ARIA accessibility violations"
}

func (t *CheckAccessibilityTool) InputSchema() json.RawMessage {
	return objectSchema(nil, map[string]any{
		"filePath": map[string]any{
			"type":        "string",
			"description": "Absolute path to a single UI template file to scan",
		},
		"projectPath": map[string]any{
			"type":        "string",
			"description": "Absolute path to a project root; the walker scans every .html/.htm/.vue/.jsx/.tsx/.svelte file underneath",
		},
	})
}

func (t *CheckAccessibilityTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.AccessibilityReportResult{})
}

func (t *CheckAccessibilityTool) Execute(_ context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling check_accessibility request")

	target := resolveAccessibilityTarget(request)
	if target == "" {
		return domain.NewErrorResult("either filePath or projectPath is required"), nil
	}

	result, err := t.analyzer.Analyze(target)
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
