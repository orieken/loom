package docchange

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/state"
)

var clock = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return clock }

var goRepo = map[string]string{
	"go.mod":          "module example.com/x\n\ngo 1.22\n",
	"cmd/app/main.go": "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { lib.Do() }\n",
	"lib/lib.go":      "package lib\n\n// Do does.\nfunc Do() {}\n",
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
	return err == nil
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRepo creates a committed git repository.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	git(t, root, "init", "-q")
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	return root
}

func vocabulary() repodoc.Vocabulary {
	return repodoc.NewVocabularyForTest("api", "state", "tooling")
}

// documentedRun runs the documenter with a scripted mock over every
// directory, producing real, verified candidates under .loom/runs/.
func documentedRun(t *testing.T, root string, mutate func(*state.RepoDocState)) string {
	t.Helper()
	sheets, err := contextindex.CurrentSheets(root, repograph.GitLister{})
	if err != nil {
		t.Fatal(err)
	}
	scripts := map[string]mock.Script{}
	for _, sheet := range sheets {
		cited := "files"
		if len(sheet.Files) == 0 {
			cited = "children"
		}
		doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: sheet.Path, InputHash: sheet.InputHash,
			Purpose: "does " + sheet.Path, Responsibilities: []string{"resp of " + sheet.Path}, Layer: state.LayerApplication, Tags: []string{"api"},
			OpenQuestions: []string{"why " + sheet.Path + "?"},
			Evidence:      []state.DocEvidence{{Claim: "does " + sheet.Path, Fields: []string{cited}}, {Claim: "resp of " + sheet.Path, Fields: []string{cited}}}}
		if mutate != nil {
			mutate(&doc)
		}
		raw, _ := json.Marshal(doc)
		approved, _ := json.Marshal(state.RepoDocAuditState{SchemaVersion: state.SchemaVersion, Path: sheet.Path, InputHash: sheet.InputHash, Verdict: state.VerdictApproved})
		scripts[repodoc.StageID(sheet.Path)] = mock.Script{Payload: raw}
		scripts[repodoc.AuditStageID(sheet.Path)] = mock.Script{Payload: approved}
	}
	runner := repodoc.Runner{Provider: mock.New(scripts), Root: root, Vocabulary: vocabulary(), Recorded: contextindex.Index{Entries: map[string]contextindex.Entry{}}, Now: runClock()}
	tasks := repodoc.Plan(runner.Recorded, sheets, repodoc.Options{})
	record, err := runner.Run(context.Background(), tasks)
	if err != nil || record.Failed()+record.Unverified() != 0 {
		t.Fatalf("documenting: %+v %v", record, err)
	}
	return record.RunID
}

var runCounter int

// runClock gives each run a distinct id even within one second.
func runClock() func() time.Time {
	return func() time.Time {
		runCounter++
		return clock.Add(time.Duration(runCounter) * time.Minute)
	}
}

func plan(t *testing.T, root string, options Options) Change {
	t.Helper()
	runCounter++
	change, err := Plan(PlanInput{Root: root, Options: options, Vocabulary: vocabulary(), Lister: repograph.GitLister{},
		Now: func() time.Time { return clock.Add(time.Duration(runCounter) * time.Hour) }})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return change
}

// approveAndApply runs the whole gate.
func approveAndApply(t *testing.T, root, id string) Result {
	t.Helper()
	if _, err := Approve(root, id, clock); err != nil {
		t.Fatalf("approve: %v", err)
	}
	result, err := Apply(root, id, clock)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return result
}

// snapshot returns every file under root outside .loom/changes and .git,
// with its content, to prove what a step did and did not touch.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		slashed := filepath.ToSlash(relative)
		if entry.IsDir() {
			if slashed == ".git" || slashed == ".loom/changes" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, readErr := os.ReadFile(path)
		out[slashed] = string(raw)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func targets(change Change, action string) []string {
	var out []string
	for _, artifact := range change.Artifacts {
		if artifact.Action == action {
			out = append(out, artifact.Target)
		}
	}
	sort.Strings(out)
	return out
}

func artifactFor(t *testing.T, change Change, target string) Artifact {
	t.Helper()
	for _, artifact := range change.Artifacts {
		if artifact.Target == target {
			return artifact
		}
	}
	t.Fatalf("no artifact for %s in %+v", target, targets(change, ActionCreate))
	return Artifact{}
}

type stateDoc = state.RepoDocState

func gitLister() repograph.Lister { return repograph.GitLister{} }

func repodocEmptyVocabulary() repodoc.Vocabulary { return repodoc.NewVocabularyForTest("nothing") }
