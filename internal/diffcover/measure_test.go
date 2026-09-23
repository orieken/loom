package diffcover_test

import (
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
)

const module = "example.com/m"

// blocks: lines 1-3 ran, 5-6 did not, 8 is a boundary shared by a block that
// ran and one that did not. Lines 4 and 7 are in no block (blank or comment).
var profile = diffcover.Profile{
	module + "/pkg/a.go": {
		{StartLine: 1, EndLine: 3, Covered: true},
		{StartLine: 5, EndLine: 6, Covered: false},
		{StartLine: 7, EndLine: 8, Covered: false},
		{StartLine: 8, EndLine: 9, Covered: true},
	},
}

func TestMeasureCountsOnlyExecutableChangedLines(t *testing.T) {
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module,
		Changes: diffcover.Changes{"pkg/a.go": {1, 2, 4, 5, 8}},
	})

	// 4 is in no block; 8 is covered because one of its blocks ran.
	if report.Executable != 4 || report.Covered != 3 {
		t.Errorf("executable=%d covered=%d, want 4 and 3", report.Executable, report.Covered)
	}
	if want := []diffcover.Line{{File: "pkg/a.go", Line: 5}}; !reflect.DeepEqual(report.Uncovered, want) {
		t.Errorf("uncovered = %v, want %v", report.Uncovered, want)
	}
	if report.Percent() != 75 {
		t.Errorf("percent = %v, want 75", report.Percent())
	}
}

// Silence about a file is not evidence it is tested. A changed production
// file the profile never mentions fails the gate even at 100%.
func TestAnUnmeasuredFileFailsWhateverThePercentage(t *testing.T) {
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module,
		Changes: diffcover.Changes{"pkg/a.go": {1}, "pkg/untested.go": {1, 2}},
	})

	if !reflect.DeepEqual(report.Unmeasured, []string{"pkg/untested.go"}) {
		t.Fatalf("unmeasured = %v, want [pkg/untested.go]", report.Unmeasured)
	}
	if report.Percent() != 100 || report.Passes(85) {
		t.Errorf("percent=%v passes=%v, want 100%% and a failure", report.Percent(), report.Passes(85))
	}
}

func TestTestFilesTestdataAndNonGoAreNotProductionCode(t *testing.T) {
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module,
		Changes: diffcover.Changes{
			"pkg/a_test.go":              {1},
			"pkg/testdata/fixture.go":    {1},
			"testdata/root.go":           {1},
			"docs/readme.md":             {1},
			"scripts/check-something.sh": {1},
		},
	})

	if report.Executable != 0 || len(report.Unmeasured) != 0 || len(report.Excluded) != 0 {
		t.Errorf("report = %+v, want nothing measured, unmeasured or excluded", report)
	}
}

func TestExcludedFilesAndDirectoriesAreListedNotMeasured(t *testing.T) {
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module,
		Changes: diffcover.Changes{
			"pkg/special.go":                 {1},
			"examples/embedding/main.go":     {1},
			"examples/embedding/sub/tool.go": {1},
			"examplesish/main.go":            {1},
		},
		Excluded: map[string]bool{"pkg/special.go": true, "examples/embedding/": true},
	})

	want := []string{"examples/embedding/main.go", "examples/embedding/sub/tool.go", "pkg/special.go"}
	if !reflect.DeepEqual(report.Excluded, want) {
		t.Errorf("excluded = %v, want %v", report.Excluded, want)
	}
	// A directory entry is a directory, not a name prefix.
	if !reflect.DeepEqual(report.Unmeasured, []string{"examplesish/main.go"}) {
		t.Errorf("unmeasured = %v, want [examplesish/main.go]", report.Unmeasured)
	}
}

func TestThresholdIsInclusive(t *testing.T) {
	for _, test := range []struct {
		name                string
		covered, executable int
		wantPass            bool
	}{
		{"exactly at threshold", 17, 20, true},
		{"just under", 16, 20, false},
		{"nothing executable", 0, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := diffcover.Report{Covered: test.covered, Executable: test.executable}
			if got := report.Passes(85); got != test.wantPass {
				t.Errorf("%d/%d passes 85%% = %v, want %v", test.covered, test.executable, got, test.wantPass)
			}
		})
	}
}
