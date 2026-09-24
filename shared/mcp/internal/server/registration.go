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

// RegisterTools registers every registry entry with the MCP server. Every
// tool's input schema is compiled first (L2.1): one that does not compile
// stops registration, rather than surfacing on the first call.
func (h *Handler) RegisterTools(s *server.MCPServer) error {
	h.logger.Info("Registering tools")

	registrations := h.registry.All()
	validators := make([]*argumentValidator, 0, len(registrations))
	for _, registration := range registrations {
		validator, err := compileArgumentValidator(registration.Tool)
		if err != nil {
			return err
		}
		validators = append(validators, validator)
	}
	for index, registration := range registrations {
		s.AddTool(mcpToolDefinition(registration.Tool), h.validatedToolHandler(registration, validators[index]))
	}

	h.logger.Info("Tools registered successfully")
	return nil
}
