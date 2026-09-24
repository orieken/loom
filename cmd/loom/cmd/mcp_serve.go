package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/register"
	"github.com/spf13/cobra"
)

type mcpServeFlags struct {
	logFile string
	root    string
}

var mcpServeArgs mcpServeFlags

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the framework MCP tools over stdio",
	Long: `Serve the framework MCP tools over stdio transport.

Structured JSON logs go to stderr (or --log-file) — never stdout,
which carries the MCP wire protocol. The server blocks until the
client closes stdin or the process receives SIGINT.

Tool calls are traced (roadmap L3.8) when OTEL_EXPORTER_OTLP_ENDPOINT is
set; no trace file is written. An inherited TRACEPARENT is adopted when
present, so a tool call made during a "loom run" stage lands under it —
best-effort, since loom does not spawn this process.

Every tool's path arguments are confined to --root (default: the directory
the server starts in). A path that resolves outside it — "/", "../../etc",
or a symbolic link pointing out — is rejected, and walks never follow links.`,
	Args: cobra.NoArgs,
	RunE: runMCPServe,
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
	mcpServeCmd.Flags().StringVar(&mcpServeArgs.logFile, "log-file", "", "append structured JSON logs to this file instead of stderr")
	mcpServeCmd.Flags().StringVar(&mcpServeArgs.root, "root", ".", "workspace root every tool path argument is confined to")
}

func runMCPServe(_ *cobra.Command, _ []string) error {
	logWriter, closeLog, err := openMCPLogWriter(mcpServeArgs.logFile)
	if err != nil {
		return err
	}
	defer closeLog()
	return serveMCP(logWriter, mcpServeArgs.root)
}

func serveMCP(logWriter io.Writer, root string) error {
	session, err := startMCPTelemetry()
	if err != nil {
		return err
	}
	defer shutdownMCPTelemetry(session)
	mcpServer := server.NewMCPServer("loom", version, server.WithToolCapabilities(true))
	// `loom mcp serve` IS the deprecation notice's recommended replacement, so
	// it legitimately keeps using the compat wrapper until D.2's embedding API
	// grows an in-binary adapter.
	if err := register.FrameworkToolsTracedAt(mcpServer, logWriter, session, root); err != nil {
		return fmt.Errorf("register framework tools: %w", err)
	}
	if err := server.ServeStdio(mcpServer); err != nil {
		return fmt.Errorf("mcp serve: %w", err)
	}
	return nil
}

// startMCPTelemetry traces tool calls only when an OTLP endpoint is
// configured. Unlike `loom run`, this server writes no trace file by
// default: it is spawned by a host application and may outlive many runs,
// so there is no run to scope a file to, and an unbounded jsonl in an
// unpredictable place is worse than none.
func startMCPTelemetry() (*telemetry.Session, error) {
	return telemetry.Start(telemetry.Options{Version: version})
}

func shutdownMCPTelemetry(session *telemetry.Session) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	_ = session.Shutdown(ctx)
}

func openMCPLogWriter(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stderr, func() {}, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}
