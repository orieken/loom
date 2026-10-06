package repoquery

import (
	"sort"

	"github.com/orieken/loom/internal/repograph"
)

// Edge is one relationship, with how it was established. An approximate edge
// must never be read as fact, so the label travels with every one.
type Edge struct {
	Path       string `json:"path"`
	Confidence string `json:"confidence" jsonschema:"enum=exact,enum=approximate"`
}

// DependenciesOf lists what a path imports.
func (s Snapshot) DependenciesOf(raw string) ([]Edge, error) {
	return s.related(raw, true)
}

// DependentsOf lists what imports a path.
func (s Snapshot) DependentsOf(raw string) ([]Edge, error) {
	return s.related(raw, false)
}

// related walks one hop out of the resolved nodes in either direction.
func (s Snapshot) related(raw string, outgoing bool) ([]Edge, error) {
	nodes, err := s.nodesFor(raw)
	if err != nil {
		return nil, err
	}
	self := nodeIDs(nodes)
	best := map[string]string{}
	for _, edge := range s.Graph.Edges {
		from, to := edge.From, edge.To
		if !outgoing {
			from, to = to, from
		}
		if self[from] && !self[to] {
			best[to] = strongest(best[to], string(edge.Confidence))
		}
	}
	return sortedEdges(best), nil
}

// strongest keeps exact over approximate when two edges reach the same node.
func strongest(current, candidate string) string {
	if current == string(repograph.Exact) || candidate == "" {
		return current
	}
	return candidate
}

// sortedEdges orders exact edges first, then by path, so the facts lead and
// the heuristics trail.
func sortedEdges(byPath map[string]string) []Edge {
	edges := make([]Edge, 0, len(byPath))
	for path, confidence := range byPath {
		edges = append(edges, Edge{Path: path, Confidence: confidence})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Confidence != edges[j].Confidence {
			return edges[i].Confidence == string(repograph.Exact)
		}
		return edges[i].Path < edges[j].Path
	})
	return edges
}
