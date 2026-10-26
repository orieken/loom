package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// A halted run must be recorded. Most runs anyone investigates are halted
// ones, so a store that only knew about clean completions would miss the
// interesting half.
func TestAHaltedRunIsArchivedAndIngested(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock")
	run.Dir = projectDir
	if output, err := run.CombinedOutput(); err == nil {
		t.Fatalf("run should halt at confirm-design\n%s", output)
	}

	// Archived beside the artifacts, in git rather than in a temporary
	// workspace — that copy is the durable record.
	for _, name := range []string{orchestrator.RunStateFileName, orchestrator.RunEventsFileName} {
		path := filepath.Join(projectDir, "docs", "features", "user-auth", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was not archived: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, memory.DefaultDir, memory.DefaultFileName)); err != nil {
		t.Fatalf("no episodic store was written: %v", err)
	}
}

// Recording is best-effort and must never fail a run. An unwritable store
// path is the cheapest way to prove it.
func TestAnUnwritableStoreDoesNotChangeHowARunEnds(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	// Occupy the memory directory's path with a file.
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("create .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, memory.DefaultDir), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("create blocker: %v", err)
	}

	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	if err == nil {
		t.Fatalf("run should still halt at confirm-design\n%s", output)
	}
	assertStagesCompleted(t, projectDir, stagesBeforeFirstGate, true)
	if !strings.Contains(string(output), "memory: could not record this run") {
		t.Errorf("the failure was not reported:\n%s", output)
	}
}

// Backfill is what makes the archived records worth committing: a deleted
// store must be rebuildable from what is in git.
func TestIngestRebuildsTheStoreFromTheArchive(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock")
	run.Dir = projectDir
	if output, err := run.CombinedOutput(); err == nil {
		t.Fatalf("run should halt at confirm-design\n%s", output)
	}
	storePath := filepath.Join(projectDir, memory.DefaultDir, memory.DefaultFileName)
	if err := os.RemoveAll(filepath.Join(projectDir, memory.DefaultDir)); err != nil {
		t.Fatalf("delete store: %v", err)
	}

	assertLoomOutput(t, binary, projectDir, []string{"memory", "ingest"}, "1 of 1 archived runs ingested")
	if _, err := os.Stat(storePath); err != nil {
		t.Errorf("store was not recreated: %v", err)
	}
	assertLoomOutput(t, binary, projectDir, []string{"memory", "runs"}, "user-auth")
}

func assertLoomOutput(t *testing.T, binary, projectDir string, args []string, want string) {
	t.Helper()
	output, code := runLoom(t, binary, projectDir, args...)
	if code != 0 {
		t.Fatalf("loom %v exited %d:\n%s", args, code, output)
	}
	if !strings.Contains(output, want) {
		t.Errorf("loom %v output does not contain %q:\n%s", args, want, output)
	}
}

// The retries output must state which reading of "more than N" produced its
// rows, since the phrase has two defensible meanings.
func TestRetriesOutputStatesItsThreshold(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()

	output, code := runLoom(t, binary, projectDir, "memory", "retries")
	if code != 0 {
		t.Fatalf("memory retries exited %d:\n%s", code, output)
	}
	if !strings.Contains(output, "more than 2 iterations") || !strings.Contains(output, "3 attempts") {
		t.Errorf("output does not state the threshold it applied:\n%s", output)
	}
}

// A project with no store yet must get an empty answer, not an error.
func TestQueryingBeforeAnyRunIsNotAnError(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()

	output, code := runLoom(t, binary, projectDir, "memory", "runs")
	if code != 0 {
		t.Fatalf("memory runs exited %d on a fresh project:\n%s", code, output)
	}
	if !strings.Contains(output, "No runs recorded yet") {
		t.Errorf("unexpected output on an empty store:\n%s", output)
	}
}

// The archived record is committed, so it must not name the checkout it ran
// in. This is the call site's test: every internal/memory test passes a root
// itself, and none would notice recordEpisode passing "".
func TestRecordEpisodeArchivesWithoutTheProjectRoot(t *testing.T) {
	root, workspace := workspaceWithAbsoluteSpec(t)
	var stderr strings.Builder
	command := &cobra.Command{}
	command.SetErr(&stderr)

	recordEpisode(command, workspace, "user-auth")

	archived := readArchivedState(t, root)
	if strings.Contains(archived, root+string(filepath.Separator)) {
		t.Errorf("the archived run state names the project root %s:\n%s", root, archived)
	}
	if !strings.Contains(archived, `"specPath": "features/user-auth.md"`) {
		t.Errorf("the spec path was not made relative:\n%s", archived)
	}
}

// A root that cannot be resolved is reported, and the record is still
// archived — unchanged, because archiving is best-effort.
func TestRecordEpisodeArchivesUnchangedWhenTheRootCannotBeResolved(t *testing.T) {
	root, workspace := workspaceWithAbsoluteSpec(t)
	resolve := archiveRoot
	t.Cleanup(func() { archiveRoot = resolve })
	archiveRoot = func() (string, error) { return "", errors.New("getwd: no such directory") }
	var stderr strings.Builder
	command := &cobra.Command{}
	command.SetErr(&stderr)

	recordEpisode(command, workspace, "user-auth")

	if !strings.Contains(stderr.String(), "memory: could not resolve repository root: getwd: no such directory") {
		t.Errorf("the failure was not reported:\n%s", stderr.String())
	}
	if archived := readArchivedState(t, root); !strings.Contains(archived, filepath.Join(root, "features", "user-auth.md")) {
		t.Errorf("the record was not archived unchanged:\n%s", archived)
	}
}

func workspaceWithAbsoluteSpec(t *testing.T) (string, string) {
	t.Helper()
	t.Chdir(t.TempDir())
	root, err := archiveRoot()
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, ".claude", "feature-workspace", "user-auth")
	state := orchestrator.NewRunState(orchestrator.DefaultDeliverFeaturePlanName, orchestrator.CreatedByExecutor)
	state.FeatureName = "user-auth"
	state.SpecPath = filepath.Join(root, "features", "user-auth.md")
	if err := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName)).Save(state); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func readArchivedState(t *testing.T, root string) string {
	t.Helper()
	archived, err := os.ReadFile(filepath.Join(root, "docs", "features", "user-auth", orchestrator.RunStateFileName))
	if err != nil {
		t.Fatalf("the run state was not archived: %v", err)
	}
	return string(archived)
}
