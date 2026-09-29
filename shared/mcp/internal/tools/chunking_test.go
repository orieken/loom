package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestADocumentSplitsAtItsSectionHeadings(t *testing.T) {
	chunks := chunkDocument("# Title\n\nintro\n\n## First\n\none\n\n## Second\n\ntwo\n")
	if len(chunks) != 3 {
		t.Fatalf("chunks = %+v, want intro and two sections", chunks)
	}
	assertChunk(t, chunks[0], "", "# Title", "one")
	assertChunk(t, chunks[1], "First", "## First", "two")
	assertChunk(t, chunks[2], "Second", "## Second", "one")
}

// assertChunk checks a chunk's heading, how its text begins, and that it
// holds nothing of a neighbouring section.
func assertChunk(t *testing.T, chunk docChunk, heading, begins, excludes string) {
	t.Helper()
	if chunk.heading != heading || !strings.HasPrefix(chunk.text, begins) || strings.Contains(chunk.text, excludes) {
		t.Errorf("chunk = %+v, want heading %q, beginning %q, without %q", chunk, heading, begins, excludes)
	}
}

func TestFrontmatterIsNotEmbedded(t *testing.T) {
	chunks := chunkDocument("---\nname: Secret Name\ntags: [x]\n---\n\nbody text\n")
	if len(chunks) != 1 || strings.Contains(chunks[0].text, "Secret Name") || !strings.Contains(chunks[0].text, "body text") {
		t.Errorf("chunks = %+v, want the body alone", chunks)
	}
	unterminated := chunkDocument("---\nname: never closed\n")
	if len(unterminated) != 1 || !strings.Contains(unterminated[0].text, "never closed") {
		t.Errorf("an unterminated block was dropped: %+v", unterminated)
	}
}

func TestEmptySectionsAreDropped(t *testing.T) {
	if chunks := chunkDocument("## Only\n"); len(chunks) != 1 {
		t.Errorf("chunks = %+v, want the heading's own section", chunks)
	}
	if chunks := chunkDocument("   \n\n"); len(chunks) != 0 {
		t.Errorf("a blank document produced %+v", chunks)
	}
}

// A long section splits at whitespace within the limit, never mid-word, and
// loses nothing.
func TestALongSectionSplitsAtWhitespaceAndLosesNothing(t *testing.T) {
	// Six runes a word, so a cut at the bare limit (a multiple of five, not
	// six) would land mid-word.
	body := strings.Repeat("words ", 900) // 5400 runes
	chunks := chunkDocument(body)
	if len(chunks) != 3 {
		t.Fatalf("%d chunks, want 3", len(chunks))
	}
	var rejoined strings.Builder
	for _, chunk := range chunks {
		if runes := utf8.RuneCountInString(chunk.text); runes > maxChunkRunes {
			t.Errorf("chunk of %d runes exceeds %d", runes, maxChunkRunes)
		}
		if !strings.HasSuffix(chunk.text, " ") && chunk.text != chunks[len(chunks)-1].text {
			t.Errorf("chunk ends mid-word: %q", chunk.text[len(chunk.text)-10:])
		}
		rejoined.WriteString(chunk.text)
	}
	if rejoined.String() != body {
		t.Error("splitting lost or changed text")
	}
}

// Text with no whitespace is cut at the limit — on a rune boundary, so a
// multi-byte character is never split.
func TestUnbrokenTextIsCutOnARuneBoundary(t *testing.T) {
	body := strings.Repeat("é", maxChunkRunes+10)
	chunks := chunkDocument(body)
	if len(chunks) != 2 || utf8.RuneCountInString(chunks[0].text) != maxChunkRunes || !utf8.ValidString(chunks[0].text) {
		t.Errorf("chunks of %d and %d runes, want a clean cut at %d", utf8.RuneCountInString(chunks[0].text), utf8.RuneCountInString(chunks[1].text), maxChunkRunes)
	}
}

func TestAnExcerptIsOneBoundedLine(t *testing.T) {
	short := excerptOf(docChunk{text: "## Heading\n\nsome   words\n"})
	if short != "## Heading some words" {
		t.Errorf("excerpt = %q", short)
	}
	long := excerptOf(docChunk{text: strings.Repeat("ü", excerptRunes+5)})
	if utf8.RuneCountInString(long) != excerptRunes+3 || !strings.HasSuffix(long, "...") {
		t.Errorf("long excerpt has %d runes", utf8.RuneCountInString(long))
	}
}

// Only the body survives, exactly — whatever the frontmatter held, and even
// when it held nothing. An empty block was once missed and embedded whole.
func TestFrontmatterIsRemovedExactly(t *testing.T) {
	cases := map[string]string{
		"---\nname: x\ntags: [a]\n---\nbody text\n": "body text\n",
		"---\n---\nbody text\n":                     "body text\n",
		"---\n\n---\nbody text\n":                   "body text\n",
	}
	for document, want := range cases {
		if got := stripFrontmatter(document); got != want {
			t.Errorf("stripFrontmatter(%q) = %q, want %q", document, got, want)
		}
	}
}

// A section that fits exactly is one chunk: the limit is inclusive.
func TestASectionOfExactlyTheLimitIsOneChunk(t *testing.T) {
	body := strings.Repeat("ab ", maxChunkRunes/3) + strings.Repeat("x", maxChunkRunes%3)
	if runes := utf8.RuneCountInString(body); runes != maxChunkRunes {
		t.Fatalf("fixture is %d runes, want exactly %d", runes, maxChunkRunes)
	}
	if chunks := chunkDocument(body); len(chunks) != 1 {
		t.Errorf("%d chunks for a section of exactly %d runes, want 1", len(chunks), maxChunkRunes)
	}
}

// An excerpt that fits exactly is not marked as cut.
func TestAnExcerptOfExactlyTheLimitIsNotMarkedCut(t *testing.T) {
	exact := strings.Repeat("e", excerptRunes)
	if got := excerptOf(docChunk{text: exact}); got != exact {
		t.Errorf("an excerpt of exactly %d runes became %d runes", excerptRunes, utf8.RuneCountInString(got))
	}
}
