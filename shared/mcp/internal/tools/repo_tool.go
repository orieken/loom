package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/orieken/loom/internal/repoquery"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// The repo tools answer questions about a repository from the context index
// `loom index` wrote (ADR-010, Phase 7). Each result carries the graph's
// revision and whether it still matches the working tree, so a caller knows
// whether it is reading the repository as it is or as it was.

// Defaults and ceilings that keep every answer small: progressive disclosure,
// not a dump of the graph.
const (
	defaultRepoLimit = 50
	maxRepoLimit     = 200
	defaultFindLimit = 20
	maxFindLimit     = 50
)

// staleHint is attached to any answer read from a graph that no longer matches
// the working tree.
const staleHint = "the repo graph is out of date with the working tree; run `loom index` before relying on this"

// RepoEnvelope is the part of every repo-tool result that says how far to
// trust it.
type RepoEnvelope struct {
	Success bool `json:"success"`
	// GraphSHA is the commit the graph was built at.
	GraphSHA string `json:"graphSha"`
	// GraphHash identifies the graph's content; it changes when the graph does.
	GraphHash string `json:"graphHash"`
	// Fresh says the graph matches the working tree now.
	Fresh bool   `json:"fresh"`
	Hint  string `json:"hint,omitempty"`
}

func envelopeFor(snapshot *repoquery.Snapshot) RepoEnvelope {
	envelope := RepoEnvelope{Success: true, GraphSHA: snapshot.GraphSHA(), GraphHash: snapshot.Graph.GraphHash, Fresh: snapshot.Fresh}
	if !snapshot.Fresh {
		envelope.Hint = staleHint
	}
	return envelope
}

// repoHandler produces a tool's result from a snapshot. It returns the result
// value, or an error: a *repoquery.UnknownPathError and plain validation
// errors become typed tool errors.
type repoHandler func(snapshot *repoquery.Snapshot, request domain.ToolRequest, envelope RepoEnvelope) (any, error)

// repoTool is the shared shape of the six tools: a name, a schema, the
// arguments safe to record, and a handler.
type repoTool struct {
	logger      *logging.Logger
	store       *repoquery.Store
	name        string
	description string
	input       json.RawMessage
	output      any
	safe        []string
	handle      repoHandler
}

func (t *repoTool) Name() string                  { return t.name }
func (t *repoTool) Description() string           { return t.description }
func (t *repoTool) InputSchema() json.RawMessage  { return t.input }
func (t *repoTool) OutputSchema() json.RawMessage { return reflectSchema(t.output) }

// SafeArgumentNames declares which arguments may be recorded verbatim in
// telemetry (guardrail #9). Free text is never among them.
func (t *repoTool) SafeArgumentNames() []string { return t.safe }

func (t *repoTool) Execute(_ context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	t.logger.Info("Handling repo request", "tool", t.name)
	if t.store == nil {
		return unavailable("no repository is configured on this server"), nil
	}
	snapshot, err := t.store.Snapshot()
	if errors.Is(err, repoquery.ErrNoIndex) {
		return unavailable(err.Error()), nil
	}
	if err != nil {
		t.logger.Error("Repo snapshot failed", "tool", t.name, "error", err)
		return operationFailure("reading the repo index", err), nil
	}
	result, err := t.handle(&snapshot, request, envelopeFor(&snapshot))
	if err != nil {
		return queryFailure(err), nil
	}
	return marshalToolResult(t.logger, result, t.name)
}

// queryFailure maps a query error to a typed tool error. A path or argument
// the caller got wrong is a validation error, naming the field.
func queryFailure(err error) *domain.ToolResult {
	var unknown *repoquery.UnknownPathError
	if errors.As(err, &unknown) {
		return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorValidation, err.Error()).WithField("path"))
	}
	if errors.Is(err, repoquery.ErrNoIndex) {
		return unavailable(err.Error())
	}
	return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorValidation, err.Error()))
}

// limitArg reads a bounded positive integer argument.
func limitArg(request domain.ToolRequest, name string, fallback, ceiling int) int {
	value, ok := request.Args[name].(float64)
	if !ok || value < 1 {
		return fallback
	}
	if int(value) > ceiling {
		return ceiling
	}
	return int(value)
}

// capped returns at most limit items and whether any were cut.
func capped[T any](items []T, limit int) ([]T, bool) {
	if len(items) <= limit {
		return items, false
	}
	return items[:limit], true
}

func pathProperty(what string) map[string]any {
	return map[string]any{
		"type":        "string",
		"minLength":   1,
		"description": what + ": a repo-relative directory or source file, as in the repo graph (not a filesystem path)",
	}
}

func limitProperty(fallback, ceiling int) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     1,
		"maximum":     ceiling,
		"description": "Most results to return (default " + itoa(fallback) + ")",
	}
}
