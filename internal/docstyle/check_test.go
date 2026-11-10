package docstyle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckReportsAFindingFromASingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	writeFile(t, path, longDescriptiveSentence+"\n")
	report, err := Check([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != RuleSentenceLength {
		t.Fatalf("report = %+v", report)
	}
}

func TestCheckOnACleanFileReportsNoFindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	writeFile(t, path, "This file stays well inside every rule the checker enforces.\n")
	report, err := Check([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestCheckRecursesIntoADirectoryAndSkipsDotDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nested, "doc.md"), longDescriptiveSentence+"\n")
	hidden := filepath.Join(root, ".git")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(hidden, "doc.md"), longDescriptiveSentence+"\n")

	report, err := Check([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestCheckOfAMissingPathReturnsAClearError(t *testing.T) {
	_, err := Check([]string{filepath.Join(t.TempDir(), "does-not-exist.md")})
	if err == nil {
		t.Fatal("a missing path must be a usage error, not a silent zero-finding report")
	}
}

func TestCheckLoadsDomainDictionaryFromTheCurrentWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeFile(t, filepath.Join(root, dictionaryFileName), fixtureDictionary)
	docPath := filepath.Join(root, "doc.md")
	writeFile(t, docPath, "The PageObject handles requests.\n")

	report, err := Check([]string{docPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != RuleUbiquitousLanguage {
		t.Fatalf("report = %+v", report)
	}
}

func TestCheckWithoutADomainDictionaryStillRunsEveryOtherRule(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	docPath := filepath.Join(root, "doc.md")
	writeFile(t, docPath, "Please utilize the API correctly.\n")

	report, err := Check([]string{docPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != RulePlainVocabulary {
		t.Fatalf("report = %+v", report)
	}
}
