package factory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/worktree"
)

// Fetch-before-read (the request behind this test file): a pass fetches its
// configured remote once, then reads intake/ from the fetched remote-
// tracking ref, so a promotion merged on GitHub is seen without a person
// pulling. A repository with no such remote, or --no-fetch, reads the local
// branch exactly as before this behavior existed.

// fetchSpy counts Repository.Fetch calls, to verify the one-per-pass bound
// (AC1) without caring what fetch itself does.
type fetchSpy struct {
	*worktree.Git
	calls int
}

func (r *fetchSpy) Fetch(remote, branch string) error {
	r.calls++
	return r.Git.Fetch(remote, branch)
}

func TestAPassFetchesTheRemoteOnceAndClaimsOrdersPromotedThere(t *testing.T) {
	factory := newRemoteTestFactory(t, false)
	spy := &fetchSpy{Git: worktree.New(factory.root)}
	factory.supervisor.deps.Repository = spy

	commit := factory.pushOnly(
		orderSpec{id: "alpha", omitBaseBranch: true},
		orderSpec{id: "bravo", omitBaseBranch: true},
	)
	factory.tick()

	if spy.calls != 1 {
		t.Errorf("Fetch called %d times for one pass claiming two orders, want exactly 1", spy.calls)
	}
	for _, id := range []string{"alpha", "bravo"} {
		factory.assertPhase(id, PhaseRunning)
		if got := factory.claimOf(id).SourceCommit; got != commit {
			t.Errorf("%s claimed sourceCommit = %q, want the fetched commit %q", id, got, commit)
		}
	}
}

func TestAClaimWithNoBaseBranchStartsFromTheFetchedRemoteRef(t *testing.T) {
	factory := newRemoteTestFactory(t, false)
	factory.pushOnly(orderSpec{id: "alpha", omitBaseBranch: true})
	factory.tick()

	claim := factory.claimOf("alpha")
	if claim.BaseBranch != "origin/main" {
		t.Errorf("BaseBranch = %q, want origin/main", claim.BaseBranch)
	}
	branchCommit, err := worktree.New(claim.Worktree).ResolveCommit(claim.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if branchCommit != claim.SourceCommit {
		t.Errorf("worktree branch %s is at %q, want it to start from the fetched commit %q",
			claim.Branch, branchCommit, claim.SourceCommit)
	}
}

func TestAPassNeverDisturbsThePersonsOwnCheckout(t *testing.T) {
	factory := newRemoteTestFactory(t, false, orderSpec{id: "alpha"})
	beforeHead := runGit(t, factory.root, "rev-parse", "HEAD")
	beforeBranch := runGit(t, factory.root, "rev-parse", "--abbrev-ref", "HEAD")
	writeTestFile(t, filepath.Join(factory.root, "WIP.md"), "uncommitted\n")
	runGit(t, factory.root, "add", "WIP.md")
	beforeStatus := runGit(t, factory.root, "status", "--porcelain")

	factory.pushOnly(orderSpec{id: "bravo", omitBaseBranch: true})
	factory.tick()

	if got := runGit(t, factory.root, "rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("HEAD changed from %q to %q", beforeHead, got)
	}
	if got := runGit(t, factory.root, "rev-parse", "--abbrev-ref", "HEAD"); got != beforeBranch {
		t.Errorf("current branch changed from %q to %q", beforeBranch, got)
	}
	if got := runGit(t, factory.root, "status", "--porcelain"); got != beforeStatus {
		t.Errorf("working tree or index changed:\nbefore: %q\nafter:  %q", beforeStatus, got)
	}
}

func TestAFailedFetchContinuesThePassAndLogsOneLine(t *testing.T) {
	factory := newRemoteTestFactory(t, false, orderSpec{id: "alpha", omitBaseBranch: true})
	if err := os.RemoveAll(factory.upstream); err != nil {
		t.Fatal(err)
	}
	factory.output.Reset()

	factory.tick()

	factory.assertPhase("alpha", PhaseRunning)
	if got := factory.outputCount("fetch"); got != 1 {
		t.Errorf("fetch-failure lines printed = %d, want exactly 1 (output: %q)", got, factory.output.String())
	}
	if claim := factory.claimOf("alpha"); claim.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want the local branch main after a fetch that never resolved origin/main", claim.BaseBranch)
	}
}

func TestNoRemoteByTheConfiguredNameReadsTheLocalBranchUnchanged(t *testing.T) {
	factory := newTestFactory(t, orderSpec{id: "alpha"})
	spy := &fetchSpy{Git: worktree.New(factory.root)}
	factory.supervisor.deps.Repository = spy
	factory.output.Reset()

	factory.tick()

	if spy.calls != 0 {
		t.Errorf("Fetch called %d times with no remote configured, want 0", spy.calls)
	}
	if got := factory.outputCount("fetch"); got != 0 {
		t.Errorf("fetch-related output printed with no remote configured: %q", factory.output.String())
	}
	factory.assertPhase("alpha", PhaseRunning)
}

func TestNoFetchReadsTheLocalBranchEvenWhenARemoteExists(t *testing.T) {
	factory := newRemoteTestFactory(t, true, orderSpec{id: "alpha", omitBaseBranch: true})
	spy := &fetchSpy{Git: worktree.New(factory.root)}
	factory.supervisor.deps.Repository = spy
	factory.output.Reset()

	factory.tick()

	if spy.calls != 0 {
		t.Errorf("Fetch called %d times with --no-fetch, want 0", spy.calls)
	}
	if got := factory.outputCount("fetch"); got != 0 {
		t.Errorf("fetch-related output printed with --no-fetch: %q", factory.output.String())
	}
	if claim := factory.claimOf("alpha"); claim.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want the local branch main with --no-fetch", claim.BaseBranch)
	}
}

func TestALaterFetchDoesNotMoveARunningOrdersClaimedCommit(t *testing.T) {
	factory := newRemoteTestFactory(t, false)
	original := factory.pushOnly(orderSpec{id: "alpha", omitBaseBranch: true})
	factory.launcher.script("alpha", run{hold: true, state: completedState})
	factory.tick()
	factory.assertPhase("alpha", PhaseRunning)
	if got := factory.claimOf("alpha").SourceCommit; got != original {
		t.Fatalf("initial claim sourceCommit = %q, want %q", got, original)
	}

	factory.advanceUpstream()
	factory.tick()

	if got := factory.claimOf("alpha").SourceCommit; got != original {
		t.Errorf("claimed sourceCommit changed to %q after a later fetch moved the remote forward, want it to stay %q",
			got, original)
	}
}
