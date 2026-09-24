package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The docs indexer walks through analyzers.CollectFiles (roadmap L2.3), so a
// symbolic link inside the docs never pulls a file from outside the workspace
// into the index.
func TestTheDocsIndexIgnoresLinkedFilesFromOutside(t *testing.T) {
	retriever := indexedDocsWithAnOutsideLink(t)

	refs, err := retriever.Retrieve(context.Background(), "launch", nil, "")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(refs) != 1 || refs[0].Title == "Secret" {
		t.Errorf("retrieved %+v for %q, want only the guide", refs, "launch")
	}
}

// indexedDocsWithAnOutsideLink indexes a docs directory holding one real
// guide and a link to a document outside it; both mention "launch".
func indexedDocsWithAnOutsideLink(t *testing.T) *BM25Retriever {
	t.Helper()
	outside, docs := t.TempDir(), t.TempDir()
	WriteFile(t, filepath.Join(outside, "secret.md"), "# Secret\n\nlaunch codes\n")
	WriteFile(t, filepath.Join(docs, "guide.md"), "# Guide\n\nlaunch checklist\n")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(docs, "linked.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	if err := retriever.EnsureIndex(context.Background(), []string{docs}); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	return retriever
}

// Every document is indexed, not just the first. A negated error check in
// the indexing loop stopped after one file and still passed a one-document
// test — the report-only mutation job's third run found it (L3.59).
func TestTheDocsIndexIndexesEveryDocument(t *testing.T) {
	docs := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		WriteFile(t, filepath.Join(docs, name), "# "+name+"\n\nshared keyword\n")
	}
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	if err := retriever.EnsureIndex(context.Background(), []string{docs}); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	refs, err := retriever.Retrieve(context.Background(), "keyword", nil, "")
	if err != nil || len(refs) != 3 {
		t.Errorf("retrieved %d documents (err %v), want all three indexed", len(refs), err)
	}
}
