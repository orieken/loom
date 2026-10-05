package repograph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OutputDir is the sidecar directory every artifact is written to.
const OutputDir = ".loom"

// GraphFile is the graph's path within OutputDir.
const GraphFile = "graph.json"

// Result is what Index did.
type Result struct {
	Graph   Graph
	Written bool
}

// Index lists the repo, builds its graph and writes .loom/graph.json — but
// only when the graph's content changed, so a rerun on an unchanged repo
// writes nothing.
func Index(root string, lister Lister, extractors []Extractor, now func() time.Time) (Result, error) {
	repo, err := openRepo(root, lister)
	if err != nil {
		return Result{}, err
	}
	meta := Meta{GitSHA: lister.HeadSHA(root), GeneratedAt: now().UTC().Format(time.RFC3339)}
	graph, err := Build(repo, extractors, meta)
	if err != nil {
		return Result{}, err
	}
	written, err := writeGraph(root, graph)
	if err != nil {
		return Result{}, err
	}
	return Result{Graph: graph, Written: written}, nil
}

func openRepo(root string, lister Lister) (Repo, error) {
	listed, err := lister.TrackedFiles(root)
	if err != nil {
		return nil, err
	}
	ignore := loadIgnore(root)
	var files []string
	for _, file := range listed {
		if !ignore.Matches(file) && !strings.HasPrefix(file, OutputDir+"/") && fileExists(root, file) {
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return dirRepo{root: root, files: files}, nil
}

func loadIgnore(root string) *Ignore {
	text, err := os.ReadFile(filepath.Join(root, IgnoreFileName))
	if err != nil {
		return ParseIgnore("")
	}
	return ParseIgnore(string(text))
}

// fileExists skips a tracked file deleted from the working tree.
func fileExists(root, file string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(file)))
	return err == nil
}

// writeGraph returns true when it wrote. An existing graph with the same
// schema version and hash is left alone, keeping its original metadata.
func writeGraph(root string, graph Graph) (bool, error) {
	path := filepath.Join(root, OutputDir, GraphFile)
	if existing, ok := readGraph(path); ok && existing.GraphHash == graph.GraphHash && existing.SchemaVersion == graph.SchemaVersion {
		return false, nil
	}
	raw, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode graph: %w", err)
	}
	return true, writeAtomically(path, append(raw, '\n'))
}

func readGraph(path string) (Graph, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Graph{}, false
	}
	var graph Graph
	if json.Unmarshal(raw, &graph) != nil {
		return Graph{}, false
	}
	return graph, true
}

// writeAtomically writes beside the target and renames, so a reader never
// sees a half-written graph.
func writeAtomically(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
