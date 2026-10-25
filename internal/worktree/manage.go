package worktree

// Worktree management for the factory (roadmap L3.66). Two runs sharing one
// checkout see each other's edits, and the undeclared-edit check above would
// attribute one run's change to the other's stage. A git worktree per run
// gives each its own index and its own working tree on its own branch, so
// isolation is structural rather than a rule a model must remember.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RepositoryRoot is the top of the working tree containing dir.
func RepositoryRoot(dir string) (string, error) {
	output, err := New(dir).run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ResolveCommit returns the commit a ref names, trying the ref as given and
// then as a branch on origin — a clone often has `origin/main` and no local
// `main` at all.
func (g *Git) ResolveCommit(ref string) (string, error) {
	for _, candidate := range []string{ref, "origin/" + ref} {
		output, err := g.run("rev-parse", "--verify", "--quiet", candidate+"^{commit}")
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}
	return "", fmt.Errorf("no commit named %q (looked for %s and origin/%s)", ref, ref, ref)
}

// AddWorktree checks out branch at path, creating the branch from base when
// it does not exist yet. Existing work on the branch is kept: a re-claimed
// order continues where it stopped rather than starting over.
func (g *Git) AddWorktree(path, branch, base string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if g.hasBranch(branch) {
		_, err := g.run("worktree", "add", path, branch)
		return err
	}
	baseCommit, err := g.ResolveCommit(base)
	if err != nil {
		return err
	}
	_, err = g.run("worktree", "add", "-b", branch, path, baseCommit)
	return err
}

// RemoveWorktree deletes the worktree at path and git's record of it. The
// branch stays: it is the run's result.
func (g *Git) RemoveWorktree(path string) error {
	_, err := g.run("worktree", "remove", "--force", path)
	return err
}

func (g *Git) hasBranch(branch string) bool {
	_, err := g.run("rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// ShowFile reads path as committed at commit.
func (g *Git) ShowFile(commit, path string) ([]byte, error) {
	return g.run("show", commit+":"+path)
}

// ListFiles lists the files directly under dir as committed at commit,
// repo-relative. A directory the commit does not have lists nothing.
func (g *Git) ListFiles(commit, dir string) ([]string, error) {
	output, err := g.run("ls-tree", "--name-only", commit, dir+"/")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// ChangedBetween lists the files that differ between two commits,
// repo-relative.
func (g *Git) ChangedBetween(from, to string) ([]string, error) {
	output, err := g.run("diff", "--name-only", from, to)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// pushTimeout bounds a push: a network call with no deadline is how an
// unattended supervisor hangs forever on a credential prompt.
const pushTimeout = 5 * time.Minute

// committerFallback is the identity a commit uses when the repository has
// none configured, so an unattended commit cannot fail on a missing name.
var committerFallback = []string{
	"GIT_AUTHOR_NAME=loom factory", "GIT_AUTHOR_EMAIL=factory@loom.invalid",
	"GIT_COMMITTER_NAME=loom factory", "GIT_COMMITTER_EMAIL=factory@loom.invalid",
}

// CommitAll stages every change except the excluded paths and commits it,
// returning the new commit. A run that changed nothing still gets a commit,
// so the branch always records that the run finished.
//
// It stages everything and then unstages the exclusions, rather than passing
// them as `:(exclude)` pathspecs: git counts an exclude pathspec naming an
// ignored path as asking to add it, and fails with "The following paths are
// ignored". Most repositories ignore .claude/memory/, so the first real
// factory commit failed there; the e2e fixture ignores nothing and never did.
func (g *Git) CommitAll(message string, exclude []string) (string, error) {
	if _, err := g.run("add", "-A", "--", "."); err != nil {
		return "", err
	}
	if len(exclude) > 0 {
		if _, err := g.run(append([]string{"reset", "-q", "--"}, exclude...)...); err != nil {
			return "", err
		}
	}
	if err := g.runWithEnv(g.identityEnv(), "commit", "--allow-empty", "-q", "-m", message); err != nil {
		return "", err
	}
	return g.ResolveCommit("HEAD")
}

func (g *Git) identityEnv() []string {
	if output, err := g.run("config", "user.email"); err == nil && strings.TrimSpace(string(output)) != "" {
		return nil
	}
	return committerFallback
}

// Push publishes branch to origin, never prompting for credentials.
func (g *Git) Push(branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "push", "-u", "origin", branch)
	command.Dir = g.root
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git push %s: %w: %s", branch, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *Git) runWithEnv(env []string, args ...string) error {
	command := exec.Command("git", args...)
	command.Dir = g.root
	command.Env = append(os.Environ(), env...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s in %s: %w: %s", args[0], g.root, err, strings.TrimSpace(string(output)))
	}
	return nil
}
