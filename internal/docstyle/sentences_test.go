package docstyle

import "testing"

const testFile = "doc.md"

func TestCheckSentenceLengthReportsA26WordDescriptiveSentence(t *testing.T) {
	// 26 words.
	text := "Finally, this paragraph intentionally contains a sentence that runs well past the twenty five word descriptive limit so the checker reports it as a finding here."
	findings := CheckSentenceLength(testFile, []Line{{Number: 5, Text: text}})
	if len(findings) != 1 || findings[0].Rule != RuleSentenceLength || findings[0].Line != 5 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckSentenceLengthDoesNotReportAShortDescriptiveSentence(t *testing.T) {
	text := "This paragraph stays well under the word limit so the checker reports nothing here."
	findings := CheckSentenceLength(testFile, []Line{{Number: 1, Text: text}})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckSentenceLengthReportsA21WordNumberedListStep(t *testing.T) {
	// 21 words after the "1. " marker.
	text := "1. This numbered list step intentionally runs long enough to pass the twenty word list step limit for the checker today now."
	findings := CheckSentenceLength(testFile, []Line{{Number: 3, Text: text}})
	if len(findings) != 1 || findings[0].Rule != RuleSentenceLength {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckSentenceLengthDoesNotReportAShortNumberedListStep(t *testing.T) {
	text := "1. This numbered list step stays short enough."
	findings := CheckSentenceLength(testFile, []Line{{Number: 1, Text: text}})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestCheckSentenceLengthJudgesA21WordBulletedItemByTheDescriptiveThreshold(t *testing.T) {
	// The same 21 words that trigger the 20-word list-step threshold above,
	// but written as a bulleted item: 21 is under the 25-word descriptive
	// threshold a bulleted item is judged against, so nothing is reported.
	text := "- This numbered list step intentionally runs long enough to pass the twenty word list step limit for the checker today now."
	findings := CheckSentenceLength(testFile, []Line{{Number: 1, Text: text}})
	if len(findings) != 0 {
		t.Fatalf("a 21-word bulleted item must not be judged against the 20-word list-step threshold: %+v", findings)
	}
}
