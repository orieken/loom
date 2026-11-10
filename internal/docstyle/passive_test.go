package docstyle

import "testing"

func TestCheckPassiveVoiceReportsAFileLevelEstimateOverTheThreshold(t *testing.T) {
	var lines []Line
	for i := 1; i <= 17; i++ {
		lines = append(lines, Line{Number: i, Text: "The team ships a feature today."})
	}
	lines = append(lines,
		Line{Number: 18, Text: "The report was written yesterday."},
		Line{Number: 19, Text: "The task is done now."},
		Line{Number: 20, Text: "The form was completed already."},
	)
	// 3 of 20 sentences are passive: 15%.
	findings := CheckPassiveVoice(testFile, lines)
	if len(findings) != 1 || findings[0].Rule != RulePassiveVoice || findings[0].Excerpt != "estimate: ~15% passive" {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckPassiveVoiceDoesNotReportAtOrBelowTheThreshold(t *testing.T) {
	var lines []Line
	for i := 1; i <= 20; i++ {
		lines = append(lines, Line{Number: i, Text: "The team ships a feature today."})
	}
	findings := CheckPassiveVoice(testFile, lines)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckPassiveVoiceWithNoProseReportsNothing(t *testing.T) {
	if findings := CheckPassiveVoice(testFile, nil); len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}
