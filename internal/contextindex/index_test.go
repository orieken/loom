package contextindex

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factsheet"
)

func sheet(path, hash string) factsheet.FactSheet {
	return factsheet.FactSheet{Path: path, InputHash: hash}
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		documented, current string
		want                Status
	}{
		{"", "h1", StatusUndocumented}, {"", "", StatusUndocumented},
		{"h1", "h1", StatusFresh}, {"h1", "h2", StatusStale},
	}
	for _, c := range cases {
		if got := StatusOf(c.documented, c.current); got != c.want {
			t.Errorf("StatusOf(%q, %q) = %q, want %q", c.documented, c.current, got, c.want)
		}
	}
}

func TestBuildStartsNewDirectoriesUndocumentedWithEmptyListsNotNull(t *testing.T) {
	index := Build(Index{}, []factsheet.FactSheet{sheet("a", "h1")})
	entry := index.Entries["a"]
	if entry.Status != StatusUndocumented || entry.Tags == nil || entry.OpenQuestions == nil || entry.Purpose != "" {
		t.Errorf("new entry = %+v", entry)
	}
	if index.SchemaVersion != SchemaVersion {
		t.Errorf("schema version %d", index.SchemaVersion)
	}
}

func TestBuildKeepsWhatWasRecordedAndOnlyRecomputesStatus(t *testing.T) {
	previous := Index{Entries: map[string]Entry{"a": {
		Purpose: "does a", Tags: []string{"x"}, Layer: "domain", InputHash: "h1", DocHash: "d", GeneratedAt: "t",
		Status: StatusFresh, OpenQuestions: []string{"why"},
	}}}
	changed := Build(previous, []factsheet.FactSheet{sheet("a", "h2")}).Entries["a"]
	if changed.Status != StatusStale {
		t.Errorf("a changed directory is stale, got %q", changed.Status)
	}
	changed.Status = StatusFresh
	if !reflect.DeepEqual(changed, previous.Entries["a"]) {
		t.Errorf("indexing altered recorded fields: %+v", changed)
	}
	if Build(previous, []factsheet.FactSheet{sheet("a", "h1")}).Entries["a"].Status != StatusFresh {
		t.Error("an unchanged directory stays fresh")
	}
}

func TestBuildDropsEntriesForDirectoriesThatNoLongerExist(t *testing.T) {
	previous := Index{Entries: map[string]Entry{"gone": {InputHash: "h"}, "kept": {InputHash: "h"}}}
	index := Build(previous, []factsheet.FactSheet{sheet("kept", "h")})
	if _, exists := index.Entries["gone"]; exists || len(index.Entries) != 1 {
		t.Errorf("entries = %v", index.Entries)
	}
}

func TestCheckClassifiesEveryKindOfDrift(t *testing.T) {
	stored := Index{Entries: map[string]Entry{
		"fresh":   {InputHash: "h1"},
		"stale":   {InputHash: "old"},
		"undoc":   {},
		"orphan":  {InputHash: "h"},
		"claimed": {InputHash: "h1", Status: StatusFresh}, // stored status lies: hash differs below
	}}
	sheets := []factsheet.FactSheet{sheet("fresh", "h1"), sheet("stale", "new"), sheet("undoc", "h"), sheet("new", "h"), sheet("claimed", "changed")}
	report := Check(stored, sheets)
	want := Report{Stale: []string{"claimed", "stale"}, Undocumented: []string{"undoc"}, Unindexed: []string{"new"}, Orphaned: []string{"orphan"}, Fresh: 1}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v\nwant     %+v", report, want)
	}
	if !report.NeedsReindex() || report.Documented() != 3 {
		t.Errorf("NeedsReindex=%v Documented=%d", report.NeedsReindex(), report.Documented())
	}
}

func TestUndocumentedAloneDoesNotNeedAReindex(t *testing.T) {
	report := Check(Index{Entries: map[string]Entry{"a": {}}}, []factsheet.FactSheet{sheet("a", "h")})
	if report.NeedsReindex() || len(report.Undocumented) != 1 {
		t.Errorf("%+v", report)
	}
}

func TestLoadReportsAMissingIndexAsNotFoundNotAnError(t *testing.T) {
	if _, found, err := Load(t.TempDir()); found || err != nil {
		t.Errorf("found=%v err=%v", found, err)
	}
}

func TestLoadRejectsCorruptAndUnknownVersionIndexes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".loom/index.json", "{not json")
	if _, _, err := Load(root); err == nil {
		t.Error("corrupt index must be an error")
	}
	writeFile(t, root, ".loom/index.json", `{"schema_version": 99, "entries": {}}`)
	if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "99") {
		t.Errorf("unknown version: %v", err)
	}
}

func TestLoadReportsAnUnreadableIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Path(root), 0o755); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	if _, _, err := Load(root); err == nil {
		t.Error("reading a directory as the index must fail")
	}
}

func TestWriteThenLoadRoundTripsAndASecondWriteChangesNothing(t *testing.T) {
	root := t.TempDir()
	index := Build(Index{}, []factsheet.FactSheet{sheet("a", "h")})
	if written, err := Write(root, index); !written || err != nil {
		t.Fatalf("first write: %v %v", written, err)
	}
	loaded, found, err := Load(root)
	if err != nil || !found || !reflect.DeepEqual(loaded, index) {
		t.Fatalf("round trip: %+v found=%v err=%v", loaded, found, err)
	}
	if written, _ := Write(root, index); written {
		t.Error("identical index was rewritten")
	}
}

func TestGeneratedSchemaPinsTheVersion(t *testing.T) {
	raw, err := GenerateSchema()
	if err != nil || !strings.Contains(string(raw), `"const": 1`) {
		t.Errorf("schema: %v\n%s", err, raw)
	}
}
