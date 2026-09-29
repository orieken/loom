// Package tools - corpus_search_tool.go
//
// CorpusSearchTool exposes a CorpusIndex as an MCP tool: search_docs over the
// docs corpus (BM25), and search_features over the feature archive (BM25 and,
// when an embedder is configured, vectors — roadmap L3.4).
package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// corpusSearchSpec is what distinguishes one corpus search tool from another.
type corpusSearchSpec struct {
	name, description, queryDescription string
	// pathArgument names the argument choosing the corpus root, defaulting
	// to defaultPath.
	pathArgument, defaultPath string
	// unconfigured is the not_found message when the server has no index.
	unconfigured string
}

var searchDocsSpec = corpusSearchSpec{
	name:             "search_docs",
	description:      "Search the installed project's markdown docs (docs/features/, docs/adrs/, docs/patterns/, docs/runbooks/) via BM25. Returns lexically-ranked references (paths + snippets, never content copies) for the calling LLM to filter semantically.",
	queryDescription: "Free-text query. Whitespace-split into tokens; each token is matched via fts5 against doc titles (10x weight) and bodies (1x weight).",
	pathArgument:     "docsPath",
	defaultPath:      "docs/",
	unconfigured:     "no docs retriever is configured on this server",
}

var searchFeaturesSpec = corpusSearchSpec{
	name:             "search_features",
	description:      "Search the feature archive (docs/features/) for prior deliveries like the one described — \"have we built something like this before?\". Ranks by BM25 and, when the server has an embedding provider, by meaning too, fused by reciprocal rank: a query can find a delivery that shares none of its words. Returns references (paths + excerpts, never content copies).",
	queryDescription: "Describe the feature in your own words; with embeddings configured, paraphrase is fine.",
	pathArgument:     "featuresPath",
	defaultPath:      "docs/features/",
	unconfigured:     "no feature archive index is configured on this server",
}

// CorpusSearchTool implements domain.Tool for search over one corpus.
type CorpusSearchTool struct {
	spec   corpusSearchSpec
	logger *logging.Logger
	index  CorpusIndex
	root   WorkspaceRoot
}

// NewSearchDocsTool searches the docs corpus. A nil index is a server
// configured without one: the tool then fails as not_found.
func NewSearchDocsTool(logger *logging.Logger, index CorpusIndex, root WorkspaceRoot) *CorpusSearchTool {
	return &CorpusSearchTool{spec: searchDocsSpec, logger: logger, index: index, root: root}
}

// NewSearchFeaturesTool searches the feature archive.
func NewSearchFeaturesTool(logger *logging.Logger, index CorpusIndex, root WorkspaceRoot) *CorpusSearchTool {
	return &CorpusSearchTool{spec: searchFeaturesSpec, logger: logger, index: index, root: root}
}

func (t *CorpusSearchTool) Name() string { return t.spec.name }

func (t *CorpusSearchTool) Description() string { return t.spec.description }

func (t *CorpusSearchTool) InputSchema() json.RawMessage {
	return objectSchema([]string{"query"}, map[string]any{
		"query": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": t.spec.queryDescription,
		},
		t.spec.pathArgument: map[string]any{
			"type":        "string",
			"description": "Corpus root to index and search. Defaults to \"" + t.spec.defaultPath + "\"" + pathNote,
		},
	})
}

func (t *CorpusSearchTool) OutputSchema() json.RawMessage {
	return reflectSchema(&DocSearchResult{})
}

func (t *CorpusSearchTool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling corpus search request", "tool", t.spec.name)

	query := request.StringArg("query")
	if query == "" {
		return missingArgument("query", "query is required"), nil
	}
	// Resolved first: a path outside the workspace is refused whether or not
	// an index is configured, rather than slipping through as "no results".
	corpusRoot, err := t.requestedRoot(request)
	if err != nil {
		return pathFailure(t.spec.pathArgument, err), nil
	}
	if t.index == nil {
		return unavailable(t.spec.unconfigured), nil
	}
	return t.search(ctx, corpusRoot, query)
}

func (t *CorpusSearchTool) search(ctx context.Context, corpusRoot, query string) (*domain.ToolResult, error) {
	if err := t.index.EnsureIndex(ctx, []string{corpusRoot}); err != nil {
		t.logger.Error("Corpus index refresh failed", "tool", t.spec.name, "error", err)
		return operationFailure("index refresh", err), nil
	}
	refs, err := t.index.SearchWithin(ctx, corpusRoot, query)
	if err != nil {
		t.logger.Error("Corpus retrieval failed", "tool", t.spec.name, "error", err)
		return operationFailure("retrieval", err), nil
	}
	result := DocSearchResult{
		Success:   true,
		Query:     query,
		TotalHits: len(refs),
		Matches:   convertReferencesToDocMatches(refs),
	}
	return marshalToolResult(t.logger, result, t.spec.name)
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
// telemetry (tools.SafeArguments, guardrail #9). `query` is absent
// deliberately: it is free text a caller composed, and guardrail #9 keeps it
// off the span as a hash and a length.
func (t *CorpusSearchTool) SafeArgumentNames() []string {
	return []string{t.spec.pathArgument}
}

// requestedRoot is the path argument, or the corpus's default, resolved
// inside the workspace root.
func (t *CorpusSearchTool) requestedRoot(request domain.ToolRequest) (string, error) {
	path := request.StringArg(t.spec.pathArgument)
	if path == "" {
		path = t.spec.defaultPath
	}
	return t.root.Resolve(path)
}
