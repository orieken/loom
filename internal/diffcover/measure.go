package diffcover

import (
	"path"
	"sort"
	"strings"
)

// Line is one changed line, named by repository-relative path.
type Line struct {
	File string
	Line int
}

// Report is the coverage of a change.
type Report struct {
	Executable int      // changed lines some coverage block spans
	Covered    int      // of those, lines a test executed
	Uncovered  []Line   // executable changed lines no test executed
	Unmeasured []string // changed Go files the profile does not mention
	// Statementless lists changed files absent from the profile because they
	// hold no function body — declarations only, nothing a test could run.
	Statementless []string
	Excluded      []string // changed Go files skipped by name, with no measurement
}

// Percent is Covered/Executable as a percentage. A change with no executable
// lines is fully covered: there is nothing in it a test could have run.
func (report Report) Percent() float64 {
	if report.Executable == 0 {
		return 100
	}
	return 100 * float64(report.Covered) / float64(report.Executable)
}

// Passes reports whether the change meets threshold and every changed file
// was measured. An unmeasured file fails regardless of the percentage.
func (report Report) Passes(threshold float64) bool {
	return len(report.Unmeasured) == 0 && report.Percent() >= threshold
}

// Measurement is what Measure needs to judge a change.
type Measurement struct {
	Profile    Profile
	Changes    Changes
	ModulePath string // prefix of profile paths, e.g. github.com/orieken/loom
	// Excluded names repository-relative paths skipped without measurement.
	// An entry ending in "/" excludes that directory — for a nested module,
	// whose files this module's profile can never contain.
	Excluded map[string]bool
	// HasStatements reports whether a file holds anything coverage could
	// instrument. A file absent from the profile fails as unmeasured unless
	// this says it has no statements; nil, or any doubt, means it does.
	HasStatements func(file string) bool
}

// Measure computes the coverage of every changed, measurable Go line.
// Test files, testdata and non-Go files are not production code and are
// ignored; anything else is either measured, explicitly excluded, or
// reported as unmeasured.
func Measure(measurement Measurement) Report {
	report := Report{}
	for _, file := range sortedFiles(measurement.Changes) {
		if !IsProductionGo(file) {
			continue
		}
		if IsExcluded(measurement.Excluded, file) {
			report.Excluded = append(report.Excluded, file)
			continue
		}
		blocks, measured := measurement.Profile[measurement.ModulePath+"/"+file]
		if !measured {
			report.addUnprofiled(file, measurement.HasStatements)
			continue
		}
		report.addFile(file, measurement.Changes[file], blocks)
	}
	return report
}

// addUnprofiled files a changed file the profile never mentions. Go's
// coverage profile has blocks only for statements inside function bodies, so
// a declarations-only file is absent by construction, not unmeasured.
func (report *Report) addUnprofiled(file string, hasStatements func(string) bool) {
	if hasStatements != nil && !hasStatements(file) {
		report.Statementless = append(report.Statementless, file)
		return
	}
	report.Unmeasured = append(report.Unmeasured, file)
}

func (report *Report) addFile(file string, lines []int, blocks []Block) {
	for _, line := range lines {
		executable, covered := lineCoverage(line, blocks)
		if !executable {
			continue
		}
		report.Executable++
		if covered {
			report.Covered++
			continue
		}
		report.Uncovered = append(report.Uncovered, Line{File: file, Line: line})
	}
}

// lineCoverage reports whether any block spans line, and whether any block
// spanning it ran. Blocks sharing a boundary line are both consulted, so a
// line is covered if either side of it executed.
func lineCoverage(line int, blocks []Block) (bool, bool) {
	executable, covered := false, false
	for _, block := range blocks {
		if line < block.StartLine || line > block.EndLine {
			continue
		}
		executable = true
		covered = covered || block.Covered
	}
	return executable, covered
}

// IsExcluded reports whether file is named in excluded, directly or by a
// directory entry ending in "/".
func IsExcluded(excluded map[string]bool, file string) bool {
	if excluded[file] {
		return true
	}
	for directory := path.Dir(file); directory != "." && directory != "/"; directory = path.Dir(directory) {
		if excluded[directory+"/"] {
			return true
		}
	}
	return false
}

// IsProductionGo reports whether file is Go source that ships: not a test,
// not testdata.
func IsProductionGo(file string) bool {
	if path.Ext(file) != ".go" || strings.HasSuffix(file, "_test.go") {
		return false
	}
	return !strings.HasPrefix(file, "testdata/") && !strings.Contains(file, "/testdata/")
}

func sortedFiles(changes Changes) []string {
	files := make([]string, 0, len(changes))
	for file := range changes {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}
