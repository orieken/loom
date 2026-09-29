package tools

// Semantic search over a corpus (roadmap L3.4). Documents are chunked by
// section, each chunk embedded, and a query answered by exact cosine
// similarity in Go over the chunks under the searched root.
//
// Exact search in Go rather than sqlite-vec: release builds are
// CGO_ENABLED=0, and the pure-Go sqlite driver cannot load a native
// extension. sqlite-vec's vec0 is itself exact brute-force KNN, so at a docs
// corpus's scale — thousands of chunks, 768 dimensions — nothing is lost.
//
// A refresh commits file by file, not as one transaction like BM25's: an
// embedding costs a network call per batch, and a first index that outlives
// its deadline keeps the files it finished, so the next call resumes rather
// than restarts.

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
)

const vectorSchema = `CREATE TABLE IF NOT EXISTS vec_files (
	path        TEXT PRIMARY KEY,
	modified_ns INTEGER NOT NULL,
	size        INTEGER NOT NULL,
	model       TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS vec_chunks (
	path      TEXT NOT NULL,
	ordinal   INTEGER NOT NULL,
	title     TEXT NOT NULL,
	excerpt   TEXT NOT NULL,
	embedding BLOB NOT NULL,
	PRIMARY KEY (path, ordinal)
)`

// excerptRunes bounds a reference's excerpt: context, never a content copy.
const excerptRunes = 200

// VectorIndex implements CorpusIndex by embedding similarity.
type VectorIndex struct {
	db       *sql.DB
	embedder Embedder
	files    docFiles
	mutex    sync.RWMutex
}

// NewVectorIndex opens (or creates) the vector store at dbPath.
func NewVectorIndex(dbPath string, embedder Embedder) (*VectorIndex, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("vector index: create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("vector index: open %q: %w", dbPath, err)
	}
	if _, err := db.Exec(vectorSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("vector index: schema: %w", err)
	}
	return &VectorIndex{db: db, embedder: embedder, files: osDocFiles{}}, nil
}

// Close releases the database handle.
func (v *VectorIndex) Close() error { return v.db.Close() }

// EnsureIndex embeds what changed under each root and evicts what is gone.
func (v *VectorIndex) EnsureIndex(ctx context.Context, roots []string) error {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	for _, root := range roots {
		if info, err := v.files.Stat(root); err != nil || !info.IsDir() {
			continue
		}
		if err := v.reconcile(ctx, root); err != nil {
			return fmt.Errorf("vector index: index %q: %w", root, err)
		}
	}
	return nil
}

func (v *VectorIndex) reconcile(ctx context.Context, root string) error {
	files, err := analyzers.CollectFiles(ctx, root, isMarkdownExtension)
	if err != nil {
		return err
	}
	known, err := v.knownFiles(root)
	if err != nil {
		return err
	}
	plan, err := planRefresh(ctx, v.files, files, known)
	if err != nil {
		return err
	}
	if err := v.evictAll(plan.removed); err != nil {
		return err
	}
	return v.embedChanged(ctx, plan.changed)
}

// knownFiles is every file indexed under root. A file embedded by a different
// model is known with a zero fingerprint, so it is embedded again.
func (v *VectorIndex) knownFiles(root string) (map[string]fingerprint, error) {
	lower, upper := pathRange(root)
	rows, err := v.db.Query("SELECT path, modified_ns, size, model FROM vec_files WHERE path >= ? AND path < ?", lower, upper)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := map[string]fingerprint{}
	for rows.Next() {
		var path, model string
		var stamp fingerprint
		if err := rows.Scan(&path, &stamp.modifiedNS, &stamp.size, &model); err != nil {
			return nil, err
		}
		if model != v.embedder.Model() {
			stamp = fingerprint{}
		}
		known[path] = stamp
	}
	return known, rows.Err()
}

func (v *VectorIndex) evictAll(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := evictVectors(tx, path); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func evictVectors(tx *sql.Tx, path string) error {
	if _, err := tx.Exec("DELETE FROM vec_chunks WHERE path = ?", path); err != nil {
		return err
	}
	_, err := tx.Exec("DELETE FROM vec_files WHERE path = ?", path)
	return err
}

// embedChanged embeds each changed file and commits it on its own, in a
// stable order so an interrupted index resumes where it stopped.
func (v *VectorIndex) embedChanged(ctx context.Context, changed map[string]fingerprint) error {
	paths := make([]string, 0, len(changed))
	for path := range changed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := v.embedFile(ctx, path, changed[path]); err != nil {
			return cancellationOr(ctx, err)
		}
	}
	return nil
}

// embeddedChunk is a chunk ready to store.
type embeddedChunk struct {
	excerpt string
	vector  []float32
}

// embedFile replaces one file's chunks. An unreadable file is evicted rather
// than left answering with text it no longer has, as in the BM25 index.
func (v *VectorIndex) embedFile(ctx context.Context, path string, stamp fingerprint) error {
	body, err := v.files.ReadFile(path)
	if err != nil {
		return v.evictAll([]string{path})
	}
	chunks, err := v.embedChunks(ctx, chunkDocument(string(body)))
	if err != nil {
		return err
	}
	return v.storeFile(path, documentTitle(path, body), stamp, chunks)
}

func (v *VectorIndex) embedChunks(ctx context.Context, chunks []docChunk) ([]embeddedChunk, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	texts := make([]string, len(chunks))
	for index, chunk := range chunks {
		texts[index] = chunk.text
	}
	vectors, err := v.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return nil, err
	}
	embedded := make([]embeddedChunk, len(chunks))
	for index, chunk := range chunks {
		embedded[index] = embeddedChunk{excerpt: excerptOf(chunk), vector: normalized(vectors[index])}
	}
	return embedded, nil
}

func (v *VectorIndex) storeFile(path, title string, stamp fingerprint, chunks []embeddedChunk) error {
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	if err := v.writeFile(tx, path, title, stamp, chunks); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (v *VectorIndex) writeFile(tx *sql.Tx, path, title string, stamp fingerprint, chunks []embeddedChunk) error {
	if err := evictVectors(tx, path); err != nil {
		return err
	}
	for ordinal, chunk := range chunks {
		if _, err := tx.Exec("INSERT INTO vec_chunks (path, ordinal, title, excerpt, embedding) VALUES (?, ?, ?, ?, ?)",
			path, ordinal, title, chunk.excerpt, encodeVector(chunk.vector)); err != nil {
			return err
		}
	}
	_, err := tx.Exec("INSERT INTO vec_files (path, modified_ns, size, model) VALUES (?, ?, ?, ?)",
		path, stamp.modifiedNS, stamp.size, v.embedder.Model())
	return err
}

// excerptOf is the chunk's heading and opening words, collapsed to one line.
func excerptOf(chunk docChunk) string {
	text := strings.Join(strings.Fields(chunk.text), " ")
	if runes := []rune(text); len(runes) > excerptRunes {
		text = string(runes[:excerptRunes]) + "..."
	}
	return text
}

// SearchWithin ranks the documents under root by their best chunk's cosine
// similarity to query. With nothing indexed it answers empty without asking
// the embedder anything.
func (v *VectorIndex) SearchWithin(ctx context.Context, root, query string) ([]Reference, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	chunks, err := v.chunksUnder(root)
	if err != nil || len(chunks) == 0 {
		return nil, err
	}
	vector, err := v.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	return bestPerDocument(normalized(vector), chunks), nil
}

// storedChunk is a chunk as searched.
type storedChunk struct {
	path, title, excerpt string
	vector               []float32
}

func (v *VectorIndex) chunksUnder(root string) ([]storedChunk, error) {
	lower, upper := pathRange(root)
	rows, err := v.db.Query("SELECT path, title, excerpt, embedding FROM vec_chunks WHERE path >= ? AND path < ?", lower, upper)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []storedChunk
	for rows.Next() {
		var chunk storedChunk
		var blob []byte
		if err := rows.Scan(&chunk.path, &chunk.title, &chunk.excerpt, &blob); err != nil {
			return nil, err
		}
		chunk.vector = decodeVector(blob)
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

// bestPerDocument scores every chunk and keeps each document's best, so a
// long document is one hit, not one per section.
func bestPerDocument(query []float32, chunks []storedChunk) []Reference {
	best := map[string]Reference{}
	for _, chunk := range chunks {
		if len(chunk.vector) != len(query) {
			continue // embedded by another model; re-embedded on the next refresh
		}
		score := dot(query, chunk.vector)
		if current, seen := best[chunk.path]; !seen || score > current.Relevance {
			best[chunk.path] = Reference{Path: chunk.path, Title: chunk.title, Summary: chunk.excerpt, Relevance: score}
		}
	}
	return topReferences(best)
}

func topReferences(best map[string]Reference) []Reference {
	ranked := make([]Reference, 0, len(best))
	for _, reference := range best {
		ranked = append(ranked, reference)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Relevance != ranked[j].Relevance {
			return ranked[i].Relevance > ranked[j].Relevance
		}
		return ranked[i].Path < ranked[j].Path
	})
	return ranked[:min(len(ranked), maxCorpusResults)]
}

// normalized scales vector to unit length, so a dot product is the cosine.
// A zero vector stays zero: it matches nothing, rather than dividing by zero.
func normalized(vector []float32) []float32 {
	var sum float64
	for _, value := range vector {
		sum += float64(value) * float64(value)
	}
	if sum == 0 {
		return vector
	}
	scale := float32(1 / math.Sqrt(sum))
	unit := make([]float32, len(vector))
	for index, value := range vector {
		unit[index] = value * scale
	}
	return unit
}

func dot(a, b []float32) float64 {
	var sum float64
	for index := range a {
		sum += float64(a[index]) * float64(b[index])
	}
	return sum
}

// encodeVector stores a vector as little-endian float32s.
func encodeVector(vector []float32) []byte {
	blob := make([]byte, 4*len(vector))
	for index, value := range vector {
		binary.LittleEndian.PutUint32(blob[4*index:], math.Float32bits(value))
	}
	return blob
}

func decodeVector(blob []byte) []float32 {
	vector := make([]float32, len(blob)/4)
	for index := range vector {
		vector[index] = math.Float32frombits(binary.LittleEndian.Uint32(blob[4*index:]))
	}
	return vector
}
