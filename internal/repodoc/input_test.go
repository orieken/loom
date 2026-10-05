package repodoc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/state"
)

func childFixture() summaries {
	return summaries{
		thisRun: map[string]state.RepoDocState{"svc/api": validDoc("svc/api", "h-api")},
		recorded: contextindex.Index{Entries: map[string]contextindex.Entry{
			"svc/api":   {Purpose: "recorded api", InputHash: "x"},
			"svc/store": {Purpose: "recorded store", Tags: []string{"state"}, InputHash: "h"},
			"svc/ghost": {Purpose: "never documented"}, // no input hash: not documented
		}},
	}
}

func TestChildSummaryPrefersThisRunOverTheIndex(t *testing.T) {
	got := childFixture().forChildren([]string{"svc/api", "svc/store"})
	if got[0].Purpose != "does svc/api" || got[0].Status != summaryDocumented {
		t.Errorf("this run wins: %+v", got[0])
	}
	if got[1].Purpose != "recorded store" || got[1].Tags[0] != "state" {
		t.Errorf("recorded fallback: %+v", got[1])
	}
}

func TestChildWithoutADocumentedEntryIsUndocumented(t *testing.T) {
	got := childFixture().forChildren([]string{"svc/ghost", "svc/none"})
	for _, summary := range got {
		if summary.Status != summaryUndocumented || summary.Purpose != "" {
			t.Errorf("an entry without an input hash, or no entry, is undocumented: %+v", summary)
		}
	}
}

func TestBuildInputHasTheSheetChildrenAndAllowedTagsAndNothingElse(t *testing.T) {
	task := Task{Path: "svc", Sheet: tree()[1]}
	input, err := buildInput(task, summaries{thisRun: map[string]state.RepoDocState{}, recorded: emptyIndex()}, newVocabulary([]Tag{{Name: "api", Description: "serves"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(input) != 3 {
		t.Fatalf("exactly three blocks, got %d: no raw source rides along", len(input))
	}
	var sheet map[string]any
	if err := json.Unmarshal(input[inputFactSheet], &sheet); err != nil || sheet["input_hash"] != "h-svc" {
		t.Errorf("fact sheet block: %v %v", sheet, err)
	}
	if !strings.Contains(string(input[inputChildSummaries]), `"svc/api"`) || !strings.Contains(string(input[inputAllowedTags]), `"api"`) {
		t.Errorf("children %s tags %s", input[inputChildSummaries], input[inputAllowedTags])
	}
}
