package state

// RepoDocAuditState is the repo-doc-auditor's verdict on one documenter
// result (ADR-010, Phase 5). The deterministic verifier has already confirmed
// every name, tag and citation exists; this judges what a program cannot —
// whether the cited facts actually support the claim made on them.
//
// It is typed so the loop's exit condition is a field lookup, like the review
// loop's (roadmap L2.17), never a parse of prose.

import "fmt"

// DocAuditFinding is one claim the facts do not support, and what to do.
type DocAuditFinding struct {
	Claim       string `json:"claim" jsonschema:"required,description=The purpose or responsibility or invariant text as written in the document"`
	Problem     string `json:"problem" jsonschema:"required,description=Why the cited fact-sheet fields do not establish the claim"`
	Instruction string `json:"instruction" jsonschema:"required,description=What the documenter should do: soften to an open question or cite a field that supports it or remove the claim"`
	Blocking    bool   `json:"blocking,omitempty" jsonschema:"description=False for a suggestion that does not need a revision"`
}

// RepoDocAuditState is the typed output of the repo-doc-auditor.
type RepoDocAuditState struct {
	SchemaVersion int               `json:"schemaVersion" jsonschema:"required"`
	Path          string            `json:"path" jsonschema:"required,description=The directory audited: copy it from the fact sheet"`
	InputHash     string            `json:"inputHash" jsonschema:"required,description=The fact sheet's input_hash: copy it exactly"`
	Verdict       Verdict           `json:"verdict" jsonschema:"required"`
	Findings      []DocAuditFinding `json:"findings"`
}

// IsApproved reads the verdict field: the loop's exit condition.
func (a RepoDocAuditState) IsApproved() bool { return a.Verdict == VerdictApproved }

// Validate enforces what the loop cannot work without. As with the review, it
// does not re-judge the verdict; it requires that a rejection says what to fix.
func (a RepoDocAuditState) Validate() error {
	return firstError(
		requireSchemaVersion(a.SchemaVersion),
		requireText("path", a.Path),
		requireText("inputHash", a.InputHash),
		requireVerdict(a.Verdict),
		validateAuditFindings(a.Findings),
		a.requireActionableChanges(),
	)
}

func validateAuditFindings(findings []DocAuditFinding) error {
	for index, finding := range findings {
		field := fmt.Sprintf("findings[%d]", index)
		if finding.Claim == "" || finding.Problem == "" || finding.Instruction == "" {
			return &ValidationError{Field: field, Reason: "needs a claim and a problem and an instruction"}
		}
	}
	return nil
}

// requireActionableChanges stops a CHANGES_REQUESTED that names nothing the
// documenter could act on from spinning the loop, and an APPROVED that carries
// a blocking finding from contradicting itself.
func (a RepoDocAuditState) requireActionableChanges() error {
	blocking := 0
	for _, finding := range a.Findings {
		if finding.Blocking {
			blocking++
		}
	}
	if a.Verdict == VerdictChangesRequested && blocking == 0 {
		return &ValidationError{Field: "findings", Reason: "CHANGES_REQUESTED needs at least one blocking finding to act on"}
	}
	if a.Verdict == VerdictApproved && blocking > 0 {
		return &ValidationError{Field: "verdict", Reason: "is APPROVED but a finding is blocking"}
	}
	return nil
}

func (a RepoDocAuditState) render() string {
	var out string
	out += fmt.Sprintf("# Audit: %s\n\nVerdict: %s\n\n", a.Path, a.Verdict.literal())
	for _, finding := range a.Findings {
		marker := "suggestion"
		if finding.Blocking {
			marker = "blocking"
		}
		out += fmt.Sprintf("- **%s** (%s) — %s Fix: %s\n", finding.Claim, marker, finding.Problem, finding.Instruction)
	}
	return out
}
