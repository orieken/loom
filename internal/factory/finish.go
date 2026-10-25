package factory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tree is a worktree the finish line commits and pushes.
// internal/worktree.Git satisfies it.
type Tree interface {
	Digest() (string, error)
	ChangedPaths() ([]string, error)
	CommitAll(message string, exclude []string) (string, error)
	ChangedBetween(from, to string) ([]string, error)
	Push(branch string) error
}

// TreeOpener opens the worktree at dir.
type TreeOpener func(dir string) Tree

// PullRequests opens a pull request. It is called from one place only, after
// a human approved that exact proposal (gate #5, Always Human).
type PullRequests interface {
	Open(dir string, proposal Proposal) (string, error)
}

// commitExclusions stay out of a run's commit: the run's working state, the
// factory's own files, and the episodic memory store `loom run` writes —
// a per-machine database, not part of the delivered change.
var commitExclusions = []string{".claude/feature-workspace", StateDir, ".claude/memory"}

// conventionalTypes maps an order's type to its commit prefix.
var conventionalTypes = map[string]string{"feature": "feat", "bug": "fix", "chore": "chore", "spike": "docs"}

// advanceFinishLine moves every completed run toward its finish (roadmap
// L3.69), one step at a time, until each waits on a human or is shipped.
func (s *Supervisor) advanceFinishLine() error {
	for step := 0; step < finishSteps; step++ {
		advanced, err := s.finishPass()
		if err != nil || !advanced {
			return err
		}
	}
	return nil
}

// finishSteps bounds the passes: completed → committed → pushed → proposed
// or shipped is at most four moves for any order.
const finishSteps = 4

func (s *Supervisor) finishPass() (bool, error) {
	orders, err := s.Orders()
	if err != nil {
		return false, err
	}
	advanced := false
	for _, order := range Sorted(orders) {
		moved, err := s.finishStep(order)
		if err != nil {
			return false, err
		}
		advanced = advanced || moved
	}
	return advanced, nil
}

func (s *Supervisor) finishStep(order *OrderState) (bool, error) {
	switch order.Phase {
	case PhaseCompleted:
		return s.finishCompleted(order)
	case PhaseCommitted:
		return s.push(order)
	case PhasePushed:
		return s.afterPush(order)
	case PhaseProposed:
		return s.openApprovedPR(order)
	}
	return false, nil
}

// finishCompleted ships a `finish: none` order as it stands — its worktree
// is left for a human — and commits any other once a human approved the
// commit for the tree as it is now.
func (s *Supervisor) finishCompleted(order *OrderState) (bool, error) {
	if order.Claim.Finish == "none" {
		return true, s.record(Entry{Event: EventShipped, Order: order.ID})
	}
	if !hasApproval(order, GateCommit) {
		return false, nil
	}
	tree := s.deps.Trees(order.Claim.Worktree)
	digest, err := tree.Digest()
	if err != nil {
		return false, err
	}
	if digest != order.Approval.Digest {
		return true, s.record(Entry{Event: EventApprovalDropped, Order: order.ID, Gate: GateCommit,
			Reason: "the worktree changed after the commit was approved — review the inbox and approve again"})
	}
	commit, err := tree.CommitAll(commitMessage(order), commitExclusions)
	if err != nil {
		return false, fmt.Errorf("commit %s: %w", order.ID, err)
	}
	return true, s.record(Entry{Event: EventCommitted, Order: order.ID, Commit: commit})
}

func hasApproval(order *OrderState, gate string) bool {
	return order.Approval != nil && order.Approval.Gate == gate
}

// push publishes the branch. A failure is recorded once per distinct reason
// and retried every tick: a push usually fails for a reason a human fixes
// outside the factory, such as credentials.
func (s *Supervisor) push(order *OrderState) (bool, error) {
	err := s.deps.Trees(order.Claim.Worktree).Push(order.Claim.Branch)
	if err == nil {
		return true, s.record(Entry{Event: EventPushed, Order: order.ID})
	}
	if reason := "push: " + err.Error(); reason != order.Reason {
		return false, s.record(Entry{Event: EventFinishFailed, Order: order.ID, Reason: reason})
	}
	return false, nil
}

// afterPush ships a `finish: branch` order and proposes a pull request for
// a `finish: pr` one. Proposing is all it does: opening the PR waits on a
// human in the inbox.
func (s *Supervisor) afterPush(order *OrderState) (bool, error) {
	if order.Claim.Finish != "pr" {
		return true, s.ship(order)
	}
	proposal := s.proposalFor(order)
	return true, s.record(Entry{Event: EventProposed, Order: order.ID, Proposal: &proposal})
}

// openApprovedPR opens the pull request only when a human approved this
// exact proposal. This is the only call to PullRequests.Open in the package.
func (s *Supervisor) openApprovedPR(order *OrderState) (bool, error) {
	if !hasApproval(order, GateOpenPR) || order.Approval.Digest != order.Proposal.Digest() {
		return false, nil
	}
	url, err := s.deps.Pulls.Open(order.Claim.Worktree, *order.Proposal)
	if err != nil {
		if reason := "open pull request: " + err.Error(); reason != order.Reason {
			return false, s.record(Entry{Event: EventFinishFailed, Order: order.ID, Reason: reason})
		}
		return false, nil
	}
	if err := s.record(Entry{Event: EventPROpened, Order: order.ID, URL: url}); err != nil {
		return false, err
	}
	return true, s.ship(order)
}

// ship records the order done, archives the run's workspace — its state,
// timeline and artifacts are never committed, and removing the worktree
// would otherwise delete them — then removes the worktree. The order is
// that way round so a crash in between leaves a shipped order with a
// leftover worktree, never a missing worktree for an unshipped one.
func (s *Supervisor) ship(order *OrderState) error {
	if err := s.record(Entry{Event: EventShipped, Order: order.ID}); err != nil {
		return err
	}
	if err := archiveRun(order, s.config.Layout.RunArchivePath(order.ID)); err != nil {
		s.say("%s shipped, but its run state was not archived, so its worktree is kept: %v", order.ID, err)
		return nil
	}
	if err := s.deps.Repository.RemoveWorktree(order.Claim.Worktree); err != nil {
		s.say("%s shipped, but its worktree was not removed: %v", order.ID, err)
	}
	return nil
}

func commitMessage(order *OrderState) string {
	return fmt.Sprintf("%s\n\nFactory order %s, claimed from %s at %s.\n", subject(order), order.ID,
		order.Claim.OrderPath, order.Claim.SourceCommit)
}

func subject(order *OrderState) string {
	prefix := conventionalTypes[order.Claim.Type]
	if prefix == "" {
		prefix = "chore"
	}
	title := order.Claim.Title
	if title == "" {
		title = "deliver " + order.ID
	}
	return fmt.Sprintf("%s(%s): %s", prefix, order.ID, title)
}

// proposalFor builds the pull request a human will be asked to open. Its
// body is the delivery summary (summary.go).
func (s *Supervisor) proposalFor(order *OrderState) Proposal {
	return Proposal{Title: subject(order), Body: s.proposalBody(order),
		Base: order.Claim.BaseBranch, Head: order.Claim.Branch}
}

// GHPullRequests opens pull requests with the GitHub CLI.
type GHPullRequests struct {
	Timeout time.Duration
}

// Open runs `gh pr create` in dir and returns the new pull request's URL.
func (g GHPullRequests) Open(dir string, proposal Proposal) (string, error) {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "gh", "pr", "create", "--base", proposal.Base, "--head", proposal.Head,
		"--title", proposal.Title, "--body", proposal.Body)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return lastLine(string(output)), nil
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// archiveRun copies the run's workspace out of the worktree.
func archiveRun(order *OrderState, destination string) error {
	source := filepath.Dir(RunStatePath(order.Claim.Worktree, order.ID))
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
