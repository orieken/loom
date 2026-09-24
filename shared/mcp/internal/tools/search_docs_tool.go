// Package tools - search_docs_tool.go
//
// SearchDocsTool exposes the BM25Retriever (see bm25_retriever.go) as
// the MCP tool an agent calls to search the installed project's docs corpus.
package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// DocIndexer refreshes a docs index for the given corpus roots.
type DocIndexer interface {
	EnsureIndex(ctx context.Context, corpusPaths []string) error
}

const defaultDocsPath = "docs/"

// SearchDocsTool implements domain.Tool for BM25 docs-corpus search.
type SearchDocsTool struct {
	logger    *logging.Logger
	retriever Retriever
	indexer   DocIndexer
	root      WorkspaceRoot
}

// NewSearchDocsTool wires the tool with its retriever and indexer.
func NewSearchDocsTool(logger *logging.Logger, retriever Retriever, indexer DocIndexer, root WorkspaceRoot) *SearchDocsTool {
	return &SearchDocsTool{logger: logger, retriever: retriever, indexer: indexer, root: root}
}

func (t *SearchDocsTool) Name() string { return "search_docs" }

func (t *SearchDocsTool) Description() string {
	return "Search the installed project's markdown docs (docs/features/, docs/adrs/, docs/patterns/, docs/runbooks/) via BM25. Returns lexically-ranked references (paths + snippets, never content copies) for the calling LLM to filter semantically."
}

func (t *SearchDocsTool) InputSchema() json.RawMessage {
	return objectSchema([]string{"query"}, map[string]any{
		"query": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Free-text query. Whitespace-split into tokens; each token is matched via fts5 against doc titles (10x weight) and bodies (1x weight).",
		},
		"docsPath": map[string]any{
			"type":        "string",
			"description": "Corpus root to index and search. Defaults to \"docs/\"" + pathNote,
		},
	})
}

func (t *SearchDocsTool) OutputSchema() json.RawMessage {
	return reflectSchema(&DocSearchResult{})
}

func (t *SearchDocsTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling search_docs request")

	query := request.StringArg("query")
	if query == "" {
		return missingArgument("query", "query is required"), nil
	}

	// Resolved first: a path outside the workspace is refused whether or not
	// a retriever is configured, rather than slipping through as "no results".
	docsPath, err := t.requestedDocsPath(request)
	if err != nil {
		return pathFailure("docsPath", err), nil
	}
	if t.retriever == nil || t.indexer == nil {
		return unavailable("no docs retriever is configured on this server"), nil
	}

	if err := t.indexer.EnsureIndex(ctx, []string{docsPath}); err != nil {
		t.logger.Error("Docs index refresh failed", "error", err, "docsPath", docsPath)
		return operationFailure("index refresh", err), nil
	}

	refs, err := t.retriever.Retrieve(ctx, query, nil, "")
	if err != nil {
		t.logger.Error("Docs retrieval failed", "error", err)
		return operationFailure("retrieval", err), nil
	}

	result := DocSearchResult{
		Success:   true,
		Query:     query,
		TotalHits: len(refs),
		Matches:   convertReferencesToDocMatches(refs),
	}
	return marshalToolResult(t.logger, result, "search_docs")
}

func convertReferencesToDocMatches(refs []Reference) []DocMatch {
	matches := make([]DocMatch, 0, len(refs))
	for _, r := range refs {
		matches = append(matches, DocMatch{
			Title:     r.Title,
			Path:      r.Path,
			Summary:   r.Summary,
			Relevance: r.Relevance,
		})
	}
	return matches
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9). `query` is absent deliberately: it is free text a caller composed, and
// guardrail #9 keeps it off the span as a hash and a length.
func (t *SearchDocsTool) SafeArgumentNames() []string {
	return []string{"docsPath"}
}

// requestedDocsPath is the docsPath argument, or the default docs directory,
// resolved inside the workspace root.
func (t *SearchDocsTool) requestedDocsPath(request domain.ToolRequest) (string, error) {
	docsPath := request.StringArg("docsPath")
	if docsPath == "" {
		docsPath = defaultDocsPath
	}
	return t.root.Resolve(docsPath)
}
