package server

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/orieken/loom/shared/mcp/internal/domain"
)

// Tools returns every registered domain.Tool, sorted by name.
func (h *Handler) Tools() []domain.Tool {
	registrations := h.registry.All()
	toolList := make([]domain.Tool, 0, len(registrations))
	for _, registration := range registrations {
		toolList = append(toolList, registration.Tool)
	}
	return toolList
}

// RegisterTools registers every registry entry with the MCP server.
func (h *Handler) RegisterTools(s *server.MCPServer) error {
	h.logger.Info("Registering tools")

	for _, registration := range h.registry.All() {
		s.AddTool(mcpToolDefinition(registration.Tool), h.mcpToolHandler(registration))
	}

	h.logger.Info("Tools registered successfully")
	return nil
}
