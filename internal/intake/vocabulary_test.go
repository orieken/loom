package intake

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// vocabularyFields pairs each closed-vocabulary field with the Go set it must
// match. TEMPLATE.md and README.md are checked against these, never the
// reverse, so a drift always names the Go set as the source of truth.
var vocabularyFields = []struct {
	name    string
	allowed map[string]bool
}{
	{"type", knownTypes},
	{"status", knownStatuses},
	{"source", knownSources},
	{"trust", knownTrust},
	{"finish", knownFinishes},
	{"autonomy", knownAutonomies},
}

func templatePath() string {
	return intakeDocPath("TEMPLATE.md")
}

func readmePath() string {
	return intakeDocPath("README.md")
}

// intakeDocPath resolves a file under templates/claude-feature-team/intake/,
// the same runtime.Caller idiom TestTheShippedTemplateParses already uses so
// the test works regardless of the caller's working directory.
func intakeDocPath(name string) string {
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "..", "templates", "claude-feature-team", "intake", name)
}

// frontmatterOf reads path and returns the text between its --- fences,
// reusing Parse's own split so a vocabulary comment outside the frontmatter
// (e.g. inside the HOW TO USE comment block) can never be misread as one.
func frontmatterOf(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	frontmatter, _, _, err := splitFrontmatter(string(source))
	if err != nil {
		t.Fatalf("split frontmatter of %s: %v", path, err)
	}
	return frontmatter
}

// fieldCommentValues returns the pipe-separated values listed in field's
// trailing "# a | b | c" comment, and false if no such comment is found.
func fieldCommentValues(frontmatter, field string) ([]string, bool) {
	comment, found := fieldComment(frontmatter, field)
	if !found {
		return nil, false
	}
	values := splitPipeList(cutAtEmDash(comment))
	return values, len(values) > 0
}

// fieldComment finds the line starting with "<field>:" and returns the text
// after its first "#". Anchoring on the line start, not just the substring,
// is what keeps prose mentioning the field elsewhere from being misread.
func fieldComment(frontmatter, field string) (string, bool) {
	prefix := field + ":"
	for _, line := range strings.Split(frontmatter, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if hash := strings.Index(line, "#"); hash >= 0 {
			return line[hash+1:], true
		}
		return "", false
	}
	return "", false
}

// cutAtEmDash drops any explanatory prose a comment adds after an em dash,
// so it is never mistaken for a vocabulary value.
func cutAtEmDash(comment string) string {
	if i := strings.Index(comment, "—"); i >= 0 {
		return comment[:i]
	}
	return comment
}

// splitPipeList splits "a | b | c" into its trimmed, non-empty values.
func splitPipeList(list string) []string {
	var values []string
	for _, part := range strings.Split(list, "|") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

// statusTableValues reads README.md's Lifecycle table and returns the first
// column's backtick-quoted status values, nil if the table cannot be found.
func statusTableValues(readme string) []string {
	lines := strings.Split(readme, "\n")
	header := indexOfStatusTableHeader(lines)
	if header < 0 {
		return nil
	}
	var values []string
	for _, line := range lines[header+2:] { // +2 skips the header and its --- separator
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		if value, ok := firstTableCell(line); ok {
			values = append(values, value)
		}
	}
	return values
}

func indexOfStatusTableHeader(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|") && strings.Contains(line, "`status`") {
			return i
		}
	}
	return -1
}

// firstTableCell returns a markdown table row's first cell, backticks
// stripped, and false if that cell is empty.
func firstTableCell(row string) (string, bool) {
	cells := strings.Split(row, "|")
	if len(cells) < 2 {
		return "", false
	}
	cell := strings.Trim(strings.TrimSpace(cells[1]), "`")
	return cell, cell != ""
}

// vocabularyReporter is the slice of *testing.T that assertVocabularyMatches
// needs. TestVocabularyCheckCatchesAMismatch passes a recorder instead of a
// real *testing.T: a failing t.Run subtest would fail its parent too, which
// would make the very test that proves the check can fail itself fail.
type vocabularyReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// errorRecorder observes whether a comparison reported a mismatch, without
// taking part in Go's subtest pass/fail propagation.
type errorRecorder struct {
	failed bool
}

func (r *errorRecorder) Helper() {}

func (r *errorRecorder) Errorf(_ string, _ ...any) {
	r.failed = true
}

// assertVocabularyMatches fails naming field, document, and every value
// present on only one side of the comparison, never just a boolean mismatch.
func assertVocabularyMatches(t vocabularyReporter, field, document string, got []string, want map[string]bool) {
	t.Helper()
	onlyInDocument := sortedValuesNotIn(got, want)
	onlyInGoSet := sortedValuesNotIn(setMembers(want), set(got...))
	if len(onlyInDocument) == 0 && len(onlyInGoSet) == 0 {
		return
	}
	t.Errorf("field %q: %s and vocabulary.go disagree\n  only in %s: %v\n  only in vocabulary.go: %v",
		field, document, document, onlyInDocument, onlyInGoSet)
}

func sortedValuesNotIn(values []string, allowed map[string]bool) []string {
	var missing []string
	for _, value := range values {
		if !allowed[value] {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func setMembers(members map[string]bool) []string {
	values := make([]string, 0, len(members))
	for value := range members {
		values = append(values, value)
	}
	return values
}

// TestTemplateVocabularyMatchesGoSets proves AC1, AC3 and AC4: every closed-
// vocabulary field's comment in TEMPLATE.md lists exactly the values
// vocabulary.go allows, order ignored, and a field whose comment cannot be
// found or parsed fails naming that field rather than passing vacuously.
func TestTemplateVocabularyMatchesGoSets(t *testing.T) {
	frontmatter := frontmatterOf(t, templatePath())
	for _, field := range vocabularyFields {
		t.Run(field.name, func(t *testing.T) {
			values, ok := fieldCommentValues(frontmatter, field.name)
			if !ok {
				t.Fatalf("could not find a parseable %q comment in TEMPLATE.md", field.name)
			}
			assertVocabularyMatches(t, field.name, "TEMPLATE.md", values, field.allowed)
		})
	}
}

// TestReadmeStatusTableMatchesGoSets proves AC2: the Lifecycle table's first
// column lists exactly the values knownStatuses allows.
func TestReadmeStatusTableMatchesGoSets(t *testing.T) {
	source, err := os.ReadFile(readmePath())
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	values := statusTableValues(string(source))
	if len(values) == 0 {
		t.Fatalf("could not find a parseable status table in README.md")
	}
	assertVocabularyMatches(t, "status", "README.md", values, knownStatuses)
}

// TestFieldCommentValuesFailsClosedWhenUnparseable proves AC4: a field whose
// comment is missing or absent reports ok=false, never an empty-but-valid
// value list. An empty valid list would make assertVocabularyMatches compare
// {} against the Go set and report every known value as "missing" instead of
// the real problem — the comment could not be read at all — so the two
// failure modes must stay distinguishable at this layer, not just at the
// message text in TestTemplateVocabularyMatchesGoSets.
func TestFieldCommentValuesFailsClosedWhenUnparseable(t *testing.T) {
	cases := []struct {
		name        string
		frontmatter string
	}{
		{"field absent from frontmatter entirely", "intake: 1\nid: x\n"},
		{"field present with no trailing comment", "status: draft\n"},
		{"field name only appears as a substring of another key", "order-status: draft # draft | ready\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok := fieldCommentValues(c.frontmatter, "status")
			if ok {
				t.Fatalf("fieldCommentValues(%q, \"status\"): got ok=true, want ok=false", c.frontmatter)
			}
		})
	}
}

// TestVocabularyCheckCatchesAMismatch proves AC5: the same comparison, run
// against a template deliberately edited to disagree with vocabulary.go,
// fails. This is what shows the two tests above are not a vacuous pass.
func TestVocabularyCheckCatchesAMismatch(t *testing.T) {
	frontmatter := frontmatterOf(t, filepath.Join("testdata", "mismatched_vocabulary_template.md"))

	recorder := &errorRecorder{}
	for _, field := range vocabularyFields {
		values, ok := fieldCommentValues(frontmatter, field.name)
		if !ok {
			t.Fatalf("could not find a parseable %q comment in the fixture", field.name)
		}
		assertVocabularyMatches(recorder, field.name, "fixture", values, field.allowed)
	}
	if !recorder.failed {
		t.Fatal("comparison against a deliberately mismatched template passed; it cannot detect real drift")
	}
}
