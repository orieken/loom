package gitdiff

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The adapter test runs real git in a throwaway repository: the flags passed
// to git are the thing under test, and a fake would only restate them.
func TestGitDiffReturnsZeroContextHunksForGoFilesOnly(t *testing.T) {
	repository := newRepository(t)
	write(t, filepath.Join(repository, "a.go"), "package a\n\nfunc A() {}\n\nfunc B() {}\n")
	write(t, filepath.Join(repository, "notes.md"), "changed\n")

	reader, err := Diff("HEAD")
	if err != nil {
		t.Fatalf("gitDiff: %v", err)
	}
	output := readAll(t, reader)

	if !strings.Contains(output, "+++ b/a.go") || !strings.Contains(output, "@@ -3,0 +4,2 @@") {
		t.Errorf("diff lacks the zero-context hunk for a.go:\n%s", output)
	}
	if strings.Contains(output, "notes.md") {
		t.Errorf("diff includes a non-Go file:\n%s", output)
	}
}

// An untracked file is invisible to `git diff <base>`; left out, a new file
// would pass locally without being measured at all.
func TestGitDiffReportsAnUntrackedGoFileAsWhollyAdded(t *testing.T) {
	repository := newRepository(t)
	write(t, filepath.Join(repository, "fresh.go"), "package a\n\nfunc Fresh() {}")
	write(t, filepath.Join(repository, "fresh.md"), "not go\n")

	reader, err := Diff("HEAD")
	if err != nil {
		t.Fatalf("gitDiff: %v", err)
	}
	output := readAll(t, reader)

	// Three lines, the last without a trailing newline.
	if !strings.Contains(output, "+++ b/fresh.go\n@@ -0,0 +1,3 @@") {
		t.Errorf("untracked fresh.go is not reported as added:\n%s", output)
	}
	if strings.Contains(output, "fresh.md") {
		t.Errorf("an untracked non-Go file was reported:\n%s", output)
	}
}

func TestGitDiffReportsAnUnknownBase(t *testing.T) {
	newRepository(t)
	if _, err := Diff("no-such-revision"); err == nil || !strings.Contains(err.Error(), "no-such-revision") {
		t.Errorf("err = %v, want an error naming the base", err)
	}
}

func newRepository(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() {}\n")
	write(t, filepath.Join(dir, "notes.md"), "original\n")
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "."},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "--quiet", "-m", "base"},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readAll(t *testing.T, reader io.Reader) string {
	t.Helper()
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read diff: %v", err)
	}
	return string(content)
}
