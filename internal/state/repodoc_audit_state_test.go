package state_test

import (
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func blockingFinding() state.DocAuditFinding {
	return state.DocAuditFinding{Claim: "signs every record", Problem: "Digest does not establish signing", Instruction: "move it to openQuestions", Blocking: true}
}

func approvedAudit() state.RepoDocAuditState {
	return state.RepoDocAuditState{SchemaVersion: state.SchemaVersion, Path: "internal/ledger", InputHash: "sha256:h", Verdict: state.VerdictApproved}
}

func rejectedAudit() state.RepoDocAuditState {
	audit := approvedAudit()
	audit.Verdict, audit.Findings = state.VerdictChangesRequested, []state.DocAuditFinding{blockingFinding()}
	return audit
}

func TestValidAuditsPass(t *testing.T) {
	for name, audit := range map[string]state.RepoDocAuditState{"approved": approvedAudit(), "rejected": rejectedAudit()} {
		if err := audit.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAuditValidationNamesTheBrokenField(t *testing.T) {
	cases := map[string]func(*state.RepoDocAuditState){
		"schemaVersion": func(a *state.RepoDocAuditState) { a.SchemaVersion = 0 },
		"path":          func(a *state.RepoDocAuditState) { a.Path = "" },
		"inputHash":     func(a *state.RepoDocAuditState) { a.InputHash = "" },
		"verdict":       func(a *state.RepoDocAuditState) { a.Verdict = "MAYBE" },
		"findings[0]":   func(a *state.RepoDocAuditState) { a.Findings[0].Instruction = "" },
	}
	for field, breakIt := range cases {
		t.Run(field, func(t *testing.T) {
			audit := rejectedAudit()
			breakIt(&audit)
			assertValidationNames(t, audit.Validate(), field)
		})
	}
}

func TestEveryFindingNeedsAClaimAProblemAndAnInstruction(t *testing.T) {
	for _, mutate := range []func(*state.DocAuditFinding){
		func(f *state.DocAuditFinding) { f.Claim = "" },
		func(f *state.DocAuditFinding) { f.Problem = "" },
		func(f *state.DocAuditFinding) { f.Instruction = "" },
	} {
		audit := rejectedAudit()
		mutate(&audit.Findings[0])
		assertValidationNames(t, audit.Validate(), "findings[0]")
	}
}

// A rejection with nothing to act on would spin the loop.
func TestChangesRequestedNeedsABlockingFinding(t *testing.T) {
	audit := rejectedAudit()
	audit.Findings[0].Blocking = false
	assertValidationNames(t, audit.Validate(), "findings")
	audit.Findings = nil
	assertValidationNames(t, audit.Validate(), "findings")
}

func TestAnApprovalMayCarrySuggestionsButNotABlockingFinding(t *testing.T) {
	audit := approvedAudit()
	suggestion := blockingFinding()
	suggestion.Blocking = false
	audit.Findings = []state.DocAuditFinding{suggestion}
	if err := audit.Validate(); err != nil {
		t.Errorf("an approval with a suggestion is valid: %v", err)
	}
	audit.Findings = []state.DocAuditFinding{blockingFinding()}
	assertValidationNames(t, audit.Validate(), "verdict")
}

func TestIsApprovedReadsTheVerdictField(t *testing.T) {
	if !approvedAudit().IsApproved() || rejectedAudit().IsApproved() {
		t.Error("the loop's exit condition must be the verdict field")
	}
}

func TestAuditDecodesRejectsUnknownFieldsAndHasASchema(t *testing.T) {
	if _, err := state.Decode(state.KindRepoDocAudit, mustJSON(t, rejectedAudit())); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := state.Decode(state.KindRepoDocAudit, []byte(`{"schemaVersion":2,"extra":1}`)); err == nil {
		t.Error("an invented field must be rejected")
	}
	schema, ok := state.SchemaForKind(state.KindRepoDocAudit)
	if !ok || !strings.Contains(string(schema), "CHANGES_REQUESTED") {
		t.Errorf("the schema must tell the agent the legal verdicts: %v", ok)
	}
}

func TestAuditRendersItsVerdictAndFindings(t *testing.T) {
	name, body, err := state.RenderView(state.KindRepoDocAudit, mustJSON(t, rejectedAudit()))
	if err != nil || name != "repo-doc-audit.md" {
		t.Fatalf("%q %v", name, err)
	}
	for _, want := range []string{"# Audit: internal/ledger", "CHANGES REQUESTED", "signs every record", "blocking", "move it to openQuestions"} {
		if !strings.Contains(body, want) {
			t.Errorf("view lacks %q:\n%s", want, body)
		}
	}
	suggestion := approvedAudit()
	suggestion.Findings = []state.DocAuditFinding{{Claim: "c", Problem: "p", Instruction: "i"}}
	if _, body, _ := state.RenderView(state.KindRepoDocAudit, mustJSON(t, suggestion)); !strings.Contains(body, "(suggestion)") {
		t.Errorf("a non-blocking finding is a suggestion:\n%s", body)
	}
}
