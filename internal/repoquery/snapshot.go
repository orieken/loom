// Package repoquery answers questions about a repository from its stored
// context index: what a path depends on, what depends on it, where programs
// start, what a change would reach, and what a directory is for (ADR-010,
// Phase 7). It reads the artifacts `loom index` wrote and never extracts
// anything itself, except to check whether they still match the working tree.
//
// Every answer carries the graph's revision and a freshness flag, so a caller
// knows whether it is reading the repository as it is or as it was.
package repoquery

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
)

// ErrNoIndex is returned when the repository has not been indexed.
var ErrNoIndex = errors.New("this repository has no repo graph: run `loom index` first")

// DefaultFreshnessTTL bounds how often the working tree is re-extracted to
// check freshness. Extraction reads every tracked source file, so it is done
// at most once per interval however many questions arrive.
const DefaultFreshnessTTL = 30 * time.Second

// Snapshot is the stored index at one moment, with whether it still matches
// the working tree.
type Snapshot struct {
	Graph repograph.Graph
	Index contextindex.Index
	// HasIndex says index.json exists; a repo may have a graph and no index.
	HasIndex bool
	// Fresh says the graph on disk is the graph the working tree would
	// produce now.
	Fresh bool

	root   string
	facts  map[string]factsheet.FactSheet
	loaded bool
	cache  *factsCache
}

// GraphSHA is the commit the graph was built at.
func (s Snapshot) GraphSHA() string { return s.Graph.GitSHA }

// Facts returns the stored fact sheets, read on first use.
func (s *Snapshot) Facts() (map[string]factsheet.FactSheet, error) {
	if s.loaded {
		return s.facts, nil
	}
	var facts map[string]factsheet.FactSheet
	var err error
	if s.cache != nil {
		facts, err = s.cache.get(s.root, s.Graph.GraphHash)
	} else {
		facts, err = factsheet.LoadAll(s.root)
	}
	if err != nil {
		return nil, fmt.Errorf("read fact sheets: %w", err)
	}
	s.facts, s.loaded = facts, true
	return facts, nil
}

// factsCache keeps the parsed fact sheets between calls. A snapshot is built per
// call, and without this every entrypoints or describe call re-read and parsed
// every sheet in .loom/facts. It is reloaded when the graph changes and at most
// once per interval otherwise — the same bound the freshness check has, so a
// sheet edited by a body-only change can lag by that long.
type factsCache struct {
	ttl time.Duration
	now func() time.Time

	mu        sync.Mutex
	graphHash string
	loadedAt  time.Time
	sheets    map[string]factsheet.FactSheet
}

func (c *factsCache) get(root, graphHash string) (map[string]factsheet.FactSheet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sheets != nil && c.graphHash == graphHash && c.now().Sub(c.loadedAt) < c.ttl {
		return c.sheets, nil
	}
	sheets, err := factsheet.LoadAll(root)
	if err != nil {
		return nil, err
	}
	c.sheets, c.graphHash, c.loadedAt = sheets, graphHash, c.now()
	return sheets, nil
}

// Store loads snapshots for one repository.
type Store struct {
	root   string
	lister repograph.Lister
	ttl    time.Duration
	now    func() time.Time

	facts *factsCache

	mu         sync.Mutex
	checkedAt  time.Time
	checkedFor string
	liveHash   string
	haveLive   bool
	liveFailed error
}

// NewStore returns a Store for a repository root.
func NewStore(root string, lister repograph.Lister) *Store {
	store := &Store{root: root, lister: lister, ttl: DefaultFreshnessTTL, now: time.Now}
	store.facts = &factsCache{ttl: store.ttl, now: func() time.Time { return store.now() }}
	return store
}

// Snapshot reads the stored graph and index and checks them against the
// working tree. A missing graph is ErrNoIndex. A freshness check that cannot
// run — not a git repository, say — reports not fresh rather than failing the
// question: the answer is still useful, and the flag says not to trust it.
func (s *Store) Snapshot() (Snapshot, error) {
	if s.root == "" {
		return Snapshot{}, ErrNoIndex
	}
	graph, found, err := repograph.LoadGraph(s.root)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		return Snapshot{}, ErrNoIndex
	}
	index, hasIndex, err := contextindex.Load(s.root)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Graph: graph, Index: index, HasIndex: hasIndex, Fresh: s.matchesWorkingTree(graph), root: s.root, cache: s.sharedFacts()}, nil
}

// matchesWorkingTree compares the stored graph's hash with a freshly extracted
// one, re-extracting at most once per ttl. A new stored graph — `loom index`
// ran since — invalidates the cached extraction at once.
func (s *Store) matchesWorkingTree(stored repograph.Graph) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expired := s.now().Sub(s.checkedAt) >= s.ttl
	if !s.haveLive || expired || s.checkedFor != stored.GraphHash {
		s.liveHash, s.liveFailed = s.extractHash()
		s.checkedAt, s.checkedFor, s.haveLive = s.now(), stored.GraphHash, true
	}
	return s.liveFailed == nil && s.liveHash == stored.GraphHash
}

func (s *Store) extractHash() (string, error) {
	repo, err := repograph.OpenRepo(s.root, s.lister)
	if err != nil {
		return "", err
	}
	graph, err := repograph.Build(repo, repograph.DefaultExtractors(), repograph.Meta{})
	if err != nil {
		return "", err
	}
	return graph.GraphHash, nil
}

// sharedFacts is the store's cache, with the store's current interval: tests
// and callers that change ttl after construction are honoured.
func (s *Store) sharedFacts() *factsCache {
	s.facts.ttl = s.ttl
	return s.facts
}
