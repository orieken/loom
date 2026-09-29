# Adapter: Vector (Feature Archive Semantic Search)

**Corpus**: `project-features`
**Files**: `<project>/docs/features/*/` (feature deliveries and retrospectives)
**Implementation home**: `loom mcp serve` — the `search_features` tool (roadmap L3.4)

## Rationale

The "have we built something like this before?" query is inherently semantic. A feature spec may say
"user preference sync" while a prior delivery called it "settings persistence across devices". Keyword
matching misses this. Cosine similarity on embeddings captures it.

The feature archive is the right scope for vector search because:
- Bounded size — one subdirectory per delivered feature, grows linearly with deliveries.
- High-value semantic queries — analyst asking about prior work is where vector search pays off most.
- Manageable churn — feature deliveries are complete documents, not actively-edited source files.

BM25 handles `docs/adrs/` and `docs/patterns/` (keyword retrieval is sufficient for those structured
bodies); vector handles `docs/features/` (semantic similarity is the meaningful query there).

## Implementation

**Built in roadmap L3.4** (2026-09-24) — as a fused half of the `search_features` MCP tool in
`loom mcp serve`, not the standalone `saturday-mcp` tool this document first planned.

| Tool | Corpus | Query type |
|---|---|---|
| `search_features` | `docs/features/` | BM25, fused by reciprocal rank with cosine similarity when an embedder is configured |

### Vector store

- **Backend**: exact cosine similarity computed in Go, over vectors stored as little-endian float32
  BLOBs in sqlite (the pure-Go `modernc.org/sqlite` driver). **Not `sqlite-vec`**: it is a C extension,
  release builds are `CGO_ENABLED=0`, and the pure-Go driver cannot load a native extension. Using it
  would have meant replacing the sqlite driver and the BM25 storage built on it. `sqlite-vec`'s `vec0` is
  exact brute-force KNN too, so at a feature archive's scale — thousands of sections — nothing is lost.
- **Storage**: `.claude/rag/features-vectors.sqlite` beside `features-fts5.sqlite` (gitignored with `.claude/rag/`).
- **Embedding provider**: behind the `Embedder` interface. Ollama is the one adapter
  (`LOOM_EMBEDDINGS=ollama`, `LOOM_EMBEDDING_MODEL` default `nomic-embed-text`, `OLLAMA_HOST`). Off by
  default: an embedding call leaves the process. `nomic-embed-text`'s task prefixes (`search_document:`,
  `search_query:`) are applied by the adapter, which is where a model's conventions belong.
- **Dimensions**: whatever the model returns (768 for `nomic-embed-text`). A section embedded by a model
  of another width is skipped at query time and re-embedded on the next refresh.

### Algorithm

1. Refresh: walk the root, stat each markdown file against a `(modification time, size, model)`
   fingerprint, and embed only what changed — per `##` section, at most ~2000 characters each. Each file
   commits on its own, so an interrupted first index resumes rather than restarts. Deleted files are evicted.
2. Query: with nothing indexed under the root, answer empty without calling the provider. Otherwise embed
   the query, score every section under the root by cosine similarity, keep each document's best section,
   and return the top 10 as references — path, title, and the section's opening words as the excerpt.

### Index management

Incremental on every search, as above; there is no separate build step. The `install.sh --full` pass and
`/reindex` skill this section once named do not exist and are not needed.

### Proof

`TestAParaphraseBM25MissesIsAnsweredByTheVectorAdapter` (opt-in, `LOOM_OLLAMA_TEST=1`): "user preference
sync" shares no word with a delivery titled "Settings persistence across devices". BM25 returns nothing;
the vector adapter and the hybrid both rank it first.

## LightRAG as v2

The design pack (`docs/aos/AOS_Governance_Design_Pack/06-LightRAG-Strategy.md`) names LightRAG as the
preferred graph-aware vector backend. The pure-Go exact search is the simpler v1 baseline — no Python runtime
dependency, no graph traversal overhead, straightforward upgrade path. If telemetry reveals that
flat cosine similarity misses relationship-aware queries (e.g., "features that depend on
the auth system"), upgrade to LightRAG via a new ADR. The adapter interface stays unchanged.

## References

- Interface: [`../retriever.interface.md`](../retriever.interface.md)
- ADR: [`../../../docs/adrs/ADR-002-corpus-aware-retrieval-strategy.md`](../../../docs/adrs/ADR-002-corpus-aware-retrieval-strategy.md)
- Design pack: `docs/aos/AOS_Governance_Design_Pack/06-LightRAG-Strategy.md`
- Implementation: `shared/mcp/internal/tools/vector_index.go`, `ollama_embedder.go`, `corpus_index.go`
