package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrEmbeddingModelMissing is a configured model the provider does not have.
// Retrying will not fetch it; a person must.
var ErrEmbeddingModelMissing = errors.New("embedding model is not available on the provider")

// OllamaConfig configures an OllamaEmbedder.
type OllamaConfig struct {
	// Endpoint is the Ollama base URL, e.g. http://localhost:11434.
	Endpoint string
	// Model is the embedding model name, e.g. nomic-embed-text.
	Model string
	// DocumentPrefix and QueryPrefix are the model's task prefixes, if any.
	DocumentPrefix, QueryPrefix string
	// Timeout bounds each HTTP request (guardrail #5).
	Timeout time.Duration
}

// nomicPrefixes are nomic-embed-text's documented task prefixes. Without them
// the model still embeds, but ranks measurably worse.
var nomicPrefixes = [2]string{"search_document: ", "search_query: "}

// ollamaBatchSize bounds how many texts go in one request, so a large first
// index is many short requests rather than one that outlives its timeout.
const ollamaBatchSize = 32

// OllamaEmbedder embeds through a local or remote Ollama server's /api/embed.
type OllamaEmbedder struct {
	config OllamaConfig
	client *http.Client
}

// NewOllamaEmbedder builds an embedder. A nomic-embed model gets its task
// prefixes unless the config sets its own.
func NewOllamaEmbedder(config OllamaConfig) *OllamaEmbedder {
	if strings.HasPrefix(config.Model, "nomic-embed") && config.DocumentPrefix == "" && config.QueryPrefix == "" {
		config.DocumentPrefix, config.QueryPrefix = nomicPrefixes[0], nomicPrefixes[1]
	}
	return &OllamaEmbedder{config: config, client: &http.Client{Timeout: config.Timeout}}
}

// Model names the embedding model.
func (e *OllamaEmbedder) Model() string { return e.config.Model }

// EmbedDocuments embeds texts in batches.
func (e *OllamaEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += ollamaBatchSize {
		batch := texts[start:min(start+ollamaBatchSize, len(texts))]
		embedded, err := e.embed(ctx, withPrefix(e.config.DocumentPrefix, batch))
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, embedded...)
	}
	return vectors, nil
}

// EmbedQuery embeds one query.
func (e *OllamaEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	embedded, err := e.embed(ctx, withPrefix(e.config.QueryPrefix, []string{text}))
	if err != nil {
		return nil, err
	}
	return embedded[0], nil
}

func withPrefix(prefix string, texts []string) []string {
	prefixed := make([]string, len(texts))
	for index, text := range texts {
		prefixed[index] = prefix + text
	}
	return prefixed
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// embed makes one request, checking the provider answered for every text.
func (e *OllamaEmbedder) embed(ctx context.Context, texts []string) ([][]float32, error) {
	response, err := e.post(ctx, texts)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := statusError(response.StatusCode, e.config.Model); err != nil {
		return nil, err
	}
	var decoded ollamaEmbedResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("ollama: decode embeddings: %w", err)
	}
	if len(decoded.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama: %d embeddings for %d texts", len(decoded.Embeddings), len(texts))
	}
	return decoded.Embeddings, nil
}

func (e *OllamaEmbedder) post(ctx context.Context, texts []string) (*http.Response, error) {
	body, err := json.Marshal(ollamaEmbedRequest{Model: e.config.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(e.config.Endpoint, "/")+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return nil, unreachable(ctx, err)
	}
	return response, nil
}

// unreachable classifies a request that got no answer: the caller's own
// cancellation stays itself; anything else — refused, timed out — is the
// provider being unavailable, which is transient.
func unreachable(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("%w: %v", ErrEmbedderUnavailable, err)
}

// statusError maps a non-OK answer: 404 is the model missing, 5xx the
// provider failing (transient), and anything else a request it rejected.
func statusError(status int, model string) error {
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %q (ollama pull %s)", ErrEmbeddingModelMissing, model, model)
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: ollama answered %d", ErrEmbedderUnavailable, status)
	}
	return fmt.Errorf("ollama rejected the embedding request: %d", status)
}
