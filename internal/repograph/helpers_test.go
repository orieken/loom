package repograph

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const fixturesDir = "../../tests/fixtures/repograph"

var fixedNow = func() time.Time { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

// walkLister lists every file under a directory: a Lister for fixture repos
// that are not git repositories themselves.
type walkLister struct{}

func (walkLister) TrackedFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	return files, err
}

func (walkLister) HeadSHA(string) string { return "fixture" }

// indexFixture indexes a fixture repo into a scratch copy so nothing is
// written into the checked-in tree.
func indexFixture(t *testing.T, name string) Result {
	t.Helper()
	root := copyTree(t, filepath.Join(fixturesDir, name))
	result, err := Index(root, walkLister{}, DefaultExtractors(), fixedNow)
	if err != nil {
		t.Fatalf("index %s: %v", name, err)
	}
	return result
}

func copyTree(t *testing.T, source string) string {
	t.Helper()
	target := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, relative), 0o755)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(target, relative), content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return target
}

// gitRepo creates a real git repository holding the given files, all
// committed.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	runIn(t, root, "init", "-q")
	runIn(t, root, "config", "user.email", "test@example.com")
	runIn(t, root, "config", "user.name", "Test")
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	runIn(t, root, "add", "-A")
	runIn(t, root, "commit", "-q", "-m", "fixture")
	return root
}

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

func runIn(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func hasEdge(graph Graph, from, to string, confidence Confidence) bool {
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Confidence == confidence {
			return true
		}
	}
	return false
}

func hasUnresolved(graph Graph, from, spec, reason string) bool {
	for _, item := range graph.Unresolved {
		if item.From == from && item.Spec == spec && item.Reason == reason {
			return true
		}
	}
	return false
}

func hasNode(graph Graph, id string) bool {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}
