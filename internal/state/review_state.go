package state

// ReviewState is the code-reviewer's output, typed so the loop condition
// reads a field instead of grepping prose (roadmap L2.17).
// review-contract.md says `## Overall Status` "must contain exactly one of
// APPROVED or CHANGES REQUESTED, since the orchestrator's CHANGES
// REQUESTED loop parses this literal string" — that parse is what the
// verdict field replaces for runs under the executor. The rendered view
// still emits the literal, because the markdown pipeline still greps it.

import "fmt"

// Verdict is the code-reviewer's decision, and the loop's exit condition.
type Verdict string

// The two verdicts review-contract.md allows.
const (
	VerdictApproved         Verdict = "APPROVED"
	VerdictChangesRequested Verdict = "CHANGES_REQUESTED"
)

// literal renders a verdict as the bolded string the markdown pipeline
// greps for. CHANGES_REQUESTED is spelled with a space there.
func (v Verdict) literal() string {
	if v == VerdictChangesRequested {
		return "CHANGES REQUESTED"
	}
	return string(v)
}

// DesignScore is the four-dimension rating review-contract.md requires,
// each 1-5. code-reviewer.md's own rule: all four at 3 or higher is an
// approval, anything below 3 is changes requested.
type DesignScore struct {
	Clarity  int `json:"clarity" jsonschema:"required,minimum=1,maximum=5"`
	Cohesion int `json:"cohesion" jsonschema:"required,minimum=1,maximum=5"`
	Coupling int `json:"coupling" jsonschema:"required,minimum=1,maximum=5"`
	Craft    int `json:"craft" jsonschema:"required,minimum=1,maximum=5"`
}

// dimensions lets validation and rendering walk the score without four
// near-identical branches.
func (d DesignScore) dimensions() []struct {
	Name  string
	Score int
} {
	return []struct {
		Name  string
		Score int
	}{
		{"Clarity", d.Clarity}, {"Cohesion", d.Cohesion},
		{"Coupling", d.Coupling}, {"Craft", d.Craft},
	}
}

// Finding is one thing the developer must address before the next round.
// It is structured because it is projected back into the developer's input:
// a finding the next iteration cannot act on is not worth recording.
type Finding struct {
	Operation   string `json:"operation" jsonschema:"required,description=A named refactoring operation or violation, e.g. Extract Function"`
	File        string `json:"file" jsonschema:"required,description=Repo-relative path, with lines where useful"`
	Smell       string `json:"smell" jsonschema:"required,description=What is wrong, in terms of the code"`
	Instruction string `json:"instruction" jsonschema:"required,description=What to do about it"`
	Blocking    bool   `json:"blocking,omitempty" jsonschema:"description=False for the non-blocking suggestions an approval may still carry"`
}

// ReviewState is the typed form of code-review-report.md.
type ReviewState struct {
	SchemaVersion int     `json:"schemaVersion" jsonschema:"required"`
	Feature       string  `json:"feature" jsonschema:"required"`
	Verdict       Verdict `json:"verdict" jsonschema:"required"`

	DesignNarrative string      `json:"designNarrative" jsonschema:"required,description=2-3 sentences on what the code does architecturally"`
	DesignScore     DesignScore `json:"designScore" jsonschema:"required"`

	SecuritySurface    []string `json:"securitySurface,omitempty"`
	PerformanceSurface []string `json:"performanceSurface,omitempty"`
	TestDesignReview   []string `json:"testDesignReview,omitempty"`
	SelfReviewCheck    []string `json:"selfReviewCheck,omitempty" jsonschema:"description=Whether the developer's self-review matched reality"`

	Findings  []Finding `json:"findings,omitempty"`
	Retrieval Retrieval `json:"retrieval,omitempty"`
}

// Validate enforces what the loop and the developer cannot work without.
// Note what it deliberately does NOT do: it does not re-judge whether
// APPROVED was the right call. review-contract.md assigns that to the
// security-reviewer, the qa-engineer, and the human — so an approval
// carrying non-blocking suggestions is valid, and so is one the reader
// disagrees with.
func (r ReviewState) Validate() error {
	return firstError(
		requireSchemaVersion(r.SchemaVersion),
		requireText("feature", r.Feature),
		requireVerdict(r.Verdict),
		requireText("designNarrative", r.DesignNarrative),
		r.DesignScore.validate(),
		validateFindings(r.Findings),
		r.requireActionableChanges(),
	)
}

func requireVerdict(verdict Verdict) error {
	if verdict == VerdictApproved || verdict == VerdictChangesRequested {
		return nil
	}
	return &ValidationError{Field: "verdict",
		Reason: fmt.Sprintf("is %q, want APPROVED or CHANGES_REQUESTED", verdict)}
}

func (d DesignScore) validate() error {
	for _, dimension := range d.dimensions() {
		if dimension.Score < 1 || dimension.Score > 5 {
			return &ValidationError{Field: "designScore." + dimension.Name,
				Reason: fmt.Sprintf("is %d, want a rating from 1 to 5", dimension.Score)}
		}
	}
	return nil
}

// requireActionableChanges is the one rule the loop genuinely depends on:
// a verdict of CHANGES_REQUESTED with nothing for the developer to act on
// would spin the loop to its bound with no possible progress.
func (r ReviewState) requireActionableChanges() error {
	if r.Verdict != VerdictChangesRequested || r.hasBlockingFinding() {
		return nil
	}
	return &ValidationError{Field: "findings",
		Reason: "must include at least one blocking finding when the verdict is CHANGES_REQUESTED — the next iteration has nothing to act on otherwise"}
}

func (r ReviewState) hasBlockingFinding() bool {
	for _, finding := range r.Findings {
		if finding.Blocking {
			return true
		}
	}
	return false
}

func validateFindings(findings []Finding) error {
	for index, finding := range findings {
		if finding.Operation == "" || finding.Instruction == "" {
			return &ValidationError{Field: fmt.Sprintf("findings[%d]", index),
				Reason: "needs an operation and an instruction — a finding the developer cannot act on is not worth recording"}
		}
	}
	return nil
}

// IsApproved is the loop's exit condition, read from a field.
func (r ReviewState) IsApproved() bool { return r.Verdict == VerdictApproved }
