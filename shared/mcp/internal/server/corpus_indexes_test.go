package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// Embeddings leave the process, so they are off unless asked for by name.
func TestEmbeddingsAreOffUnlessAProviderIsNamed(t *testing.T) {
	for _, value := range []string{"", "openai", "OLLAMA"} {
		t.Setenv(embeddingsEnvVar, value)
		if embedder := configuredEmbedder(); embedder != nil {
			t.Errorf("%s=%q configured %v", embeddingsEnvVar, value, embedder)
		}
	}
}

func TestOllamaIsConfiguredWithADefaultModelThatCanBeOverridden(t *testing.T) {
	t.Setenv(embeddingsEnvVar, "ollama")
	t.Setenv(embeddingModelEnvVar, "")
	if embedder := configuredEmbedder(); embedder == nil || embedder.Model() != defaultEmbedModel {
		t.Errorf("default embedder = %v", embedder)
	}
	t.Setenv(embeddingModelEnvVar, "mxbai-embed-large")
	if embedder := configuredEmbedder(); embedder == nil || embedder.Model() != "mxbai-embed-large" {
		t.Errorf("overridden embedder = %v", embedder)
	}
}

// The feature archive ranks by BM25 alone without an embedder, and fuses
// BM25 with vectors with one — and each search says which it used.
func TestTheFeatureIndexIsHybridOnlyWithAnEmbedder(t *testing.T) {
	logger := logging.NewLogger(&bytes.Buffer{})
	t.Setenv(embeddingsEnvVar, "")
	lexical := tracedFeatureIndex(t, logger, "bm25")
	if _, isBM25 := lexical.inner.(*tools.BM25Retriever); !isBM25 {
		t.Errorf("without an embedder the index is %T, want BM25 alone", lexical.inner)
	}
	t.Setenv(embeddingsEnvVar, "ollama")
	hybrid := tracedFeatureIndex(t, logger, "hybrid")
	if _, isHybrid := hybrid.inner.(*tools.HybridIndex); !isHybrid {
		t.Errorf("with an embedder the index is %T, want a HybridIndex", hybrid.inner)
	}
}

// tracedFeatureIndex builds the feature index and checks it is traced as the
// feature archive with the given backend.
func tracedFeatureIndex(t *testing.T, logger *logging.Logger, backend string) tracedIndex {
	t.Helper()
	index, err := featuresIndex(logger, t.TempDir())
	traced, ok := index.(tracedIndex)
	if err != nil || !ok || traced.backend != backend || traced.corpus != "project-features" {
		t.Fatalf("feature index = %#v (err %v), want traced project-features/%s", index, err, backend)
	}
	return traced
}

func TestAnUnusableRagDirectoryFailsTheFeatureIndex(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	logger := logging.NewLogger(&bytes.Buffer{})
	t.Setenv(embeddingsEnvVar, "")
	if _, err := featuresIndex(logger, blocker); err == nil {
		t.Error("a rag directory that is a file was accepted")
	}
	t.Setenv(embeddingsEnvVar, "ollama")
	if _, err := featuresIndex(logger, blocker); err == nil {
		t.Error("a rag directory that is a file was accepted for vectors")
	}
}

// search_features is configured only where the project has a .claude
// directory; elsewhere it answers not_found rather than indexing somewhere
// unexpected.
func TestSearchFeaturesIsConfiguredOnlyUnderAClaudeDirectory(t *testing.T) {
	logger := logging.NewLogger(&bytes.Buffer{})
	t.Setenv(embeddingsEnvVar, "")
	bare := t.TempDir()
	t.Chdir(bare)
	root, err := tools.NewWorkspaceRoot(bare)
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if kind := searchFeaturesKind(t, newSearchFeaturesTool(logger, root)); kind != domain.ErrorNotFound {
		t.Errorf("without .claude: kind %q, want not_found", kind)
	}

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(project)
	projectRoot, err := tools.NewWorkspaceRoot(project)
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if kind := searchFeaturesKind(t, newSearchFeaturesTool(logger, projectRoot)); kind != "" {
		t.Errorf("with .claude: kind %q, want a search", kind)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude", "rag", "features-fts5.sqlite")); err != nil {
		t.Errorf("the feature index was not created under .claude/rag: %v", err)
	}
}

// searchFeaturesKind runs a search of the project root and returns its
// failure kind, or "" for a result.
func searchFeaturesKind(t *testing.T, tool domain.Tool) domain.ErrorKind {
	t.Helper()
	result, err := tool.Execute(context.Background(), domain.ToolRequest{Args: map[string]any{"query": "q", "featuresPath": "."}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		return ""
	}
	return result.Error.Kind
}

// fixedIndex answers with fixed references.
type fixedIndex struct {
	found     []tools.Reference
	err       error
	refreshed []string
}

func (f *fixedIndex) EnsureIndex(_ context.Context, roots []string) error {
	f.refreshed = append(f.refreshed, roots...)
	return f.err
}

func (f *fixedIndex) SearchWithin(context.Context, string, string) ([]tools.Reference, error) {
	return f.found, f.err
}

func TestTheTracingDecoratorPassesEverythingThrough(t *testing.T) {
	inner := &fixedIndex{found: []tools.Reference{{Path: "/corpus/a/README.md"}}}
	traced := tracedIndex{inner: inner, corpus: "c", backend: "b"}
	if err := traced.EnsureIndex(context.Background(), []string{"/corpus"}); err != nil || len(inner.refreshed) != 1 {
		t.Errorf("refresh err %v, refreshed %v", err, inner.refreshed)
	}
	found, err := traced.SearchWithin(context.Background(), "/corpus", "q")
	if err != nil || len(found) != 1 || found[0].Path != "/corpus/a/README.md" {
		t.Errorf("found %v err %v", found, err)
	}
	broken := errors.New("down")
	if _, err := (tracedIndex{inner: &fixedIndex{err: broken}}).SearchWithin(context.Background(), "/corpus", "q"); !errors.Is(err, broken) {
		t.Errorf("err = %v, want the index's own", err)
	}
}

// The top hit is recorded relative to the corpus: an absolute path would
// carry the user's home directory into the trace.
func TestTheTopHitIsRelativeToTheCorpus(t *testing.T) {
	if got := topHit("/home/someone/project/docs/features", []tools.Reference{{Path: "/home/someone/project/docs/features/sync/README.md"}}); got != "sync/README.md" {
		t.Errorf("topHit = %q", got)
	}
	if got := topHit("/corpus", nil); got != "" {
		t.Errorf("no hits gave %q", got)
	}
	if got := topHit("relative/root", []tools.Reference{{Path: "/absolute/path.md"}}); got != "" {
		t.Errorf("an unrelatable path gave %q, want nothing rather than an absolute path", got)
	}
}
