package state_test

import (
	"fmt"
	"testing"

	"github.com/orieken/loom/internal/state"
)

// sizerFor returns a FileSizer over a fixed table, standing in for the
// filesystem the executor injects.
func sizerFor(sizes map[string]int64) state.FileSizer {
	return func(path string) (int64, error) {
		size, known := sizes[path]
		if !known {
			return 0, fmt.Errorf("cannot read %s", path)
		}
		return size, nil
	}
}

func manifestOver(paths ...string) state.ContextState {
	pinned := make([]state.PinnedFile, 0, len(paths))
	for _, path := range paths {
		pinned = append(pinned, state.PinnedFile{Path: path, Reason: "needed"})
	}
	return state.ContextState{SchemaVersion: state.SchemaVersion, Feature: "console-log-filtering",
		Tier: "analyst", PinnedFiles: pinned}
}

// The third real run's own pinned set, with the byte counts recorded in the
// audit. context-engineer.md asked the model to estimate tokens at
// "line count x 8 chars/line / 4 chars/token" and it reported 1,350 against
// a real ~9,100 — roughly 7x under, presented with a per-file breakdown, a
// recomputation, a percentage and an OK status.
func TestTheThirdRunsManifestIsNoLongerSevenTimesUnder(t *testing.T) {
	const agentClaimedTokens = 1350
	sizes := map[string]int64{
		"ARCHITECTURE_RULES.md":                              14949,
		"DOMAIN_DICTIONARY.md":                               18530,
		"packages/saturday-core/src/utils/console-logger.ts": 916,
	}
	manifest := manifestOver("ARCHITECTURE_RULES.md", "DOMAIN_DICTIONARY.md",
		"packages/saturday-core/src/utils/console-logger.ts")

	manifest.Measure(sizerFor(sizes))

	// bytes/4 over the same three files: 3737 + 4632 + 229.
	const want = (14949 + 18530 + 916) / 4
	if got := manifest.Budget.EstimatedTokens; got != want {
		t.Fatalf("estimate = %d, want %d", got, want)
	}
	if manifest.Budget.EstimatedTokens < agentClaimedTokens*4 {
		t.Errorf("estimate %d is still close to the agent's %d — the 7x gap is not closed",
			manifest.Budget.EstimatedTokens, agentClaimedTokens)
	}
}

// The per-line rule the old prompt used is wrong by an order of magnitude
// for prose, which is the specific claim L3.25 rests on.
func TestThePerLineRuleIsWrongByAnOrderOfMagnitudeForProse(t *testing.T) {
	const architectureRulesLines, architectureRulesBytes = 188, 14949

	perLineEstimate := int64(architectureRulesLines * 8 / 4)
	measured := state.EstimateTokens(architectureRulesBytes)

	if measured < perLineEstimate*5 {
		t.Errorf("bytes/4 gave %d and lines*2 gave %d; the gap L3.25 documents is not reproduced",
			measured, perLineEstimate)
	}
}

// Whatever an agent writes in the budget is replaced by measurement. The
// budget being asserted rather than computed is the defect, so an asserted
// value surviving would leave it intact.
func TestAnAgentSuppliedBudgetIsDiscarded(t *testing.T) {
	manifest := manifestOver("a.md")
	manifest.Budget = &state.ContextBudget{EstimatedTokens: 42, Status: state.BudgetStatusOK}

	manifest.Measure(sizerFor(map[string]int64{"a.md": 40000}))

	if manifest.Budget.EstimatedTokens != 10000 {
		t.Errorf("estimate = %d, want the measured 10000 rather than the agent's 42",
			manifest.Budget.EstimatedTokens)
	}
}

// A budget 7x under reports OK right up to the point it overflows. Now that
// it is measured, the overflow is reported.
func TestABudgetOverTheTierLimitReportsWarning(t *testing.T) {
	manifest := manifestOver("huge.md")
	manifest.Tier = "reviewer" // 40% of 200k = 80,000 tokens

	manifest.Measure(sizerFor(map[string]int64{"huge.md": 81_000 * 4}))

	if manifest.Budget.Status != state.BudgetStatusWarning {
		t.Errorf("status = %q for 81k tokens against an 80k budget, want WARNING", manifest.Budget.Status)
	}
}

func TestEachTierCarriesItsOwnLimit(t *testing.T) {
	cases := map[string]int64{"analyst": 120_000, "developer": 160_000, "reviewer": 80_000}
	for tier, want := range cases {
		if got := state.TierLimitTokens(tier); got != want {
			t.Errorf("tier %q limit = %d, want %d", tier, got, want)
		}
	}
	// An unrecognised tier must not buy a bigger allowance than a known one.
	if got := state.TierLimitTokens("wizard"); got != cases["reviewer"] {
		t.Errorf("unknown tier limit = %d, want the strictest (%d)", got, cases["reviewer"])
	}
}

// A file that cannot be read is reported, never counted as zero: silently
// omitting it produces exactly the kind of confident undercount L3.25 is
// about.
func TestAnUnreadablePinnedFileIsReportedNotCountedAsZero(t *testing.T) {
	manifest := manifestOver("present.md", "missing.md")

	manifest.Measure(sizerFor(map[string]int64{"present.md": 4000}))

	if manifest.Budget.Unmeasured != 1 {
		t.Errorf("unmeasured count = %d, want 1", manifest.Budget.Unmeasured)
	}
	view := manifest.Budget.Summary("analyst")
	if view == "" {
		t.Error("budget summary is empty")
	}
	var reported bool
	for _, file := range manifest.Budget.Files {
		if file.Path == "missing.md" && file.Unmeasured != "" {
			reported = true
		}
	}
	if !reported {
		t.Error("the unreadable file is absent from the breakdown, so the total looks complete")
	}
}

// A budget that fits but could not see every file is not a pass. Reporting
// OK on a knowably short total is the same defect L3.25 exists for, one
// level down.
func TestABudgetMissingAFileDoesNotReportOK(t *testing.T) {
	manifest := manifestOver("present.md", "missing.md")

	manifest.Measure(sizerFor(map[string]int64{"present.md": 4000}))

	if manifest.Budget.Status != state.BudgetStatusIncomplete {
		t.Errorf("status = %q with an unmeasured file, want INCOMPLETE", manifest.Budget.Status)
	}
}

// Over budget stays over budget: a missing file must not downgrade a real
// overflow into an unknown.
func TestAnOverflowOutranksAnUnmeasuredFile(t *testing.T) {
	manifest := manifestOver("huge.md", "missing.md")
	manifest.Tier = "reviewer"

	manifest.Measure(sizerFor(map[string]int64{"huge.md": 81_000 * 4}))

	if manifest.Budget.Status != state.BudgetStatusWarning {
		t.Errorf("status = %q, want WARNING — an overflow is not downgraded by a missing file",
			manifest.Budget.Status)
	}
}
