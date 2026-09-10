package state

import "fmt"

// renderQA writes qa-report.md with the exact headings qa-contract.md
// requires, including the `Failed: N` line its validation rule reads in the
// markdown pipeline.
func renderQA(qa QAState) string {
	doc := &document{}
	doc.frontmatter(qa.frontmatter())
	doc.title("QA Report: " + qa.Feature)
	doc.section("## Test Files Created", bullets(qa.TestFilesCreated))
	doc.section("## Test Files Modified", bullets(qa.TestFilesModified))
	doc.section("## Coverage Summary", coverageLines(qa.Coverage))
	doc.section("## Test Results", testResultLines(qa.TestResults))
	doc.section("## Accessibility Check", bullets(qa.AccessibilityCheck))
	doc.section("## Bugs Found", bugLines(qa.BugsFound))
	doc.section("## Known Gaps", gapLines(qa.KnownGaps))
	doc.section("## Notes for Tech Writer", bullets(qa.NotesForTechWriter))
	return doc.String()
}

func coverageLines(coverage CoverageSummary) []string {
	lines := []string{fmt.Sprintf("- Acceptance criteria covered: %d/%d",
		coverage.AcceptanceCriteriaCovered, coverage.AcceptanceCriteriaTotal)}
	if coverage.NewTests > 0 {
		lines = append(lines, fmt.Sprintf("- Total new tests: %d", coverage.NewTests))
	}
	return append(lines, statementCoverageLines(coverage)...)
}

// statementCoverageLines names every unit measured and leads with the
// weakest, because that is the number a coverage bar is judged against
// (roadmap L3.31). A single unqualified percentage let run 4's report read
// as clearing 85% while the package holding most of the change sat at 43.1%.
func statementCoverageLines(coverage CoverageSummary) []string {
	lowest, measured := coverage.LowestStatementCoverage()
	if !measured {
		return nil
	}
	lines := []string{fmt.Sprintf("- Statement coverage, lowest unit: **%.1f%%** (`%s`)",
		lowest.Percent, lowest.Unit)}
	for _, entry := range coverage.Statements {
		lines = append(lines, fmt.Sprintf("  - `%s`: %.1f%%", entry.Unit, entry.Percent))
	}
	return lines
}

func testResultLines(results TestResults) []string {
	lines := []string{
		fmt.Sprintf("- Passed: %d", results.Passed),
		fmt.Sprintf("- Failed: %d", results.Failed),
		fmt.Sprintf("- Skipped: %d", results.Skipped),
	}
	for _, reason := range results.SkipReasons {
		lines = append(lines, "  - "+reason)
	}
	return lines
}

func bugLines(bugs []Bug) []string {
	lines := make([]string, 0, len(bugs))
	for _, bug := range bugs {
		lines = append(lines, fmt.Sprintf("- %s: %s", bug.Description, bug.Resolution))
	}
	return lines
}

func gapLines(gaps []KnownGap) []string {
	lines := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		lines = append(lines, fmt.Sprintf("- %s: %s", gap.Criterion, gap.Reason))
	}
	return lines
}

func (q QAState) frontmatter() frontmatter {
	touched := append(append([]string{}, q.TestFilesCreated...), q.TestFilesModified...)
	return newFrontmatter(q.Feature, "", q.Retrieval, touched)
}

// render satisfies the renderable interface.
func (q QAState) render() string { return renderQA(q) }
