package tools

// A corpus index answers queries within one root it keeps current (roadmap
// L3.4). The root travels with every search: before it did, a docs search
// ranked across every root ever indexed, so a search of docs/a could return
// hits from docs/b — the leak L2.7 found and left for this item.

import (
	"context"
	"errors"
	"sort"
)

// CorpusIndex keeps an index of corpus roots current and searches one of
// them. BM25Retriever, VectorIndex and HybridIndex implement it.
type CorpusIndex interface {
	// EnsureIndex brings the index of each root up to date with the disk.
	EnsureIndex(ctx context.Context, roots []string) error
	// SearchWithin returns references under root only, best first, at most
	// maxCorpusResults.
	SearchWithin(ctx context.Context, root, query string) ([]Reference, error)
}

// maxCorpusResults is the retriever contract's top-K bound
// (shared/rag/retriever.interface.md).
const maxCorpusResults = 10

// rrfConstant is reciprocal-rank fusion's k. 60 is the value from the
// original paper (Cormack et al., 2009) and the common default; it damps the
// weight of the very top ranks so one list cannot dominate the fusion.
const rrfConstant = 60

// HybridIndex searches several indexes over the same roots and fuses their
// rankings. Scores from different backends are not comparable — a BM25 rank
// and a cosine similarity live on different scales — so fusion uses rank
// alone.
type HybridIndex struct {
	indexes []CorpusIndex
}

// NewHybridIndex fuses indexes, in the order given for ties.
func NewHybridIndex(indexes ...CorpusIndex) *HybridIndex {
	return &HybridIndex{indexes: indexes}
}

// EnsureIndex refreshes every index, stopping at the first failure.
func (h *HybridIndex) EnsureIndex(ctx context.Context, roots []string) error {
	for _, index := range h.indexes {
		if err := index.EnsureIndex(ctx, roots); err != nil {
			return err
		}
	}
	return nil
}

// SearchWithin searches every index and fuses the rankings. One index
// failing fails the search: a hybrid result silently missing half its
// evidence would read as a complete one.
func (h *HybridIndex) SearchWithin(ctx context.Context, root, query string) ([]Reference, error) {
	rankings := make([][]Reference, 0, len(h.indexes))
	for _, index := range h.indexes {
		found, err := index.SearchWithin(ctx, root, query)
		if err != nil {
			return nil, err
		}
		rankings = append(rankings, found)
	}
	return fuseReciprocalRank(rankings...), nil
}

// fuseReciprocalRank merges rankings by reciprocal-rank fusion: each path
// scores the sum of 1/(k+rank) over the lists it appears in, so a path both
// backends rank well beats one only a single backend ranks first. The
// reference kept for a path is the one from the earliest list holding it.
func fuseReciprocalRank(rankings ...[]Reference) []Reference {
	scores := map[string]float64{}
	kept := map[string]Reference{}
	var order []string
	for _, ranking := range rankings {
		for rank, reference := range ranking {
			if _, seen := kept[reference.Path]; !seen {
				kept[reference.Path] = reference
				order = append(order, reference.Path)
			}
			scores[reference.Path] += 1 / float64(rrfConstant+rank+1)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	return fusedReferences(order, kept, scores)
}

func fusedReferences(order []string, kept map[string]Reference, scores map[string]float64) []Reference {
	fused := make([]Reference, 0, min(len(order), maxCorpusResults))
	for _, path := range order[:min(len(order), maxCorpusResults)] {
		reference := kept[path]
		reference.Relevance = scores[path]
		fused = append(fused, reference)
	}
	return fused
}

// ErrEmbedderUnavailable is an embedding provider that could not be reached
// or did not answer in time. It is transient: the provider may come back,
// and the tool middleware backs off and retries it (L2.6).
var ErrEmbedderUnavailable = errors.New("embedding provider unavailable")
