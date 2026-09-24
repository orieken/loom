package server

import (
	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// Handler owns the tool registry and orchestrates MCP registration.
// Constructed via New; tools are wired in tool_provider.go.
type Handler struct {
	logger   *logging.Logger
	registry *domain.Registry
	// session traces tool calls (roadmap L3.8). A nil session is the normal
	// case and is safe to use: the server runs untraced unless an OTLP
	// endpoint is configured.
	session *telemetry.Session
	// resilience is the retry and breaker policy every tool call runs under
	// (roadmap L2.6).
	resilience resiliencePolicy
}

// New constructs a Handler wired with all framework M1 tools.
func New(logger *logging.Logger) *Handler {
	return NewAt(logger, WorkingDirectoryRoot(logger))
}

// NewAt constructs a Handler whose tools read only under root (roadmap L2.3).
func NewAt(logger *logging.Logger, root tools.WorkspaceRoot) *Handler {
	return &Handler{
		logger:     logger,
		registry:   buildFrameworkRegistry(logger, root),
		resilience: defaultResiliencePolicy,
	}
}

// WithTracing attaches a telemetry session so tool calls are traced. Passing
// nil leaves the handler untraced, which is the default.
func (h *Handler) WithTracing(session *telemetry.Session) {
	h.session = session
}

// Registry returns the handler's tool registry.
func (h *Handler) Registry() *domain.Registry {
	return h.registry
}
