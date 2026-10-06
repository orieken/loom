package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/orieken/loom/cmd/loom/internal/levels"
	"github.com/orieken/loom/cmd/loom/internal/mcpprobe"
)

const mcpProbeTimeout = 5 * time.Second

// healthLevelProbe gathers on-disk and live-server evidence for the level
// assessment. Level 2+ bundles install at fixed .claude/ destinations; the
// platform-dependent level 1 layout is resolved through the install manifest,
// so a non-claude-only install may under-report level 1 evidence — a known
// limitation, and the honest direction to fail in.
type healthLevelProbe struct {
	target        string
	manifestPaths []string
}

func (probe healthLevelProbe) pathBundleInstalled(bundle levels.Bundle) (bool, string) {
	var missing []string
	for _, source := range bundle.Paths {
		if !probe.sourceInstalled(source) {
			missing = append(missing, path.Base(source))
		}
	}
	if len(missing) > 0 {
		return false, fmt.Sprintf("bundle %q not fully installed (missing %s)", bundle.ID, strings.Join(missing, ", "))
	}
	return true, fmt.Sprintf("bundle %q installed", bundle.ID)
}

func (probe healthLevelProbe) sourceInstalled(source string) bool {
	if pathExistsIn(probe.target, levels.InstallDestination(source)) {
		return true
	}
	return probe.recordedAndPresent(path.Base(source))
}

func (probe healthLevelProbe) recordedAndPresent(base string) bool {
	for _, recorded := range probe.manifestPaths {
		if pathHasSegment(recorded, base) && pathExistsIn(probe.target, recorded) {
			return true
		}
	}
	return false
}

func pathHasSegment(recorded, base string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(recorded), "/") {
		if segment == base {
			return true
		}
	}
	return false
}

func pathExistsIn(target, relative string) bool {
	_, err := os.Stat(filepath.Join(target, relative))
	return err == nil
}

func (probe healthLevelProbe) mcpServerAnswering() (bool, string) {
	server, err := loomMCPServerEntry(filepath.Join(probe.target, ".mcp.json"))
	if err != nil {
		return false, "MCP server not configured: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), mcpProbeTimeout)
	defer cancel()
	tools, err := mcpprobe.ToolsList(ctx, probe.target, server.Command, server.Args...)
	if err != nil {
		return false, "MCP server configured but not answering tools/list: " + err.Error()
	}
	return true, fmt.Sprintf("MCP server answering tools/list (%d tools)", len(tools))
}

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func loomMCPServerEntry(configPath string) (mcpServerEntry, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return mcpServerEntry{}, fmt.Errorf("read .mcp.json: %w", err)
	}
	var config struct {
		Servers map[string]mcpServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return mcpServerEntry{}, fmt.Errorf("parse .mcp.json: %w", err)
	}
	server, found := config.Servers["loom"]
	if !found {
		return mcpServerEntry{}, fmt.Errorf(".mcp.json has no \"loom\" server entry")
	}
	return server, nil
}

func (probe healthLevelProbe) telemetryStreamPresent() (bool, string) {
	stream := filepath.Join(".claude", "telemetry", "events.jsonl")
	info, err := os.Stat(filepath.Join(probe.target, stream))
	if err != nil || info.Size() == 0 {
		return false, "telemetry stream absent or empty (" + stream + ")"
	}
	return true, "telemetry stream present (" + stream + ")"
}

func (probe healthLevelProbe) policiesPresent() (bool, string) {
	pattern := filepath.Join(probe.target, ".claude", "policies", "*.policy.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return false, "no policy files (.claude/policies/*.policy.yaml)"
	}
	return true, fmt.Sprintf("%d policy file(s) in .claude/policies", len(matches))
}

// repoIndexPresent is the evidence that the repo graph has been built: the
// MCP repo tools read it, and without it they answer "run loom index".
func (probe healthLevelProbe) repoIndexPresent() (bool, string) {
	graph := filepath.Join(".loom", "graph.json")
	if !pathExistsIn(probe.target, graph) {
		return false, "repo graph not built (" + graph + "): run `loom index`"
	}
	return true, "repo graph present (" + graph + ")"
}
