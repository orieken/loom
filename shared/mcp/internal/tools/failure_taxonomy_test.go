package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
)

// Roadmap L2.5, done when: no tool returns a success payload on a failure
// path. Every failure below must come back as IsError with a typed error whose
// Kind says what the caller should do, the same error as a JSON envelope in the
// content, and no "success" key anywhere in it.

type failureCase struct {
	tool  string
	args  map[string]any
	ctx   func() context.Context
	kind  domain.ErrorKind
	field string
}

func background() context.Context { return context.Background() }

func expiredContext() context.Context {
	// A deadline already past ends the context with DeadlineExceeded at once,
	// and a cancel after that does not change its error.
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	cancel()
	return ctx
}

// failingRetriever is a retriever whose store is broken.
type failingRetriever struct{}

func (failingRetriever) Retrieve(context.Context, string, []string, string) ([]Reference, error) {
	return nil, errors.New("corpus store unreadable")
}

func (failingRetriever) EnsureIndex(context.Context, []string) error { return nil }

func taxonomyTools(t *testing.T, root WorkspaceRoot) map[string]domain.Tool {
	t.Helper()
	logger := SilentLogger()
	tools := rootedTools(t, root)
	tools["search_ki"] = NewSearchKITool(logger, NewKICorpusRetriever([]string{root.Dir()}))
	tools["search_ki unconfigured"] = NewSearchKITool(logger, nil)
	tools["search_ki broken"] = NewSearchKITool(logger, failingRetriever{})
	tools["search_docs unconfigured"] = NewSearchDocsTool(logger, nil, nil, root)
	tools["search_docs broken"] = NewSearchDocsTool(logger, failingRetriever{}, failingRetriever{}, root)
	tools["validate_artifact"] = NewValidateArtifactTool(logger, analyzers.NewArtifactContractAnalyzer(), "", root)
	return tools
}

// pathFailureCases derives, for every path argument of every rooted tool, the
// three ways a path fails, so a tool added to rootedTools is covered here too.
func pathFailureCases() []failureCase {
	var cases []failureCase
	for tool, args := range validArguments {
		for _, key := range pathKeys(args) {
			cases = append(cases,
				failureCase{tool, withArgument(args, key, "../outside"), background, domain.ErrorPermission, key},
				failureCase{tool, withArgument(args, key, "no-such-path"), background, domain.ErrorNotFound, key},
				failureCase{tool, withArgument(args, key, "  "), background, domain.ErrorValidation, key},
			)
		}
	}
	return cases
}

var failureCases = []failureCase{
	{"analyze_complexity", map[string]any{}, background, domain.ErrorValidation, "projectPath"},
	{"verify_dependencies", map[string]any{}, background, domain.ErrorValidation, "projectPath"},
	{"check_accessibility", map[string]any{}, background, domain.ErrorValidation, "filePath"},
	{"check_accessibility", map[string]any{"filePath": "../outside"}, background, domain.ErrorPermission, "filePath"},
	{"check_ubiquitous_language", map[string]any{"dictionaryPath": "DOMAIN_DICTIONARY.md"}, background, domain.ErrorValidation, "projectPath"},
	{"check_ubiquitous_language", map[string]any{"projectPath": "."}, background, domain.ErrorValidation, "dictionaryPath"},
	{"search_docs", map[string]any{}, background, domain.ErrorValidation, "query"},
	{"search_ki", map[string]any{}, background, domain.ErrorValidation, "query"},
	{"validate_artifact", map[string]any{}, background, domain.ErrorValidation, "artifactPath"},
	{"validate_artifact", map[string]any{"artifactPath": "main.go"}, background, domain.ErrorValidation, "contractPath"},
	{"validate_artifact", map[string]any{"artifactPath": "../outside"}, background, domain.ErrorPermission, "artifactPath"},
	{"validate_artifact", map[string]any{"artifactPath": "main.go", "contractPath": "../outside"}, background, domain.ErrorPermission, "contractPath"},
	{"search_docs unconfigured", map[string]any{"query": "guide"}, background, domain.ErrorNotFound, ""},
	{"search_ki unconfigured", map[string]any{"query": "guide"}, background, domain.ErrorNotFound, ""},
	{"search_docs broken", map[string]any{"query": "guide", "docsPath": "docs"}, background, domain.ErrorInternal, ""},
	{"search_ki broken", map[string]any{"query": "guide"}, background, domain.ErrorInternal, ""},
	{"analyze_complexity", map[string]any{"projectPath": "."}, cancelledContext, domain.ErrorCancelled, ""},
	{"analyze_complexity", map[string]any{"projectPath": "."}, expiredContext, domain.ErrorTransient, ""},
	{"search_docs", map[string]any{"query": "guide", "docsPath": "docs"}, cancelledContext, domain.ErrorCancelled, ""},
	{"search_ki", map[string]any{"query": "guide"}, expiredContext, domain.ErrorTransient, ""},
}

func TestEveryFailurePathReturnsATypedErrorAndNeverASuccessPayload(t *testing.T) {
	tools := taxonomyTools(t, rootedProject(t))
	for _, tc := range append(pathFailureCases(), failureCases...) {
		t.Run(fmt.Sprintf("%s %v %s", tc.tool, tc.args, tc.kind), func(t *testing.T) {
			tool, ok := tools[tc.tool]
			if !ok {
				t.Fatalf("no tool %q in the fixture", tc.tool)
			}
			result, err := tool.Execute(tc.ctx(), BuildRequest(tc.args))
			if err != nil {
				t.Fatalf("Execute returned a transport error: %v", err)
			}
			assertTypedFailure(t, result, tc.kind, tc.field)
		})
	}
}

func assertTypedFailure(t *testing.T, result *domain.ToolResult, kind domain.ErrorKind, field string) {
	t.Helper()
	text := ExtractText(t, result)
	if !result.IsError || result.Error == nil {
		t.Fatalf("not a typed failure: IsError=%v Error=%v %s", result.IsError, result.Error, text)
	}
	if result.Error.Kind != kind || result.Error.Field != field {
		t.Errorf("kind=%q field=%q, want kind=%q field=%q (%s)", result.Error.Kind, result.Error.Field, kind, field, result.Error.Message)
	}
	if result.Error.Retryable != (kind == domain.ErrorTransient) {
		t.Errorf("retryable=%v for kind %q", result.Error.Retryable, kind)
	}
	assertEnvelope(t, text, *result.Error)
}

// assertEnvelope checks the content an MCP client sees: the same typed error,
// and nothing a client could read as success.
func assertEnvelope(t *testing.T, text string, want domain.ToolError) {
	t.Helper()
	var envelope struct {
		Error domain.ToolError `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("content is not an error envelope: %v: %s", err, text)
	}
	if envelope.Error.Kind != want.Kind || envelope.Error.Message != want.Message || envelope.Error.Field != want.Field {
		t.Errorf("envelope %+v disagrees with the typed error %+v", envelope.Error, want)
	}
	if strings.Contains(text, `"success"`) {
		t.Errorf("a failure carries a success field: %s", text)
	}
}

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		err  error
		want domain.ErrorKind
	}{
		{fmt.Errorf("walk: %w", context.Canceled), domain.ErrorCancelled},
		{fmt.Errorf("walk: %w", context.DeadlineExceeded), domain.ErrorTransient},
		{fmt.Errorf("path %w", ErrOutsideWorkspace), domain.ErrorPermission},
		{fmt.Errorf("open: %w", fs.ErrPermission), domain.ErrorPermission},
		{ErrEmptyPath, domain.ErrorValidation},
		{fmt.Errorf("%w (limits)", analyzers.ErrWalkTooLarge), domain.ErrorValidation},
		{fmt.Errorf("no contract — %w", errNeedsContractPath), domain.ErrorValidation},
		{fmt.Errorf("path %w", ErrPathNotFound), domain.ErrorNotFound},
		{fmt.Errorf("read: %w", fs.ErrNotExist), domain.ErrorNotFound},
		{errors.New("unexpected"), domain.ErrorInternal},
	}
	for _, tc := range cases {
		if got := classifyFailure(tc.err); got != tc.want {
			t.Errorf("classifyFailure(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
