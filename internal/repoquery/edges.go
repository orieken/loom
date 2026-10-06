package repoquery

import (
	"path"
	"sort"

	"github.com/orieken/loom/internal/repograph"
)

// Edge is one relationship, with how it was established. An approximate edge
// must never be read as fact, so the label travels with every one.
type Edge struct {
	Path       string `json:"path"`
	Confidence string `json:"confidence" jsonschema:"enum=exact,enum=approximate"`
	// Layer is the architectural layer the context index records for the other
	// end of the edge, when it has been documented. It makes a layer-boundary
	// check one query: ask for a directory's dependencies and read the layers.
	Layer string `json:"layer,omitempty"`
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
	return s.withLayers(sortedEdges(best)), nil
}

// withLayers fills each edge's layer from the index. An undocumented directory
// has none, and an edge says so by omitting it rather than guessing.
func (s Snapshot) withLayers(edges []Edge) []Edge {
	if !s.HasIndex {
		return edges
	}
	layers := s.layersByNode()
	for index := range edges {
		edges[index].Layer = layers[edges[index].Path]
	}
	return edges
}

// layersByNode maps a node id to the layer recorded for its directory.
func (s Snapshot) layersByNode() map[string]string {
	layers := make(map[string]string, len(s.Graph.Nodes))
	for _, node := range s.Graph.Nodes {
		if len(node.Files) > 0 {
			layers[node.ID] = s.Index.Entries[path.Dir(node.Files[0])].Layer
		}
	}
	return layers
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
