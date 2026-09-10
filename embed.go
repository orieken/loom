// Package loom exposes the framework content embedded in the loom binary.
package loom

import (
	"embed"

	mcpassets "github.com/orieken/loom/shared/mcp"
)

// FrameworkFS holds all shared framework content baked in at compile time.
// It lives at the module root because go:embed cannot traverse parent directories.
//
//go:embed all:shared/agents all:shared/skills all:shared/rules all:shared/configs all:shared/contracts all:shared/schemas all:shared/workflows all:shared/orchestration all:shared/plans all:shared/telemetry all:shared/hooks all:shared/evaluation shared/levels.yaml shared/ARCHITECTURE_RULES.md shared/DOMAIN_DICTIONARY.md shared/VERSION all:templates/claude-feature-team
var FrameworkFS embed.FS

// MCPFS holds the MCP reference source embedded from shared/mcp.
var MCPFS = mcpassets.SourceFS
