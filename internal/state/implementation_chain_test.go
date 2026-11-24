package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func validImplementation() state.ImplementationState {
	return state.ImplementationState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "user-auth",
		FilesCreated:  []string{"internal/identity/session.go"},
		FilesModified: []string{"internal/identity/user.go"},
		SelfReview: []state.ChecklistItem{
			{Item: "Every public method has an intention-revealing name", Checked: true},
			{Item: "Cyclomatic complexity < 7 on all new functions", Checked: true},
		},
		SimpleDesign: []state.ChecklistItem{{Item: "Passes the tests", Checked: true}},
		RefactoringLog: []state.RefactoringEntry{
			{Operation: "Extract Function", Target: "issueSession", Reason: "two responsibilities"},
		},
	}
}

func validSecurity() state.SecurityState {
	stride := make([]state.StrideAnalysis, 0, 6)
	for _, category := range []state.StrideCategory{
		state.StrideSpoofing, state.StrideTampering, state.StrideRepudiation,
		state.StrideInformationDisclosure, state.StrideDenialOfService, state.StrideElevationOfPrivilege,
	} {
		stride = append(stride, state.StrideAnalysis{Category: category, Assessed: "considered; nothing found"})
	}
	return state.SecurityState{
		SchemaVersion:      state.SchemaVersion,
		Feature:            "user-auth",
		ThreatModelSummary: "Session issuing crosses a trust boundary at sign-in.",
		Stride:             stride,
	}
}

func validQA() state.QAState {
	return state.QAState{
		SchemaVersion:    state.SchemaVersion,
		Feature:          "user-auth",
		TestFilesCreated: []string{"internal/identity/session_test.go"},
		Coverage:         state.CoverageSummary{AcceptanceCriteriaCovered: 3, AcceptanceCriteriaTotal: 3, NewTests: 11},
		TestResults:      state.TestResults{Passed: 11, Failed: 0},
	}
}

// TestGreenSuiteRuleIsANumberNotAGrep is the qa-contract rule made real:
// "`## Test Results` must show `Failed: 0` ... A non-zero failure count is
// a FAIL, not a warning."
func TestGreenSuiteRuleIsANumberNotAGrep(t *testing.T) {
	qa := validQA()
	if !qa.IsGreen() || qa.Validate() != nil {
		t.Fatal("a green suite was rejected")
	}

	qa.TestResults.Failed = 2
	if qa.IsGreen() {
		t.Error("a suite with failures reads as green")
	}
	assertValidationNames(t, qa.Validate(), "testResults.failed")
}

func TestCoverageCannotExceedTheCriteriaThatExist(t *testing.T) {
	qa := validQA()
	qa.Coverage.AcceptanceCriteriaCovered = 5
	qa.Coverage.AcceptanceCriteriaTotal = 3

	assertValidationNames(t, qa.Validate(), "coverage.acceptanceCriteriaCovered")
}

// TestBlockingFindingsMustCarryAFix is the security-contract rule made
// real: "A Critical/High finding with `Fix applied: Recommendation only` is
// a FAIL".
func TestBlockingFindingsMustCarryAFix(t *testing.T) {
	for _, severity := range []state.Severity{state.SeverityCritical, state.SeverityHigh} {
		t.Run(string(severity), func(t *testing.T) {
			security := validSecurity()
			security.Findings = []state.SecurityFinding{{
				Severity: severity, Title: "session token in a log line",
				Location: "internal/identity/session.go:88", Description: "the token is logged at info",
			}}

			err := security.Validate()

			assertValidationNames(t, err, "fixApplied")
			if !strings.Contains(err.Error(), "session token in a log line") {
				t.Errorf("error should name the offending finding: %v", err)
			}
			security.Findings[0].FixApplied = "redacted the token before logging"
			if err := security.Validate(); err != nil {
				t.Errorf("a fixed finding was still rejected: %v", err)
			}
		})
	}
}

// TestLowerSeverityFindingsMayBeRecommendations pins the other half: the
// rule applies to Critical and High only.
func TestLowerSeverityFindingsMayBeRecommendations(t *testing.T) {
	security := validSecurity()
	security.Findings = []state.SecurityFinding{{
		Severity: state.SeverityMedium, Title: "verbose error message",
		Location: "internal/identity/user.go:20", Description: "leaks whether an account exists",
	}}

	if err := security.Validate(); err != nil {
		t.Errorf("a MEDIUM recommendation was rejected: %v", err)
	}
}

func TestCriticalFindingsAreReadableAsAField(t *testing.T) {
	security := validSecurity()
	if security.HasBlockingFindings() {
		t.Error("a clean review reports blocking findings")
	}

	security.Findings = []state.SecurityFinding{{
		Severity: state.SeverityCritical, Title: "auth bypass", Location: "x.go:1",
		Description: "the guard is skipped", FixApplied: "restored the guard",
	}}
	if !security.HasBlockingFindings() {
		t.Error("a CRITICAL finding is not visible to the pipeline's blocking rule")
	}
}

func TestStrideRequiresAllSixCategories(t *testing.T) {
	security := validSecurity()
	security.Stride = security.Stride[:5]

	err := security.Validate()

	assertValidationNames(t, err, "stride")
	if !strings.Contains(err.Error(), "ELEVATION_OF_PRIVILEGE") {
		t.Errorf("error should name the missing category: %v", err)
	}
}

// TestUnfilledChecklistFails is implementation-contract's rule: an
// unfilled checklist "means the developer skipped self-review rather than
// completed it".
func TestUnfilledChecklistFails(t *testing.T) {
	implementation := validImplementation()
	implementation.SelfReview = nil

	assertValidationNames(t, implementation.Validate(), "selfReview")
}

// TestUncheckedItemsAreStillAnAnsweredChecklist keeps the distinction the
// contract makes: it requires the checklist to be *filled in*, not that
// every box is ticked. A developer honestly recording a "no" has done the
// self-review.
func TestUncheckedItemsAreStillAnAnsweredChecklist(t *testing.T) {
	implementation := validImplementation()
	implementation.SelfReview = []state.ChecklistItem{
		{Item: "No function exceeds 30 LOC", Checked: false, Note: "parseConfig is 34; split is queued"},
	}

	if err := implementation.Validate(); err != nil {
		t.Errorf("an honestly-answered checklist was rejected: %v", err)
	}
}

func TestImplementationMustNameAFile(t *testing.T) {
	implementation := validImplementation()
	implementation.FilesCreated = nil
	implementation.FilesModified = nil

	assertValidationNames(t, implementation.Validate(), "filesCreated")
}

// TestADeletionOnlyImplementationTouchedSomething is the spec's own case: an
// order that only renames or deletes files must not park on its own correct
// work for naming nothing in filesCreated/filesModified.
func TestADeletionOnlyImplementationTouchedSomething(t *testing.T) {
	implementation := validImplementation()
	implementation.FilesCreated = nil
	implementation.FilesModified = nil
	implementation.FilesDeleted = []string{"internal/identity/old-session.go"}

	if err := implementation.Validate(); err != nil {
		t.Errorf("a deletion-only implementation was rejected: %v", err)
	}
}

// TestRenameArrowIsRejectedOnEveryImplementationField pins the AC: "old ->
// new" in a single entry fails validation before claim-checking runs, with
// a message describing the two-field encoding.
func TestRenameArrowIsRejectedOnEveryImplementationField(t *testing.T) {
	cases := map[string]func(*state.ImplementationState){
		"filesCreated":  func(i *state.ImplementationState) { i.FilesCreated = []string{"old.go -> new.go"} },
		"filesModified": func(i *state.ImplementationState) { i.FilesModified = []string{"old.go -> new.go"} },
		"filesDeleted":  func(i *state.ImplementationState) { i.FilesDeleted = []string{"old.go -> new.go"} },
	}
	for field, breakField := range cases {
		t.Run(field, func(t *testing.T) {
			implementation := validImplementation()
			breakField(&implementation)

			err := implementation.Validate()

			assertValidationNames(t, err, field)
			if !strings.Contains(err.Error(), "filesCreated") || !strings.Contains(err.Error(), "filesDeleted") {
				t.Errorf("error %q does not describe the two-field rename encoding", err)
			}
		})
	}
}

// TestRenameArrowIsRejectedOnSecurityFilesModified extends the same rule to
// SecurityState, which shares requireNoRenameArrows via its own PathClaims().
func TestRenameArrowIsRejectedOnSecurityFilesModified(t *testing.T) {
	security := validSecurity()
	security.FilesModified = []string{"old.go -> new.go"}

	err := security.Validate()

	assertValidationNames(t, err, "filesModified")
	if !strings.Contains(err.Error(), "filesDeleted") {
		t.Errorf("error %q does not describe the two-field rename encoding", err)
	}
}

func TestImplementationChainRoundTrips(t *testing.T) {
	cases := map[state.Kind]any{
		state.KindImplementation: validImplementation(),
		state.KindSecurity:       validSecurity(),
		state.KindQA:             validQA(),
	}
	for kind, document := range cases {
		t.Run(string(kind), func(t *testing.T) {
			raw, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := state.Decode(kind, raw); err != nil {
				t.Fatalf("Decode: %v", err)
			}
		})
	}
}

func TestImplementationChainRenderersEmitContractHeadings(t *testing.T) {
	cases := []struct {
		kind     state.Kind
		document any
		file     string
		headings []string
	}{
		{state.KindImplementation, validImplementation(), "implementation-notes.md", []string{
			"## Files Created", "## Files Modified", "## Files Deleted", "## Interface Design", "## Named Refactoring Log",
			"## Self-Review Checklist", "## Simple Design Verification", "## Key Decisions",
			"## Deviations from Analysis", "## Dependencies Added", "## Notes for QA", "## Notes for DevOps",
		}},
		{state.KindSecurity, validSecurity(), "security-report.md", []string{
			"## Threat Model Summary", "## Dependency Audit", "## STRIDE Analysis", "### Spoofing",
			"### Tampering", "### Repudiation", "### Information Disclosure", "### Denial of Service",
			"### Elevation of Privilege", "## Findings", "## Files Modified", "## Security Checklist",
			"## Notes for QA", "## Notes for Tech Writer",
		}},
		{state.KindQA, validQA(), "qa-report.md", []string{
			"## Test Files Created", "## Test Files Modified", "## Test Files Deleted", "## Coverage Summary", "## Test Results",
			"## Accessibility Check", "## Bugs Found", "## Known Gaps", "## Notes for Tech Writer",
		}},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			name, body := renderOf(t, tc.kind, tc.document)
			if name != tc.file {
				t.Errorf("rendered to %q, want %q", name, tc.file)
			}
			for _, heading := range tc.headings {
				if !strings.Contains(body, "\n"+heading+"\n") {
					t.Errorf("%s is missing the contract heading %q", tc.file, heading)
				}
			}
		})
	}
}

func renderOf(t *testing.T, kind state.Kind, document any) (string, string) {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	name, body, err := state.RenderView(kind, raw)
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	return name, body
}

// TestRenderedViewsKeepWhatTheMarkdownPipelineGreps covers the literals the
// existing contracts' validation rules read.
func TestRenderedViewsKeepWhatTheMarkdownPipelineGreps(t *testing.T) {
	_, qaBody := renderOf(t, state.KindQA, validQA())
	if !strings.Contains(qaBody, "- Failed: 0") {
		t.Errorf("qa-report must still show a Failed count:\n%s", qaBody)
	}

	security := validSecurity()
	security.Findings = []state.SecurityFinding{{
		Severity: state.SeverityCritical, Title: "auth bypass", Location: "x.go:1",
		Description: "the guard is skipped", FixApplied: "restored the guard",
	}}
	_, securityBody := renderOf(t, state.KindSecurity, security)
	if !strings.Contains(securityBody, "**Fix applied**: restored the guard") {
		t.Errorf("security-report must still carry the Fix applied line:\n%s", securityBody)
	}

	_, implementationBody := renderOf(t, state.KindImplementation, validImplementation())
	if !strings.Contains(implementationBody, "- [x] ") {
		t.Errorf("implementation-notes must still render checklist marks:\n%s", implementationBody)
	}
}

func TestImplementationChainCarriesRetrievalFrontmatter(t *testing.T) {
	_, body := renderOf(t, state.KindImplementation, validImplementation())

	if !strings.HasPrefix(body, "---\n") {
		t.Fatal("rendered implementation notes have no frontmatter block")
	}
	if !strings.Contains(body, `files_touched: ["internal/identity/session.go", "internal/identity/user.go"]`) {
		t.Errorf("files_touched was not derived from the files the implementation names:\n%s", body[:400])
	}
}

// TestFilesDeletedIsVisibleInTheRenderedView is the spec's own motivation: a
// deletion-only implementation must be visible in implementation-notes.md,
// not only in the typed JSON.
func TestFilesDeletedIsVisibleInTheRenderedView(t *testing.T) {
	implementation := validImplementation()
	implementation.FilesDeleted = []string{"internal/identity/old-session.go"}

	_, body := renderOf(t, state.KindImplementation, implementation)

	if !strings.Contains(body, "- internal/identity/old-session.go") {
		t.Errorf("rendered implementation notes do not list the deleted file:\n%s", body)
	}
	if !strings.Contains(body, `files_touched: ["internal/identity/session.go", "internal/identity/user.go", "internal/identity/old-session.go"]`) {
		t.Errorf("files_touched does not include the deleted file:\n%s", body[:400])
	}
}
