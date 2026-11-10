package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const docsCheckFixtureFinding = "Finally, this paragraph intentionally contains a sentence that runs well past the twenty five word descriptive limit so the checker reports it as a finding here."

func writeDocsCheckFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDocsCheckDefaultExitsZeroAndPrintsEachFinding(t *testing.T) {
	path := writeDocsCheckFixture(t, docsCheckFixtureFinding+"\n")
	output, err := runDocsCommand(t, runDocsCheck, docsFlags{}, "", path)
	if err != nil {
		t.Fatalf("default invocation must exit 0 even with findings: %v", err)
	}
	assertContainsAll(t, output, "sentence-length", "finding(s) skipped")
}

func TestDocsCheckStrictExitsNonZeroWhenThereIsAFinding(t *testing.T) {
	path := writeDocsCheckFixture(t, docsCheckFixtureFinding+"\n")
	_, err := runDocsCommand(t, runDocsCheck, docsFlags{strict: true}, "", path)
	if !errors.Is(err, errDocStyleFindingsFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDocsCheckStrictExitsZeroWithNoFindings(t *testing.T) {
	path := writeDocsCheckFixture(t, "This file stays well inside every rule the checker enforces.\n")
	_, err := runDocsCommand(t, runDocsCheck, docsFlags{strict: true}, "", path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestDocsCheckOfAMissingPathReturnsAClearErrorNotAFinding(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.md")
	_, err := runDocsCommand(t, runDocsCheck, docsFlags{}, "", missing)
	if err == nil {
		t.Fatal("a missing path must be a usage error")
	}
}

func TestDocsCheckHelpDocumentsTheNounClusterOmission(t *testing.T) {
	if !strings.Contains(docsCheckCmd.Long, "consecutive nouns") || !strings.Contains(docsCheckCmd.Long, "part-of-speech") {
		t.Errorf("Long = %q", docsCheckCmd.Long)
	}
}

func TestWritingStyleDocNamesTheCommandAndItsMechanicalLimits(t *testing.T) {
	path := filepath.Join(repoRoot(t), "shared", "rules", "writing-style.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, string(content), "loom docs check", "consecutive nouns", "reviewer")
}

// repoRoot walks up from the package directory to the module root, so the
// test does not depend on where `go test` was invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the module root")
	return ""
}
