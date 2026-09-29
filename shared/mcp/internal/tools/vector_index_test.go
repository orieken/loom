package tools

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// conceptEmbedder is a deterministic stand-in for a model: each word maps to
// the axis of its concept, so "preference" and "settings" land together. It
// tests the index's mechanics — never meaning, which only the real-model test
// (vector_integration_test.go) can show.
type conceptEmbedder struct {
	model     string
	mutex     sync.Mutex
	embedded  []string // every document text embedded, in order
	queries   int
	failAfter int // fail EmbedDocuments once this many documents are embedded; 0 never
}

var concepts = map[string]int{
	"settings": 0, "preference": 0, "preferences": 0,
	"sync": 1, "persistence": 1, "devices": 1,
	"refund": 2, "payment": 2, "payments": 2,
	"login": 3, "throttle": 3,
}

const conceptDimensions = 5 // the last axis holds every word outside a concept

func (c *conceptEmbedder) Model() string { return c.model }

func (c *conceptEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.failAfter > 0 && len(c.embedded)+len(texts) > c.failAfter {
		return nil, errors.New("embedder down")
	}
	c.embedded = append(c.embedded, texts...)
	vectors := make([][]float32, len(texts))
	for index, text := range texts {
		vectors[index] = conceptVector(text)
	}
	return vectors, nil
}

func (c *conceptEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	c.mutex.Lock()
	c.queries++
	c.mutex.Unlock()
	return conceptVector(text), nil
}

func (c *conceptEmbedder) takeEmbedded() []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	embedded := c.embedded
	c.embedded = nil
	return embedded
}

func conceptVector(text string) []float32 {
	vector := make([]float32, conceptDimensions)
	for _, word := range strings.Fields(strings.ToLower(text)) {
		axis, known := concepts[strings.Trim(word, ".,#:")]
		if !known {
			axis = conceptDimensions - 1
		}
		vector[axis]++
	}
	return vector
}

func newTestVectorIndex(t *testing.T, embedder Embedder) *VectorIndex {
	t.Helper()
	index, err := NewVectorIndex(filepath.Join(t.TempDir(), "rag", "vectors.db"), embedder)
	if err != nil {
		t.Fatalf("NewVectorIndex: %v", err)
	}
	t.Cleanup(func() { _ = index.Close() })
	return index
}

func vectorHits(t *testing.T, index CorpusIndex, root, query string) []string {
	t.Helper()
	refs, err := index.SearchWithin(context.Background(), root, query)
	if err != nil {
		t.Fatalf("SearchWithin: %v", err)
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, filepath.Base(ref.Path))
	}
	return names
}

func TestTheVectorIndexRanksByConceptNotByWord(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "settings.md"), "# Settings persistence\n\nsettings persistence across devices\n")
	WriteFile(t, filepath.Join(docs, "refunds.md"), "# Refunds\n\nrefund a payment\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	refreshIndex(t, index, docs)
	if got := vectorHits(t, index, docs, "preference sync"); len(got) != 2 || got[0] != "settings.md" {
		t.Errorf("ranked %v, want settings.md first", got)
	}
}

// A long document is one hit, scored by its best section.
func TestADocumentIsScoredByItsBestSection(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "mixed.md"), "# Mixed\n\nunrelated words here\n\n## Refunds\n\nrefund payment refund\n")
	WriteFile(t, filepath.Join(docs, "weak.md"), "# Weak\n\nrefund plus many other unrelated words spread thin\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	refreshIndex(t, index, docs)
	refs, err := index.SearchWithin(context.Background(), docs, "refund payment")
	if err != nil || len(refs) != 2 || filepath.Base(refs[0].Path) != "mixed.md" {
		t.Fatalf("refs = %+v (err %v), want mixed.md once and first", refs, err)
	}
	// The title follows the rule BM25 uses: frontmatter name, else the file name.
	if !strings.HasPrefix(refs[0].Summary, "## Refunds") || refs[0].Title != "mixed" {
		t.Errorf("reference = %+v, want the Refunds section's excerpt and the file-name title", refs[0])
	}
}

func TestAVectorSearchStaysWithinItsRoot(t *testing.T) {
	base := t.TempDir()
	docs, sibling := filepath.Join(base, "docs"), filepath.Join(base, "docs2")
	WriteFile(t, filepath.Join(docs, "a.md"), "settings\n")
	WriteFile(t, filepath.Join(sibling, "b.md"), "settings\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	refreshIndex(t, index, docs)
	refreshIndex(t, index, sibling)
	if got := vectorHits(t, index, docs, "settings"); len(got) != 1 || got[0] != "a.md" {
		t.Errorf("searching docs found %v, want only a.md", got)
	}
}

// Only what changed is embedded, and what is gone is evicted.
func TestTheVectorIndexEmbedsOnlyWhatChanged(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "a.md"), "settings\n")
	WriteFile(t, filepath.Join(docs, "b.md"), "refund\n")
	embedder := &conceptEmbedder{model: "m"}
	index := newTestVectorIndex(t, embedder)
	refreshIndex(t, index, docs)
	assertEmbedded(t, "first refresh", embedder, "settings", "refund")
	refreshIndex(t, index, docs)
	assertEmbedded(t, "unchanged refresh", embedder)

	editLater(t, filepath.Join(docs, "a.md"), "settings sync\n")
	if err := os.Remove(filepath.Join(docs, "b.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	refreshIndex(t, index, docs)
	assertEmbedded(t, "after one edit", embedder, "settings sync")
	if got := vectorHits(t, index, docs, "refund"); len(got) != 1 || got[0] != "a.md" {
		t.Errorf("after deleting b.md found %v, want only a.md", got)
	}
}

// assertEmbedded checks the texts embedded since the last check, in the
// order the index embeds them (sorted by path).
func assertEmbedded(t *testing.T, what string, embedder *conceptEmbedder, want ...string) {
	t.Helper()
	got := embedder.takeEmbedded()
	if len(got) != len(want) {
		t.Errorf("%s embedded %q, want %q", what, got, want)
		return
	}
	for index := range want {
		if strings.TrimSpace(got[index]) != want[index] {
			t.Errorf("%s embedded %q, want %q", what, got, want)
			return
		}
	}
}

// editLater rewrites path with a modification time an hour on, so the edit
// is seen whatever the filesystem's timestamp resolution.
func editLater(t *testing.T, path, body string) {
	t.Helper()
	WriteFile(t, path, body)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// Vectors from two models are not comparable: a model change re-embeds all.
func TestChangingTheModelReembedsEverything(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "a.md"), "settings\n")
	dbPath := filepath.Join(t.TempDir(), "vectors.db")
	first, err := NewVectorIndex(dbPath, &conceptEmbedder{model: "old"})
	if err != nil {
		t.Fatalf("NewVectorIndex: %v", err)
	}
	refreshIndex(t, first, docs)
	_ = first.Close()

	embedder := &conceptEmbedder{model: "new"}
	second, err := NewVectorIndex(dbPath, embedder)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	refreshIndex(t, second, docs)
	if embedded := embedder.takeEmbedded(); len(embedded) != 1 {
		t.Errorf("a new model embedded %v, want the file again", embedded)
	}
}

// A first index that fails partway keeps what it finished, and the next
// refresh embeds only the rest.
func TestAnInterruptedIndexResumesWhereItStopped(t *testing.T) {
	docs := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		WriteFile(t, filepath.Join(docs, name), name+" settings\n")
	}
	embedder := &conceptEmbedder{model: "m", failAfter: 2}
	index := newTestVectorIndex(t, embedder)
	if err := index.EnsureIndex(context.Background(), []string{docs}); err == nil {
		t.Fatal("a failing embedder refreshed without error")
	}
	if got := vectorHits(t, index, docs, "settings"); len(got) != 2 {
		t.Errorf("after the failure found %v, want the two files finished before it", got)
	}

	embedder.failAfter = 0
	embedder.takeEmbedded()
	refreshIndex(t, index, docs)
	if embedded := embedder.takeEmbedded(); len(embedded) != 1 || !strings.HasPrefix(embedded[0], "c.md") {
		t.Errorf("the resumed refresh embedded %v, want only c.md", embedded)
	}
}

// With nothing indexed there is nothing to compare, so the query is not
// sent to the provider at all.
func TestAnEmptyIndexNeverEmbedsTheQuery(t *testing.T) {
	embedder := &conceptEmbedder{model: "m"}
	index := newTestVectorIndex(t, embedder)
	if refs, err := index.SearchWithin(context.Background(), t.TempDir(), "settings"); err != nil || len(refs) != 0 || embedder.queries != 0 {
		t.Errorf("refs %v err %v queries %d, want nothing asked", refs, err, embedder.queries)
	}
	if refs, err := index.SearchWithin(context.Background(), t.TempDir(), "   "); err != nil || refs != nil {
		t.Errorf("a blank query returned %v %v", refs, err)
	}
}

func TestAnUnreadableDocIsEvictedFromTheVectorIndex(t *testing.T) {
	docs := t.TempDir()
	locked := filepath.Join(docs, "locked.md")
	WriteFile(t, locked, "settings\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	refreshIndex(t, index, docs)

	WriteFile(t, locked, "settings changed\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	refreshIndex(t, index, docs)
	if got := vectorHits(t, index, docs, "settings"); len(got) != 0 {
		t.Errorf("an unreadable file is still found: %v", got)
	}
}

// A root that is not a directory is not a corpus, as in the BM25 index.
func TestAVectorRootThatIsAFileIndexesNothing(t *testing.T) {
	source := filepath.Join(t.TempDir(), "notes.md")
	WriteFile(t, source, "settings\n")
	embedder := &conceptEmbedder{model: "m"}
	index := newTestVectorIndex(t, embedder)
	refreshIndex(t, index, source)
	if embedded := embedder.takeEmbedded(); len(embedded) != 0 {
		t.Errorf("a file root embedded %v", embedded)
	}
}

func TestAVectorIndexOnABrokenStoreFails(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "a.md"), "settings\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	_ = index.Close()
	if err := index.EnsureIndex(context.Background(), []string{docs}); err == nil {
		t.Error("refresh on a closed store succeeded")
	}
	if _, err := index.SearchWithin(context.Background(), docs, "settings"); err == nil {
		t.Error("search on a closed store succeeded")
	}
}

func TestNewVectorIndexRejectsAnUnusablePath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	WriteFile(t, blocker, "x")
	if _, err := NewVectorIndex(filepath.Join(blocker, "sub", "vectors.db"), &conceptEmbedder{}); err == nil {
		t.Error("a store under a file was created")
	}
}

func TestVectorsSurviveEncodingAndNormalise(t *testing.T) {
	vector := []float32{3, -4, 0.5}
	decoded := decodeVector(encodeVector(vector))
	for index := range vector {
		if decoded[index] != vector[index] {
			t.Fatalf("round trip = %v, want %v", decoded, vector)
		}
	}
	unit := normalized([]float32{3, 4})
	if unit[0] != 0.6 || unit[1] != 0.8 {
		t.Errorf("normalized = %v, want [0.6 0.8]", unit)
	}
	if zero := normalized([]float32{0, 0}); zero[0] != 0 || zero[1] != 0 {
		t.Errorf("a zero vector normalised to %v", zero)
	}
}

// A chunk embedded by a model of another width is skipped, not compared.
func TestAChunkOfAnotherWidthIsSkipped(t *testing.T) {
	chunks := []storedChunk{
		{path: "/r/wide.md", vector: []float32{1, 0, 0}},
		{path: "/r/fit.md", vector: []float32{1, 0}},
	}
	if refs := bestPerDocument([]float32{1, 0}, chunks); len(refs) != 1 || refs[0].Path != "/r/fit.md" {
		t.Errorf("refs = %+v, want only the chunk of matching width", refs)
	}
}

func TestTiesRankByPathAndResultsAreBounded(t *testing.T) {
	var chunks []storedChunk
	for _, name := range []string{"k", "c", "a", "d", "e", "f", "g", "h", "i", "j", "b", "l"} {
		chunks = append(chunks, storedChunk{path: "/r/" + name + ".md", vector: []float32{1}})
	}
	refs := bestPerDocument([]float32{1}, chunks)
	if len(refs) != maxCorpusResults || refs[0].Path != "/r/a.md" || refs[1].Path != "/r/b.md" {
		t.Errorf("refs = %d starting %v, want %d starting a, b", len(refs), refs[:2], maxCorpusResults)
	}
}

// Relevance is cosine similarity: the angle, not the length. A document that
// is only about the query scores 1 however the query is phrased, and one that
// mentions it among other things scores less — whatever either one's length.
func TestRelevanceIsCosineSimilarity(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "z-focused.md"), "settings\n")
	WriteFile(t, filepath.Join(docs, "a-diluted.md"), "refund refund refund refund settings\n")
	index := newTestVectorIndex(t, &conceptEmbedder{model: "m"})
	refreshIndex(t, index, docs)
	refs, err := index.SearchWithin(context.Background(), docs, "settings settings")
	if err != nil || len(refs) != 2 || filepath.Base(refs[0].Path) != "z-focused.md" {
		t.Fatalf("refs = %+v (err %v), want the focused document first", refs, err)
	}
	if math.Abs(refs[0].Relevance-1) > 1e-6 || math.Abs(refs[1].Relevance-1/math.Sqrt(17)) > 1e-6 {
		t.Errorf("relevance %v and %v, want 1 and 1/sqrt(17)", refs[0].Relevance, refs[1].Relevance)
	}
}
