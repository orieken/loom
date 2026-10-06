package genblock

import (
	"strings"
	"testing"
)

const human = "# Title\n\nHand-written intro.\n"

func TestAFileThatDoesNotExistGetsJustTheBlock(t *testing.T) {
	text, status, _ := Markdown.Upsert("", false, "hello")
	if status != Created || !strings.HasPrefix(text, "<!-- loom:generated:start hash=sha256:") || !strings.Contains(text, "\nhello\n<!-- loom:generated:end -->\n") {
		t.Errorf("%v\n%s", status, text)
	}
}

func TestAFileWithNoBlockKeepsEveryHumanLineAndGainsTheBlockAtTheEnd(t *testing.T) {
	text, status, _ := Markdown.Upsert(human, true, "generated")
	if status != Appended || !strings.HasPrefix(text, human) || !strings.Contains(text, "generated") {
		t.Errorf("%v\n%s", status, text)
	}
}

func TestRegenerationReplacesOnlyTheBlockAndLeavesTextOutsideUnchanged(t *testing.T) {
	first, _, _ := Markdown.Upsert(human, true, "version one")
	withTail := first + "\nA human footer written after generation.\n"
	second, status, reason := Markdown.Upsert(withTail, true, "version two")
	if status != Replaced || reason != "" {
		t.Fatalf("%v %s", status, reason)
	}
	if !strings.HasPrefix(second, human) || !strings.HasSuffix(second, "\nA human footer written after generation.\n") {
		t.Errorf("text outside the markers changed:\n%s", second)
	}
	if strings.Contains(second, "version one") || !strings.Contains(second, "version two") {
		t.Errorf("the block was not replaced:\n%s", second)
	}
}

func TestRegeneratingIdenticalContentChangesNothing(t *testing.T) {
	first, _, _ := Markdown.Upsert(human, true, "same")
	again, status, _ := Markdown.Upsert(first, true, "same")
	if status != Unchanged || again != first {
		t.Errorf("%v", status)
	}
}

func TestAHandEditedBlockIsAConflictAndTheFileIsLeftAlone(t *testing.T) {
	first, _, _ := Markdown.Upsert(human, true, "generated text")
	edited := strings.Replace(first, "generated text", "generated text, plus my correction", 1)
	text, status, reason := Markdown.Upsert(edited, true, "something new")
	if status != Conflict || text != edited || !strings.Contains(reason, "edited by hand") {
		t.Errorf("a hand-edited block must never be overwritten: %v %q", status, reason)
	}
}

func TestAStartMarkerWithoutAnEndIsAConflictNotAGuess(t *testing.T) {
	broken := "intro\n<!-- loom:generated:start hash=sha256:x -->\nno end\n"
	text, status, reason := Markdown.Upsert(broken, true, "new")
	if status != Conflict || text != broken || !strings.Contains(reason, "no end marker") {
		t.Errorf("%v %q", status, reason)
	}
}

func TestAnEmptyExistingFileJustReceivesTheBlock(t *testing.T) {
	text, status, _ := Markdown.Upsert("\n", true, "x")
	if status != Appended || strings.HasPrefix(text, "\n\n") {
		t.Errorf("%v %q", status, text)
	}
}

func TestEverySyntaxRoundTripsThroughUpsert(t *testing.T) {
	for name, syntax := range map[string]Syntax{
		"markdown": Markdown, "go": LineComment("//"), "block": BlockComment(), "docstring": Docstring(),
	} {
		t.Run(name, func(t *testing.T) {
			created, _, _ := syntax.Upsert("", false, "line one\n\nline three")
			if _, status, _ := syntax.Upsert(created, true, "line one\n\nline three"); status != Unchanged {
				t.Errorf("a freshly written block must read back unchanged, got %v\n%s", status, created)
			}
			if _, status, _ := syntax.Upsert(created, true, "different"); status != Replaced {
				t.Errorf("a changed body must replace, got %v", status)
			}
		})
	}
}

func TestLineCommentBodyKeepsBlankLinesWithoutTrailingSpaces(t *testing.T) {
	block := LineComment("//").Render("a\n\nb")
	if !strings.Contains(block, "\n// a\n//\n// b\n") {
		t.Errorf("%q", block)
	}
}

func TestExtractReturnsTheLiteralAndRecordedHash(t *testing.T) {
	text, _, _ := Markdown.Upsert("", false, "body")
	literal, recorded, found := Extract(text)
	if !found || literal != "body" || recorded != Hash("body") {
		t.Errorf("%q %q %v", literal, recorded, found)
	}
	if _, _, found := Extract(human); found {
		t.Error("no block, nothing to extract")
	}
	if _, _, found := Extract("<!-- loom:generated:start hash=x -->\nnever ends"); found {
		t.Error("a damaged block is not extractable")
	}
}

func TestHashIsStableAndContentSensitive(t *testing.T) {
	a, again, b := Hash("a"), Hash("a"), Hash("b")
	if a != again || a == b || !strings.HasPrefix(a, "sha256:") {
		t.Error("hash")
	}
}

func TestOnlyTheFirstBlockIsManaged(t *testing.T) {
	one, _, _ := Markdown.Upsert("", false, "first")
	two := one + "\n" + Markdown.Render("second") + "\n"
	updated, status, _ := Markdown.Upsert(two, true, "first")
	if status != Unchanged || updated != two {
		t.Errorf("%v", status)
	}
}

func TestAMarkerWhoseHashIsUnreadableIsAConflictNotASecondBlock(t *testing.T) {
	text := "intro\n<!-- loom:generated:start hash= -->\nbody\n<!-- loom:generated:end -->\n"
	updated, status, reason := Markdown.Upsert(text, true, "new")
	if status != Conflict || updated != text || reason == "" {
		t.Errorf("a damaged marker must be reported, not worked around: %v %q", status, reason)
	}
}

func TestAMarkerLineEndingAtHashEqualsIsAlsoDamagedNotProse(t *testing.T) {
	text := "intro\n// loom:generated:start hash=\nbody\n"
	updated, status, _ := Markdown.Upsert(text, true, "new")
	if status != Conflict || updated != text {
		t.Errorf("an anchored marker with nothing after hash= is damaged: %v", status)
	}
}

// A README that explains the marker syntax must not be read as holding a block.
func TestProseAndCodeFencesThatQuoteTheMarkerAreNotBlocks(t *testing.T) {
	doc := "# Markers\n\nA block starts with `<!-- loom:generated:start hash=sha256:abc -->` and ends with\n`<!-- loom:generated:end -->`.\n\n```html\n<!-- loom:generated:start hash=sha256:abc -->\nexample\n<!-- loom:generated:end -->\n```\n"
	if _, _, found := Extract(doc); found {
		t.Fatal("quoted markers are not blocks")
	}
	updated, status, _ := Markdown.Upsert(doc, true, "real")
	if status != Appended || !strings.HasPrefix(updated, doc) {
		t.Errorf("the document is kept whole and a real block appended: %v", status)
	}
	if again, status, _ := Markdown.Upsert(updated, true, "real v2"); status != Replaced || !strings.Contains(again, "```html\n<!-- loom:generated:start hash=sha256:abc -->\nexample") {
		t.Errorf("only the real block is replaced; the quoted example survives: %v\n%s", status, again)
	}
}

func TestEveryRealMarkerFormStillLocates(t *testing.T) {
	for name, syntax := range map[string]Syntax{"markdown": Markdown, "go": LineComment("//"), "block": BlockComment(), "docstring": Docstring()} {
		text := "human\n" + syntax.Render("x") + "\nmore human\n"
		if _, status, _ := syntax.Upsert(text, true, "x"); status != Unchanged {
			t.Errorf("%s: %v", name, status)
		}
	}
}

// Windows and core.autocrlf checkouts turn LF into CRLF. The block must still
// verify, still be replaceable, and the file must stay CRLF.
func TestAFileWithCRLFLineEndingsVerifiesAndKeepsItsLineEndings(t *testing.T) {
	lf, _, _ := Markdown.Upsert(human, true, "version one")
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	if same, status, _ := Markdown.Upsert(crlf, true, "version one"); status != Unchanged || same != crlf {
		t.Fatalf("a CRLF checkout of an unedited block is not a hand edit: %v", status)
	}
	updated, status, _ := Markdown.Upsert(crlf, true, "version two")
	if status != Replaced || !strings.Contains(updated, "version two") {
		t.Fatalf("%v", status)
	}
	if strings.Contains(strings.ReplaceAll(updated, "\r\n", ""), "\n") {
		t.Errorf("the file must stay CRLF throughout: %q", updated)
	}
	if _, _, found := Extract(crlf); !found {
		t.Error("Extract must read CRLF text")
	}
}

func TestAHandEditedBlockInACRLFFileIsStillAConflict(t *testing.T) {
	lf, _, _ := Markdown.Upsert(human, true, "generated")
	edited := strings.ReplaceAll(strings.Replace(lf, "\ngenerated\n", "\ngenerated, edited\n", 1), "\n", "\r\n")
	if text, status, _ := Markdown.Upsert(edited, true, "new"); status != Conflict || text != edited {
		t.Errorf("%v", status)
	}
}
