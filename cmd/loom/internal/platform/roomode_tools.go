package platform

import (
	"sort"
	"strings"
)

func displayName(slug string) string {
	words := strings.Fields(strings.ReplaceAll(slug, "-", " "))
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func inferGroups(rawTools string) []string {
	if strings.TrimSpace(rawTools) == "*" {
		return []string{"read", "edit", "command", "mcp", "browser"}
	}
	groups := make(map[string]bool)
	for _, tool := range strings.Split(rawTools, ",") {
		addToolGroup(groups, strings.TrimSpace(tool))
	}
	return sortedGroups(groups)
}

func addToolGroup(groups map[string]bool, tool string) {
	toolGroups := map[string]string{
		"Read": "read", "Glob": "read", "Grep": "read", "WebFetch": "read", "WebSearch": "read",
		"Write": "edit", "Edit": "edit", "MultiEdit": "edit", "NotebookEdit": "edit", "Bash": "command",
		"Agent": "mcp", "Artifact": "mcp", "TaskCreate": "mcp", "TaskUpdate": "mcp", "SendMessage": "mcp",
	}
	if group := toolGroups[tool]; group != "" {
		groups[group] = true
	}
	// An MCP tool is mcp__<server>__<tool>; Roo files all of them under "mcp".
	if strings.HasPrefix(tool, "mcp__") {
		groups["mcp"] = true
	}
}

func sortedGroups(groups map[string]bool) []string {
	if len(groups) == 0 {
		return []string{"read"}
	}
	order := map[string]int{"read": 0, "edit": 1, "command": 2, "mcp": 3, "browser": 4}
	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}
	sort.Slice(result, func(left, right int) bool { return order[result[left]] < order[result[right]] })
	return result
}
