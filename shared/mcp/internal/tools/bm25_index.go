package tools

// Incremental docs indexing (roadmap L2.7). Every search used to re-read the
// whole corpus and commit one transaction per file; deleted docs were never
// evicted. Now a refresh stats each file, reads only those whose fingerprint
// changed, evicts those that are gone, and commits it all as one transaction —
// so a cancelled refresh leaves the previous index whole.

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
)

// docFiles is the filesystem as the index sees it: a seam, so a test can
// count what a refresh reads.
type docFiles interface {
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
}

type osDocFiles struct{}

func (osDocFiles) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }
func (osDocFiles) ReadFile(path string) ([]byte, error)  { return os.ReadFile(path) }

// docsFilesSchema records what each indexed file looked like when it was
// read. Additive: an index built before it simply has no fingerprints, and
// its files are read once more on the first refresh.
const docsFilesSchema = `CREATE TABLE IF NOT EXISTS docs_files (
	path        TEXT PRIMARY KEY,
	modified_ns INTEGER NOT NULL,
	size        INTEGER NOT NULL
)`

func createDocsSchema(db *sql.DB) error {
	_, err := db.Exec(docsFTSSchema + ";\n" + docsFilesSchema)
	return err
}

// fingerprint is how a file is recognised as unchanged without reading it.
type fingerprint struct {
	modifiedNS, size int64
}

func fingerprintOf(info fs.FileInfo) fingerprint {
	return fingerprint{modifiedNS: info.ModTime().UnixNano(), size: info.Size()}
}

// refreshPlan is what one refresh of a root must write.
type refreshPlan struct {
	changed map[string]fingerprint
	removed []string
}

// reconcile brings root's part of the index up to date with the disk. The
// walk goes through analyzers.CollectFiles (L2.3), so it never follows a
// symbolic link out of the workspace and stops at the walk ceilings.
func (r *BM25Retriever) reconcile(ctx context.Context, root string) error {
	files, err := analyzers.CollectFiles(ctx, root, isMarkdownExtension)
	if err != nil {
		return err
	}
	known, err := r.knownFiles(root)
	if err != nil {
		return err
	}
	plan, err := r.planRefresh(ctx, files, known)
	if err != nil {
		return err
	}
	return r.applyRefresh(ctx, plan)
}

// planRefresh compares the disk with what the index knows, reading nothing.
func (r *BM25Retriever) planRefresh(ctx context.Context, files []string, known map[string]fingerprint) (refreshPlan, error) {
	plan := refreshPlan{changed: map[string]fingerprint{}}
	present := make(map[string]bool, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return refreshPlan{}, err
		}
		info, err := r.files.Stat(file)
		if err != nil {
			continue // gone between the walk and now: evicted below
		}
		present[file] = true
		if current := fingerprintOf(info); known[file] != current {
			plan.changed[file] = current
		}
	}
	plan.removed = absentFrom(known, present)
	return plan, nil
}

// absentFrom lists the known paths that are no longer on disk.
func absentFrom(known map[string]fingerprint, present map[string]bool) []string {
	var removed []string
	for path := range known {
		if !present[path] {
			removed = append(removed, path)
		}
	}
	return removed
}

// knownFiles is every path the index holds under root, with its fingerprint.
// A path indexed before fingerprints existed is known with a zero one, so it
// is re-read if present and evicted if not.
func (r *BM25Retriever) knownFiles(root string) (map[string]fingerprint, error) {
	known, err := r.indexedPaths(root)
	if err != nil {
		return nil, err
	}
	return known, r.addFingerprints(root, known)
}

func (r *BM25Retriever) indexedPaths(root string) (map[string]fingerprint, error) {
	rows, err := r.db.Query("SELECT path FROM docs_fts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := map[string]fingerprint{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		if isUnder(path, root) {
			known[path] = fingerprint{}
		}
	}
	return known, rows.Err()
}

func (r *BM25Retriever) addFingerprints(root string, known map[string]fingerprint) error {
	rows, err := r.db.Query("SELECT path, modified_ns, size FROM docs_files")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var stamp fingerprint
		if err := rows.Scan(&path, &stamp.modifiedNS, &stamp.size); err != nil {
			return err
		}
		if isUnder(path, root) {
			known[path] = stamp
		}
	}
	return rows.Err()
}

func isUnder(path, root string) bool {
	return strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// applyRefresh writes a plan in one transaction: all of it, or — on an error
// or a cancelled context — none of it.
func (r *BM25Retriever) applyRefresh(ctx context.Context, plan refreshPlan) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := r.writePlan(tx, plan); err != nil {
		_ = tx.Rollback()
		return cancellationOr(ctx, err)
	}
	return tx.Commit()
}

// cancellationOr prefers the context's own error. A transaction begun with a
// context rolls itself back when that context ends, and the next write then
// fails with "transaction has already been committed or rolled back" — an
// internal-looking error that would count against the tool's breaker, for a
// call its caller simply stopped (L2.5, L2.6).
func cancellationOr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (r *BM25Retriever) writePlan(tx *sql.Tx, plan refreshPlan) error {
	for _, path := range plan.removed {
		if err := evict(tx, path); err != nil {
			return err
		}
	}
	// No context check per file: the transaction was begun with ctx, so once
	// it ends every write fails at once and the loop stops (L2.7 mutation X11).
	for path, stamp := range plan.changed {
		if err := r.reindex(tx, path, stamp); err != nil {
			return err
		}
	}
	return nil
}

func evict(tx *sql.Tx, path string) error {
	if _, err := tx.Exec("DELETE FROM docs_fts WHERE path = ?", path); err != nil {
		return err
	}
	_, err := tx.Exec("DELETE FROM docs_files WHERE path = ?", path)
	return err
}

// reindex replaces one file's row. A file that cannot be read is evicted
// rather than left indexed with stale text.
func (r *BM25Retriever) reindex(tx *sql.Tx, path string, stamp fingerprint) error {
	body, err := r.files.ReadFile(path)
	if err != nil {
		return evict(tx, path)
	}
	if err := evict(tx, path); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO docs_fts (path, title, body) VALUES (?, ?, ?)",
		path, documentTitle(path, body), string(body)); err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO docs_files (path, modified_ns, size) VALUES (?, ?, ?)",
		path, stamp.modifiedNS, stamp.size)
	return err
}

func documentTitle(path string, body []byte) string {
	if title := extractFrontmatterName(body); title != "" {
		return title
	}
	return titleFromFilename(path)
}
