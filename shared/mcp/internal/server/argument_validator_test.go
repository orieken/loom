package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// mainArgument is, for every framework tool, the argument a malformed call
// gets wrong, and a complete valid call.
var mainArgument = map[string]struct {
	field string
	valid map[string]any
}{
	"analyze_complexity":        {"projectPath", map[string]any{"projectPath": "."}},
	"check_accessibility":       {"projectPath", map[string]any{"projectPath": "."}},
	"check_ubiquitous_language": {"dictionaryPath", map[string]any{"projectPath": ".", "dictionaryPath": "D.md"}},
	"verify_dependencies":       {"projectPath", map[string]any{"projectPath": "."}},
	"search_docs":               {"query", map[string]any{"query": "anything"}},
	"search_ki":                 {"query", map[string]any{"query": "anything"}},
	"validate_artifact":         {"artifactPath", map[string]any{"artifactPath": "analysis.md"}},
}

// callTool runs one call through the same handler the MCP server registers.
func callTool(t *testing.T, name string, args map[string]any) (*mcp.CallToolResult, argumentReport) {
	t.Helper()
	handler := New(logging.NewLogger(&bytes.Buffer{}))
	for _, registration := range handler.registry.All() {
		if registration.Tool.Name() != name {
			continue
		}
		request := mcp.CallToolRequest{}
		request.Params.Name = name
		request.Params.Arguments = args
		result, err := handler.mcpToolHandler(registration)(context.Background(), request)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return result, reportOf(result)
	}
	t.Fatalf("tool %q is not registered", name)
	return nil, argumentReport{}
}

func reportOf(result *mcp.CallToolResult) argumentReport {
	var report argumentReport
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			_ = json.Unmarshal([]byte(text.Text), &report)
		}
	}
	return report
}

func assertViolation(t *testing.T, result *mcp.CallToolResult, report argumentReport, field, problemFragment string) {
	t.Helper()
	if !result.IsError || report.Error != "invalid arguments" {
		t.Fatalf("want an invalid-arguments error, got %+v", report)
	}
	for _, violation := range report.Violations {
		if violation.Field == field && strings.Contains(violation.Problem, problemFragment) {
			return
		}
	}
	t.Errorf("no violation on %q containing %q in %+v", field, problemFragment, report.Violations)
}

// The L2.1 done-when, for every framework tool: a malformed call returns a
// field-level validation error before the tool runs.
func TestAMalformedCallNamesTheFieldForEveryTool(t *testing.T) {
	for name, spec := range mainArgument {
		t.Run(name+" wrong type", func(t *testing.T) {
			args := copyArgs(spec.valid)
			args[spec.field] = 42
			result, report := callTool(t, name, args)
			assertViolation(t, result, report, spec.field, "string")
		})
		t.Run(name+" unknown argument", func(t *testing.T) {
			args := copyArgs(spec.valid)
			args["projectPth"] = "."
			result, report := callTool(t, name, args)
			assertViolation(t, result, report, "projectPth", "not an argument")
		})
		t.Run(name+" passes validation when well-formed", func(t *testing.T) {
			_, report := callTool(t, name, spec.valid)
			if report.Error == "invalid arguments" {
				t.Errorf("a well-formed call was rejected: %+v", report.Violations)
			}
		})
	}
}

func TestAMissingRequiredArgumentIsReportedAgainstThatArgument(t *testing.T) {
	for name, spec := range mainArgument {
		if name == "check_accessibility" {
			continue // one of two is required; covered below
		}
		t.Run(name, func(t *testing.T) {
			args := copyArgs(spec.valid)
			delete(args, spec.field)
			result, report := callTool(t, name, args)
			assertViolation(t, result, report, spec.field, "required")
		})
	}
}

func TestTheTightenedConstraintsAreEnforced(t *testing.T) {
	result, report := callTool(t, "analyze_complexity", map[string]any{"projectPath": ".", "maxComplexity": 0})
	assertViolation(t, result, report, "maxComplexity", "minimum")

	result, report = callTool(t, "analyze_complexity", map[string]any{"projectPath": ".", "maxLines": 2.5})
	assertViolation(t, result, report, "maxLines", "integer")

	result, report = callTool(t, "check_accessibility", map[string]any{})
	assertViolation(t, result, report, "filePath", "required")
	assertViolation(t, result, report, "projectPath", "required")
	for _, violation := range report.Violations {
		if strings.Contains(violation.Problem, "anyOf") {
			t.Errorf("the combinator itself was reported: %+v", violation)
		}
	}

	result, report = callTool(t, "search_ki", map[string]any{"query": "x", "tags": []any{"ok", 7}})
	assertViolation(t, result, report, "tags/1", "string")

	result, report = callTool(t, "search_docs", map[string]any{"query": ""})
	assertViolation(t, result, report, "query", "minLength")
}

// A call with no arguments object at all is validated as an empty one.
func TestACallWithNoArgumentsIsValidatedAsEmpty(t *testing.T) {
	result, report := callTool(t, "verify_dependencies", nil)
	assertViolation(t, result, report, "projectPath", "required")
}

func TestEveryFrameworkSchemaCompiles(t *testing.T) {
	for _, registration := range FrameworkRegistry(logging.NewLogger(&bytes.Buffer{})).All() {
		if _, err := compileArgumentValidator(registration.Tool); err != nil {
			t.Errorf("%s: %v", registration.Tool.Name(), err)
		}
	}
	if len(mainArgument) != len(pathArguments)+len(pathlessTools) {
		t.Errorf("mainArgument covers %d tools, the registry has %d", len(mainArgument), len(pathArguments)+len(pathlessTools))
	}
}

// A schema that does not compile stops registration rather than surfacing on
// the first call; so does a tool that declares none.
func TestRegistrationRefusesAToolWhoseSchemaCannotBeCompiled(t *testing.T) {
	for name, schema := range map[string]json.RawMessage{
		"broken":   json.RawMessage(`{"type": 12}`),
		"not json": json.RawMessage(`{`),
		"missing":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			tool := &stubTool{name: "bad_" + name, inputSchema: schema}
			if name == "missing" {
				tool.inputSchema = json.RawMessage(" ")
			}
			registry := domain.NewRegistry()
			if err := registry.Register(domain.ToolRegistration{Tool: tool}); err != nil {
				t.Fatalf("Register: %v", err)
			}
			handler := NewAt(logging.NewLogger(&bytes.Buffer{}), tools.WorkspaceRoot{})
			handler.registry = registry
			if err := handler.RegisterTools(server.NewMCPServer("t", "0")); err == nil {
				t.Error("registration accepted a tool whose schema cannot be compiled")
			}
			if _, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: tool})(context.Background(), mcp.CallToolRequest{}); err == nil {
				t.Error("a handler for an uncompilable schema ran the call")
			}
		})
	}
}

func copyArgs(args map[string]any) map[string]any {
	copied := make(map[string]any, len(args))
	for key, value := range args {
		copied[key] = value
	}
	return copied
}
