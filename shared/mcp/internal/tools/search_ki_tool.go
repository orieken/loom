package tools

import (
	"context"
	"encoding/json"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// SearchKITool surfaces Knowledge Items and ADRs from the framework corpus.
type SearchKITool struct {
	logger    *logging.Logger
	retriever Retriever
}

// NewSearchKITool wires the tool with its retriever.
func NewSearchKITool(logger *logging.Logger, retriever Retriever) *SearchKITool {
	return &SearchKITool{logger: logger, retriever: retriever}
}

func (t *SearchKITool) Name() string { return "search_ki" }

func (t *SearchKITool) Description() string {
	return "Search the framework's Knowledge Items and ADRs by query, tags, and domain. Returns lexically-ranked references (paths + summaries, never content copies) for the calling LLM to filter semantically."
}

func (t *SearchKITool) InputSchema() json.RawMessage {
	return objectSchema([]string{"query"}, map[string]any{
		"query": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Free-text query. Whitespace-split into tokens; matches against KI/ADR titles and summaries.",
		},
		"tags": map[string]any{
			"type":        "array",
			"description": "Optional tag filter. Exact tag matches boost relevance strongly.",
			"items":       map[string]any{"type": "string"},
		},
		"domain": map[string]any{
			"type":        "string",
			"description": "Optional domain / bounded-context filter. Exact domain match boosts relevance.",
		},
	})
}

func (t *SearchKITool) OutputSchema() json.RawMessage {
	return reflectSchema(&KISearchResult{})
}

func (t *SearchKITool) Execute(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling search_ki request")

	query := request.StringArg("query")
	if query == "" {
		return missingArgument("query", "query is required"), nil
	}

	tags := extractStringArray(request.Args["tags"])
	boundedContext := request.StringArg("domain")

	if t.retriever == nil {
		return unavailable("no KI retriever is configured on this server"), nil
	}

	refs, err := t.retriever.Retrieve(ctx, query, tags, boundedContext)
	if err != nil {
		t.logger.Error("KI retrieval failed", "error", err)
		return operationFailure("retrieval", err), nil
	}

	result := KISearchResult{
		Success:   true,
		Query:     query,
		TotalHits: len(refs),
		Matches:   convertReferencesToMatches(refs),
	}
	return marshalToolResult(t.logger, result, "search_ki")
}

func extractStringArray(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func convertReferencesToMatches(refs []Reference) []KIMatch {
	matches := make([]KIMatch, 0, len(refs))
	for _, r := range refs {
		matches = append(matches, KIMatch{
			Title:     r.Title,
			Path:      r.Path,
			Summary:   r.Summary,
			Tags:      r.Tags,
			Relevance: r.Relevance,
		})
	}
	return matches
}

func marshalToolResult(logger *logging.Logger, v any, toolName string) (*domain.ToolResult, error) {
	resultJSON, err := marshalJSON(v)
	if err != nil {
		logger.Error("Failed to marshal result", "tool", toolName, "error", err)
		return operationFailure("formatting the result", err), nil
	}
	return domain.NewTextResult(string(resultJSON)), nil
}

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (tools.SafeArguments, guardrail #9). `query` is absent deliberately: it is free text a caller composed, and
// guardrail #9 keeps it off the span as a hash and a length.
func (t *SearchKITool) SafeArgumentNames() []string {
	return []string{"tags", "domain"}
}
