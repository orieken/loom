package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/repograph"
)

func runContextIndexCheck(t *testing.T, target string) (string, *healthCheck) {
	t.Helper()
	var output bytes.Buffer
	check := &healthCheck{request: healthRequest{target: target, display: target}, output: healthOutput{writer: &output}}
	check.verifyContextIndex()
	return output.String(), check
}

// documentEveryEntry rewrites the stored index as the repo-documenter will:
// each entry records the hash it was documented against.
func documentEveryEntry(t *testing.T, target string) {
	t.Helper()
	index, _, err := contextindex.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	sheets, err := contextindex.CurrentSheets(target, repograph.GitLister{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sheet := range sheets {
		entry := index.Entries[sheet.Path]
		entry.InputHash = sheet.InputHash
		index.Entries[sheet.Path] = entry
	}
	raw, _ := json.Marshal(index)
	if err := os.WriteFile(contextindex.Path(target), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHealthIsSilentWhenTheProjectHasNoContextIndex(t *testing.T) {
	output, check := runContextIndexCheck(t, committedGoRepo(t))
	if output != "" || check.warnings != 0 || check.failures != 0 {
		t.Errorf("an unindexed project must not be nagged: %q", output)
	}
}

func TestHealthReportsAnIndexedButUndocumentedProjectAsCurrent(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	output, check := runContextIndexCheck(t, root)
	if !strings.Contains(output, "context index current (3 paths indexed, none documented yet)") || check.warnings != 0 {
		t.Errorf("output %q warnings %d", output, check.warnings)
	}
}

func TestHealthWarnsAboutStaleEntriesAfterAFileChanges(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	documentEveryEntry(t, root)
	if output, check := runContextIndexCheck(t, root); !strings.Contains(output, "context index fresh (3 paths)") || check.warnings != 0 {
		t.Fatalf("a documented, unchanged tree is fresh: %q", output)
	}
	writeRepoFile(t, root, "b/b.go", "package b\n\nvar V = 2\n")
	output, check := runContextIndexCheck(t, root)
	if !strings.Contains(output, "2 path(s) stale — changed since documented: ., b") || check.warnings != 1 || check.failures != 0 {
		t.Errorf("output %q warnings %d failures %d", output, check.warnings, check.failures)
	}
}

func TestHealthWarnsAboutUnindexedAndOrphanedPaths(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	writeRepoFile(t, root, "c/c.go", "package c\n")
	if out, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if err := os.RemoveAll(filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	output, _ := runContextIndexCheck(t, root)
	for _, want := range []string{"not in the index — run loom index: c", "indexed but no longer exist — run loom index: b"} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q lacks %q", output, want)
		}
	}
}

func TestHealthWarnsAboutUndocumentedPathsOnceDocumentationIsInUse(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	documentEveryEntry(t, root)
	index, _, _ := contextindex.Load(root)
	delete(index.Entries, "a")
	index.Entries["a"] = contextindex.Entry{}
	raw, _ := json.Marshal(index)
	if err := os.WriteFile(contextindex.Path(root), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	output, _ := runContextIndexCheck(t, root)
	if !strings.Contains(output, "1 path(s) undocumented: a") {
		t.Errorf("output %q", output)
	}
}

func TestHealthNamesAtMostFivePathsAndCountsTheRest(t *testing.T) {
	var output bytes.Buffer
	check := &healthCheck{output: healthOutput{writer: &output}}
	check.warnAboutPaths([]string{"a", "b", "c", "d", "e", "f", "g"}, "stale")
	if !strings.Contains(output.String(), "a, b, c, d, e (+2 more)") {
		t.Errorf("output %q", output.String())
	}
	check.warnAboutPaths(nil, "stale")
	if check.warnings != 1 {
		t.Error("no paths means no warning")
	}
}

func dirWithIndexFile(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contextindex.Path(root), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHealthWarnsRatherThanFailsOnACorruptIndex(t *testing.T) {
	output, check := runContextIndexCheck(t, dirWithIndexFile(t, "{broken"))
	if !strings.Contains(output, "context index unreadable") || check.failures != 0 || check.warnings != 1 {
		t.Errorf("output %q", output)
	}
}

func TestHealthWarnsRatherThanFailsWhenTheTreeCannotBeListed(t *testing.T) {
	output, check := runContextIndexCheck(t, dirWithIndexFile(t, `{"schema_version":1,"entries":{}}`))
	if !strings.Contains(output, "freshness not checked") || check.failures != 0 || check.warnings != 1 {
		t.Errorf("output %q", output)
	}
}

func TestHealthListsExactlyFivePathsWithoutAMoreSuffix(t *testing.T) {
	var output bytes.Buffer
	check := &healthCheck{output: healthOutput{writer: &output}}
	check.warnAboutPaths([]string{"a", "b", "c", "d", "e"}, "stale")
	check.warnAboutPaths([]string{"a", "b", "c", "d", "e", "f"}, "stale")
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if strings.Contains(lines[0], "more") || !strings.Contains(lines[1], "(+1 more)") {
		t.Errorf("five is the cap, six overflows by one:\n%s", output.String())
	}
}

// A project that only ran `loom index` has no install manifest; its index
// freshness must still be reported.
func TestHealthReportsContextIndexEvenWithoutAnInstallManifest(t *testing.T) {
	root := committedGoRepo(t)
	runIndexIn(t, root)
	var output bytes.Buffer
	err := executeHealth(healthRequest{target: root, display: root}, healthContent(), &output)
	if err == nil || !strings.Contains(output.String(), "manifest missing") {
		t.Fatalf("a missing manifest still fails: %v", err)
	}
	if !strings.Contains(output.String(), "context index current (3 paths indexed, none documented yet)") {
		t.Errorf("index freshness not reported:\n%s", output.String())
	}
}
