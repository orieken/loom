package docstyle

import "testing"

func TestFindingStringRendersFileLineRuleExcerpt(t *testing.T) {
	finding := Finding{File: "README.md", Line: 12, Rule: RuleSentenceLength, Excerpt: "this sentence runs long"}
	if got, want := finding.String(), "README.md:12: sentence-length: this sentence runs long"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
