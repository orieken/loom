// Package tools - bm25_retriever.go
//
// BM25 retrieval over the installed project's docs corpus. Per framework
// ADR-002 (Corpus-Aware Retrieval Strategy), this is retrieval tier 2:
// prose retrieval via sqlite-fts5 with the built-in bm25() ranking
// function.
package tools

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"

	// Register the pure-Go sqlite driver under the name "sqlite".
	_ "modernc.org/sqlite"
)

// BM25Retriever implements Retriever using fts5's built-in bm25()
// ranking.
type BM25Retriever struct {
	db     *sql.DB
	dbPath string
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
	if _, err := db.Exec(docsFTSSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bm25 retriever: schema init failed: %w", err)
	}
	return &BM25Retriever{db: db, dbPath: dbPath}, nil
}

// Close releases the underlying database handle.
func (r *BM25Retriever) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// EnsureIndex walks each corpus root and (re)indexes every .md / .mdx file.
func (r *BM25Retriever) EnsureIndex(corpusPaths []string) error {
	for _, root := range corpusPaths {
		if strings.TrimSpace(root) == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		if walkErr := r.walkAndIndex(root); walkErr != nil {
			return fmt.Errorf("bm25 retriever: walk %q: %w", root, walkErr)
		}
	}
	return nil
}

// walkAndIndex indexes every markdown file under root through the same
// collector the analyzers use (roadmap L2.3): it never follows a symbolic
// link out of the workspace, and it stops at the walk ceilings.
func (r *BM25Retriever) walkAndIndex(root string) error {
	files, err := analyzers.CollectFiles(root, isMarkdownExtension)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := r.indexFile(file); err != nil {
			return err
		}
	}
	return nil
}

func isMarkdownExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

func (r *BM25Retriever) indexFile(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	title := extractFrontmatterName(body)
	if title == "" {
		title = titleFromFilename(path)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM docs_fts WHERE path = ?", path); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(
		"INSERT INTO docs_fts (path, title, body) VALUES (?, ?, ?)",
		path, title, string(body),
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
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
func (r *BM25Retriever) Retrieve(query string, tags []string, domain string) ([]Reference, error) {
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
	rows, err := r.db.Query(
		`SELECT path, title,
		        snippet(docs_fts, 2, '[', ']', '...', 20) AS summary,
		        bm25(docs_fts, 1.0, 10.0, 1.0) AS relevance
		 FROM docs_fts
		 WHERE docs_fts MATCH ?
		 ORDER BY relevance
		 LIMIT ?`,
		escaped, bm25MaxResults,
	)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

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
