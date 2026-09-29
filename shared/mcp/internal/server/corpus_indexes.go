package server

// Corpus index construction for the search tools (roadmaps L2.7, L3.4). The
// indexes live under .claude/rag/ in the directory the server started in;
// every search is traced as a retrieval.queried span by tracedIndex.

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// Environment configuring the embedding provider. Embeddings are off unless
// LOOM_EMBEDDINGS names a provider: an embedding call leaves the process, and
// that is the user's decision, not a default.
const (
	embeddingsEnvVar     = "LOOM_EMBEDDINGS"
	embeddingModelEnvVar = "LOOM_EMBEDDING_MODEL"
	ollamaHostEnvVar     = "OLLAMA_HOST"
	defaultOllamaHost    = "http://localhost:11434"
	defaultEmbedModel    = "nomic-embed-text"
	embeddingTimeout     = 30 * time.Second
)

func newSearchDocsTool(logger *logging.Logger, root tools.WorkspaceRoot) domain.Tool {
	dbPath, ok := docsFTSDBPath()
	if !ok {
		logger.Info("BM25 retriever not initialised — search_docs will return no-corpus diagnostic responses")
		return tools.NewSearchDocsTool(logger, nil, root)
	}
	retriever, err := tools.NewBM25Retriever(dbPath)
	if err != nil {
		logger.Warn("BM25 retriever init failed — search_docs will return no-corpus diagnostic responses", "dbPath", dbPath, "error", err)
		return tools.NewSearchDocsTool(logger, nil, root)
	}
	return tools.NewSearchDocsTool(logger, tracedIndex{inner: retriever, corpus: "project-docs", backend: "bm25"}, root)
}

// newSearchFeaturesTool builds the feature archive's index: BM25 always,
// fused with vectors when an embedding provider is configured.
func newSearchFeaturesTool(logger *logging.Logger, root tools.WorkspaceRoot) domain.Tool {
	ragDir, ok := ragDirectory()
	if !ok {
		logger.Info("No .claude/rag directory — search_features will return no-corpus diagnostic responses")
		return tools.NewSearchFeaturesTool(logger, nil, root)
	}
	index, err := featuresIndex(logger, ragDir)
	if err != nil {
		logger.Warn("Feature archive index init failed — search_features will return no-corpus diagnostic responses", "error", err)
		return tools.NewSearchFeaturesTool(logger, nil, root)
	}
	return tools.NewSearchFeaturesTool(logger, index, root)
}

func featuresIndex(logger *logging.Logger, ragDir string) (tools.CorpusIndex, error) {
	lexical, err := tools.NewBM25Retriever(filepath.Join(ragDir, "features-fts5.sqlite"))
	if err != nil {
		return nil, err
	}
	embedder := configuredEmbedder()
	if embedder == nil {
		logger.Info("No embedding provider configured — search_features ranks by BM25 alone", "enable", embeddingsEnvVar+"=ollama")
		return tracedIndex{inner: lexical, corpus: "project-features", backend: "bm25"}, nil
	}
	semantic, err := tools.NewVectorIndex(filepath.Join(ragDir, "features-vectors.sqlite"), embedder)
	if err != nil {
		return nil, err
	}
	return tracedIndex{inner: tools.NewHybridIndex(lexical, semantic), corpus: "project-features", backend: "hybrid"}, nil
}

// configuredEmbedder is the provider LOOM_EMBEDDINGS names, or nil.
func configuredEmbedder() tools.Embedder {
	if os.Getenv(embeddingsEnvVar) != "ollama" {
		return nil
	}
	return tools.NewOllamaEmbedder(tools.OllamaConfig{
		Endpoint: envOr(ollamaHostEnvVar, defaultOllamaHost),
		Model:    envOr(embeddingModelEnvVar, defaultEmbedModel),
		Timeout:  embeddingTimeout,
	})
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func docsFTSDBPath() (string, bool) {
	if override := os.Getenv("DOCS_FTS_PATH"); override != "" {
		return override, true
	}
	ragDir, ok := ragDirectory()
	return filepath.Join(ragDir, "docs-fts5.sqlite"), ok
}

// ragDirectory is .claude/rag under the working directory, when .claude
// exists there.
func ragDirectory() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(cwd, ".claude")); err != nil {
		return "", false
	}
	return filepath.Join(cwd, ".claude", "rag"), true
}

// tracedIndex records every search as a retrieval.queried span (L3.4). It
// sits here, in the adapter layer, so the indexes stay free of telemetry
// (guardrail #8) — the retriever contract's "callers may emit telemetry
// around Retrieve calls".
type tracedIndex struct {
	inner           tools.CorpusIndex
	corpus, backend string
}

func (t tracedIndex) EnsureIndex(ctx context.Context, roots []string) error {
	return t.inner.EnsureIndex(ctx, roots)
}

func (t tracedIndex) SearchWithin(ctx context.Context, root, query string) ([]tools.Reference, error) {
	ctx, span := telemetry.StartRetrieval(ctx, telemetry.RetrievalQuery{Corpus: t.corpus, Backend: t.backend, Query: query})
	refs, err := t.inner.SearchWithin(ctx, root, query)
	span.End(telemetry.RetrievalResult{Hits: len(refs), TopHit: topHit(root, refs), Err: err})
	return refs, err
}

// topHit is the best reference relative to the corpus root.
func topHit(root string, refs []tools.Reference) string {
	if len(refs) == 0 {
		return ""
	}
	// Rel yields "" for a path it cannot relate to root — never the
	// absolute path.
	relative, _ := filepath.Rel(root, refs[0].Path)
	return filepath.ToSlash(relative)
}
