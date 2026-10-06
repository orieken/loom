package repoquery

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/repograph"
)

// maxSuggestions caps the near-miss paths offered with an unknown path.
const maxSuggestions = 5

// UnknownPathError says a path is not in the graph, and offers near misses so
// a caller that guessed can correct itself without another search.
type UnknownPathError struct {
	Path        string
	Suggestions []string
}

func (e *UnknownPathError) Error() string {
	message := fmt.Sprintf("%q is not a directory or source file in the repo graph; use a repo-relative path", e.Path)
	if len(e.Suggestions) > 0 {
		message += ". Did you mean: " + strings.Join(e.Suggestions, ", ")
	}
	return message
}

// nodesFor resolves a path to graph nodes. A directory resolves to its node —
// or its `dir[lang]` nodes, if it holds several languages — and a source file
// resolves to the node that lists it.
func (s Snapshot) nodesFor(raw string) ([]repograph.Node, error) {
	target := cleanPath(raw)
	var found []repograph.Node
	for _, node := range s.Graph.Nodes {
		if nodeMatches(node, target) {
			found = append(found, node)
		}
	}
	if len(found) == 0 {
		return nil, &UnknownPathError{Path: raw, Suggestions: s.suggest(target)}
	}
	return found, nil
}

func cleanPath(raw string) string {
	cleaned := path.Clean(strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/")))
	return strings.TrimPrefix(cleaned, "./")
}

func nodeMatches(node repograph.Node, target string) bool {
	if node.ID == target || strings.HasPrefix(node.ID, target+"[") {
		return true
	}
	for _, file := range node.Files {
		if file == target {
			return true
		}
	}
	return false
}

// suggest returns node ids that contain the path as a substring.
func (s Snapshot) suggest(target string) []string {
	var near []string
	for _, node := range s.Graph.Nodes {
		if target != "" && target != "." && strings.Contains(node.ID, target) {
			near = append(near, node.ID)
		}
	}
	sort.Strings(near)
	if len(near) > maxSuggestions {
		near = near[:maxSuggestions]
	}
	return near
}

// nodeIDs lists the ids of resolved nodes.
func nodeIDs(nodes []repograph.Node) map[string]bool {
	ids := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		ids[node.ID] = true
	}
	return ids
}
