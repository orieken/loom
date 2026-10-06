// Package fixture builds real, indexed repositories for tests of code that reads
// the context index. It is test support: no production code imports it.
package fixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/repoindex"
)

// Indexed creates a committed git repository holding files and indexes
// it exactly as `loom index` does, so tests of the tools that read the index
// work against real artifacts rather than hand-written ones. It skips the test
// when git is not installed.
func Indexed(t testing.TB, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for name, content := range files {
		writeFixtureFile(t, root, name, content)
	}
	run("add", "-A")
	run("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "fixture")
	if _, err := repoindex.Run(root, repograph.GitLister{}, time.Now); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFixtureFile(t testing.TB, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
