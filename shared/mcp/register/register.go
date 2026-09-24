// Package register exposes the framework's MCP tools for embedding in external
// MCP servers.
//
// The supported embedding path is Frameworks, which returns loom's built-in
// tool registrations as a transport-free tools.Registry — merge it into your
// own registry (tools.Registry.Merge) and adapt it to your server's MCP
// library and version. See examples/embedding/ for a complete server.
package register

import (
	"io"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	mcpserver "github.com/orieken/loom/shared/mcp/internal/server"
	internaltools "github.com/orieken/loom/shared/mcp/internal/tools"
	"github.com/orieken/loom/tools"
)

// Frameworks returns the framework's built-in tool registrations:
//
//   - search_ki           — BM25 full-text search over Knowledge Items
//   - check_ubiquitous_language — validates domain term usage
//   - analyze_complexity  — cyclomatic complexity analysis
//   - check_accessibility — accessibility rule checks
//   - verify_dependencies — dependency boundary validation
//   - search_docs         — documentation search
//   - validate_artifact   — structural contract validation of pipeline artifacts
//
// Each registration carries the tool plus its timeout budget, retry class,
// and permission scope. Nothing in the returned types references mcp-go or
// loom internals, so callers adapt them to their own MCP library version.
//
// logWriter receives the tools' structured JSON log output; pass nil to use
// os.Stderr.
func Frameworks(logWriter io.Writer) *tools.Registry {
	return mcpserver.FrameworkRegistry(logging.NewLogger(resolveLogWriter(logWriter)))
}

// FrameworksAt is Frameworks with every tool's path arguments confined to
// rootDir (roadmap L2.3): a path that resolves outside it, including through
// a symbolic link, is rejected. Frameworks confines to the working directory.
func FrameworksAt(logWriter io.Writer, rootDir string) (*tools.Registry, error) {
	root, err := internaltools.NewWorkspaceRoot(rootDir)
	if err != nil {
		return nil, err
	}
	return mcpserver.FrameworkRegistryAt(logging.NewLogger(resolveLogWriter(logWriter)), root), nil
}

// FrameworkTools registers all framework tools on s, an mcp-go server pinned
// to loom's own mcp-go version.
//
// Deprecated: use Frameworks and adapt the registry to your server's own MCP
// library (see examples/embedding/), or run the standalone `loom mcp serve`.
// FrameworkTools couples callers to loom's mcp-go version pin and will be
// removed after the D.2 embedding API's first tagged release.
func FrameworkTools(s *server.MCPServer, logWriter io.Writer) error {
	return FrameworkToolsTraced(s, logWriter, nil)
}

// FrameworkToolsTraced is FrameworkTools with a telemetry session attached,
// so each tool call becomes a span (roadmap L3.8). A nil session traces
// nothing, which is what FrameworkTools passes.
func FrameworkToolsTraced(s *server.MCPServer, logWriter io.Writer, session *telemetry.Session) error {
	handler := mcpserver.New(logging.NewLogger(resolveLogWriter(logWriter)))
	handler.WithTracing(session)
	return handler.RegisterTools(s)
}

// FrameworkToolsTracedAt is FrameworkToolsTraced with every path argument
// confined to rootDir (roadmap L2.3). `loom mcp serve --root` uses it.
func FrameworkToolsTracedAt(s *server.MCPServer, logWriter io.Writer, session *telemetry.Session, rootDir string) error {
	root, err := internaltools.NewWorkspaceRoot(rootDir)
	if err != nil {
		return err
	}
	handler := mcpserver.NewAt(logging.NewLogger(resolveLogWriter(logWriter)), root)
	handler.WithTracing(session)
	return handler.RegisterTools(s)
}

func resolveLogWriter(logWriter io.Writer) io.Writer {
	if logWriter == nil {
		return os.Stderr
	}
	return logWriter
}
