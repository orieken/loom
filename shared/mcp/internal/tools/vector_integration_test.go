package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ollamaTestEnvVar opts into the tests that need a real embedding model. CI
// has none; run locally with `ollama serve` and `ollama pull nomic-embed-text`.
const ollamaTestEnvVar = "LOOM_OLLAMA_TEST"

// featureArchive is a small archive where one delivery answers the query in
// other words and four do not: the vector adapter's own motivating example
// (shared/rag/adapters/vector.md).
var featureArchive = map[string]string{
	"settings-persistence": "# Settings persistence across devices\n\nA signed-in account's choices — theme, language, notification toggles — are saved server-side and restored on every phone and laptop it signs in from.\n",
	"partial-refunds":      "# Partial refunds for card payments\n\nSupport staff can return part of a charge to the original card, and the ledger records the adjustment.\n",
	"login-rate-limiting":  "# Login rate limiting\n\nRepeated failed sign-in attempts from one address are throttled with an exponential lockout.\n",
	"invoice-export":       "# CSV export of invoices\n\nAccountants download a month of invoices as a spreadsheet file.\n",
	"image-thumbnails":     "# Image thumbnails\n\nUploaded pictures are resized into three thumbnail sizes by a background worker.\n",
}

// paraphraseQuery shares no word with the delivery it describes.
const paraphraseQuery = "user preference sync"

func writeFeatureArchive(t *testing.T) string {
	t.Helper()
	features := filepath.Join(t.TempDir(), "features")
	for name, body := range featureArchive {
		WriteFile(t, filepath.Join(features, name, "README.md"), body)
	}
	return features
}

func realEmbedder(t *testing.T) Embedder {
	t.Helper()
	if os.Getenv(ollamaTestEnvVar) == "" {
		t.Skipf("set %s=1 with a local Ollama serving nomic-embed-text to run", ollamaTestEnvVar)
	}
	return NewOllamaEmbedder(OllamaConfig{Endpoint: "http://localhost:11434", Model: "nomic-embed-text", Timeout: time.Minute})
}

// The L3.4 done-when: a conceptual paraphrase query that BM25 misses is
// answered by the vector adapter — and by search_features, which fuses both.
func TestAParaphraseBM25MissesIsAnsweredByTheVectorAdapter(t *testing.T) {
	embedder := realEmbedder(t)
	features := writeFeatureArchive(t)

	lexical, err := NewBM25Retriever(filepath.Join(t.TempDir(), "fts.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	refreshIndex(t, lexical, features)
	if found := searchPaths(t, lexical, features); len(found) != 0 {
		t.Fatalf("BM25 found %v for %q — the query is not a paraphrase BM25 misses", found, paraphraseQuery)
	}

	semantic, err := NewVectorIndex(filepath.Join(t.TempDir(), "vectors.db"), embedder)
	if err != nil {
		t.Fatalf("NewVectorIndex: %v", err)
	}
	refreshIndex(t, semantic, features)
	assertSettingsFirst(t, "vector adapter", searchPaths(t, semantic, features))
	assertSettingsFirst(t, "hybrid", searchPaths(t, NewHybridIndex(lexical, semantic), features))
}

func assertSettingsFirst(t *testing.T, what string, found []string) {
	t.Helper()
	if len(found) == 0 || !strings.Contains(found[0], "settings-persistence") {
		t.Errorf("%s ranked %v, want settings-persistence first", what, found)
	}
}

func refreshIndex(t *testing.T, index CorpusIndex, root string) {
	t.Helper()
	if err := index.EnsureIndex(context.Background(), []string{root}); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
}

func searchPaths(t *testing.T, index CorpusIndex, root string) []string {
	t.Helper()
	refs, err := index.SearchWithin(context.Background(), root, paraphraseQuery)
	if err != nil {
		t.Fatalf("SearchWithin: %v", err)
	}
	paths := make([]string, 0, len(refs))
	for _, ref := range refs {
		paths = append(paths, ref.Path)
	}
	return paths
}
