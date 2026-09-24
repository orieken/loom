package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// CheckUbiquitousLanguageTool exposes UbiquitousLanguageAnalyzer as an MCP tool.
type CheckUbiquitousLanguageTool struct {
	logger   *logging.Logger
	analyzer *analyzers.UbiquitousLanguageAnalyzer
	root     WorkspaceRoot
}

// NewCheckUbiquitousLanguageTool wires the tool with its dependencies.
func NewCheckUbiquitousLanguageTool(logger *logging.Logger, analyzer *analyzers.UbiquitousLanguageAnalyzer, root WorkspaceRoot) *CheckUbiquitousLanguageTool {
	return &CheckUbiquitousLanguageTool{logger: logger, analyzer: analyzer, root: root}
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
			"minLength":   1,
			"description": "Path to the DOMAIN_DICTIONARY.md that defines canonical terms and their forbidden synonyms" + pathNote,
		},
	})
}

func (t *CheckUbiquitousLanguageTool) OutputSchema() json.RawMessage {
	return reflectSchema(&analyzers.UbiquitousLanguageResult{})
}

func (t *CheckUbiquitousLanguageTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling check_ubiquitous_language request")

	projectPath := request.StringArg("projectPath")
	dictionaryPath := request.StringArg("dictionaryPath")

	if failure := requireBoth(projectPath, dictionaryPath); failure != nil {
		return failure, nil
	}
	projectPath, dictionaryPath, failure := t.resolvePaths(projectPath, dictionaryPath)
	if failure != nil {
		return failure, nil
	}

	result, err := t.analyzer.Analyze(ctx, projectPath, dictionaryPath)
	if err != nil {
		t.logger.Error("Ubiquitous language analysis failed", "error", err)
		return operationFailure("ubiquitous language analysis", err), nil
	}

	body, err := json.Marshal(result)
	if err != nil {
		t.logger.Error("Failed to marshal ubiquitous language result", "error", err)
		return operationFailure("formatting the result", err), nil
	}

	t.logger.Info("Ubiquitous language analysis completed", "path", projectPath, "violations", result.ViolationsCount)
	return domain.NewTextResult(string(body)), nil
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9).
func (t *CheckUbiquitousLanguageTool) SafeArgumentNames() []string {
	return []string{"projectPath", "dictionaryPath"}
}

// requireBoth names the first missing argument, so the caller knows which
// one to supply.
func requireBoth(projectPath, dictionaryPath string) *domain.ToolResult {
	const message = "both projectPath and dictionaryPath are required"
	if projectPath == "" {
		return missingArgument("projectPath", message)
	}
	if dictionaryPath == "" {
		return missingArgument("dictionaryPath", message)
	}
	return nil
}

// resolvePaths resolves both paths, or returns the failure of the first that
// does not resolve, against that argument.
func (t *CheckUbiquitousLanguageTool) resolvePaths(projectPath, dictionaryPath string) (string, string, *domain.ToolResult) {
	project, err := t.root.Resolve(projectPath)
	if err != nil {
		return "", "", pathFailure("projectPath", err)
	}
	dictionary, err := t.root.Resolve(dictionaryPath)
	if err != nil {
		return "", "", pathFailure("dictionaryPath", err)
	}
	return project, dictionary, nil
}
