package repodoc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/provider/mock"
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
	input, err := buildInput(task, summaries{thisRun: map[string]state.RepoDocState{}, recorded: emptyIndex()}, newVocabulary([]Tag{{Name: "api", Description: "serves"}}), nil)
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

func TestARecordedChildSummaryThatPredatesTheChildsLatestChangeIsStaleAndItsTextWithheld(t *testing.T) {
	known := summaries{
		thisRun: map[string]state.RepoDocState{},
		recorded: contextindex.Index{Entries: map[string]contextindex.Entry{
			"svc/api":   {Purpose: "old api purpose", InputHash: "h-old"},
			"svc/store": {Purpose: "current store purpose", InputHash: "h-store"},
		}},
		current: map[string]string{"svc/api": "h-api-now", "svc/store": "h-store"},
	}
	got := known.forChildren([]string{"svc/api", "svc/store"})
	if got[0].Status != summaryStale || got[0].Purpose != "" || len(got[0].Tags) != 0 {
		t.Errorf("a parent must not be written from a description of code that no longer exists: %+v", got[0])
	}
	if got[1].Status != summaryDocumented || got[1].Purpose != "current store purpose" {
		t.Errorf("a recorded summary that still matches is current: %+v", got[1])
	}
}

func TestAChildWithNoKnownCurrentHashKeepsItsRecordedSummary(t *testing.T) {
	known := summaries{recorded: contextindex.Index{Entries: map[string]contextindex.Entry{"svc/api": {Purpose: "p", InputHash: "h"}}}}
	if got := known.one("svc/api"); got.Status != summaryDocumented || got.Purpose != "p" {
		t.Errorf("%+v", got)
	}
}

// End to end: a child that fails this run must not leak its stale recorded text
// to its parent.
func TestAFailedChildDoesNotLeakItsStaleRecordedTextToItsParent(t *testing.T) {
	provider := scriptedProvider(t, tree())
	provider.SetScript(stageID("svc/api"), mock.Script{Err: errors.New("model unavailable")})
	runner := newRunner(provider, t.TempDir())
	runner.Recorded = contextindex.Index{Entries: map[string]contextindex.Entry{"svc/api": {Purpose: "OLD API TEXT", InputHash: "h-api-old"}}}
	runner.CurrentHashes = map[string]string{"svc/api": "h-api"}
	if _, err := runner.Run(context.Background(), tasksFor(tree())); err != nil {
		t.Fatal(err)
	}
	parent := string(provider.InputFor(stageID("svc")).UpstreamState[inputChildSummaries])
	if strings.Contains(parent, "OLD API TEXT") || !strings.Contains(parent, `"status": "stale"`) {
		t.Errorf("%s", parent)
	}
}
