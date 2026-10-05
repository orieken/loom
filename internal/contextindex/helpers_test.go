package contextindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
)

// gitRepo creates a committed git repository holding the given files.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
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

var goRepoFiles = map[string]string{
	"go.mod": "module example.com/x\n\ngo 1.22\n",
	"a/a.go": "package a\n\nimport \"example.com/x/b\"\n\nvar _ = b.V\n",
	"b/b.go": "package b\n\nvar V = 1\n",
}

func currentSheets(t *testing.T, root string) []factsheet.FactSheet {
	t.Helper()
	sheets, err := CurrentSheets(root, repograph.GitLister{})
	if err != nil {
		t.Fatalf("current sheets: %v", err)
	}
	return sheets
}

// documentAll records every directory as documented against its current hash,
// as the repo-documenter will in Phase 4.
func documentAll(index Index, sheets []factsheet.FactSheet) Index {
	for _, sheet := range sheets {
		entry := index.Entries[sheet.Path]
		entry.Purpose = "documented"
		entry.InputHash = sheet.InputHash
		index.Entries[sheet.Path] = entry
	}
	return index
}
