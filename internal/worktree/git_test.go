package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/worktree"
)

// The digest must hash content, not status.
//
// `git status --porcelain` alone would have missed run 4's violation
// entirely: accessibility-engineer edited handlers.go, a file the developer
// had already modified, so the file's status never changed — only its bytes
// did (roadmap L3.30).
func TestGitDigestSeesAContentChangeToAnAlreadyModifiedFile(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	path := filepath.Join(dir, "handlers.go")
	writeFile(t, path, "package main\n")
	commitAll(t, dir)

	// The developer modifies it.
	writeFile(t, path, "package main\n// developer\n")
	tree := worktree.New(dir)
	before := digest(t, tree)

	// A later stage modifies the same already-modified file.
	writeFile(t, path, "package main\n// developer\n// accessibility-engineer\n")

	if after := digest(t, tree); before == after {
		t.Error("the digest missed a content change to an already-modified file — " +
			"exactly the edit run 4's accessibility-engineer made")
	}
}

// An untracked file a stage creates must register too.
func TestGitDigestSeesANewUntrackedFile(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	commitAll(t, dir)
	tree := worktree.New(dir)
	before := digest(t, tree)

	writeFile(t, filepath.Join(dir, "added.go"), "package main\n")

	if after := digest(t, tree); before == after {
		t.Error("the digest missed a newly created file")
	}
}

func digest(t *testing.T, tree *worktree.Git) string {
	t.Helper()
	value, err := tree.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return value
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "test")
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "base")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// A freshly initialised repository has no HEAD, so `git diff HEAD` fails.
// That is an ordinary state — every file in it is untracked and the status
// output already describes the whole tree — and it must not make the digest
// unavailable. Before this, every stage of a run in such a repo printed a
// warning.
func TestGitDigestWorksInARepositoryWithNoCommits(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	tree := worktree.New(dir)

	before := digest(t, tree)
	writeFile(t, filepath.Join(dir, "other.go"), "package other\n")

	if after := digest(t, tree); before == after {
		t.Error("the digest did not notice a new file in a repository with no commits")
	}
}
