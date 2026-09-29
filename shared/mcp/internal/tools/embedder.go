package tools

import "context"

// Embedder turns text into vectors (roadmap L3.4). Documents and queries are
// separate calls because embedding models are commonly trained asymmetric —
// nomic-embed-text expects a task prefix on each — and the adapter, not the
// index, knows its model's convention.
type Embedder interface {
	// EmbedDocuments returns one vector per text, in order.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery returns the vector for one search query.
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	// Model names the model, so an index knows to re-embed when it changes:
	// vectors from two models are not comparable.
	Model() string
}
