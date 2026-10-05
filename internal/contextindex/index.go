// Package contextindex is the sidecar `.loom/index.json`: one entry per
// directory saying what it is for, how it was documented and whether that is
// still true (ADR-010). At this phase it holds only deterministic fields;
// purpose, tags and layer stay empty until the repo-documenter fills them.
package contextindex

import "sort"

// SchemaVersion is the index.json format version this build writes.
const SchemaVersion = 1

// File is the index's path within .loom/.
const File = "index.json"

// Status says whether an entry's documentation still matches the tree.
type Status string

const (
	// StatusFresh: documented, and the directory has not changed since.
	StatusFresh Status = "fresh"
	// StatusStale: documented, but the directory's input hash has changed.
	StatusStale Status = "stale"
	// StatusUndocumented: never documented.
	StatusUndocumented Status = "undocumented"
)

// Entry describes one directory.
//
// InputHash is the fact-sheet hash the entry was documented against, and is
// empty until it is documented. It is compared with the working tree's
// current hash to decide freshness; it is never overwritten by indexing.
type Entry struct {
	Purpose       string   `json:"purpose" jsonschema:"description=Empty until documented"`
	Tags          []string `json:"tags"`
	Layer         string   `json:"layer" jsonschema:"description=Empty until the documenter assigns one"`
	InputHash     string   `json:"input_hash" jsonschema:"description=Fact-sheet hash this entry was documented against or empty"`
	DocHash       string   `json:"doc_hash" jsonschema:"description=Hash of the generated documentation or empty"`
	GeneratedAt   string   `json:"generated_at" jsonschema:"description=When it was documented or empty"`
	Status        Status   `json:"status" jsonschema:"enum=fresh,enum=stale,enum=undocumented"`
	OpenQuestions []string `json:"open_questions"`
}

// Index is the document written to .loom/index.json, keyed by directory path.
type Index struct {
	SchemaVersion int              `json:"schema_version" jsonschema:"description=Always the SchemaVersion this build writes"`
	Entries       map[string]Entry `json:"entries"`
}

// StatusOf compares the hash an entry was documented against with the
// directory's current hash.
func StatusOf(documentedHash, currentHash string) Status {
	if documentedHash == "" {
		return StatusUndocumented
	}
	if documentedHash == currentHash {
		return StatusFresh
	}
	return StatusStale
}

func sorted(items []string) []string {
	sort.Strings(items)
	return items
}
