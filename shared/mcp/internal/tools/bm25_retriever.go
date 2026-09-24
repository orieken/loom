// Package tools - bm25_retriever.go
//
// BM25 retrieval over the installed project's docs corpus. Per framework
// ADR-002 (Corpus-Aware Retrieval Strategy), this is retrieval tier 2:
// prose retrieval via sqlite-fts5 with the built-in bm25() ranking
// function.
package tools

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// Register the pure-Go sqlite driver under the name "sqlite".
	_ "modernc.org/sqlite"
)

// BM25Retriever implements Retriever using fts5's built-in bm25()
// ranking.
//
// The mutex makes it safe for the concurrent calls an MCP server fields
// (roadmap L2.7): indexing takes it exclusively, so two refreshes never
// interleave, and a search waits for a refresh rather than reading a half-
// written index.
type BM25Retriever struct {
	db     *sql.DB
	dbPath string
	files  docFiles
	mutex  sync.RWMutex
}

const bm25MaxResults = 25

const docsFTSSchema = `CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
	path UNINDEXED,
	title,
	body,
	tokenize = 'porter unicode61'
)`

// NewBM25Retriever opens (or creates) the sqlite database at dbPath and
// ensures the docs_fts virtual table exists.
func NewBM25Retriever(dbPath string) (*BM25Retriever, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("bm25 retriever: dbPath is required")
	}
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("bm25 retriever: cannot create db dir %q: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("bm25 retriever: open %q failed: %w", dbPath, err)
	}
	if err := createDocsSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bm25 retriever: schema init failed: %w", err)
	}
	return &BM25Retriever{db: db, dbPath: dbPath, files: osDocFiles{}}, nil
}

// Close releases the underlying database handle.
func (r *BM25Retriever) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// EnsureIndex brings the index of each corpus root up to date. Only what
// changed is read — see bm25_index.go.
func (r *BM25Retriever) EnsureIndex(ctx context.Context, corpusPaths []string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, root := range corpusPaths {
		if !r.isIndexableRoot(root) {
			continue
		}
		if err := r.reconcile(ctx, root); err != nil {
			return fmt.Errorf("bm25 retriever: index %q: %w", root, err)
		}
	}
	return nil
}

// isIndexableRoot is a directory. A blank root fails the stat, so it is
// skipped rather than read as the working directory.
func (r *BM25Retriever) isIndexableRoot(root string) bool {
	info, err := r.files.Stat(root)
	return err == nil && info.IsDir()
}

func isMarkdownExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

func extractFrontmatterName(body []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	if !scanner.Scan() || scanner.Text() != "---" {
		return ""
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			return ""
		}
		if strings.HasPrefix(line, "name:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
	}
	return ""
}

// Retrieve runs an fts5 MATCH query and returns up to bm25MaxResults
// hits ordered by BM25 rank (title-column-boosted 10x over body).
func (r *BM25Retriever) Retrieve(ctx context.Context, query string, tags []string, domain string) ([]Reference, error) {
	_ = tags
	_ = domain
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}
	escaped := escapeFTS5Query(trimmed)
	if escaped == "" {
		return nil, nil
	}
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	rows, err := r.db.QueryContext(ctx,
		`SELECT path, title,
		        snippet(docs_fts, 2, '[', ']', '...', 20) AS summary,
		        bm25(docs_fts, 1.0, 10.0, 1.0) AS relevance
		 FROM docs_fts
		 WHERE docs_fts MATCH ?
		 ORDER BY relevance
		 LIMIT ?`,
		escaped, bm25MaxResults,
	)
	if ctxErr := ctx.Err(); ctxErr != nil {
		// A cancelled or timed-out search is not an empty one (roadmap L2.2);
		// the swallow below used to report it as "no results".
		return nil, ctxErr
	}
	if err != nil {
		// Unchanged: a query FTS5 cannot parse reads as no matches.
		return nil, nil
	}
	defer rows.Close()
	return scanReferences(ctx, rows)
}

// scanReferences reads ranked rows into references. A context that ends
// partway through the rows yields its error, not a partial list.
func scanReferences(ctx context.Context, rows *sql.Rows) ([]Reference, error) {
	var refs []Reference
	for rows.Next() {
		var (
			path, title, summary string
			rank                 float64
		)
		if err := rows.Scan(&path, &title, &summary, &rank); err != nil {
			continue
		}
		refs = append(refs, Reference{
			Path:      path,
			Title:     title,
			Summary:   summary,
			Relevance: -rank,
		})
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr // stopped partway through the rows; a partial list is not a result
	}
	return refs, nil
}

func escapeFTS5Query(q string) string {
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		escaped := strings.ReplaceAll(t, `"`, `""`)
		quoted = append(quoted, `"`+escaped+`"`)
	}
	return strings.Join(quoted, " ")
}
