package repoquery

import (
	"fmt"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/repograph"
)

// DefaultImpactDepth and MaxImpactDepth bound how far a change is followed.
const (
	DefaultImpactDepth = 3
	MaxImpactDepth     = 10
)

// Impacted is one directory a change would reach.
type Impacted struct {
	Path string `json:"path"`
	// Distance is the fewest imports between the change and this directory.
	Distance int `json:"distance"`
	// Confidence is exact only if some path of that length uses exact edges
	// throughout. One heuristic edge makes the whole chain approximate.
	Confidence string `json:"confidence" jsonschema:"enum=exact,enum=approximate"`
}

// Impact returns the directories that transitively depend on the given paths,
// out to depth imports, nearest first. The changed paths themselves are not
// listed. Every path must be in the graph: a typo is an error, not a silently
// smaller blast radius.
func (s Snapshot) Impact(paths []string, depth int) ([]Impacted, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	if depth <= 0 {
		depth = DefaultImpactDepth
	}
	if depth > MaxImpactDepth {
		depth = MaxImpactDepth
	}
	seeds, err := s.seedNodes(paths)
	if err != nil {
		return nil, err
	}
	return sortImpacted(walkDependents(reverseAdjacency(s.Graph.Edges), seeds, depth)), nil
}

// seedNodes resolves every path, reporting all the unknown ones together.
func (s Snapshot) seedNodes(paths []string) (map[string]bool, error) {
	seeds := map[string]bool{}
	var unknown []string
	for _, raw := range paths {
		nodes, err := s.nodesFor(raw)
		if err != nil {
			unknown = append(unknown, raw)
			continue
		}
		for id := range nodeIDs(nodes) {
			seeds[id] = true
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("not in the repo graph: %s", strings.Join(unknown, ", "))
	}
	return seeds, nil
}

type importer struct {
	from  string
	exact bool
}

// reverseAdjacency maps a node to the nodes that import it.
func reverseAdjacency(edges []repograph.Edge) map[string][]importer {
	reverse := map[string][]importer{}
	for _, edge := range edges {
		reverse[edge.To] = append(reverse[edge.To], importer{from: edge.From, exact: edge.Confidence == repograph.Exact})
	}
	return reverse
}

// reach is what is known about one node while walking.
type reach struct {
	distance int
	exact    bool
}

// walkDependents is a breadth-first walk over importers. A node keeps its
// shortest distance; at that distance it is exact if any shortest path is.
func walkDependents(reverse map[string][]importer, seeds map[string]bool, depth int) map[string]reach {
	found := map[string]reach{}
	frontier := map[string]bool{}
	for id := range seeds {
		frontier[id] = true
		found[id] = reach{distance: 0, exact: true}
	}
	for distance := 1; distance <= depth && len(frontier) > 0; distance++ {
		next := map[string]bool{}
		for id := range frontier {
			for _, edge := range reverse[id] {
				advance(found, next, edge, found[id], distance)
			}
		}
		frontier = next
	}
	for id := range seeds {
		delete(found, id)
	}
	return found
}

func advance(found map[string]reach, next map[string]bool, edge importer, via reach, distance int) {
	exact := via.exact && edge.exact
	current, seen := found[edge.from]
	switch {
	case !seen:
		found[edge.from] = reach{distance: distance, exact: exact}
		next[edge.from] = true
	case current.distance == distance && exact && !current.exact:
		found[edge.from] = reach{distance: distance, exact: true}
	}
}

func sortImpacted(found map[string]reach) []Impacted {
	out := make([]Impacted, 0, len(found))
	for id, r := range found {
		confidence := string(repograph.Approximate)
		if r.exact {
			confidence = string(repograph.Exact)
		}
		out = append(out, Impacted{Path: id, Distance: r.distance, Confidence: confidence})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance != out[j].Distance {
			return out[i].Distance < out[j].Distance
		}
		return out[i].Path < out[j].Path
	})
	return out
}
