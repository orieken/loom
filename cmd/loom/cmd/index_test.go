package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexCommandWritesGraphThenReportsUnchanged(t *testing.T) {
	root := committedGoRepo(t)
	first := runIndexIn(t, root)
	assertContainsAll(t, first,
		"2 nodes, 1 edges (1 exact, 0 approximate), 0 unresolved: wrote .loom/graph.json",
		"3 fact sheets: 3 written, 0 removed",
		"3 index entries (0 fresh, 0 stale, 3 undocumented): wrote .loom/index.json")
	if _, err := os.Stat(filepath.Join(root, ".loom/facts/a.json")); err != nil {
		t.Errorf("fact sheet not written: %v", err)
	}
	assertContainsAll(t, runIndexIn(t, root),
		"unchanged, nothing written",
		"3 fact sheets: 0 written, 0 removed",
		"(0 fresh, 0 stale, 3 undocumented): unchanged")
}

func assertContainsAll(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("output %q lacks %q", output, want)
		}
	}
}

func committedGoRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.22\n",
		"a/a.go": "package a\n\nimport \"example.com/x/b\"\n\nvar _ = b.V\n",
		"b/b.go": "package b\n\nvar V = 1\n",
	}
	for name, content := range files {
		writeRepoFile(t, root, name, content)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func writeRepoFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runIndexIn(t *testing.T, root string) string {
	t.Helper()
	var out bytes.Buffer
	indexCmd.SetOut(&out)
	indexRoot = root
	if err := runIndex(indexCmd, nil); err != nil {
		t.Fatalf("loom index: %v", err)
	}
	return out.String()
}

func TestIndexCommandFailsOutsideARepository(t *testing.T) {
	indexRoot = t.TempDir()
	if err := runIndex(indexCmd, nil); err == nil {
		t.Error("indexing a non-repository must fail")
	}
}
