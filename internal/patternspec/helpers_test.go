package patternspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeReferences is the ReferenceResolver of the tests: a fixed set of
// skills and tools.
type fakeReferences struct {
	skills map[string]bool
	tools  map[string]bool
}

func (references fakeReferences) HasSkill(name string) bool { return references.skills[name] }
func (references fakeReferences) HasTool(name string) bool  { return references.tools[name] }

// knownReferences resolves the names the valid fixtures use.
var knownReferences = fakeReferences{
	skills: map[string]bool{"review-pr": true},
	tools:  map[string]bool{"check_ubiquitous_language": true},
}

// fixtureExpectation is what an invalid fixture declares in its header:
//
//	# layer: structure | document | contract
//	# expect: failure steps[0].executor | is required
type fixtureExpectation struct {
	layer    string
	severity Severity
	field    string
	message  string
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return source
}

func fixturePaths(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", dir, "*"+FileSuffix))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures in testdata/%s (%v)", dir, err)
	}
	return paths
}

// expectationOf reads the header of a fixture, or of its .expect sidecar
// when the fixture itself must stay empty.
func expectationOf(t *testing.T, path string) fixtureExpectation {
	t.Helper()
	header := string(readFixture(t, path))
	if sidecar := strings.TrimSuffix(path, FileSuffix) + ".expect"; header == "" {
		header = string(readFixture(t, sidecar))
	}
	var expectation fixtureExpectation
	for _, line := range strings.Split(header, "\n") {
		expectation = withHeaderLine(t, expectation, line)
	}
	if expectation.layer == "" || expectation.field == "" {
		t.Fatalf("%s: header needs '# layer:' and '# expect:'", path)
	}
	return expectation
}

func withHeaderLine(t *testing.T, expectation fixtureExpectation, line string) fixtureExpectation {
	t.Helper()
	if layer, found := strings.CutPrefix(line, "# layer: "); found {
		expectation.layer = layer
	}
	if expect, found := strings.CutPrefix(line, "# expect: "); found {
		severity, rest, _ := strings.Cut(expect, " ")
		field, message, _ := strings.Cut(rest, " | ")
		expectation.severity, expectation.field, expectation.message = Severity(severity), field, message
	}
	return expectation
}

// parseAndValidate is everything a single file goes through apart from the
// location rules, which need a directory (see catalog_test.go).
func parseAndValidate(source []byte) []Issue {
	spec, issues := Parse(source)
	if len(issues) > 0 {
		return issues
	}
	return Validate(spec, knownReferences)
}

func hasIssue(issues []Issue, expectation fixtureExpectation) bool {
	for _, issue := range issues {
		if issue.Severity == expectation.severity && issue.Field == expectation.field && strings.Contains(issue.Message, expectation.message) {
			return true
		}
	}
	return false
}
