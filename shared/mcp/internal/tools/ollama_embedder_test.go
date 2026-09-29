package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeOllama answers /api/embed with one vector per input, recording each
// request; status, when set, answers every request with that code instead.
type fakeOllama struct {
	mutex    sync.Mutex
	requests []ollamaEmbedRequest
	status   int
	short    bool // answer one vector fewer than asked
}

func (f *fakeOllama) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var decoded ollamaEmbedRequest
	_ = json.NewDecoder(request.Body).Decode(&decoded)
	f.mutex.Lock()
	f.requests = append(f.requests, decoded)
	status, short := f.status, f.short
	f.mutex.Unlock()
	if request.URL.Path != "/api/embed" {
		writer.WriteHeader(http.StatusTeapot)
		return
	}
	if status != 0 {
		writer.WriteHeader(status)
		return
	}
	count := len(decoded.Input)
	if short {
		count--
	}
	vectors := make([][]float32, count)
	for index := range vectors {
		vectors[index] = []float32{float32(len(decoded.Input[index])), 1}
	}
	_ = json.NewEncoder(writer).Encode(ollamaEmbedResponse{Embeddings: vectors})
}

func ollamaAgainst(t *testing.T, fake *fakeOllama, model string) *OllamaEmbedder {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return NewOllamaEmbedder(OllamaConfig{Endpoint: server.URL + "/", Model: model, Timeout: 5 * time.Second})
}

// nomic-embed-text is trained with task prefixes; without them it embeds,
// but ranks worse.
func TestANomicModelGetsItsTaskPrefixes(t *testing.T) {
	fake := &fakeOllama{}
	embedder := ollamaAgainst(t, fake, "nomic-embed-text")
	if _, err := embedder.EmbedDocuments(context.Background(), []string{"a doc"}); err != nil {
		t.Fatalf("EmbedDocuments: %v", err)
	}
	if _, err := embedder.EmbedQuery(context.Background(), "a query"); err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if got := fake.requests[0].Input[0]; got != "search_document: a doc" {
		t.Errorf("document sent as %q", got)
	}
	if got := fake.requests[1].Input[0]; got != "search_query: a query" || fake.requests[1].Model != "nomic-embed-text" {
		t.Errorf("query sent as %+v", fake.requests[1])
	}
}

func TestOtherModelsAndExplicitPrefixesAreLeftAlone(t *testing.T) {
	plain := &fakeOllama{}
	if _, err := ollamaAgainst(t, plain, "mxbai-embed-large").EmbedQuery(context.Background(), "q"); err != nil || plain.requests[0].Input[0] != "q" {
		t.Errorf("a non-nomic model was prefixed: %+v (err %v)", plain.requests, err)
	}
	custom := &fakeOllama{}
	server := httptest.NewServer(custom)
	defer server.Close()
	embedder := NewOllamaEmbedder(OllamaConfig{Endpoint: server.URL, Model: "nomic-embed-text", QueryPrefix: "Q: ", Timeout: time.Second})
	if _, err := embedder.EmbedQuery(context.Background(), "q"); err != nil || custom.requests[0].Input[0] != "Q: q" {
		t.Errorf("an explicit prefix was overridden: %+v (err %v)", custom.requests, err)
	}
	if embedder.Model() != "nomic-embed-text" {
		t.Errorf("Model() = %q", embedder.Model())
	}
}

// A large first index is many short requests, and the vectors come back in
// the order of the texts.
func TestDocumentsAreEmbeddedInBoundedBatchesInOrder(t *testing.T) {
	fake := &fakeOllama{}
	texts := make([]string, ollamaBatchSize+3)
	for index := range texts {
		texts[index] = strings.Repeat("x", index)
	}
	vectors, err := ollamaAgainst(t, fake, "m").EmbedDocuments(context.Background(), texts)
	if err != nil || len(vectors) != len(texts) {
		t.Fatalf("%d vectors (err %v), want %d", len(vectors), err, len(texts))
	}
	assertBatchSizes(t, fake.requests, ollamaBatchSize, 3)
	if vectors[ollamaBatchSize+2][0] != float32(ollamaBatchSize+2) {
		t.Errorf("vectors out of order: last is %v", vectors[len(vectors)-1])
	}
}

func assertBatchSizes(t *testing.T, requests []ollamaEmbedRequest, want ...int) {
	t.Helper()
	got := make([]int, len(requests))
	for index, request := range requests {
		got[index] = len(request.Input)
	}
	if len(got) != len(want) || got[0] != want[0] || got[len(got)-1] != want[len(want)-1] {
		t.Errorf("batches of %v, want %v", got, want)
	}
}

// Each failure is classified by what retrying could do about it (L2.5).
func TestProviderFailuresAreClassified(t *testing.T) {
	cases := []struct {
		name string
		fake *fakeOllama
		want error
	}{
		{"model missing", &fakeOllama{status: http.StatusNotFound}, ErrEmbeddingModelMissing},
		{"provider failing", &fakeOllama{status: http.StatusServiceUnavailable}, ErrEmbedderUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ollamaAgainst(t, tc.fake, "m").EmbedQuery(context.Background(), "q"); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	rejected := &fakeOllama{status: http.StatusBadRequest}
	_, err := ollamaAgainst(t, rejected, "m").EmbedDocuments(context.Background(), []string{"d"})
	if err == nil || errors.Is(err, ErrEmbedderUnavailable) || errors.Is(err, ErrEmbeddingModelMissing) {
		t.Errorf("a rejected request = %v, want a plain failure", err)
	}
}

func TestAnUnreachableProviderIsTransientAndACancelledCallIsNot(t *testing.T) {
	server := httptest.NewServer(&fakeOllama{})
	endpoint := server.URL
	server.Close()
	embedder := NewOllamaEmbedder(OllamaConfig{Endpoint: endpoint, Model: "m", Timeout: time.Second})
	if _, err := embedder.EmbedQuery(context.Background(), "q"); !errors.Is(err, ErrEmbedderUnavailable) {
		t.Errorf("unreachable err = %v, want ErrEmbedderUnavailable", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := embedder.EmbedQuery(ctx, "q"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled err = %v, want context.Canceled", err)
	}
}

func TestAnAnswerMissingAVectorIsAnError(t *testing.T) {
	if _, err := ollamaAgainst(t, &fakeOllama{short: true}, "m").EmbedDocuments(context.Background(), []string{"a", "b"}); err == nil {
		t.Error("one vector for two texts was accepted")
	}
}

func TestAnUndecodableAnswerIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("not json"))
	}))
	defer server.Close()
	embedder := NewOllamaEmbedder(OllamaConfig{Endpoint: server.URL, Model: "m", Timeout: time.Second})
	if _, err := embedder.EmbedQuery(context.Background(), "q"); err == nil {
		t.Error("an undecodable answer was accepted")
	}
}

func TestAMalformedEndpointIsAnError(t *testing.T) {
	embedder := NewOllamaEmbedder(OllamaConfig{Endpoint: "http://bad host", Model: "m", Timeout: time.Second})
	if _, err := embedder.EmbedQuery(context.Background(), "q"); err == nil {
		t.Error("a malformed endpoint was accepted")
	}
}

// A provider that never answers is cut off by the configured timeout
// (guardrail #5) and reads as unavailable, not as a hang.
func TestAProviderThatNeverAnswersTimesOut(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	embedder := NewOllamaEmbedder(OllamaConfig{Endpoint: server.URL, Model: "m", Timeout: 50 * time.Millisecond})
	// Bounded, so a missing client timeout fails this test — the call then
	// ends with the context's error, not unavailable — instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	_, err := embedder.EmbedQuery(ctx, "q")
	if !errors.Is(err, ErrEmbedderUnavailable) || time.Since(started) > 2*time.Second {
		t.Errorf("err %v after %v, want unavailable within the timeout", err, time.Since(started))
	}
}

// Exactly one batch of texts is one request — never a trailing empty one,
// which a real provider may reject.
func TestAFullBatchIsOneRequest(t *testing.T) {
	fake := &fakeOllama{}
	texts := make([]string, ollamaBatchSize)
	if _, err := ollamaAgainst(t, fake, "m").EmbedDocuments(context.Background(), texts); err != nil {
		t.Fatalf("EmbedDocuments: %v", err)
	}
	if len(fake.requests) != 1 {
		t.Errorf("%d requests for exactly one batch, want 1", len(fake.requests))
	}
}
