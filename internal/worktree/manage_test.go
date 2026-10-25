package worktree_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/worktree"
)

// Roadmap L3.66: one worktree per run, on its own branch, so two runs never
// share an index or see each other's edits.

func baseRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "README.md"), "base\n")
	commitAll(t, dir)
	git(t, dir, "branch", "-M", "main")
	return dir
}

func TestTwoWorktreesKeepTheirEditsApart(t *testing.T) {
	root := baseRepo(t)
	repo := worktree.New(root)
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	if err := repo.AddWorktree(first, "factory/first", "main"); err != nil {
		t.Fatalf("add first: %v", err)
	}
	if err := repo.AddWorktree(second, "factory/second", "main"); err != nil {
		t.Fatalf("add second: %v", err)
	}

	writeFile(t, filepath.Join(first, "first.go"), "package first\n")
	writeFile(t, filepath.Join(second, "second.go"), "package second\n")

	assertChanged(t, first, "first.go")
	assertChanged(t, second, "second.go")
	assertChanged(t, root)
}

func assertChanged(t *testing.T, dir string, want ...string) {
	t.Helper()
	changed, err := worktree.New(dir).ChangedPaths()
	if err != nil {
		t.Fatalf("changed paths in %s: %v", dir, err)
	}
	if strings.Join(changed, ",") != strings.Join(want, ",") {
		t.Errorf("%s changed %v, want only %v", filepath.Base(dir), changed, want)
	}
}

// A worktree that already exists is reused, and a branch that already exists
// is checked out rather than recreated — re-claiming continues the work.
func TestAddWorktreeKeepsExistingWork(t *testing.T) {
	root := baseRepo(t)
	repo := worktree.New(root)
	path := filepath.Join(t.TempDir(), "order")
	if err := repo.AddWorktree(path, "factory/order", "main"); err != nil {
		t.Fatalf("add: %v", err)
	}
	writeFile(t, filepath.Join(path, "work.go"), "package work\n")
	if err := repo.AddWorktree(path, "factory/order", "main"); err != nil {
		t.Fatalf("re-add over an existing worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "work.go")); err != nil {
		t.Error("re-adding an existing worktree lost its work")
	}

	git(t, path, "add", "-A")
	git(t, path, "commit", "-qm", "work")
	if err := repo.RemoveWorktree(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := repo.AddWorktree(path, "factory/order", "main"); err != nil {
		t.Fatalf("re-add on the existing branch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "work.go")); err != nil {
		t.Error("a worktree re-created on its branch did not carry the branch's commits")
	}
}

func TestAddWorktreeRefusesAnUnknownBase(t *testing.T) {
	repo := worktree.New(baseRepo(t))
	if err := repo.AddWorktree(filepath.Join(t.TempDir(), "x"), "factory/x", "no-such-branch"); err == nil {
		t.Error("a worktree was created from a base that does not exist")
	}
}

func TestResolveCommitFallsBackToOrigin(t *testing.T) {
	upstream := baseRepo(t)
	clone := t.TempDir()
	git(t, clone, "clone", "-q", upstream, ".")
	git(t, clone, "checkout", "-q", "-b", "other")
	git(t, clone, "branch", "-D", "main")

	commit, err := worktree.New(clone).ResolveCommit("main")
	if err != nil || len(commit) != 40 {
		t.Errorf("ResolveCommit(main) = %q, %v; want origin/main's commit", commit, err)
	}
}

func TestShowFileAndListFilesReadACommit(t *testing.T) {
	root := baseRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "intake"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "intake", "a.md"), "committed\n")
	commitAll(t, root)
	writeFile(t, filepath.Join(root, "intake", "a.md"), "uncommitted\n")
	writeFile(t, filepath.Join(root, "intake", "b.md"), "untracked\n")
	repo := worktree.New(root)
	head, err := repo.ResolveCommit("main")
	if err != nil {
		t.Fatal(err)
	}

	files, err := repo.ListFiles(head, "intake")
	if err != nil || strings.Join(files, ",") != "intake/a.md" {
		t.Errorf("ListFiles = %v, %v; want only the committed intake/a.md", files, err)
	}
	content, err := repo.ShowFile(head, "intake/a.md")
	if err != nil || string(content) != "committed\n" {
		t.Errorf("ShowFile = %q, %v; want the committed content", content, err)
	}
}

func TestListFilesIsEmptyForADirectoryTheCommitLacks(t *testing.T) {
	repo := worktree.New(baseRepo(t))
	head, err := repo.ResolveCommit("main")
	if err != nil {
		t.Fatal(err)
	}
	if missing, err := repo.ListFiles(head, "features"); err != nil || len(missing) != 0 {
		t.Errorf("ListFiles(features) = %v, %v; want nothing", missing, err)
	}
}

func TestRepositoryRootFindsTheTopFromASubdirectory(t *testing.T) {
	root := baseRepo(t)
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := worktree.RepositoryRoot(sub)
	resolved, _ := filepath.EvalSymlinks(root)
	if err != nil || got != resolved {
		t.Errorf("RepositoryRoot = %q, %v; want %q", got, err, resolved)
	}
	if _, err := worktree.RepositoryRoot(t.TempDir()); err == nil {
		t.Error("a directory outside any repository has a root")
	}
}

func TestCommitAllLeavesExcludedPathsOutAndPushReachesOrigin(t *testing.T) {
	upstream := baseRepo(t)
	git(t, upstream, "config", "receive.denyCurrentBranch", "ignore")
	clone := t.TempDir()
	git(t, clone, "clone", "-q", upstream, ".")
	git(t, clone, "checkout", "-q", "-b", "factory/order")
	writeFile(t, filepath.Join(clone, "feature.go"), "package feature\n")
	if err := os.MkdirAll(filepath.Join(clone, ".claude", "feature-workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(clone, ".claude", "feature-workspace", "state.json"), "{}\n")
	tree := worktree.New(clone)

	commit, err := tree.CommitAll("feat(order): add a feature", []string{".claude/feature-workspace"})
	if err != nil || len(commit) != 40 {
		t.Fatalf("CommitAll = %q, %v", commit, err)
	}
	assertChanged(t, clone, ".claude/feature-workspace/state.json")
	if err := tree.Push("factory/order"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if pushed, err := worktree.New(upstream).ResolveCommit("factory/order"); err != nil || pushed != commit {
		t.Errorf("origin has factory/order at %q (%v), want %q", pushed, err, commit)
	}
}

// The first real factory commit failed here: the repository ignored
// .claude/memory/, and git refused an exclude pathspec naming an ignored path.
func TestCommitAllExcludesAPathTheRepositoryIgnores(t *testing.T) {
	root := baseRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), ".claude/memory/\n")
	commitAll(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".claude", "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".claude", "memory", "episodes.db"), "db\n")
	writeFile(t, filepath.Join(root, "feature.go"), "package feature\n")

	_, err := worktree.New(root).CommitAll("feat(order): add a feature", []string{".claude/memory", ".claude/factory"})
	if err != nil {
		t.Fatalf("CommitAll with an ignored exclusion: %v", err)
	}
	assertChanged(t, root)
	if _, err := worktree.New(root).ShowFile("HEAD", ".claude/memory/episodes.db"); err == nil {
		t.Error("the ignored, excluded memory store was committed")
	}
	if _, err := worktree.New(root).ShowFile("HEAD", "feature.go"); err != nil {
		t.Errorf("the run's change was not committed: %v", err)
	}
}

// An exclusion git cannot unstage fails the commit rather than committing it.
func TestCommitAllFailsOnAnExclusionOutsideTheRepository(t *testing.T) {
	root := baseRepo(t)
	writeFile(t, filepath.Join(root, "feature.go"), "package feature\n")
	if _, err := worktree.New(root).CommitAll("feat(order): x", []string{"../elsewhere"}); err == nil {
		t.Error("a commit went ahead with an exclusion git rejected")
	}
}

// A failing git command says why, not only its exit status.
func TestAFailedGitCommandCarriesItsStderr(t *testing.T) {
	_, err := worktree.New(baseRepo(t)).ShowFile("HEAD", "missing.md")
	if err == nil {
		t.Fatal("a missing file was shown")
	}
	if !strings.Contains(err.Error(), "missing.md' does not exist") {
		t.Errorf("error %q lacks git's own message", err)
	}
}

func TestCommitAllCommitsAnUnchangedTree(t *testing.T) {
	root := baseRepo(t)
	if _, err := worktree.New(root).CommitAll("chore(order): nothing changed", nil); err != nil {
		t.Errorf("an unchanged tree could not be committed: %v", err)
	}
}

func TestPushWithoutAnOriginFails(t *testing.T) {
	if err := worktree.New(baseRepo(t)).Push("main"); err == nil {
		t.Error("a push with no origin succeeded")
	}
}
