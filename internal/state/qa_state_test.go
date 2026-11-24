package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

// Coverage is reported per named unit, and the report leads with the weakest
// (roadmap L3.31).
//
// Run 4's qa-engineer reported 89.7% unqualified. It was real and correctly
// measured — for internal/runs. The feature also spanned internal/httpserver
// at 43.1%, holding the handlers, the pagination link and the page
// rendering. testing-conventions.md makes >= 85% CRITICAL, so the report read
// as clearing a bar most of the change did not.
func TestCoverageReportsTheWeakestUnitNotTheFlatteringOne(t *testing.T) {
	coverage := state.CoverageSummary{
		AcceptanceCriteriaCovered: 9, AcceptanceCriteriaTotal: 9, NewTests: 22,
		Statements: []state.PackageCoverage{
			{Unit: "internal/runs", Percent: 89.7},
			{Unit: "internal/httpserver", Percent: 43.1},
		},
	}

	lowest, measured := coverage.LowestStatementCoverage()

	if !measured {
		t.Fatal("no coverage was reported as measured")
	}
	if lowest.Unit != "internal/httpserver" || lowest.Percent != 43.1 {
		t.Errorf("lowest = %+v, want internal/httpserver at 43.1", lowest)
	}
}

// The rendered report must show every unit, so a reader cannot see only the
// good one.
func TestTheRenderedReportNamesEveryMeasuredUnit(t *testing.T) {
	payload := []byte(`{
		"schemaVersion": 2, "feature": "run-history-browsing",
		"coverage": {"acceptanceCriteriaCovered": 9, "acceptanceCriteriaTotal": 9,
			"statements": [
				{"unit": "internal/runs", "percent": 89.7},
				{"unit": "internal/httpserver", "percent": 43.1}]},
		"testResults": {"passed": 23}
	}`)

	_, body, err := state.RenderView(state.KindQA, payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, want := range []string{"internal/httpserver", "43.1", "internal/runs", "89.7"} {
		if !strings.Contains(body, want) {
			t.Errorf("the report omits %q, so a reader sees an incomplete picture:\n%s", want, body)
		}
	}
	// The weakest number is the one a bar is judged against, so it leads.
	if !strings.Contains(body, "lowest unit: **43.1%**") {
		t.Errorf("the report does not lead with the weakest unit:\n%s", body)
	}
}

func validQAForRenameTest() state.QAState {
	return state.QAState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "user-auth",
		Coverage:      state.CoverageSummary{AcceptanceCriteriaCovered: 1, AcceptanceCriteriaTotal: 1},
		TestResults:   state.TestResults{Passed: 1, Failed: 0},
	}
}

// TestRenameArrowIsRejectedOnEveryQAField pins the AC: "old -> new" in a
// single entry fails validation, with a message describing the two-field
// encoding, on every one of QAState's path-naming fields including the new
// testFilesDeleted.
func TestRenameArrowIsRejectedOnEveryQAField(t *testing.T) {
	cases := map[string]func(*state.QAState){
		"testFilesCreated":  func(q *state.QAState) { q.TestFilesCreated = []string{"old_test.go -> new_test.go"} },
		"testFilesModified": func(q *state.QAState) { q.TestFilesModified = []string{"old_test.go -> new_test.go"} },
		"testFilesDeleted":  func(q *state.QAState) { q.TestFilesDeleted = []string{"old_test.go -> new_test.go"} },
	}
	for field, breakField := range cases {
		t.Run(field, func(t *testing.T) {
			qa := validQAForRenameTest()
			breakField(&qa)

			err := qa.Validate()

			assertValidationNames(t, err, field)
			if !strings.Contains(err.Error(), "filesCreated") || !strings.Contains(err.Error(), "filesDeleted") {
				t.Errorf("error %q does not describe the two-field rename encoding", err)
			}
		})
	}
}

// TestADeletionOnlyQAReportIsAccepted mirrors the implementation-side case:
// naming only testFilesDeleted is a coherent QA report.
func TestADeletionOnlyQAReportIsAccepted(t *testing.T) {
	qa := validQAForRenameTest()
	qa.TestFilesDeleted = []string{"internal/identity/old_session_test.go"}

	if err := qa.Validate(); err != nil {
		t.Errorf("a deletion-only QA report was rejected: %v", err)
	}
}

// TestTestFilesDeletedIsVisibleInTheRenderedView mirrors the
// implementation-side case: a deleted test file must be visible in
// qa-report.md, not only in the typed JSON.
func TestTestFilesDeletedIsVisibleInTheRenderedView(t *testing.T) {
	qa := validQAForRenameTest()
	qa.TestFilesDeleted = []string{"internal/identity/old_session_test.go"}

	raw, err := json.Marshal(qa)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, body, err := state.RenderView(state.KindQA, raw)
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}

	if !strings.Contains(body, "- internal/identity/old_session_test.go") {
		t.Errorf("rendered qa-report does not list the deleted test file:\n%s", body)
	}
}
