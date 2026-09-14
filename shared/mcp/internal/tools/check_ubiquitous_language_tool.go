package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// CheckUbiquitousLanguageTool exposes UbiquitousLanguageAnalyzer as an MCP tool.
type CheckUbiquitousLanguageTool struct {
	logger   *logging.Logger
	analyzer *analyzers.UbiquitousLanguageAnalyzer
}

// NewCheckUbiquitousLanguageTool wires the tool with its dependencies.
func NewCheckUbiquitousLanguageTool(logger *logging.Logger, analyzer *analyzers.UbiquitousLanguageAnalyzer) *CheckUbiquitousLanguageTool {
	return &CheckUbiquitousLanguageTool{logger: logger, analyzer: analyzer}
}

func (t *CheckUbiquitousLanguageTool) Name() string { return "check_ubiquitous_language" }

func (t *CheckUbiquitousLanguageTool) Description() string {
	return "Scan source files for uses of unapproved synonyms defined in a DOMAIN_DICTIONARY.md, reporting each violation with the canonical replacement"
}

func (t *CheckUbiquitousLanguageTool) InputSchema() json.RawMessage {
	return objectSchema([]string{"projectPath", "dictionaryPath"}, map[string]any{
		"projectPath": projectPathProperty(),
		"dictionaryPath": map[string]any{
			"type":        "string",
			"description": "Absolute path to the DOMAIN_DICTIONARY.md that defines canonical terms and their forbidden synonyms",
		},
	})
}

func (t *CheckUbiquitousLanguageTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.UbiquitousLanguageResult{})
}

func (t *CheckUbiquitousLanguageTool) Execute(_ context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling check_ubiquitous_language request")

	projectPath := request.StringArg("projectPath")
	dictionaryPath := request.StringArg("dictionaryPath")

	if projectPath == "" || dictionaryPath == "" {
		return domain.NewErrorResult("both projectPath and dictionaryPath are required"), nil
	}

	result, err := t.analyzer.Analyze(projectPath, dictionaryPath)
	if err != nil {
		t.logger.Error("Ubiquitous language analysis failed", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Ubiquitous language analysis failed: %v", err)), nil
	}

	body, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal ubiquitous language result", "error", err)
		return domain.NewErrorResult(fmt.Sprintf("Failed to format result: %v", err)), nil
	}

	t.logger.Info("Ubiquitous language analysis completed", "path", projectPath, "violations", result.ViolationsCount)
	return domain.NewTextResult(string(body)), nil
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *CheckUbiquitousLanguageTool) SafeArgumentNames() []string {
	return []string{"projectPath", "dictionaryPath"}
}
