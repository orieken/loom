package repoquery

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/orieken/loom/internal/contextindex"
)

// Match is one directory a search found.
type Match struct {
	Path    string   `json:"path"`
	Purpose string   `json:"purpose,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Layer   string   `json:"layer,omitempty"`
	Status  string   `json:"status"`
	Score   int      `json:"score"`
}

// Weights for where a query term was found. A path or a tag is a deliberate
// name; a word in a purpose is prose, so it counts for less.
const (
	pathWeight    = 3
	tagWeight     = 2
	purposeWeight = 1
)

// FindFiles searches the context index lexically: directories carrying the tag
// (if one is given) whose path, tags or purpose contain the query's terms. It
// is deliberately not semantic — no embeddings, no model — so the same query
// always returns the same ranking.
func (s Snapshot) FindFiles(tag, query string) ([]Match, error) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	terms := terms(query)
	if tag == "" && len(terms) == 0 {
		return nil, fmt.Errorf("give a tag, a query, or both")
	}
	if !s.HasIndex {
		return nil, fmt.Errorf("%w (there is a graph but no context index yet)", ErrNoIndex)
	}
	var matches []Match
	for dir, entry := range s.Index.Entries {
		if score, ok := scoreEntry(dir, entry, tag, terms); ok {
			matches = append(matches, Match{Path: dir, Purpose: entry.Purpose, Tags: entry.Tags, Layer: entry.Layer, Status: string(entry.Status), Score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Path < matches[j].Path
	})
	return matches, nil
}

// terms splits a query into lowercase words, dropping punctuation.
func terms(query string) []string {
	return strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// scoreEntry applies the tag filter, then scores each query term. With a query,
// an entry that matches no term is excluded; with only a tag, every tagged
// entry matches.
func scoreEntry(dir string, entry contextindex.Entry, tag string, queryTerms []string) (int, bool) {
	if tag != "" && !hasTag(entry.Tags, tag) {
		return 0, false
	}
	if len(queryTerms) == 0 {
		return 1, true
	}
	score := 0
	for _, term := range queryTerms {
		score += termScore(dir, entry, term)
	}
	return score, score > 0
}

func termScore(dir string, entry contextindex.Entry, term string) int {
	score := 0
	if strings.Contains(strings.ToLower(dir), term) {
		score += pathWeight
	}
	if hasTag(entry.Tags, term) {
		score += tagWeight
	}
	if strings.Contains(strings.ToLower(entry.Purpose), term) {
		score += purposeWeight
	}
	return score
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}
