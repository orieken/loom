package tools

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/domain"
)

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// Every tool that walks passes its context to the analyzer (roadmap L2.2):
// before, all six signed Execute(_ context.Context, ...) and dropped it.
func TestEveryWalkingToolStopsOnACancelledContext(t *testing.T) {
	root := rootedProject(t)
	for name, tool := range rootedTools(t, root) {
		t.Run(name, func(t *testing.T) {
			result, err := tool.Execute(cancelledContext(), BuildRequest(validArguments[name]))
			if err != nil || !result.IsError || result.Error == nil || result.Error.Kind != domain.ErrorCancelled || !strings.Contains(ExtractText(t, result), context.Canceled.Error()) {
				t.Errorf("%s ran on a cancelled context: %v %s", name, err, ExtractText(t, result))
			}
		})
	}
}

func TestTheKICorpusRetrieverStopsOnACancelledContext(t *testing.T) {
	corpus := t.TempDir()
	WriteFile(t, filepath.Join(corpus, "ki.md"), "---\nname: launch\n---\n# Launch\n")
	retriever := NewKICorpusRetriever([]string{corpus})
	if _, err := retriever.Retrieve(cancelledContext(), "launch", nil, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if _, err := retriever.Retrieve(context.Background(), "launch", nil, ""); err != nil {
		t.Errorf("an uncancelled retrieval failed: %v", err)
	}
}

func TestTheDocsIndexStopsOnACancelledContext(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "guide.md"), "# Guide\n")
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	if err := retriever.EnsureIndex(cancelledContext(), []string{docs}); !errors.Is(err, context.Canceled) {
		t.Errorf("EnsureIndex err = %v, want context.Canceled", err)
	}
	if _, err := retriever.Retrieve(cancelledContext(), "guide", nil, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("Retrieve err = %v, want context.Canceled", err)
	}
}

type countdownContext struct {
	context.Context
	remaining int
}

func (c *countdownContext) Err() error {
	if c.remaining <= 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}

func TestTheDocsIndexStopsWhenCancelledBetweenWalkAndIndexing(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "guide.md"), "# Guide\n")
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	walkThenCancel := &countdownContext{Context: context.Background(), remaining: 2}
	if err := retriever.EnsureIndex(walkThenCancel, []string{docs}); !errors.Is(err, context.Canceled) {
		t.Errorf("EnsureIndex err = %v, want context.Canceled before indexing", err)
	}
}

// A context that ends while the rows are read yields its error, not the
// rows read so far.
func TestADocsSearchCancelledPartwayYieldsNoPartialList(t *testing.T) {
	retriever := indexedDocsWithAnOutsideLink(t)
	queryThenCancel := &countdownContext{Context: context.Background(), remaining: 1}
	if refs, err := retriever.Retrieve(queryThenCancel, "launch", nil, ""); !errors.Is(err, context.Canceled) || refs != nil {
		t.Errorf("refs=%v err=%v, want no list and context.Canceled", refs, err)
	}
}

func TestAMissingCorpusRootContributesNothing(t *testing.T) {
	retriever := NewKICorpusRetriever([]string{filepath.Join(t.TempDir(), "missing")})
	refs, err := retriever.Retrieve(context.Background(), "launch", nil, "")
	if err != nil || len(refs) != 0 {
		t.Errorf("refs=%v err=%v, want nothing and no error", refs, err)
	}
}

func TestSearchToolsPassTheirContextToRealRetrievers(t *testing.T) {
	root := rootedProject(t)
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	docs := NewSearchDocsTool(SilentLogger(), retriever, retriever, root)
	found, err := docs.Execute(context.Background(), BuildRequest(map[string]any{"query": "guide", "docsPath": "docs"}))
	if err != nil || found.IsError || !strings.Contains(ExtractText(t, found), "guide.md") {
		t.Errorf("search_docs did not find the guide: %v %s", err, ExtractText(t, found))
	}
	cancelled, _ := docs.Execute(cancelledContext(), BuildRequest(map[string]any{"query": "guide", "docsPath": "docs"}))
	if !cancelled.IsError {
		t.Errorf("search_docs ran on a cancelled context: %s", ExtractText(t, cancelled))
	}

	corpus := t.TempDir()
	WriteFile(t, filepath.Join(corpus, "ki.md"), "---\nname: launch\n---\n# Launch\n")
	ki := NewSearchKITool(SilentLogger(), NewKICorpusRetriever([]string{corpus}))
	if result, _ := ki.Execute(cancelledContext(), BuildRequest(map[string]any{"query": "launch"})); !result.IsError {
		t.Errorf("search_ki ran on a cancelled context: %s", ExtractText(t, result))
	}
}
