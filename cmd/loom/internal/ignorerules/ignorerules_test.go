package ignorerules

import (
	"os"
	"path/filepath"
	"testing"
)

const directory = ".claude/memory"

func ignoresWith(t *testing.T, content string) bool {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	isIgnored, err := IgnoresDirectory(root, directory)
	if err != nil {
		t.Fatal(err)
	}
	return isIgnored
}

func TestIgnoresTheDirectoryByAPlainPattern(t *testing.T) {
	for _, line := range []string{
		".claude/memory/", ".claude/memory", "/.claude/memory/", "/.claude/memory", ".claude/memory/*", ".claude/memory/**",
		".claude/", ".claude", "/.claude/", ".claude/*",
		// Git removes trailing spaces from a pattern.
		".claude/memory/  ", ".claude/memory  ",
	} {
		if !ignoresWith(t, "node_modules\n"+line+"\n") {
			t.Errorf("%q should ignore %s", line, directory)
		}
	}
}

func TestDoesNotIgnoreWhatItCannotJudge(t *testing.T) {
	for _, content := range []string{
		"", "# .claude/memory/\n", ".claude/memory/sessions\n", ".claude/other/\n", "memory\n", ".claude/mem\n",
		"*.log\n", ".claudex/\n", "docs/.claude/memory/\n",
		// Git keeps leading spaces as part of the pattern, so these name a
		// directory that begins with a space (`git check-ignore` confirms
		// it). This test once expected "  .claude/memory/  " to be ignored.
		// That expectation was wrong, and a false "yes" could put captured
		// session data into a commit.
		"  .claude/memory/\n", "  .claude/memory/  \n", " .claude\n",
	} {
		if ignoresWith(t, content) {
			t.Errorf("content %q should not count as ignoring %s", content, directory)
		}
	}
}

func TestANegationAfterTheRuleUndoesIt(t *testing.T) {
	for _, negation := range []string{"!.claude/memory/", "!.claude/memory/keep.txt", "!.claude/", "!*.json"} {
		if ignoresWith(t, ".claude/memory/\n"+negation+"\n") {
			t.Errorf("negation %q should undo the rule", negation)
		}
	}
	if !ignoresWith(t, "!.claude/memory/\n.claude/memory/\n") {
		t.Error("a rule after a negation should win")
	}
}

func TestAMissingIgnoreFileIgnoresNothing(t *testing.T) {
	isIgnored, err := IgnoresDirectory(t.TempDir(), directory)
	if err != nil || isIgnored {
		t.Fatalf("isIgnored = %v, err = %v", isIgnored, err)
	}
}

func TestAnUnreadableIgnoreFileIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := IgnoresDirectory(root, directory); err == nil {
		t.Fatal("a directory named .gitignore should be an error")
	}
}
