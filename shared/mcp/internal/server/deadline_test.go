package server

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// waitingTool blocks until its context is done and records whether that
// context carried a deadline.
type waitingTool struct {
	hadDeadline bool
}

func (w *waitingTool) Name() string                  { return "waiting" }
func (w *waitingTool) Description() string           { return "waits for its context" }
func (w *waitingTool) InputSchema() json.RawMessage  { return json.RawMessage(`{"type":"object"}`) }
func (w *waitingTool) OutputSchema() json.RawMessage { return nil }
func (w *waitingTool) Execute(ctx context.Context, _ domain.ToolRequest) (*domain.ToolResult, error) {
	_, w.hadDeadline = ctx.Deadline()
	<-ctx.Done()
	return domain.NewErrorResult(ctx.Err().Error()), nil
}

// The registry has carried a Timeout per tool since L2.4; L2.2 is the first
// time anything enforced it.
func TestAToolCallIsBoundedByItsRegisteredTimeout(t *testing.T) {
	tool := &waitingTool{}
	handler := New(logging.NewLogger(&bytes.Buffer{}))
	started := time.Now()
	_, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: tool, Timeout: 50 * time.Millisecond})(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("a 50ms budget ran %v", elapsed)
	}
	if !tool.hadDeadline {
		t.Error("the tool's context carried no deadline")
	}
}

func TestAToolWithNoTimeoutRunsUnderTheCallersContextAlone(t *testing.T) {
	tool := &waitingTool{}
	handler := New(logging.NewLogger(&bytes.Buffer{}))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	if _, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: tool})(ctx, mcp.CallToolRequest{}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if tool.hadDeadline {
		t.Error("a registration with no timeout was given a deadline")
	}
}

// Every framework tool declares a budget, so none runs unbounded.
func TestEveryFrameworkToolDeclaresATimeout(t *testing.T) {
	for _, registration := range FrameworkRegistry(logging.NewLogger(&bytes.Buffer{})).All() {
		if registration.Timeout <= 0 {
			t.Errorf("tool %q has no timeout", registration.Tool.Name())
		}
	}
}

func TestRegisterToolsAddsEveryRegistrationToTheServer(t *testing.T) {
	mcpServer := server.NewMCPServer("t", "0", server.WithToolCapabilities(true))
	if err := New(logging.NewLogger(&bytes.Buffer{})).RegisterTools(mcpServer); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}
	if count := len(mcpServer.ListTools()); count != len(pathArguments)+len(pathlessTools) {
		t.Errorf("server has %d tools", count)
	}
}
