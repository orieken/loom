// Package repograph extracts a deterministic import graph from a target
// repository (ADR-010). Everything here is computed in Go: no model is
// consulted, and nothing is read from the network.
package repograph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// SchemaVersion is the graph.json format version this build writes.
const SchemaVersion = 1

// SchemaDir is where the generated JSON Schema is committed, relative to the
// repo root.
const SchemaDir = "shared/schemas/repograph"

// SchemaFile is the generated schema's file name.
const SchemaFile = "graph.schema.json"

// Confidence says how an edge was established. An approximate edge must never
// be presented as fact.
type Confidence string

const (
	// Exact: the import was read from source and resolved to a tracked file
	// or package in the repo.
	Exact Confidence = "exact"
	// Approximate: resolution relied on a heuristic.
	Approximate Confidence = "approximate"
)

// EdgeKind is the only relationship extracted today.
const EdgeKind = "imports"

// Node is one directory of source files in one language.
type Node struct {
	ID    string   `json:"id" jsonschema:"description=Directory path or dir[lang] when the directory holds several languages"`
	Kind  string   `json:"kind" jsonschema:"enum=package,enum=module"`
	Lang  string   `json:"lang"`
	Files []string `json:"files" jsonschema:"description=Repo-relative paths in sorted order"`
}

// Edge is a directed dependency between two nodes.
type Edge struct {
	From       string     `json:"from"`
	To         string     `json:"to"`
	Kind       string     `json:"kind" jsonschema:"enum=imports"`
	Confidence Confidence `json:"confidence" jsonschema:"enum=exact,enum=approximate"`
}

// Unresolved is an import that could not become an edge. It is recorded
// rather than guessed.
type Unresolved struct {
	From   string `json:"from" jsonschema:"description=Repo-relative file containing the import"`
	Spec   string `json:"spec" jsonschema:"description=The import specifier as written"`
	Reason string `json:"reason" jsonschema:"enum=dynamic,enum=not-found,enum=alias-not-found,enum=parse-error"`
}

// Graph is the document written to .loom/graph.json.
//
// GraphHash covers Nodes, Edges and Unresolved only. GeneratedAt and GitSHA
// are metadata outside it, so an unchanged graph re-indexed after a commit is
// still byte-identical and is not rewritten.
type Graph struct {
	SchemaVersion int          `json:"schema_version" jsonschema:"description=Always the SchemaVersion this build writes"`
	GitSHA        string       `json:"git_sha"`
	GeneratedAt   string       `json:"generated_at"`
	GraphHash     string       `json:"graph_hash" jsonschema:"description=sha256 over nodes and edges and unresolved"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Unresolved    []Unresolved `json:"unresolved"`
}

// hashContent computes the canonical hash of the graph's content.
func hashContent(nodes []Node, edges []Edge, unresolved []Unresolved) string {
	raw, _ := json.Marshal(struct {
		Nodes      []Node       `json:"nodes"`
		Edges      []Edge       `json:"edges"`
		Unresolved []Unresolved `json:"unresolved"`
	}{nodes, edges, unresolved})
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sortNodes(nodes []Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}

func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
}

func sortUnresolved(items []Unresolved) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Spec != b.Spec {
			return a.Spec < b.Spec
		}
		return a.Reason < b.Reason
	})
}
