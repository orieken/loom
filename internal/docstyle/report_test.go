package docstyle

import (
	"strings"
	"testing"
)

func TestRenderPrintsOneLinePerFindingPlusTheSkippedSummary(t *testing.T) {
	report := Report{
		Findings: []Finding{{File: "a.md", Line: 1, Rule: RulePlainVocabulary, Excerpt: `"utilize" → "use"`}},
		Skipped:  2,
	}
	output := Render(report)
	if !strings.Contains(output, "a.md:1: plain-vocabulary:") {
		t.Errorf("output = %q", output)
	}
	if !strings.Contains(output, "2 finding(s) skipped via docstyle-ignore") {
		t.Errorf("output = %q", output)
	}
}

func TestRenderWithNoFindingsStillPrintsTheSkippedSummary(t *testing.T) {
	output := Render(Report{})
	if strings.TrimSpace(output) != "0 finding(s) skipped via docstyle-ignore" {
		t.Errorf("output = %q", output)
	}
}
