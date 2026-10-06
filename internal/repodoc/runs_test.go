package repodoc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLatestRunIsEmptyWhenThereAreNoRuns(t *testing.T) {
	root := t.TempDir()
	if id, err := LatestRun(root); id != "" || err != nil {
		t.Errorf("no .loom/runs yet: %q %v", id, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".loom", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if id, err := LatestRun(root); id != "" || err != nil {
		t.Errorf("an empty runs directory: %q %v", id, err)
	}
}

func TestLatestRunIsTheNewestTimestampAndIgnoresFiles(t *testing.T) {
	root := t.TempDir()
	runs := filepath.Join(root, ".loom", "runs")
	for _, name := range []string{"20260101T000000Z", "20261005T120000Z", "20260601T000000Z"} {
		if err := os.MkdirAll(filepath.Join(runs, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(runs, "99999999T999999Z"), []byte("a file, not a run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, err := LatestRun(root); id != "20261005T120000Z" || err != nil {
		t.Errorf("%q %v", id, err)
	}
}

func TestLatestRunReportsAnUnreadableRunsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loom", "runs"), []byte("a file where a directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LatestRun(root); err == nil {
		t.Error("a runs path that is not a directory must be an error, not 'no runs'")
	}
}

func writeCandidates(t *testing.T, dirs ...string) string {
	t.Helper()
	runDir := t.TempDir()
	for _, dir := range dirs {
		if err := writePayload(CandidatePath(runDir, dir), []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	return runDir
}

func TestCandidatesMapsFilesBackToTheirDirectories(t *testing.T) {
	runDir := writeCandidates(t, ".", "svc", "svc/api")
	found, err := Candidates(runDir)
	if err != nil || len(found) != 3 {
		t.Fatalf("%v %v", found, err)
	}
	if found["."] != CandidatePath(runDir, ".") || found["svc/api"] != CandidatePath(runDir, "svc/api") {
		t.Errorf("candidates = %v", found)
	}
}

func TestCandidatesIgnoresFilesThatAreNotJSON(t *testing.T) {
	runDir := writeCandidates(t, "svc")
	if err := os.WriteFile(filepath.Join(runDir, "docs", "notes.txt"), []byte("not a candidate"), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, _ := Candidates(runDir); len(found) != 1 {
		t.Errorf("%v", found)
	}
}

func TestCandidatesOfARunWithNoneIsEmptyNotAnError(t *testing.T) {
	found, err := Candidates(t.TempDir())
	if err != nil || len(found) != 0 {
		t.Errorf("%v %v", found, err)
	}
}

func TestCandidateDirInvertsTheMirroring(t *testing.T) {
	for dir, relative := range map[string]string{".": "_root.json", "svc": "svc.json", "a/b": "a/b.json"} {
		if got := candidateDir(relative); got != dir {
			t.Errorf("candidateDir(%q) = %q, want %q", relative, got, dir)
		}
	}
}

func TestExportedStageIDsMatchTheInternalOnes(t *testing.T) {
	if StageID("svc/api") != stageID("svc/api") || AuditStageID("svc/api") != auditStageID("svc/api") {
		t.Error("the exported helpers must not diverge from the real stage names")
	}
	vocabulary := NewVocabularyForTest("b", "a")
	if !reflect.DeepEqual(names(vocabulary), []string{"a", "b"}) {
		t.Errorf("%v", names(vocabulary))
	}
}

func TestRunsOfARepoWithNoRunsIsNil(t *testing.T) {
	if got, err := Runs(t.TempDir()); got != nil || err != nil {
		t.Errorf("no runs yet: %v %v", got, err)
	}
}

func TestRunsListsEveryRunNewestFirstAndIgnoresFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"20260101T000000Z", "20261005T120000Z", "20260601T000000Z"} {
		if err := os.MkdirAll(filepath.Join(root, ".loom", "runs", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".loom", "runs", "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Runs(root)
	if err != nil || !reflect.DeepEqual(got, []string{"20261005T120000Z", "20260601T000000Z", "20260101T000000Z"}) {
		t.Errorf("%v %v", got, err)
	}
}

func TestRunsReportsAnUnreadableRunsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loom", "runs"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Runs(root); err == nil {
		t.Error("expected an error")
	}
}
