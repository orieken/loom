package docstyle

import (
	"strings"
	"testing"
)

func TestCheckPlainVocabularyReportsPriorToWithItsReplacement(t *testing.T) {
	findings := CheckPlainVocabulary(testFile, []Line{{Number: 2, Text: "We tested the feature prior to shipping it."}})
	if len(findings) != 1 || !strings.Contains(findings[0].Excerpt, "before") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckPlainVocabularyReportsUtilizeWithItsReplacement(t *testing.T) {
	findings := CheckPlainVocabulary(testFile, []Line{{Number: 1, Text: "Please utilize the API correctly."}})
	if len(findings) != 1 || !strings.Contains(findings[0].Excerpt, "use") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckPlainVocabularyDoesNotReportPlainWords(t *testing.T) {
	findings := CheckPlainVocabulary(testFile, []Line{{Number: 1, Text: "We tested the feature before shipping it."}})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}
