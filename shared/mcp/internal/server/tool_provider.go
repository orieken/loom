package server

import (
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// analysisTimeout is the declared budget for tools that walk a project tree.
// Enforcement middleware arrives with roadmap L2.2; the registry carries the
// budget so that item has a source of truth to apply.
const analysisTimeout = 60 * time.Second

// searchTimeout is the declared budget for corpus-search tools.
const searchTimeout = 15 * time.Second

// FrameworkRegistry returns the framework's built-in tool registrations as a
// transport-free registry. It backs the public embedding API
// (register.Frameworks, roadmap D.2).
func FrameworkRegistry(logger *logging.Logger) *domain.Registry {
	return buildFrameworkRegistry(logger, WorkingDirectoryRoot(logger))
}

// FrameworkRegistryAt is FrameworkRegistry with every path argument confined
// to root (roadmap L2.3).
func FrameworkRegistryAt(logger *logging.Logger, root tools.WorkspaceRoot) *domain.Registry {
	return buildFrameworkRegistry(logger, root)
}

// WorkingDirectoryRoot is the default workspace root: the directory the
// server was started in. If it cannot be resolved, the zero root is returned,
// which rejects every path — a server that cannot say where its workspace is
// must not read anywhere.
func WorkingDirectoryRoot(logger *logging.Logger) tools.WorkspaceRoot {
	cwd, err := os.Getwd()
	if err == nil {
		root, rootErr := tools.NewWorkspaceRoot(cwd)
		if rootErr == nil {
			return root
		}
		err = rootErr
	}
	logger.Error("Workspace root unavailable; every path argument will be rejected", "error", err)
	return tools.WorkspaceRoot{}
}

// buildFrameworkRegistry constructs and registers the framework-generic tools.
// Adding a capability is one registerFrameworkTool call here — no edits to
// handler.go or registration.go (roadmap L2.4).
func buildFrameworkRegistry(logger *logging.Logger, root tools.WorkspaceRoot) *domain.Registry {
	registry := domain.NewRegistry()
	for _, entry := range frameworkRegistrations(logger, root) {
		registerFrameworkTool(logger, registry, entry)
	}
	return registry
}

func frameworkRegistrations(logger *logging.Logger, root tools.WorkspaceRoot) []domain.ToolRegistration {
	return []domain.ToolRegistration{
		readOnlyTool(tools.NewAnalyzeComplexityTool(logger, analyzers.NewComplexityAnalyzer(), root), analysisTimeout),
		readOnlyTool(tools.NewCheckAccessibilityTool(logger, analyzers.NewAccessibilityAnalyzer(), root), analysisTimeout),
		readOnlyTool(tools.NewCheckUbiquitousLanguageTool(logger, analyzers.NewUbiquitousLanguageAnalyzer(), root), analysisTimeout),
		readOnlyTool(tools.NewVerifyDependenciesTool(logger, analyzers.NewDependencyBoundaryAnalyzer(), root), analysisTimeout),
		readOnlyTool(tools.NewSearchKITool(logger, tools.NewKICorpusRetriever(frameworkCorpusPaths())), searchTimeout),
		readOnlyTool(newSearchDocsTool(logger, root), searchTimeout),
		readOnlyTool(tools.NewValidateArtifactTool(logger, analyzers.NewArtifactContractAnalyzer(), frameworkContractsDir(), root), searchTimeout),
	}
}

// readOnlyTool wraps a framework tool with the metadata every current tool
// shares: read-only permission and idempotent retry.
func readOnlyTool(tool domain.Tool, timeout time.Duration) domain.ToolRegistration {
	return domain.ToolRegistration{
		Tool:       tool,
		Timeout:    timeout,
		Retry:      domain.RetryIdempotent,
		Permission: domain.ScopeReadOnly,
	}
}

// registerFrameworkTool registers entry, logging instead of failing: a
// registration error here is a programmer error (duplicate name in the list
// above), and the server should still come up with the remaining tools.
func registerFrameworkTool(logger *logging.Logger, registry *domain.Registry, entry domain.ToolRegistration) {
	if err := registry.Register(entry); err != nil {
		logger.Error("Skipping tool registration", "error", err)
	}
}

func newSearchDocsTool(logger *logging.Logger, root tools.WorkspaceRoot) domain.Tool {
	dbPath, ok := docsFTSDBPath()
	if !ok {
		logger.Info("BM25 retriever not initialised — search_docs will return no-corpus diagnostic responses")
		return tools.NewSearchDocsTool(logger, nil, nil, root)
	}
	retriever, err := tools.NewBM25Retriever(dbPath)
	if err != nil {
		logger.Warn("BM25 retriever init failed — search_docs will return no-corpus diagnostic responses", "dbPath", dbPath, "error", err)
		return tools.NewSearchDocsTool(logger, nil, nil, root)
	}
	return tools.NewSearchDocsTool(logger, retriever, retriever, root)
}

func docsFTSDBPath() (string, bool) {
	if override := os.Getenv("DOCS_FTS_PATH"); override != "" {
		return override, true
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(cwd, ".claude")); err != nil {
		return "", false
	}
	return filepath.Join(cwd, ".claude", "rag", "docs-fts5.sqlite"), true
}

// frameworkContractsDir resolves shared/contracts/ from the install root, or
// "" when the root is unknown — validate_artifact then requires an explicit
// contractPath argument.
func frameworkContractsDir() string {
	root := os.Getenv("AI_ASSISTANT_DOTFILES_PATH")
	if root == "" {
		return ""
	}
	return filepath.Join(root, "shared", "contracts")
}

func frameworkCorpusPaths() []string {
	root := os.Getenv("AI_ASSISTANT_DOTFILES_PATH")
	if root == "" {
		return nil
	}
	return []string{
		filepath.Join(root, "shared", "knowledge"),
		filepath.Join(root, "docs", "adrs"),
	}
}
