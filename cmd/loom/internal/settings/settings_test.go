package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalUserFile is already in the layout the adapter writes (sorted
// keys, two-space indent, one trailing newline). A merge followed by an
// unmerge MUST give back exactly these bytes.
const canonicalUserFile = `{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "command": "echo a && echo b",
            "type": "command"
          }
        ],
        "matcher": "Bash"
      }
    ],
    "afterEdit": "npm test --changed"
  },
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  }
}
`

var captureHooks = []Hook{
	{Event: "PreToolUse", Command: "loom capture hook PreToolUse --root '/p'"},
	{Event: "PostToolUse", Command: "loom capture hook PostToolUse --root '/p'"},
	{Event: "Stop", Command: "loom capture hook Stop --root '/p'"},
}

var captureSelector = Selector{Prefix: "loom capture hook "}

func writeFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func rawHooks(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(readFile(t, path)), &top); err != nil {
		t.Fatal(err)
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(top["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	return hooks
}

func compact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var out strings.Builder
	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, raw); err != nil {
		t.Fatal(err)
	}
	out.Write(buffer.Bytes())
	return out.String()
}

func TestMergeCreatesTheFileWithPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.local.json")
	result, err := Merge(path, captureHooks)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Added != 3 {
		t.Fatalf("result = %+v", result)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	count, _ := CountEntries(path, captureSelector)
	if count != 3 {
		t.Fatalf("entries = %d, want 3", count)
	}
}

func TestMergeTwiceChangesNothing(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o600)
	if _, err := Merge(path, captureHooks); err != nil {
		t.Fatal(err)
	}
	first := readFile(t, path)
	result, err := Merge(path, captureHooks)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || readFile(t, path) != first {
		t.Fatalf("second merge added %d and changed the file", result.Added)
	}
}

func TestMergeKeepsUserContentAndUnknownKeys(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o600)
	if _, err := Merge(path, captureHooks); err != nil {
		t.Fatal(err)
	}
	hooks := rawHooks(t, path)
	if string(hooks["afterEdit"]) != `"npm test --changed"` {
		t.Fatalf("afterEdit = %s", hooks["afterEdit"])
	}
	if !strings.Contains(readFile(t, path), `"echo a && echo b"`) {
		t.Fatal("user command lost its characters (HTML escaping?)")
	}
	if !strings.Contains(readFile(t, path), `"Bash(go test:*)"`) {
		t.Fatal("unknown top-level key lost")
	}
	var pre []json.RawMessage
	_ = json.Unmarshal(hooks["PreToolUse"], &pre)
	if len(pre) != 2 || !strings.Contains(compact(t, pre[0]), `"matcher":"Bash"`) {
		t.Fatalf("PreToolUse = %s, want user entry first then ours", hooks["PreToolUse"])
	}
}

func TestMergeKeepsTheModeOfAnExistingFile(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o640)
	if _, err := Merge(path, captureHooks); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", info.Mode().Perm())
	}
}

func TestMergeFollowsASymbolicLink(t *testing.T) {
	resolved := writeFile(t, "{}\n", 0o600)
	link := filepath.Join(filepath.Dir(resolved), "link.json")
	if err := os.Symlink(resolved, link); err != nil {
		t.Skip("no symbolic links here")
	}
	if _, err := Merge(link, captureHooks); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a file")
	}
	if count, _ := CountEntries(resolved, captureSelector); count != 3 {
		t.Fatalf("entries in the real file = %d", count)
	}
}

func TestMergeTreatsAnEmptyFileAsAnEmptyObject(t *testing.T) {
	path := writeFile(t, "  \n", 0o600)
	result, err := Merge(path, captureHooks)
	if err != nil || result.Created || result.Added != 3 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestMergeRefusesAShapeItDoesNotUnderstand(t *testing.T) {
	cases := map[string]string{
		"event value is a string":  `{"hooks": {"PreToolUse": "echo hi"}}`,
		"event value is an object": `{"hooks": {"Stop": {"a": 1}}}`,
		"hooks is an array":        `{"hooks": []}`,
		"hooks is a string":        `{"hooks": "x"}`,
		"not JSON":                 `{"hooks": `,
		"top level is an array":    `[1]`,
		"top level is null":        `null`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, content, 0o600)
			_, err := Merge(path, captureHooks)
			var refusal *NotMergeableError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want *NotMergeableError", err)
			}
			if readFile(t, path) != content {
				t.Fatal("the file was touched")
			}
			if !strings.Contains(refusal.Error(), "Add these entries by hand") || !strings.Contains(refusal.Error(), captureHooks[0].Command) {
				t.Fatalf("message gives no instructions: %s", refusal.Error())
			}
			if checkErr := CheckMergeable(path, captureHooks); checkErr == nil {
				t.Fatal("CheckMergeable accepted the same file")
			}
		})
	}
}

func TestMergeRefusesAFileOverTheSizeCap(t *testing.T) {
	path := writeFile(t, `{"a":"`+strings.Repeat("x", maxSettingsBytes)+`"}`, 0o600)
	var refusal *NotMergeableError
	if _, err := Merge(path, captureHooks); !errors.As(err, &refusal) {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeThenUnmergeGivesBackTheCanonicalFileByteForByte(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o600)
	if _, err := Merge(path, captureHooks); err != nil {
		t.Fatal(err)
	}
	result, err := Unmerge(path, captureSelector, ExistedBefore)
	if err != nil || result.Removed != 3 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if got := readFile(t, path); got != canonicalUserFile {
		t.Fatalf("file after unmerge:\n%s\nwant:\n%s", got, canonicalUserFile)
	}
}

func TestUnmergeRemovesOnlyCaptureEntries(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o600)
	_, _ = Merge(path, captureHooks)
	_, _ = Unmerge(path, captureSelector, ExistedBefore)
	hooks := rawHooks(t, path)
	if _, has := hooks["PostToolUse"]; has {
		t.Fatal("an emptied event array stayed")
	}
	if string(hooks["afterEdit"]) != `"npm test --changed"` {
		t.Fatalf("afterEdit = %s", hooks["afterEdit"])
	}
}

func TestUnmergeUsesTheRecordedCommandsAndThePrefix(t *testing.T) {
	path := writeFile(t, "{}\n", 0o600)
	_, _ = Merge(path, []Hook{{Event: "Stop", Command: "custom-name"}, {Event: "Stop", Command: "loom capture hook Stop"}})
	result, _ := Unmerge(path, Selector{Exact: []string{"custom-name"}}, ExistedBefore)
	if result.Removed != 1 {
		t.Fatalf("exact match removed %d, want 1", result.Removed)
	}
	result, _ = Unmerge(path, captureSelector, ExistedBefore)
	if result.Removed != 1 {
		t.Fatalf("prefix match removed %d, want 1", result.Removed)
	}
}

func TestUnmergeKeepsAUserCommandInsideOurElement(t *testing.T) {
	mixed := `{"hooks":{"Stop":[{"matcher":"x","hooks":[{"type":"command","command":"loom capture hook Stop"},{"type":"command","command":"mine"}]}]}}`
	path := writeFile(t, mixed, 0o600)
	result, err := Unmerge(path, captureSelector, ExistedBefore)
	if err != nil || result.Removed != 1 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	content := readFile(t, path)
	if !strings.Contains(content, `"mine"`) || strings.Contains(content, "loom capture") || !strings.Contains(content, `"matcher": "x"`) {
		t.Fatalf("content = %s", content)
	}
}

func TestUnmergeLeavesTheFileAloneWhenNothingMatches(t *testing.T) {
	odd := `{ "hooks" : { "Stop": [ {"hooks":[{"type":"command","command":"mine"}]} ] } }`
	path := writeFile(t, odd, 0o600)
	result, err := Unmerge(path, captureSelector, CreatedByLoom)
	if err != nil || result.Removed != 0 || result.DeletedFile {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if readFile(t, path) != odd {
		t.Fatal("the file was rewritten")
	}
}

func TestUnmergeDeletesAFileLoomCreatedOnlyWhenItIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.local.json")
	_, _ = Merge(path, captureHooks)
	result, err := Unmerge(path, captureSelector, CreatedByLoom)
	if err != nil || !result.DeletedFile {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("the file stayed")
	}
}

func TestUnmergeKeepsAFileThatHoldsOtherKeys(t *testing.T) {
	path := writeFile(t, `{"model":"x"}`, 0o600)
	_, _ = Merge(path, captureHooks)
	result, _ := Unmerge(path, captureSelector, CreatedByLoom)
	if result.DeletedFile || !strings.Contains(readFile(t, path), `"model": "x"`) {
		t.Fatalf("a file with another key was deleted: %+v", result)
	}
}

func TestUnmergeKeepsAnEmptyObjectInAFileTheUserHad(t *testing.T) {
	path := writeFile(t, "{}\n", 0o600)
	_, _ = Merge(path, captureHooks)
	result, _ := Unmerge(path, captureSelector, ExistedBefore)
	if result.DeletedFile || readFile(t, path) != "{}\n" {
		t.Fatalf("file = %q, result = %+v", readFile(t, path), result)
	}
}

func TestUnmergeOfAMissingFileDoesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.json")
	result, err := Unmerge(path, captureSelector, CreatedByLoom)
	if err != nil || result.Removed != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("unmerge created the file")
	}
}

func TestSelectorMatchesExactOrPrefixOnly(t *testing.T) {
	selector := Selector{Exact: []string{"a b"}, Prefix: "loom capture hook "}
	for command, want := range map[string]bool{
		"a b": true, "a": false, "loom capture hook Stop": true, "loom capture hooks": false, "x loom capture hook ": false,
	} {
		if selector.Matches(command) != want {
			t.Errorf("Matches(%q) = %v, want %v", command, !want, want)
		}
	}
	if (Selector{}).Matches("anything") {
		t.Error("an empty selector matched")
	}
}

func TestCountEntriesReadsOnly(t *testing.T) {
	path := writeFile(t, canonicalUserFile, 0o600)
	count, err := CountEntries(path, captureSelector)
	if err != nil || count != 0 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
	if readFile(t, path) != canonicalUserFile {
		t.Fatal("a read changed the file")
	}
	if _, err := CountEntries(filepath.Join(t.TempDir(), "none"), captureSelector); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}
