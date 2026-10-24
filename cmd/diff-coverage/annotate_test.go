package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
)

func TestAFailingGateAnnotatesEachFileWithItsLines(t *testing.T) {
	var out bytes.Buffer
	annotate(&out, diffcover.Report{
		Uncovered:  []diffcover.Line{{File: "b.go", Line: 9}, {File: "a.go", Line: 7}, {File: "a.go", Line: 3}},
		Unmeasured: []string{"c.go"},
	})
	want := "::error file=c.go::changed, but absent from the coverage profile\n" +
		"::error file=a.go,line=3::uncovered changed lines: 3, 7\n" +
		"::error file=b.go,line=9::uncovered changed lines: 9\n"
	if out.String() != want {
		t.Errorf("annotations =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestAnnotationsStopAtGitHubsLimit(t *testing.T) {
	var report diffcover.Report
	for index := range 12 {
		report.Unmeasured = append(report.Unmeasured, string(rune('a'+index))+".go")
	}
	report.Uncovered = []diffcover.Line{{File: "z.go", Line: 1}}
	var out bytes.Buffer
	annotate(&out, report)
	if count := strings.Count(out.String(), "::error"); count != maxAnnotatedFiles {
		t.Errorf("annotations = %d, want %d", count, maxAnnotatedFiles)
	}
}

func TestAnnotationsStopAtTheLimitAmongUncoveredFiles(t *testing.T) {
	var report diffcover.Report
	for index := range 12 {
		report.Uncovered = append(report.Uncovered, diffcover.Line{File: string(rune('a'+index)) + ".go", Line: 1})
	}
	var out bytes.Buffer
	annotate(&out, report)
	if count := strings.Count(out.String(), "::error"); count != maxAnnotatedFiles {
		t.Errorf("annotations = %d, want %d", count, maxAnnotatedFiles)
	}
}

func TestTheGateAnnotatesOnlyUnderGitHubActions(t *testing.T) {
	project(t, "module example.com/m\n")
	diff := diffOf("+++ b/pkg/a.go\n@@ -1 +3,4 @@\n")
	t.Setenv("GITHUB_ACTIONS", "true")
	if _, output := runWith(diff, "--base", "main"); !strings.Contains(output, "::error file=pkg/a.go,line=5::") {
		t.Errorf("output lacks the annotation:\n%s", output)
	}
	t.Setenv("GITHUB_ACTIONS", "")
	if _, output := runWith(diff, "--base", "main"); strings.Contains(output, "::error") {
		t.Errorf("annotated outside GitHub Actions:\n%s", output)
	}
}
