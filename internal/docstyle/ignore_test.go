package docstyle

import "testing"

const longDescriptiveSentence = "Finally, this paragraph intentionally contains a sentence that runs well past the twenty five word descriptive limit so the checker reports it as a finding here."

func TestDirectivesMapsTheLineAfterTheCommentToItsNamedRule(t *testing.T) {
	body := "Some prose.\n<!-- docstyle-ignore: sentence-length -->\n" + longDescriptiveSentence + "\n"
	directives := Directives(body, 1)
	if directives[3] != RuleSentenceLength {
		t.Fatalf("directives = %v", directives)
	}
}

func TestSuppressRemovesAFindingWhoseLineAndRuleMatchADirective(t *testing.T) {
	findings := []Finding{{File: testFile, Line: 3, Rule: RuleSentenceLength, Excerpt: longDescriptiveSentence}}
	kept, skipped := Suppress(findings, map[int]string{3: RuleSentenceLength})
	if len(kept) != 0 || skipped != 1 {
		t.Fatalf("kept = %+v, skipped = %d", kept, skipped)
	}
}

func TestSuppressKeepsAFindingWhenTheDirectiveNamesADifferentRule(t *testing.T) {
	findings := []Finding{{File: testFile, Line: 3, Rule: RuleSentenceLength, Excerpt: longDescriptiveSentence}}
	kept, skipped := Suppress(findings, map[int]string{3: RulePlainVocabulary})
	if len(kept) != 1 || skipped != 0 {
		t.Fatalf("a directive naming a different rule must not suppress the finding: kept = %+v, skipped = %d", kept, skipped)
	}
}

func TestSuppressKeepsAFindingWithNoDirectiveAtAll(t *testing.T) {
	findings := []Finding{{File: testFile, Line: 1, Rule: RuleSentenceLength, Excerpt: longDescriptiveSentence}}
	kept, skipped := Suppress(findings, map[int]string{})
	if len(kept) != 1 || skipped != 0 {
		t.Fatalf("kept = %+v, skipped = %d", kept, skipped)
	}
}

func TestCheckIntegratesTheIgnoreDirectiveEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "Some prose.\n<!-- docstyle-ignore: sentence-length -->\n"+longDescriptiveSentence+"\n")
	report, err := Check([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestCheckStillReportsAFindingWhenTheDirectiveNamesTheWrongRule(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/doc.md"
	writeFile(t, path, "Some prose.\n<!-- docstyle-ignore: plain-vocabulary -->\n"+longDescriptiveSentence+"\n")
	report, err := Check([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Skipped != 0 {
		t.Fatalf("report = %+v", report)
	}
}
