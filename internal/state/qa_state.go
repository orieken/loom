package state

// QAState is the qa-engineer's output, modelling
// shared/contracts/qa-contract.md — including the rule that contract
// already asserts: `## Test Results` must show `Failed: 0`, "per the
// agent's own rule, tests must be green before the pipeline proceeds. A
// non-zero failure count is a FAIL, not a warning."
//
// As a grep over prose that rule could be satisfied by any line containing
// the right string. As a field it is a number.

import "fmt"

// TestResults is the run's outcome. Failed is the field the contract's rule
// turns on.
type TestResults struct {
	Passed  int `json:"passed" jsonschema:"required,minimum=0"`
	Failed  int `json:"failed" jsonschema:"minimum=0"`
	Skipped int `json:"skipped,omitempty" jsonschema:"minimum=0"`
	// SkipReasons explains any skipped tests; the template asks for a
	// reason and a count without one is not reviewable.
	SkipReasons []string `json:"skipReasons,omitempty"`
}

// PackageCoverage is one measured unit's statement coverage, with the unit
// named (roadmap L3.31).
//
// A bare percentage is the defect: run 4's qa-engineer reported
// statementCoveragePercent 89.7 with no qualifier, which was real and
// correctly measured — for internal/runs. The feature also spanned
// internal/httpserver, holding the handlers, the pagination link and the
// page rendering, at 43.1%. Neither the lower number nor the word "package"
// appeared anywhere, and testing-conventions.md makes coverage >= 85%
// CRITICAL, so a reader concluded the feature cleared a bar that most of its
// new surface did not.
//
// Not fabrication — a real measurement of the wrong scope, presented
// unqualified. Naming the unit is what makes the number checkable.
type PackageCoverage struct {
	Unit    string  `json:"unit" jsonschema:"required,description=The package or path this percentage was measured over — e.g. internal/httpserver"`
	Percent float64 `json:"percent" jsonschema:"required,minimum=0,maximum=100"`
}

// CoverageSummary is what the qa-engineer measured. AcceptanceCriteriaCovered
// and AcceptanceCriteriaTotal are separate numbers rather than an "X/Y"
// string so a ratio can actually be computed from them.
type CoverageSummary struct {
	AcceptanceCriteriaCovered int `json:"acceptanceCriteriaCovered" jsonschema:"minimum=0"`
	AcceptanceCriteriaTotal   int `json:"acceptanceCriteriaTotal" jsonschema:"minimum=0"`
	NewTests                  int `json:"newTests,omitempty" jsonschema:"minimum=0"`
	// Statements carries one entry per unit the feature touched. Every unit
	// the change spans belongs here, not the flattering one.
	Statements []PackageCoverage `json:"statements,omitempty" jsonschema:"description=Statement coverage per package or module the feature touches. Report EVERY unit the change spans, including any that fall short"`
}

// LowestStatementCoverage returns the weakest unit measured, which is the
// number a coverage bar has to be judged against. Reporting the highest of
// several is how run 4's report read as clearing a bar it did not.
func (c CoverageSummary) LowestStatementCoverage() (PackageCoverage, bool) {
	if len(c.Statements) == 0 {
		return PackageCoverage{}, false
	}
	lowest := c.Statements[0]
	for _, entry := range c.Statements[1:] {
		if entry.Percent < lowest.Percent {
			lowest = entry
		}
	}
	return lowest, true
}

// Bug is something QA found and what happened to it.
type Bug struct {
	Description string `json:"description" jsonschema:"required"`
	Resolution  string `json:"resolution" jsonschema:"required,description=How it was fixed, or why it was left"`
}

// KnownGap is an acceptance criterion that could not be tested, and why.
// This is deliberately structured: a gap without a reason is the kind of
// thing that disappears into prose.
type KnownGap struct {
	Criterion string `json:"criterion" jsonschema:"required"`
	Reason    string `json:"reason" jsonschema:"required"`
}

// QAState is the typed form of qa-report.md.
type QAState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required"`

	TestFilesCreated  []string `json:"testFilesCreated,omitempty"`
	TestFilesModified []string `json:"testFilesModified,omitempty"`

	Coverage    CoverageSummary `json:"coverage" jsonschema:"required"`
	TestResults TestResults     `json:"testResults" jsonschema:"required"`

	AccessibilityCheck []string   `json:"accessibilityCheck,omitempty"`
	BugsFound          []Bug      `json:"bugsFound,omitempty"`
	KnownGaps          []KnownGap `json:"knownGaps,omitempty"`
	NotesForTechWriter []string   `json:"notesForTechWriter,omitempty"`

	Retrieval Retrieval `json:"retrieval,omitempty"`
}

// IsGreen reports whether the suite passed. The pipeline's "tests must be
// green" rule reads this rather than a rendered line.
func (q QAState) IsGreen() bool { return q.TestResults.Failed == 0 }

// Validate enforces the contract's rule in code: a non-zero failure count
// is a FAIL, not a warning.
func (q QAState) Validate() error {
	return firstError(
		requireSchemaVersion(q.SchemaVersion),
		requireText("feature", q.Feature),
		requireGreenSuite(q.TestResults),
		requireCoherentCoverage(q.Coverage),
	)
}

func requireGreenSuite(results TestResults) error {
	if results.Failed == 0 {
		return nil
	}
	return &ValidationError{Field: "testResults.failed",
		Reason: fmt.Sprintf("is %d — the suite must be green before the pipeline proceeds, and a failure count is not a warning", results.Failed)}
}

// requireCoherentCoverage catches a report claiming to have covered more
// acceptance criteria than the feature has.
func requireCoherentCoverage(coverage CoverageSummary) error {
	if coverage.AcceptanceCriteriaCovered <= coverage.AcceptanceCriteriaTotal {
		return nil
	}
	return &ValidationError{Field: "coverage.acceptanceCriteriaCovered",
		Reason: fmt.Sprintf("is %d of %d — more criteria covered than exist",
			coverage.AcceptanceCriteriaCovered, coverage.AcceptanceCriteriaTotal)}
}
