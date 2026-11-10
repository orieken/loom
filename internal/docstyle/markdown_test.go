package docstyle

import (
	"strings"
	"testing"
)

func TestStripFrontmatterRemovesTheFenceAndReportsWhereTheBodyStarts(t *testing.T) {
	source := "---\nfeature: x\n---\nThe body starts here.\n"
	body, firstLine := StripFrontmatter(source)
	if !strings.HasPrefix(body, "The body starts here.") {
		t.Errorf("body = %q", body)
	}
	if firstLine != 4 {
		t.Errorf("firstLine = %d, want 4", firstLine)
	}
}

func TestStripFrontmatterWithNoFenceReturnsTheWholeFileAsBody(t *testing.T) {
	source := "No frontmatter here.\n"
	body, firstLine := StripFrontmatter(source)
	if body != source || firstLine != 1 {
		t.Errorf("body = %q, firstLine = %d", body, firstLine)
	}
}

func TestStripFrontmatterWithAnUnclosedFenceFailsOpenToTheWholeFile(t *testing.T) {
	source := "---\nfeature: x\nThe rest of the file, with no closing fence.\n"
	body, firstLine := StripFrontmatter(source)
	if body != source || firstLine != 1 {
		t.Errorf("an unclosed frontmatter fence must not error the whole check: body = %q, firstLine = %d", body, firstLine)
	}
}

// findText reports whether any returned Line's Text contains want.
func findText(lines []Line, want string) bool {
	for _, line := range lines {
		if strings.Contains(line.Text, want) {
			return true
		}
	}
	return false
}

func TestProseIgnoresAFencedCodeBlock(t *testing.T) {
	body := "Some prose.\n\n```\nutilize this inside a fence\n```\n\nMore prose.\n"
	lines := Prose(body, 1)
	if findText(lines, "utilize this inside a fence") {
		t.Error("a fenced code block's content must not be returned as prose")
	}
	if !findText(lines, "Some prose.") || !findText(lines, "More prose.") {
		t.Errorf("prose outside the fence must still be returned: %+v", lines)
	}
}

func TestProseIgnoresAHeading(t *testing.T) {
	body := "## This heading has exactly thirty words in a row just to see whether the sentence length rule would ever mistakenly count a heading as an ordinary descriptive prose sentence worth flagging here\n\nProse follows.\n"
	lines := Prose(body, 1)
	if findText(lines, "This heading") {
		t.Error("a heading's text must not be returned as prose")
	}
}

func TestProseIgnoresATableRow(t *testing.T) {
	body := "| utilize | before |\n|---|---|\n\nProse follows.\n"
	lines := Prose(body, 1)
	if findText(lines, "utilize") {
		t.Error("a table row's content must not be returned as prose")
	}
}

func TestProseIgnoresAnHTMLComment(t *testing.T) {
	body := "Prose before.\n<!-- utilize this comment -->\nProse after.\n<!-- a multi-line comment\nstill hides utilize here\n-->\nProse once more.\n"
	lines := Prose(body, 1)
	if findText(lines, "utilize") {
		t.Error("an HTML comment's content, single- or multi-line, must not be returned as prose")
	}
	if !findText(lines, "Prose before.") || !findText(lines, "Prose after.") || !findText(lines, "Prose once more.") {
		t.Errorf("prose around the comments must still be returned: %+v", lines)
	}
}

func TestProseStripsInlineCodeButKeepsSurroundingProse(t *testing.T) {
	body := "See `utilize` in the span, but the surrounding words remain.\n"
	lines := Prose(body, 1)
	if findText(lines, "utilize") {
		t.Error("an inline code span's content must not be returned as prose")
	}
	if !findText(lines, "surrounding words remain") {
		t.Errorf("prose outside the span must still be returned: %+v", lines)
	}
}

func TestProseDropsALinksTargetButScansItsVisibleText(t *testing.T) {
	body := "Please [utilize this](https://example.com/utilize) link.\n"
	lines := Prose(body, 1)
	if findText(lines, "example.com") || findText(lines, "https") {
		t.Error("a link's target must not be returned as prose")
	}
	if !findText(lines, "utilize this") {
		t.Errorf("a link's visible text must still be scanned like ordinary prose: %+v", lines)
	}
}
