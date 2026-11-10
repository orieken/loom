package docstyle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureDictionary = `# Domain Dictionary

| Term | Domain | Description | Synonyms to AVOID |
|---|---|---|---|
| **Agent** | Test | An agent. | ` + "`bot`" + ` |
| **Order** | Test | A work order. | ` + "`ticket`" + ` |
| **Something** | Test | A thing. | ` + "`data`" + ` |
| **BasePage** | Test | A page. | ` + "`PageObject`" + ` |
| **Delivery Route** | Test | A route. | ` + "`default route`" + ` |
`

func writeFixtureDictionary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "DOMAIN_DICTIONARY.md")
	writeFile(t, path, fixtureDictionary)
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDictionaryParsesTheSynonymsToAvoidColumn(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	var synonyms []string
	for _, entry := range entries {
		synonyms = append(synonyms, entry.synonym)
	}
	if !contains(synonyms, "PageObject") || !contains(synonyms, "default route") {
		t.Fatalf("synonyms = %v", synonyms)
	}
	if contains(synonyms, "data") {
		t.Fatalf("a skip-listed word must never become a synonym entry: %v", synonyms)
	}
}

// mixedFormatDictionary matches the real DOMAIN_DICTIONARY.md's own shape
// for a cell with no backticks at all: plain comma-separated terms, one of
// them carrying a trailing parenthetical note to strip.
const mixedFormatDictionary = `# Domain Dictionary

| Term | Domain | Description | Synonyms to AVOID |
|---|---|---|---|
| **Persona** | Test | A persona. | user profile, role model (too generic), Character |
`

func TestLoadDictionaryFallsBackToCommaSplittingAndStripsAParenthetical(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DOMAIN_DICTIONARY.md")
	writeFile(t, path, mixedFormatDictionary)
	entries, err := loadDictionary(path)
	if err != nil {
		t.Fatal(err)
	}
	var synonyms []string
	for _, entry := range entries {
		synonyms = append(synonyms, entry.synonym)
	}
	if !contains(synonyms, "user profile") || !contains(synonyms, "role model") || !contains(synonyms, "Character") {
		t.Fatalf("synonyms = %v", synonyms)
	}
	if contains(synonyms, "role model (too generic)") {
		t.Fatalf("a trailing parenthetical must be stripped from the synonym: %v", synonyms)
	}
}

func TestLoadDictionaryOfAMissingFileReturnsNoEntriesAndNoError(t *testing.T) {
	entries, err := loadDictionary(filepath.Join(t.TempDir(), "DOMAIN_DICTIONARY.md"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
}

func TestCheckUbiquitousLanguageReportsAListedSynonymNamingItsCanonicalTerm(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	findings := CheckUbiquitousLanguage(testFile, []Line{{Number: 1, Text: "The PageObject handles requests."}}, entries)
	if len(findings) != 1 || !strings.Contains(findings[0].Excerpt, "BasePage") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckUbiquitousLanguageMatchesOnlyTheExactSurfaceFormNotAPlural(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	exact := CheckUbiquitousLanguage(testFile, []Line{{Number: 1, Text: "Use the default route today."}}, entries)
	if len(exact) != 1 {
		t.Fatalf("exact surface form must be reported: %+v", exact)
	}
	pluralized := CheckUbiquitousLanguage(testFile, []Line{{Number: 1, Text: "Use the default routes today."}}, entries)
	if len(pluralized) != 0 {
		t.Fatalf("a pluralized form must not match the listed synonym's exact surface form: %+v", pluralized)
	}
}

func TestCheckUbiquitousLanguageSkipsASkipListedOrdinaryEnglishSynonym(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	findings := CheckUbiquitousLanguage(testFile, []Line{{Number: 1, Text: "We need more data here."}}, entries)
	if len(findings) != 0 {
		t.Fatalf("a skip-listed entry must never be reported, even though it is a listed synonym: %+v", findings)
	}
}

func TestLoadDictionaryDropsASingleLowercaseWordSynonym(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.synonym == "bot" || entry.synonym == "ticket" {
			t.Fatalf("a single lowercase word is ordinary English and must not become an entry: %q", entry.synonym)
		}
	}
}

func TestCheckUbiquitousLanguageSkipsASingleLowercaseWordSynonym(t *testing.T) {
	entries, err := loadDictionary(writeFixtureDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	findings := CheckUbiquitousLanguage(testFile, []Line{{Number: 1, Text: "The bot files a ticket."}}, entries)
	if len(findings) != 0 {
		t.Fatalf("a single lowercase word is ordinary English and must not be reported: %+v", findings)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
