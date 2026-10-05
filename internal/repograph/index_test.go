package repograph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var goRepoFiles = map[string]string{
	"go.mod":           "module example.com/x\n\ngo 1.22\n",
	"a/a.go":           "package a\n\nimport \"example.com/x/b\"\n\nvar _ = b.V\n",
	"b/b.go":           "package b\n\nvar V = 1\n",
	"vendor/v/v.go":    "package v\n",
	"generated/gen.go": "package generated\n\nimport \"example.com/x/a\"\n",
}

func TestRerunOnAnUnchangedRepoWritesNothing(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	first, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow)
	if err != nil || !first.Written {
		t.Fatalf("first index: written=%v err=%v", first.Written, err)
	}
	path := filepath.Join(root, OutputDir, GraphFile)
	before, _ := os.Stat(path)
	later := func() time.Time { return fixedNow().Add(48 * time.Hour) }
	second, err := Index(root, GitLister{}, DefaultExtractors(), later)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if second.Written || !after.ModTime().Equal(before.ModTime()) {
		t.Error("an unchanged repo was re-indexed with a write")
	}
}

// A new commit changes git_sha but not the graph: still no write.
func TestNewCommitThatDoesNotChangeTheGraphWritesNothing(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	if _, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "README.md", "hello")
	runIn(t, root, "add", "-A")
	runIn(t, root, "commit", "-q", "-m", "docs")
	result, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow)
	if err != nil || result.Written {
		t.Errorf("graph unchanged but written=%v err=%v", result.Written, err)
	}
}

func TestChangedGraphIsRewritten(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	if _, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "b/b.go", "package b\n\nimport \"example.com/x/a\"\n\nvar V = a.W\n")
	runIn(t, root, "add", "-A")
	result, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow)
	if err != nil || !result.Written || !hasEdge(result.Graph, "b", "a", Exact) {
		t.Errorf("changed graph not rewritten: written=%v err=%v", result.Written, err)
	}
}

func TestUntrackedGitignoredAndLoomignoredFilesNeverAppear(t *testing.T) {
	files := map[string]string{
		".gitignore":  "ignored/\n",
		".loomignore": "vendor/\ngenerated/**\n",
	}
	for name, content := range goRepoFiles {
		files[name] = content
	}
	files["ignored/i.go"] = "package ignored\n\nimport \"example.com/x/a\"\n"
	root := gitRepo(t, files)
	// Added after the commit and never `git add`ed: untracked.
	writeFile(t, root, "untracked/u.go", "package untracked\n\nimport \"example.com/x/a\"\n")

	result, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, excluded := range []string{"untracked", "ignored", "vendor/v", "generated"} {
		if hasNode(result.Graph, excluded) {
			t.Errorf("%s must not be a node", excluded)
		}
	}
	if !hasNode(result.Graph, "a") || !hasNode(result.Graph, "b") {
		t.Error("tracked, un-ignored packages must be indexed")
	}
}

func TestIndexHonoursLoomignoreInFixtureRepos(t *testing.T) {
	graph := indexFixture(t, "mixed").Graph
	if hasNode(graph, "vendor") {
		t.Error("vendor/ is in .loomignore and must not be indexed")
	}
}

func TestTrackedFileDeletedFromTheWorkingTreeIsSkipped(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	if err := os.Remove(filepath.Join(root, "b", "b.go")); err != nil {
		t.Fatal(err)
	}
	result, err := Index(root, GitLister{}, DefaultExtractors(), fixedNow)
	if err != nil || hasNode(result.Graph, "b") {
		t.Errorf("deleted file indexed: err=%v nodes=%+v", err, result.Graph.Nodes)
	}
}

func TestIndexFailsOutsideAGitRepository(t *testing.T) {
	if _, err := Index(t.TempDir(), GitLister{}, DefaultExtractors(), fixedNow); err == nil {
		t.Error("indexing a directory that is not a git repo must fail, not produce an empty graph")
	}
}
