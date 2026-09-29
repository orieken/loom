# Retriever Adapter Interface

## Contract

```
Retrieve(query: string, corpus: CorpusID) → []Reference
```

### Types

```typescript
type CorpusID =
  | "framework-ki"       // shared/knowledge/ + docs/adrs/
  | "project-docs"       // <project>/docs/ (BM25)
  | "project-features"   // <project>/docs/features/ (vector)
  | "project-source"     // <project>/src/ (DEFERRED)
  // "episodic"         // what past runs did — INTENDED, deliberately NOT a CorpusID yet.
  //                    // The store exists (roadmap L3.5): .claude/memory/episodes.db, queryable
  //                    // through `loom memory`. Every adapter below is a markdown spec with no
  //                    // running backend, so adding an entry now would specify a thing nothing
  //                    // implements. L3.4 adds it alongside a retriever that can serve it.

type Reference = {
  path: string        // repo-relative path to the canonical markdown or source file
  title: string       // human-readable label (KI name, ADR title, feature name)
  excerpt: string     // 1-2 sentence context excerpt — never a content copy
  score: number       // 0.0-1.0 relevance; interpretation is backend-specific
  corpus: CorpusID    // which corpus this result came from
}
```

### Behavior Contract

1. **Returns references, not content.** Callers must read the referenced file to get content. This prevents stale embedded copies and respects the "always verify against canonical markdown" principle from `docs/aos/AOS_Governance_Design_Pack/06-LightRAG-Strategy.md`.

2. **Graceful empty result.** If no relevant results exist, returns `[]` — never throws. Callers must handle the empty case.

3. **Top-K bounded.** Returns at most 10 results per call. Callers that need broader coverage call `Retrieve` multiple times with refined queries.

4. **Corpus isolation.** A `Retrieve` call targets exactly one corpus. Multi-corpus search is implemented by the caller making sequential calls and merging by score.

5. **No side effects.** `Retrieve` is a pure read operation. It never writes to the index, never updates `memory-registry.json`, never emits telemetry. Callers may emit telemetry around `Retrieve` calls if desired.

6. **Score is comparable within a backend, not across backends.** BM25 scores and cosine similarities live on different scales, so results are merged by **reciprocal-rank fusion**, never by score: each path scores the sum of `1/(60 + rank)` over the lists it appears in, deduplicated by path. A path two backends both rank well beats one only a single backend ranks first. *(Amended 2026-09-24, roadmap L3.4 — this rule previously prescribed a round-robin interleave, which gives a single backend's weakest result the same weight as another's best.)*

7. **Scoped to its root.** A search returns only references under the corpus root it was asked about. *(Added by L3.4: before it, a docs search ranked across every root ever indexed.)*

### Implementations

| Backend | File | Corpus | In code |
|---|---|---|---|
| LLM-as-retriever | `adapters/llm-as-retriever.md` | `framework-ki` | lexical pre-filter in `search_ki`; the calling LLM judges |
| BM25 | `adapters/bm25.md` | `project-docs`, `project-features` | `BM25Retriever` — `search_docs`, `search_features` |
| Vector | `adapters/vector.md` | `project-features` | `VectorIndex` (pure Go, opt-in) — fused into `search_features` |
| Deferred | `adapters/source-retrieval.deferred.md` | `project-source` | none |

In Go the contract is `CorpusIndex` (`shared/mcp/internal/tools/corpus_index.go`): `EnsureIndex(roots)`
keeps an index current and `SearchWithin(root, query)` answers within one root. `HybridIndex` fuses any
number of them by rule 6. Rule 5 holds: the indexes never emit telemetry; a decorator in the server's
adapter layer records each search as a `retrieval.queried` event.

### Extension

To add a new corpus:
1. Add a `CorpusID` variant above.
2. Create an `adapters/<name>.md` file documenting the implementation.
3. Register the backend in `shared/memory-registry.json` under a new source entry with `retrievalBackend: "<value>"`.
4. No existing adapter or caller needs to change.
