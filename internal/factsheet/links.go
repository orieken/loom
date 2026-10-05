package factsheet

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/repograph"
)

// links is the graph seen per directory rather than per node: a directory
// holding two languages has two nodes but one sheet.
type links struct {
	imports    map[string]map[string]repograph.Confidence
	dependents map[string]map[string]repograph.Confidence
}

func linkDirectories(graph repograph.Graph) links {
	directory := nodeDirectories(graph)
	found := links{imports: map[string]map[string]repograph.Confidence{}, dependents: map[string]map[string]repograph.Confidence{}}
	for _, edge := range graph.Edges {
		from, to := directory[edge.From], directory[edge.To]
		if from == "" || to == "" || from == to {
			continue
		}
		strengthen(found.imports, from, to, edge.Confidence)
		strengthen(found.dependents, to, from, edge.Confidence)
	}
	return found
}

// nodeDirectories maps a node id to its directory. An id may carry a
// [language] suffix, so the directory is read from the node's files.
func nodeDirectories(graph repograph.Graph) map[string]string {
	directory := map[string]string{}
	for _, node := range graph.Nodes {
		if len(node.Files) > 0 {
			directory[node.ID] = path.Dir(node.Files[0])
		}
	}
	return directory
}

// strengthen records an edge, letting an exact one win over an approximate
// one for the same pair.
func strengthen(table map[string]map[string]repograph.Confidence, owner, other string, confidence repograph.Confidence) {
	if table[owner] == nil {
		table[owner] = map[string]repograph.Confidence{}
	}
	if table[owner][other] != repograph.Exact {
		table[owner][other] = confidence
	}
}

func (l links) apply(sheet *FactSheet) {
	sheet.Imports, sheet.ApproximateImports = split(l.imports[sheet.Path])
	sheet.Dependents, sheet.ApproximateDependents = split(l.dependents[sheet.Path])
}

func split(table map[string]repograph.Confidence) (exact, approximate []string) {
	exact, approximate = []string{}, []string{}
	for other, confidence := range table {
		if confidence == repograph.Exact {
			exact = append(exact, other)
		} else {
			approximate = append(approximate, other)
		}
	}
	sort.Strings(exact)
	sort.Strings(approximate)
	return exact, approximate
}

// packageEntrypoints reads a package.json beside the sheet's files for the
// `main` and `bin` files it names, when they are tracked.
func packageEntrypoints(repo repograph.Repo, tracked map[string]bool, dir string) []string {
	if !tracked[path.Join(dir, "package.json")] {
		return nil
	}
	raw, err := repo.Read(path.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var manifest struct {
		Main string          `json:"main"`
		Bin  json.RawMessage `json:"bin"`
	}
	if json.Unmarshal(raw, &manifest) != nil {
		return nil
	}
	var found []string
	for _, target := range append([]string{manifest.Main}, binTargets(manifest.Bin)...) {
		name := strings.TrimPrefix(path.Clean(target), "./")
		if target != "" && tracked[path.Join(dir, name)] {
			found = append(found, name)
		}
	}
	return found
}

// binTargets reads package.json's `bin`, a string or a name → path object.
func binTargets(raw json.RawMessage) []string {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return []string{single}
	}
	var named map[string]string
	if json.Unmarshal(raw, &named) != nil {
		return nil
	}
	targets := make([]string, 0, len(named))
	for _, target := range named {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}
