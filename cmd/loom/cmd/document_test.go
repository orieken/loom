package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/state"
)

const testTags = "version: 1\ntags:\n  - {name: api, description: serves}\n"

// useDocumentFixtures points the command at the mock provider and a tiny
// framework FS, restoring both afterwards.
func useDocumentFixtures(t *testing.T, provider orchestrator.Provider) {
	t.Helper()
	originalFS, originalProvider := frameworkFS, documentProvider
	frameworkFS = fstest.MapFS{repodoc.VocabularyFile: {Data: []byte(testTags)}}
	documentProvider = func(string) (orchestrator.Provider, error) { return provider, nil }
	t.Cleanup(func() { frameworkFS, documentProvider = originalFS, originalProvider })
}

// answerEverything scripts a valid document for each directory of the repo.
func answerEverything(t *testing.T, root string) *mock.Provider {
	t.Helper()
	sheets, err := contextindex.CurrentSheets(root, repograph.GitLister{})
	if err != nil {
		t.Fatal(err)
	}
	scripts := map[string]mock.Script{}
	for _, sheet := range sheets {
		doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: sheet.Path, InputHash: sheet.InputHash,
			Purpose: "does " + sheet.Path, Layer: state.LayerShared,
			Evidence: []state.DocEvidence{{Claim: "does " + sheet.Path, Fields: []string{"files"}}}}
		raw, _ := json.Marshal(doc)
		scripts[documenterStage(sheet.Path)] = mock.Script{Payload: raw}
	}
	return mock.New(scripts)
}

// documenterStage mirrors the driver's stage naming for the two paths used here.
func documenterStage(dir string) string {
	if dir == "." {
		return "repo-doc-_root"
	}
	return "repo-doc-" + dir
}

func runDocumentWith(t *testing.T, flags documentFlags) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := documentRepo(context.Background(), &out, flags)
	return out.String(), err
}

func TestDocumentDryRunListsTheBottomUpPlanAndMakesNoCalls(t *testing.T) {
	root := committedGoRepo(t)
	provider := answerEverything(t, root)
	useDocumentFixtures(t, provider)
	output, err := runDocumentWith(t, documentFlags{root: root, dryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, output, "3 directories to document, children first", "a (undocumented)", "b (undocumented)", ". (undocumented)", "no model calls made")
	if len(provider.Invocations()) != 0 {
		t.Errorf("a dry run made calls: %v", provider.Invocations())
	}
	if strings.Index(output, ". (undocumented)") < strings.Index(output, "a (undocumented)") {
		t.Errorf("the root must come after its children:\n%s", output)
	}
}

func TestDocumentRunsTheAgentAndWritesCandidatesWithoutTouchingTheIndex(t *testing.T) {
	root := committedGoRepo(t)
	provider := answerEverything(t, root)
	useDocumentFixtures(t, provider)
	output, err := runDocumentWith(t, documentFlags{root: root})
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	assertContainsAll(t, output, "documenting 3 directories with repo-documenter", "3 documented, 0 failed: candidates in")
	if len(provider.Invocations()) != 3 {
		t.Errorf("invocations = %v", provider.Invocations())
	}
	runs, _ := filepath.Glob(filepath.Join(root, ".loom", "runs", "*", "run.json"))
	if len(runs) != 1 {
		t.Errorf("expected one run record, got %v", runs)
	}
	if _, found, _ := contextindex.Load(root); found {
		t.Error("`loom document` must not write the index")
	}
}

func TestDocumentSaysSoWhenEverythingIsFresh(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	documentEveryEntry(t, root)
	useDocumentFixtures(t, mock.New(nil))
	output, err := runDocumentWith(t, documentFlags{root: root})
	if err != nil || !strings.Contains(output, "nothing to document: every directory in scope is fresh") {
		t.Errorf("%v %q", err, output)
	}
}

func TestDocumentLimitAndPathBoundTheRun(t *testing.T) {
	root := committedGoRepo(t)
	useDocumentFixtures(t, answerEverything(t, root))
	output, _ := runDocumentWith(t, documentFlags{root: root, dryRun: true, limit: 1})
	assertContainsAll(t, output, "1 directories to document")
	output, _ = runDocumentWith(t, documentFlags{root: root, dryRun: true, path: "b"})
	assertContainsAll(t, output, "1 directories to document", "b (undocumented)")
}

func TestDocumentExitsNonZeroWhenADirectoryFails(t *testing.T) {
	root := committedGoRepo(t)
	provider := answerEverything(t, root)
	provider.SetScript("repo-doc-a", mock.Script{Err: errors.New("boom")})
	useDocumentFixtures(t, provider)
	output, err := runDocumentWith(t, documentFlags{root: root})
	if !errors.Is(err, errDocumentFailed) || !strings.Contains(output, "2 documented, 1 failed") {
		t.Errorf("%v\n%s", err, output)
	}
}

func TestDocumentReportsAMissingAgentDefinitionBeforeAnyCall(t *testing.T) {
	root := committedGoRepo(t)
	useDocumentFixtures(t, nil)
	documentProvider = func(string) (orchestrator.Provider, error) { return nil, errors.New("no agent definitions found") }
	if _, err := runDocumentWith(t, documentFlags{root: root}); err == nil || !strings.Contains(err.Error(), "no agent definitions") {
		t.Errorf("err = %v", err)
	}
}

func TestDocumentFailsOutsideAGitRepositoryAndOnABrokenVocabulary(t *testing.T) {
	useDocumentFixtures(t, mock.New(nil))
	if _, err := runDocumentWith(t, documentFlags{root: t.TempDir()}); err == nil {
		t.Error("a non-repository has no files to document")
	}
	frameworkFS = fstest.MapFS{}
	if _, err := runDocumentWith(t, documentFlags{root: committedGoRepo(t)}); err == nil || !strings.Contains(err.Error(), "vocabulary") && !strings.Contains(err.Error(), repodoc.VocabularyFile) {
		t.Errorf("err = %v", err)
	}
}

func TestDocumentPrintsOnlyTheFirstPlanLinesOfALongDryRun(t *testing.T) {
	var out bytes.Buffer
	var tasks []repodoc.Task
	for i := 0; i < maxPlanLines+3; i++ {
		tasks = append(tasks, repodoc.Task{Path: "d" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Status: contextindex.StatusUndocumented})
	}
	printDocumentPlan(&out, tasks)
	if !strings.Contains(out.String(), "… and 3 more") {
		t.Errorf("%s", out.String())
	}
}

func TestSummaryMentionsInterruptionAndReportedCost(t *testing.T) {
	record := repodoc.RunRecord{RunID: "r", Interrupted: true, Outcomes: []repodoc.Outcome{{Status: repodoc.StatusDocumented}},
		TotalUsage: &orchestrator.Usage{CostUSD: 1.5}}
	line := summarizeDocumentRun(record, "/root")
	assertContainsAll(t, line, "1 documented, 0 failed", "(interrupted)", "$1.5000")
}

func TestFindAgentsDirInLooksUnderTheGivenRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := findAgentsDirIn(root); err == nil {
		t.Error("no agents directory exists yet")
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, err := findAgentsDirIn(root); err != nil || !strings.HasSuffix(dir, filepath.Join(".claude", "agents")) {
		t.Errorf("%q %v", dir, err)
	}
}
