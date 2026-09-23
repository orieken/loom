package testlint_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/testlint"
)

// The fixture module is the lint's red proof, kept rather than performed
// once: every case that must be flagged is flagged, and nothing else is.
// Loosening the rule — matching any receiver's Error method, counting t.Skip,
// following a helper without checking it can fail — turns this red.
func TestFixtureFlagsExactlyTheTestsThatCannotFail(t *testing.T) {
	findings, err := testlint.Scan("testdata/fixture", "example.com/fixture")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []string{
		"TestAliasedHelperThatCannotFail",
		"TestCallsButNeverChecks",
		"TestHelperThatCannotFail",
		"TestLoggerErrorIsNotAFailure",
		"TestLogsInsteadOfFailing",
		"TestOnlySkips",
		"TestOtherPackageHelperThatCannotFail",
		"TestRecursiveHelpersThatCannotFail",
		"TestSubtestThatCannotFail",
	}
	if got := findingNames(findings); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("flagged:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestFindingsLocateTheTest(t *testing.T) {
	findings, err := testlint.Scan("testdata/fixture", "example.com/fixture")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, finding := range findings {
		if finding.Test == "TestCallsButNeverChecks" {
			if finding.File != "cases/cases_test.go" || finding.Line == 0 {
				t.Errorf("finding at %s:%d, want cases/cases_test.go with a line", finding.File, finding.Line)
			}
			return
		}
	}
	t.Fatal("TestCallsButNeverChecks was not reported")
}

func TestScanReportsAnUnparseableFile(t *testing.T) {
	if _, err := testlint.Scan("testdata/unparseable", "example.com/unparseable"); err == nil {
		t.Error("Scan accepted a file that does not parse — a check that skips what it cannot read passes vacuously")
	}
}

func findingNames(findings []testlint.Finding) []string {
	names := make([]string, 0, len(findings))
	for _, finding := range findings {
		names = append(names, finding.Test)
	}
	sort.Strings(names)
	return names
}
