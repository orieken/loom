package docstyle

import "testing"

func TestCheckModalConflictReportsAWeakModalAlongsideAnRFC2119Keyword(t *testing.T) {
	text := "The service MUST retry, but it might also log the failure."
	findings := CheckModalConflict(testFile, []Line{{Number: 4, Text: text}})
	if len(findings) != 1 || findings[0].Rule != RuleModalConflict || findings[0].Line != 4 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckModalConflictDoesNotReportAWeakModalAlone(t *testing.T) {
	text := "The service might log the failure."
	findings := CheckModalConflict(testFile, []Line{{Number: 1, Text: text}})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckModalConflictDoesNotMistakeLowercaseMayForTheKeyword(t *testing.T) {
	text := "The team might fix the issue; the outage may happen again regardless."
	findings := CheckModalConflict(testFile, []Line{{Number: 1, Text: text}})
	if len(findings) != 0 {
		t.Fatalf("lower-case 'may' is an ordinary word, not the RFC 2119 keyword: %+v", findings)
	}
}
