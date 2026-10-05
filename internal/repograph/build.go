package repograph

import (
	"fmt"
	"sort"
)

// Meta is the metadata Build stamps on a graph. It is outside the graph hash.
type Meta struct {
	GitSHA      string
	GeneratedAt string
}

type nodeKey struct{ dir, lang string }

// Build runs every extractor over the repo and assembles one sorted,
// deduplicated graph.
func Build(repo Repo, extractors []Extractor, meta Meta) (Graph, error) {
	nodes, ids := buildNodes(repo, extractors)
	edges := map[[2]string]Edge{}
	var unresolved []Unresolved
	for _, extractor := range extractors {
		extraction, err := extractor.Extract(repo)
		if err != nil {
			return Graph{}, fmt.Errorf("%s extractor: %w", extractor.Language(), err)
		}
		addEdges(edges, ids, extractor.Language(), extraction.Edges)
		unresolved = append(unresolved, extraction.Unresolved...)
	}
	return assemble(nodes, edges, unresolved, meta), nil
}

func buildNodes(repo Repo, extractors []Extractor) ([]Node, map[nodeKey]string) {
	byKey := map[nodeKey]*Node{}
	langsInDir := map[string]map[string]bool{}
	for _, extractor := range extractors {
		for _, file := range handledFiles(repo, extractor) {
			key := nodeKey{dirOf(file), extractor.Language()}
			if byKey[key] == nil {
				byKey[key] = &Node{Kind: extractor.NodeKind(), Lang: key.lang}
			}
			byKey[key].Files = append(byKey[key].Files, file)
			recordLanguage(langsInDir, key)
		}
	}
	return finishNodes(byKey, langsInDir)
}

func recordLanguage(langsInDir map[string]map[string]bool, key nodeKey) {
	if langsInDir[key.dir] == nil {
		langsInDir[key.dir] = map[string]bool{}
	}
	langsInDir[key.dir][key.lang] = true
}

// finishNodes assigns ids: the directory path, or dir[lang] when a directory
// holds more than one language.
func finishNodes(byKey map[nodeKey]*Node, langsInDir map[string]map[string]bool) ([]Node, map[nodeKey]string) {
	ids := map[nodeKey]string{}
	nodes := make([]Node, 0, len(byKey))
	for key, node := range byKey {
		node.ID = key.dir
		if len(langsInDir[key.dir]) > 1 {
			node.ID = key.dir + "[" + key.lang + "]"
		}
		sort.Strings(node.Files)
		ids[key] = node.ID
		nodes = append(nodes, *node)
	}
	sortNodes(nodes)
	return nodes, ids
}

// addEdges folds file-level imports into directory edges. An exact edge is
// never downgraded by a later approximate one for the same pair.
func addEdges(edges map[[2]string]Edge, ids map[nodeKey]string, lang string, found []FileEdge) {
	for _, fileEdge := range found {
		from, fromOK := ids[nodeKey{dirOf(fileEdge.FromFile), lang}]
		to, toOK := ids[nodeKey{fileEdge.ToDir, lang}]
		if !fromOK || !toOK || from == to {
			continue
		}
		key := [2]string{from, to}
		if existing, seen := edges[key]; seen && existing.Confidence == Exact {
			continue
		}
		edges[key] = Edge{From: from, To: to, Kind: EdgeKind, Confidence: fileEdge.Confidence}
	}
}

func assemble(nodes []Node, edgeMap map[[2]string]Edge, unresolved []Unresolved, meta Meta) Graph {
	edges := make([]Edge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}
	sortEdges(edges)
	unresolved = dedupeUnresolved(unresolved)
	return Graph{
		SchemaVersion: SchemaVersion,
		GitSHA:        meta.GitSHA,
		GeneratedAt:   meta.GeneratedAt,
		GraphHash:     hashContent(nodes, edges, unresolved),
		Nodes:         nodes,
		Edges:         edges,
		Unresolved:    unresolved,
	}
}

func dedupeUnresolved(items []Unresolved) []Unresolved {
	seen := map[Unresolved]bool{}
	out := []Unresolved{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sortUnresolved(out)
	return out
}
