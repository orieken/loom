package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func approvedReview() state.ReviewState {
	return state.ReviewState{
		SchemaVersion:   state.SchemaVersion,
		Feature:         "user-auth",
		Verdict:         state.VerdictApproved,
		DesignNarrative: "Session issuing sits in the use-case layer behind a repository interface.",
		DesignScore:     state.DesignScore{Clarity: 4, Cohesion: 4, Coupling: 5, Craft: 4},
		TestDesignReview: []string{
			"YES — session_test.go: expired-token case fails if the expiry check is removed.",
		},
	}
}

func changesRequestedReview() state.ReviewState {
	review := approvedReview()
	review.Verdict = state.VerdictChangesRequested
	review.DesignScore.Cohesion = 2
	review.Findings = []state.Finding{{
		Operation: "Extract Function", File: "internal/identity/session.go:40-95",
		Smell:       "issueSession both validates credentials and writes the audit record",
		Instruction: "Extract the audit write into recordSignIn(session)",
		Blocking:    true,
	}}
	return review
}

func TestVerdictIsAFieldNotAString(t *testing.T) {
	if !approvedReview().IsApproved() {
		t.Error("an approved review does not read as approved")
	}
	if changesRequestedReview().IsApproved() {
		t.Error("a changes-requested review read as approved")
	}
}

func TestReviewValidationNamesTheOffendingField(t *testing.T) {
	cases := map[string]func(*state.ReviewState){
		"feature":         func(r *state.ReviewState) { r.Feature = "" },
		"verdict":         func(r *state.ReviewState) { r.Verdict = "LGTM" },
		"designNarrative": func(r *state.ReviewState) { r.DesignNarrative = "" },
		"designScore":     func(r *state.ReviewState) { r.DesignScore.Craft = 9 },
		"schemaVersion":   func(r *state.ReviewState) { r.SchemaVersion = 99 },
	}
	for field, breakField := range cases {
		t.Run(field, func(t *testing.T) {
			review := approvedReview()
			breakField(&review)
			assertValidationNames(t, review.Validate(), field)
		})
	}
}

// A review that answers nothing about the tests in its diff is the defect
// L3.41 was built to remove, arriving through the typed path instead of the
// prose one. review-contract.md has always listed `## Test Design Review`
// as required, and `validate-artifact` checks the heading — but an empty
// list renders as the word "None" under that heading, so the rendered
// artifact satisfied the markdown contract while carrying no judgment.
//
// Required unconditionally, which is what the contract says. A diff that
// changes no tests answers in a sentence; "there were none to judge" and
// "we did not look" are different facts and the reader cannot tell them
// apart from an absent list.
func TestAReviewMustAnswerWhetherItsTestsWouldFail(t *testing.T) {
	review := approvedReview()
	review.TestDesignReview = nil

	assertValidationNames(t, review.Validate(), "testDesignReview")

	// Saying the diff had no tests is a valid answer. Silence is not.
	review.TestDesignReview = []string{"No tests added or modified in this diff."}
	if err := review.Validate(); err != nil {
		t.Errorf("a review stating the diff changed no tests was rejected: %v", err)
	}
}

// TestChangesRequestedNeedsSomethingActionable is the one rule the loop
// depends on: a rejection with nothing to act on would spin to the bound
// with no possible progress.
func TestChangesRequestedNeedsSomethingActionable(t *testing.T) {
	review := changesRequestedReview()
	review.Findings = nil

	assertValidationNames(t, review.Validate(), "findings")

	// A non-blocking suggestion is not enough to send the developer back.
	review.Findings = []state.Finding{{
		Operation: "Rename Variable", File: "x.go", Smell: "unclear", Instruction: "rename it", Blocking: false,
	}}
	if review.Validate() == nil {
		t.Error("a rejection carrying only non-blocking suggestions was accepted")
	}
}

// TestApprovalMayCarryNonBlockingSuggestions pins what validation
// deliberately does not do: review-contract.md assigns judging whether
// APPROVED was the right call to the security-reviewer, the qa-engineer,
// and the human — not to this schema.
func TestApprovalMayCarryNonBlockingSuggestions(t *testing.T) {
	review := approvedReview()
	review.Findings = []state.Finding{{
		Operation: "Rename Variable", File: "internal/identity/session.go",
		Smell: "tmp is not a name", Instruction: "call it pendingSession", Blocking: false,
	}}

	if err := review.Validate(); err != nil {
		t.Errorf("an approval with minor suggestions was rejected: %v", err)
	}
}

func TestFindingMustBeActionable(t *testing.T) {
	review := changesRequestedReview()
	review.Findings[0].Instruction = ""

	assertValidationNames(t, review.Validate(), "findings[0]")
}

func TestReviewRoundTripsThroughItsSchema(t *testing.T) {
	raw, err := json.Marshal(changesRequestedReview())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, err := state.Decode(state.KindReview, raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	review, ok := decoded.(*state.ReviewState)
	if !ok || review.Verdict != state.VerdictChangesRequested || len(review.Findings) != 1 {
		t.Errorf("decoded review = %+v", decoded)
	}
}

// TestRenderedReviewKeepsTheLiteralTheMarkdownPipelineGreps is the
// compatibility requirement: review-contract.md says the loop parses that
// exact string, and the markdown pipeline still does.
func TestRenderedReviewKeepsTheLiteralTheMarkdownPipelineGreps(t *testing.T) {
	for _, tc := range []struct {
		review  state.ReviewState
		literal string
	}{
		{approvedReview(), "**APPROVED**"},
		{changesRequestedReview(), "**CHANGES REQUESTED**"},
	} {
		raw, err := json.Marshal(tc.review)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		name, body, err := state.RenderView(state.KindReview, raw)
		if err != nil {
			t.Fatalf("RenderView: %v", err)
		}
		if name != "code-review-report.md" {
			t.Errorf("rendered to %q, want the contract's filename", name)
		}
		if !strings.Contains(body, tc.literal) {
			t.Errorf("rendered review does not carry %q:\n%s", tc.literal, body)
		}
	}
}

func TestRenderedReviewCarriesEveryContractHeading(t *testing.T) {
	raw, err := json.Marshal(changesRequestedReview())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	_, body, err := state.RenderView(state.KindReview, raw)
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}

	for _, heading := range []string{
		"## Overall Status", "## Design Narrative", "## Design Score", "## Security Surface",
		"## Performance Surface", "## Test Design Review",
		"## Verification of Developer Self-Review", "## Feedback for the Developer",
	} {
		if !strings.Contains(body, "\n"+heading+"\n") {
			t.Errorf("rendered review is missing the contract heading %q", heading)
		}
	}
	// The contract requires all four dimensions with a numeric rating.
	for _, dimension := range []string{"Clarity", "Cohesion", "Coupling", "Craft"} {
		if !strings.Contains(body, "**"+dimension+"**") {
			t.Errorf("rendered review is missing the %s score", dimension)
		}
	}
}

// TestReviewProjectionGivesTheDeveloperOnlyWhatItActsOn covers the loop's
// feedback path: the next iteration gets instructions, not an assessment.
func TestReviewProjectionGivesTheDeveloperOnlyWhatItActsOn(t *testing.T) {
	raw, err := json.Marshal(changesRequestedReview())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	projected, err := state.ProjectionFor("developer", state.KindReview, raw)
	if err != nil {
		t.Fatalf("ProjectFor: %v", err)
	}

	assertDeveloperInput(t, projected)
	assertProjectionOmits(t, projected, "designScore", "securitySurface", "designNarrative")
}

func assertDeveloperInput(t *testing.T, projected []byte) {
	t.Helper()
	var received state.DeveloperInput
	if err := json.Unmarshal(projected, &received); err != nil {
		t.Fatalf("projection does not parse: %v", err)
	}
	if received.Verdict != state.VerdictChangesRequested || len(received.Findings) != 1 {
		t.Fatalf("projection = %+v, want the verdict and its findings", received)
	}
	if received.Findings[0].Instruction != changesRequestedReview().Findings[0].Instruction {
		t.Error("the instruction was altered in transit; projections select fields, they do not summarise")
	}
}
