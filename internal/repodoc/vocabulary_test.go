package repodoc

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func framework(content string) fstest.MapFS {
	return fstest.MapFS{VocabularyFile: {Data: []byte(content)}}
}

const baseTags = "version: 1\ntags:\n  - {name: api, description: serves}\n  - {name: auth, description: login}\n"

func names(vocabulary Vocabulary) []string {
	var out []string
	for _, tag := range vocabulary.Tags() {
		out = append(out, tag.Name)
	}
	return out
}

func TestVocabularyLoadsFrameworkTagsSorted(t *testing.T) {
	vocabulary, err := LoadVocabulary(framework("version: 1\ntags:\n  - {name: zeta}\n  - {name: alpha}\n"), t.TempDir())
	if err != nil || !reflect.DeepEqual(names(vocabulary), []string{"alpha", "zeta"}) {
		t.Errorf("%v %v", names(vocabulary), err)
	}
}

func projectTags(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ProjectVocabularyFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestProjectVocabularyExtendsTheFrameworkList(t *testing.T) {
	root := projectTags(t, "version: 1\ntags:\n  - {name: billing, description: invoices}\n")
	vocabulary, err := LoadVocabulary(framework(baseTags), root)
	if err != nil || !reflect.DeepEqual(names(vocabulary), []string{"api", "auth", "billing"}) {
		t.Errorf("tags = %v err=%v", names(vocabulary), err)
	}
	if !vocabulary.Contains("auth") || vocabulary.Contains("nope") {
		t.Error("Contains is wrong")
	}
}

func TestProjectMayRedescribeATagButNeverRemoveOne(t *testing.T) {
	root := projectTags(t, "version: 1\ntags:\n  - {name: api, description: our API}\n")
	vocabulary, err := LoadVocabulary(framework(baseTags), root)
	if err != nil || !vocabulary.Contains("auth") {
		t.Fatalf("a project file must not remove framework tags: %v %v", names(vocabulary), err)
	}
	if description := vocabulary.Tags()[0].Description; description != "our API" {
		t.Errorf("a project may re-describe a tag, got %q", description)
	}
}

func TestVocabularyFailsLoudlyOnAMissingFrameworkFileOrBadYAML(t *testing.T) {
	if _, err := LoadVocabulary(fstest.MapFS{}, t.TempDir()); err == nil || !strings.Contains(err.Error(), VocabularyFile) {
		t.Errorf("missing framework file: %v", err)
	}
	if _, err := LoadVocabulary(framework("tags: [unclosed"), t.TempDir()); err == nil {
		t.Error("malformed yaml must fail")
	}
}

func TestVocabularyRejectsANamelessTag(t *testing.T) {
	if _, err := LoadVocabulary(framework("version: 1\ntags:\n  - {description: nameless}\n"), t.TempDir()); err == nil || !strings.Contains(err.Error(), "no name") {
		t.Errorf("a nameless tag: %v", err)
	}
}

func TestVocabularyReportsAnUnreadableProjectFileByName(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loom", "tags.yaml"), 0o755); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	if _, err := LoadVocabulary(framework(baseTags), root); err == nil || !strings.Contains(err.Error(), ProjectVocabularyFile) {
		t.Errorf("unreadable project file: %v", err)
	}
}

func TestShippedVocabularyParsesAndHasNoDuplicates(t *testing.T) {
	raw, err := os.ReadFile("../../" + VocabularyFile)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := parseVocabulary(raw, VocabularyFile)
	if err != nil || len(tags) < 20 {
		t.Fatalf("shipped vocabulary: %d tags, %v", len(tags), err)
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if seen[tag.Name] || tag.Description == "" {
			t.Errorf("tag %q is duplicated or undescribed", tag.Name)
		}
		seen[tag.Name] = true
	}
}
