package repoindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/repograph"
)

func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.22\n", "a/a.go": "package a\n\nimport \"example.com/x/b\"\n\nvar _ = b.V\n", "b/b.go": "package b\n\nvar V = 1\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestRunProducesTheGraph(t *testing.T) {
	result, err := Run(repo(t), repograph.GitLister{}, time.Now)
	if err != nil || !result.Graph.Written || len(result.Graph.Graph.Edges) != 1 {
		t.Errorf("%+v %v", result.Graph.Graph.Edges, err)
	}
}

func TestRunProducesTheFactSheetsAndTheIndex(t *testing.T) {
	result, err := Run(repo(t), repograph.GitLister{}, time.Now)
	if err != nil || len(result.Sheets) != 3 || result.Facts.Written != 3 {
		t.Fatalf("%+v %v", result.Facts, err)
	}
	if !result.IndexWritten || len(result.Index.Entries) != 3 {
		t.Errorf("index written=%v entries=%d", result.IndexWritten, len(result.Index.Entries))
	}
}

func TestARerunOverAnUnchangedRepoWritesNothingAtAll(t *testing.T) {
	root := repo(t)
	if _, err := Run(root, repograph.GitLister{}, time.Now); err != nil {
		t.Fatal(err)
	}
	again, err := Run(root, repograph.GitLister{}, time.Now)
	if err != nil || again.Graph.Written || again.Facts.Written != 0 || again.IndexWritten {
		t.Errorf("%+v %v", again, err)
	}
}

func TestRunFailsOutsideAGitRepository(t *testing.T) {
	if _, err := Run(t.TempDir(), repograph.GitLister{}, time.Now); err == nil {
		t.Error("there is nothing to index")
	}
}

func TestRunFailsLoudlyOnACorruptExistingIndex(t *testing.T) {
	root := repo(t)
	if err := os.MkdirAll(filepath.Join(root, ".loom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loom", "index.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, repograph.GitLister{}, time.Now); err == nil {
		t.Error("a corrupt index must not be silently replaced")
	}
}
