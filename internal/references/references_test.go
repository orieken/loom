package references_test

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/references"
)

const fixtureRoot = "testdata/fixture"

var fixtureRules = references.Rules{
	Retired:      map[string]string{"test-driven-developer": "folded into developer (ADR-009)"},
	Placeholders: map[string]bool{"foo": true},
	Historical:   []string{"docs/history/", "*CHANGELOG.md", "*_test.go", "exact/record.md"},
}

func fixtureFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(fixtureRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(fixtureRoot, path)
		files = append(files, filepath.ToSlash(relative))
		return err
	})
	if err != nil {
		t.Fatalf("walk fixture: %v", err)
	}
	return files
}

// The fixture is the check's red proof, kept: every reference that must be
// reported is, and nothing else — not the resolving paths, the placeholder,
// the retired name inside a longer token, the historical files, the test
// file or the binary.
func TestFixtureReportsExactlyTheDanglingReferences(t *testing.T) {
	findings, err := references.Scan(fixtureRoot, fixtureFiles(t), fixtureRules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	type located struct {
		File, Reference string
		Line            int
	}
	var got []located
	for _, finding := range findings {
		got = append(got, located{finding.File, finding.Reference, finding.Line})
	}
	// exact/record.md is historical by an exact-path entry; its neighbour
	// is not. The mutation job showed that without an exact entry here, a
	// negated exact match turned every file historical and still passed.
	want := []located{
		{"exact/live.md", "shared/agents/also-gone.md", 1},
		{"live.md", "shared/agents/gone.md", 2},
		{"live.md", "shared/skills/vanished/SKILL.md", 3},
		{"live.md", "shared/workflows/tdd.md", 3},
		{"live.md", "test-driven-developer", 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings:\n  %+v\nwant:\n  %+v", got, want)
	}
}

func TestARetiredFindingSaysWhy(t *testing.T) {
	findings, err := references.Scan(fixtureRoot, []string{"live.md"}, fixtureRules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, finding := range findings {
		if finding.Reference == "test-driven-developer" {
			if finding.Problem != "retired: folded into developer (ADR-009)" {
				t.Errorf("problem = %q, want the recorded reason", finding.Problem)
			}
			return
		}
	}
	t.Fatal("the retired name was not reported")
}

func TestScanReportsAFileItCannotRead(t *testing.T) {
	if _, err := references.Scan(fixtureRoot, []string{"no-such-file.md"}, fixtureRules); err == nil {
		t.Error("a listed file that cannot be read was skipped — a check that skips what it cannot read passes vacuously")
	}
}
