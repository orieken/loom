package repoquery

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/repoindex"
)

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// layered is a repo whose imports form a chain: cmd -> svc -> store, with api
// also importing store.
var layered = map[string]string{
	"go.mod":          "module example.com/x\n\ngo 1.22\n",
	"cmd/app/main.go": "package main\n\nimport \"example.com/x/svc\"\n\nfunc main() { svc.Run() }\n",
	"svc/svc.go":      "package svc\n\nimport \"example.com/x/store\"\n\nfunc Run() { store.Get() }\n",
	"api/api.go":      "package api\n\nimport \"example.com/x/store\"\n\nfunc Serve() { store.Get() }\n",
	"store/store.go":  "package store\n\nfunc Get() {}\n",
	"store/helper.go": "package store\n",
	"docs/guide.md":   "# Guide\n",
}

// indexedRepo creates a git repo and runs what `loom index` does, so the
// stored artifacts are real.
func indexedRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	git(t, root, "init", "-q")
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	reindex(t, root)
	return root
}

// reindex does what `loom index` does.
func reindex(t *testing.T, root string) {
	t.Helper()
	if _, err := repoindex.Run(root, repograph.GitLister{}, time.Now); err != nil {
		t.Fatal(err)
	}
}

func newTestStore(root string) *Store { return NewStore(root, repograph.GitLister{}) }

func snapshotOf(t *testing.T, root string) *Snapshot {
	t.Helper()
	snapshot, err := newTestStore(root).Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return &snapshot
}

// document records a purpose and tags for a directory the way an applied
// changeset would.
func document(t *testing.T, root, dir, purpose, layer string, tags ...string) {
	t.Helper()
	index, _, err := contextindex.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sheets, _ := factsheet.LoadAll(root)
	entry := index.Entries[dir]
	entry.Purpose, entry.Layer, entry.Tags, entry.InputHash = purpose, layer, tags, sheets[dir].InputHash
	index.Entries[dir] = entry
	// Build recomputes every status, as applying a changeset does.
	index = contextindex.Build(index, sheetList(sheets))
	if _, err := contextindex.Write(root, index); err != nil {
		t.Fatal(err)
	}
}

func paths(edges []Edge) []string {
	out := []string{}
	for _, edge := range edges {
		out = append(out, edge.Path)
	}
	return out
}

func sheetList(sheets map[string]factsheet.FactSheet) []factsheet.FactSheet {
	out := make([]factsheet.FactSheet, 0, len(sheets))
	for _, sheet := range sheets {
		out = append(out, sheet)
	}
	return out
}
