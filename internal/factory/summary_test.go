package factory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// The first real factory PR's description said only that no summary had
// been written, and pointed at a worktree deleted moments later. The
// description is now the delivery summary, built from the run's own state.

// changedFiles is a tree that answers only what the summary asks of it.
type changedFiles struct {
	Tree
	paths []string
	err   error
}

func (c changedFiles) ChangedBetween(string, string) ([]string, error) { return c.paths, c.err }

func summarySupervisor(tree Tree) *Supervisor {
	return New(Config{}, Dependencies{Trees: func(string) Tree { return tree }})
}

func summaryOrder(worktree string) *OrderState {
	return &OrderState{ID: "alpha", Commit: "c2", Claim: Claim{
		Worktree: worktree, OrderPath: "intake/alpha.md", SourceCommit: "0123456789abcdef0123",
		Budget: intake.Budget{MaxUSD: 25},
	}}
}

func writeTypedState(t *testing.T, worktree, stage, kind string) string {
	t.Helper()
	script, ok := mock.TypedScript(kind)
	if !ok {
		t.Fatalf("the mock has no %s document", kind)
	}
	path := filepath.Join(filepath.Dir(RunStatePath(worktree, "alpha")), "state", stage+".json")
	writeTestFile(t, path, string(script.Payload))
	return path
}

func saveRunState(t *testing.T, worktree string, runState *orchestrator.RunState) {
	t.Helper()
	if err := orchestrator.NewStateStore(RunStatePath(worktree, "alpha")).Save(runState); err != nil {
		t.Fatal(err)
	}
}

func deliveredRun(t *testing.T, worktree string) *orchestrator.RunState {
	t.Helper()
	completed := orchestrator.StageStatusCompleted
	runState := stateWith(map[string]orchestrator.StageRecord{
		"analyst": {Status: completed, Sequence: 1, Agent: "analyst", StateKind: "analysis",
			ArtifactPath: writeTypedState(t, worktree, "analyst", "analysis"), Usage: &orchestrator.Usage{CostUSD: 1.5}},
		"data-engineer": {Status: orchestrator.StageStatusSkipped, Sequence: 2, SkipReason: "no data-model change"},
		"code-reviewer": {Status: completed, Sequence: 3, Agent: "code-reviewer", StateKind: "review",
			ArtifactPath: writeTypedState(t, worktree, "code-reviewer", "review"), Usage: &orchestrator.Usage{CostUSD: 0.5}},
		"security-reviewer": {Status: completed, Sequence: 4, Agent: "security-reviewer", StateKind: "security",
			ArtifactPath: writeTypedState(t, worktree, "security-reviewer", "security")},
		"qa-engineer": {Status: completed, Sequence: 5, Agent: "qa-engineer", StateKind: "qa",
			ArtifactPath: writeTypedState(t, worktree, "qa-engineer", "qa")},
	})
	runState.Approvals = map[string]orchestrator.Approval{"confirm-design": {
		ApprovedAt: time.Date(2026, 10, 24, 23, 0, 0, 0, time.UTC), Method: orchestrator.ApprovalMethodFlag, Approver: "oscar",
	}}
	return runState
}

func TestThePullRequestDescriptionIsTheDeliverySummary(t *testing.T) {
	worktree := t.TempDir()
	writeTestFile(t, filepath.Join(worktree, ClaimPath("alpha")),
		"# Deliver alpha\n\n## Summary\n\nTie the vocabulary to the Go sets.\n\n### Detail\n\nKept.\n\n## Out of Scope\n\nDropped.\n")
	saveRunState(t, worktree, deliveredRun(t, worktree))
	tree := changedFiles{paths: []string{"internal/alpha.go", "docs/features/alpha/analysis.md"}}

	body := summarySupervisor(tree).proposalBody(summaryOrder(worktree))

	for _, want := range []string{
		"work order `alpha` (`intake/alpha.md` at `0123456789ab`)",
		"## Request\n\nTie the vocabulary to the Go sets.\n\n### Detail\n\nKept.\n",
		"- **Code review** (`code-reviewer`): APPROVED",
		"- **Security review** (`security-reviewer`): no findings",
		"- **Tests** (`qa-engineer`): 1 passed, 0 failed, 0 skipped",
		"| `analyst` | analyst | completed | $1.50 |",
		"| `data-engineer` | — | skipped: no data-model change | — |",
		"| `security-reviewer` | security-reviewer | completed | — |",
		"Measured spend: **$2.00** of the order's $25.00 ceiling.",
		"- `confirm-design`: approved by oscar via flag at 2026-10-24 23:00 UTC",
		"## Changed files\n\n- `internal/alpha.go`\n",
		"committed in this pull request under `docs/features/alpha/`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("description lacks %q:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"Dropped.", "- `docs/features/alpha/analysis.md`", "## The run's own summary", "Analysis"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("description holds %q:\n%s", unwanted, body)
		}
	}
}

func TestTheDescriptionAppendsASummaryTheRunWroteItself(t *testing.T) {
	worktree := t.TempDir()
	writeTestFile(t, filepath.Join(worktree, "docs", "features", "alpha", "delivery-summary.md"), "  Shipped it.\n")

	body := summarySupervisor(changedFiles{}).proposalBody(summaryOrder(worktree))

	if !strings.Contains(body, "## The run's own summary\n\nShipped it.\n") {
		t.Errorf("description lacks the run's summary:\n%s", body)
	}
	if !strings.Contains(body, "None outside the delivery record.") {
		t.Errorf("an empty change list was not said:\n%s", body)
	}
}

// Every part that cannot be read says so in place: a summary that silently
// dropped the stages would read as a run that had none.
func TestTheDescriptionSaysWhatItCouldNotRead(t *testing.T) {
	worktree := t.TempDir()
	order := summaryOrder(worktree)
	order.Claim.Budget.MaxUSD = 0
	tree := changedFiles{err: errors.New("no such commit")}

	if body := summarySupervisor(tree).proposalBody(order); !strings.Contains(body, "The run left no state") ||
		!strings.Contains(body, "The changed files could not be listed: no such commit") {
		t.Errorf("description without state or files:\n%s", body)
	}

	writeTestFile(t, RunStatePath(worktree, "alpha"), "{not json")
	if body := summarySupervisor(tree).proposalBody(order); !strings.Contains(body, "The run's state could not be read") {
		t.Errorf("description with unreadable state:\n%s", body)
	}

	runState := stateWith(map[string]orchestrator.StageRecord{
		"code-reviewer": {Status: orchestrator.StageStatusCompleted, Sequence: 1, StateKind: "review",
			ArtifactPath: filepath.Join(worktree, "missing.json")},
		"qa-engineer": {Status: orchestrator.StageStatusCompleted, Sequence: 2, StateKind: "qa",
			ArtifactPath: RunStatePath(worktree, "alpha")},
	})
	saveRunState(t, worktree, runState)
	body := summarySupervisor(tree).proposalBody(order)
	for _, want := range []string{"- `code-reviewer`: its state could not be read", "- `qa-engineer`: its state could not be decoded",
		"(the order set no USD ceiling)"} {
		if !strings.Contains(body, want) {
			t.Errorf("description lacks %q:\n%s", want, body)
		}
	}
}

func TestFindingCountsNameEachSeverity(t *testing.T) {
	findings := []state.SecurityFinding{{Severity: state.SeverityLow}, {Severity: state.SeverityCritical}, {Severity: state.SeverityLow}}
	if got := findingCounts(findings); got != "3 finding(s): 1 critical, 2 low" {
		t.Errorf("findingCounts = %q", got)
	}
}

func TestSectionBodyIsEmptyForAMissingHeading(t *testing.T) {
	if got := sectionBody("# Title\n\n## Other\n\ntext\n", "Summary"); got != "" {
		t.Errorf("sectionBody = %q, want nothing", got)
	}
}

func TestShortCommitKeepsAShortCommit(t *testing.T) {
	if got := shortCommit("abc"); got != "abc" {
		t.Errorf("shortCommit = %q", got)
	}
}

// A claim snapshot that is missing leaves the request out rather than
// failing the description.
func TestTheRequestIsLeftOutWithoutAClaimSnapshot(t *testing.T) {
	worktree := t.TempDir()
	if body := summarySupervisor(changedFiles{}).proposalBody(summaryOrder(worktree)); strings.Contains(body, "## Request") {
		t.Errorf("a request appeared with no snapshot:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(worktree, ClaimPath("alpha"))); err == nil {
		t.Fatal("the test wrote a snapshot")
	}
}
