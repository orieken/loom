package state_test

import (
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
