package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

// countingFiles is the real filesystem, counting what a refresh reads. hook,
// when set, runs before each read; a name in vanished fails to stat, as a
// file deleted between the walk and the stat would.
type countingFiles struct {
	osDocFiles
	mutex    sync.Mutex
	read     []string
	hook     func()
	vanished map[string]bool
}

func (c *countingFiles) Stat(path string) (os.FileInfo, error) {
	c.mutex.Lock()
	gone := c.vanished[filepath.Base(path)]
	c.mutex.Unlock()
	if gone {
		return nil, os.ErrNotExist
	}
	return c.osDocFiles.Stat(path)
}

func (c *countingFiles) ReadFile(path string) ([]byte, error) {
	c.mutex.Lock()
	c.read = append(c.read, filepath.Base(path))
	hook := c.hook
	c.mutex.Unlock()
	if hook != nil {
		hook()
	}
	return c.osDocFiles.ReadFile(path)
}

// takeReads returns the files read since the last call, sorted.
func (c *countingFiles) takeReads() []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	read := c.read
	c.read = nil
	sort.Strings(read)
	return read
}

// countedRetriever is a retriever over a fresh database whose reads are counted.
func countedRetriever(t *testing.T) (*BM25Retriever, *countingFiles) {
	t.Helper()
	retriever, err := NewBM25Retriever(filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatalf("NewBM25Retriever: %v", err)
	}
	t.Cleanup(func() { _ = retriever.Close() })
	files := &countingFiles{}
	retriever.files = files
	return retriever, files
}

func refresh(t *testing.T, retriever *BM25Retriever, roots ...string) {
	t.Helper()
	if err := retriever.EnsureIndex(context.Background(), roots); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
}

// hits is the base names of the documents a query finds, sorted.
func hits(t *testing.T, retriever *BM25Retriever, query string) []string {
	t.Helper()
	refs, err := retriever.Retrieve(context.Background(), query, nil, "")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, filepath.Base(ref.Path))
	}
	sort.Strings(names)
	return names
}

func assertNames(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", what, got, want)
		return
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("%s = %v, want %v", what, got, want)
			return
		}
	}
}

// L2.7 done-when, first half: the second identical search reads no file.
func TestASecondIdenticalSearchReadsNoFile(t *testing.T) {
	root := rootedProject(t)
	retriever, files := countedRetriever(t)
	WriteFile(t, filepath.Join(root.Dir(), "docs", "setup.md"), "# Setup\n\nguide to setup\n")
	search := NewSearchDocsTool(SilentLogger(), retriever, retriever, root)
	request := BuildRequest(map[string]any{"query": "guide", "docsPath": "docs"})

	if result, err := search.Execute(context.Background(), request); err != nil || result.IsError {
		t.Fatalf("first search failed: %v %s", err, ExtractText(t, result))
	}
	assertNames(t, "first search read", files.takeReads(), "guide.md", "setup.md")

	if result, err := search.Execute(context.Background(), request); err != nil || result.IsError {
		t.Fatalf("second search failed: %v %s", err, ExtractText(t, result))
	}
	assertNames(t, "second search read", files.takeReads())
}

// L2.7 done-when, second half: a deleted doc disappears from results.
func TestADeletedDocDisappearsFromResults(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "kept.md"), "# Kept\n\nrelease notes\n")
	WriteFile(t, filepath.Join(docs, "gone.md"), "# Gone\n\nrelease plan\n")
	retriever, _ := countedRetriever(t)
	refresh(t, retriever, docs)
	assertNames(t, "before deletion", hits(t, retriever, "release"), "gone.md", "kept.md")

	if err := os.Remove(filepath.Join(docs, "gone.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	refresh(t, retriever, docs)
	assertNames(t, "after deletion", hits(t, retriever, "release"), "kept.md")
}

// Only what changed is read, and the change is searchable. The edit keeps
// the file's size, so the modification time alone must reveal it.
func TestOnlyAChangedDocIsReadAndItsNewTextIsSearchable(t *testing.T) {
	docs := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		WriteFile(t, filepath.Join(docs, name), "# "+name+"\n\nfirst draft\n")
	}
	retriever, files := countedRetriever(t)
	refresh(t, retriever, docs)
	files.takeReads()

	beta := filepath.Join(docs, "beta.md")
	WriteFile(t, beta, "# beta.md\n\nfinal draft\n") // same length as "first draft"
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(beta, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	refresh(t, retriever, docs)
	assertNames(t, "refresh read", files.takeReads(), "beta.md")
	assertNames(t, "new text", hits(t, retriever, "final"), "beta.md")
	assertNames(t, "old text", hits(t, retriever, "first"), "alpha.md", "gamma.md")
}

// An index built before fingerprints existed has rows and no fingerprints: a
// row whose file is gone is evicted, and one whose file remains is re-read
// once and then left alone.
func TestAnIndexFromBeforeFingerprintsIsReconciled(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "present.md"), "# Present\n\nlegacy row\n")
	retriever, files := countedRetriever(t)
	for _, name := range []string{"present.md", "vanished.md"} {
		if _, err := retriever.db.Exec("INSERT INTO docs_fts (path, title, body) VALUES (?, ?, ?)",
			filepath.Join(docs, name), name, "legacy row"); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}

	refresh(t, retriever, docs)
	assertNames(t, "legacy rows after refresh", hits(t, retriever, "legacy"), "present.md")
	assertNames(t, "first refresh read", files.takeReads(), "present.md")
	refresh(t, retriever, docs)
	assertNames(t, "second refresh read", files.takeReads())
}

// A refresh is one transaction: cancelled partway through its writes, it
// leaves the previous index whole rather than half-updated.
func TestACancelledRefreshLeavesThePreviousIndexWhole(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "one.md"), "# One\n\nversion old\n")
	WriteFile(t, filepath.Join(docs, "two.md"), "# Two\n\nversion old\n")
	retriever, files := countedRetriever(t)
	refresh(t, retriever, docs)

	for _, name := range []string{"one.md", "two.md"} {
		WriteFile(t, filepath.Join(docs, name), "# "+name+"\n\nversion newer\n")
	}
	ctx, cancel := context.WithCancel(context.Background())
	files.takeReads()
	files.hook = cancel // the first file read ends the refresh before the second
	if err := retriever.EnsureIndex(ctx, []string{docs}); !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureIndex err = %v, want context.Canceled", err)
	}
	files.hook = nil
	if read := files.takeReads(); len(read) != 1 {
		t.Errorf("a cancelled refresh read %v, want it to stop after the first", read)
	}
	assertNames(t, "old text after the cancelled refresh", hits(t, retriever, "old"), "one.md", "two.md")
	assertNames(t, "new text after the cancelled refresh", hits(t, retriever, "newer"))

	refresh(t, retriever, docs)
	assertNames(t, "new text after a full refresh", hits(t, retriever, "newer"), "one.md", "two.md")
}

// Refreshing one root touches only its own rows — including a sibling whose
// name merely starts with the same characters.
func TestRefreshingOneRootLeavesAnotherAlone(t *testing.T) {
	base := t.TempDir()
	docs, sibling := filepath.Join(base, "docs"), filepath.Join(base, "docs2")
	WriteFile(t, filepath.Join(docs, "a.md"), "# A\n\nshared term\n")
	WriteFile(t, filepath.Join(sibling, "b.md"), "# B\n\nshared term\n")
	retriever, files := countedRetriever(t)
	refresh(t, retriever, docs, sibling)
	files.takeReads()

	refresh(t, retriever, docs)
	assertNames(t, "after refreshing docs alone", hits(t, retriever, "shared"), "a.md", "b.md")
	assertNames(t, "reads", files.takeReads())
}

// MCP servers take concurrent calls. Refreshes and searches interleaved from
// many goroutines neither fail nor corrupt the index.
func TestConcurrentRefreshesAndSearchesAreSafe(t *testing.T) {
	docs := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		WriteFile(t, filepath.Join(docs, name), "# "+name+"\n\ncommon word\n")
	}
	retriever, _ := countedRetriever(t)
	var group sync.WaitGroup
	failures := make(chan error, 32)
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := retriever.EnsureIndex(context.Background(), []string{docs}); err != nil {
				failures <- err
			}
			if _, err := retriever.Retrieve(context.Background(), "common", nil, ""); err != nil {
				failures <- err
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent call failed: %v", err)
	}
	assertNames(t, "index after concurrent refreshes", hits(t, retriever, "common"), "a.md", "b.md", "c.md")
}

// A file that is still listed but can no longer be read is evicted, not left
// searchable under text it no longer has.
func TestAnUnreadableDocIsEvictedRatherThanLeftStale(t *testing.T) {
	docs := t.TempDir()
	locked := filepath.Join(docs, "locked.md")
	WriteFile(t, locked, "# Locked\n\nstale sentence\n")
	retriever, _ := countedRetriever(t)
	refresh(t, retriever, docs)

	WriteFile(t, locked, "# Locked\n\nnew sentence, longer\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	refresh(t, retriever, docs)
	assertNames(t, "stale text", hits(t, retriever, "stale"))
}

// A file deleted between the walk and its stat is evicted like any other.
func TestADocThatVanishesDuringARefreshIsEvicted(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "brief.md"), "# Brief\n\nfleeting note\n")
	retriever, files := countedRetriever(t)
	refresh(t, retriever, docs)

	files.vanished = map[string]bool{"brief.md": true}
	refresh(t, retriever, docs)
	assertNames(t, "after it vanished", hits(t, retriever, "fleeting"))
}

// A document's frontmatter name is its title; without one, its file name is.
func TestADocIsTitledByItsFrontmatterName(t *testing.T) {
	docs := t.TempDir()
	WriteFile(t, filepath.Join(docs, "named.md"), "---\nname: Release Runbook\n---\n\nrunbook steps\n")
	retriever, _ := countedRetriever(t)
	refresh(t, retriever, docs)
	refs, err := retriever.Retrieve(context.Background(), "runbook", nil, "")
	if err != nil || len(refs) != 1 || refs[0].Title != "Release Runbook" {
		t.Errorf("refs = %+v (err %v), want one titled by its frontmatter", refs, err)
	}
}

// A root that is a file is not a corpus. Walked, it would admit that file
// whatever its extension, and — nothing lying "under" a file — never evict
// it again.
func TestARootThatIsAFileIndexesNothing(t *testing.T) {
	source := filepath.Join(t.TempDir(), "main.go")
	WriteFile(t, source, "package main // unmistakable\n")
	retriever, files := countedRetriever(t)
	refresh(t, retriever, source)
	assertNames(t, "indexed from a file root", hits(t, retriever, "unmistakable"))
	assertNames(t, "reads", files.takeReads())
}

// A store that fails is reported, never read as an empty index: the search
// would otherwise answer "nothing found" for a corpus it could not read — the
// failure-as-success L2.5 removed.
func TestAFailingIndexStoreIsReportedNotReadAsEmpty(t *testing.T) {
	cases := []struct {
		name     string
		sabotage string
	}{
		{"the store is closed", ""},
		{"fingerprints cannot be read", "DROP TABLE docs_files"},
		{"a fingerprint cannot be written", "CREATE TRIGGER refuse_insert BEFORE INSERT ON docs_files BEGIN SELECT RAISE(ABORT, 'refused'); END"},
		{"an eviction cannot be written", "CREATE TRIGGER refuse_delete BEFORE DELETE ON docs_files BEGIN SELECT RAISE(ABORT, 'refused'); END"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs := t.TempDir()
			WriteFile(t, filepath.Join(docs, "kept.md"), "# Kept\n\nsome text\n")
			WriteFile(t, filepath.Join(docs, "gone.md"), "# Gone\n\nsome text\n")
			retriever, _ := countedRetriever(t)
			refresh(t, retriever, docs)
			// A change to write and a removal to evict, so every write path runs.
			WriteFile(t, filepath.Join(docs, "kept.md"), "# Kept\n\nsome longer text\n")
			if err := os.Remove(filepath.Join(docs, "gone.md")); err != nil {
				t.Fatalf("remove: %v", err)
			}
			breakStore(t, retriever, tc.sabotage)
			if err := retriever.EnsureIndex(context.Background(), []string{docs}); err == nil {
				t.Error("a failing store refreshed without error")
			}
		})
	}
}

func breakStore(t *testing.T, retriever *BM25Retriever, statement string) {
	t.Helper()
	if statement == "" {
		if err := retriever.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		return
	}
	if _, err := retriever.db.Exec(statement); err != nil {
		t.Fatalf("break store: %v", err)
	}
}

// A blank root is not the current directory: it is skipped.
func TestABlankRootIsSkipped(t *testing.T) {
	retriever, files := countedRetriever(t)
	refresh(t, retriever, "  ")
	assertNames(t, "reads", files.takeReads())
}
