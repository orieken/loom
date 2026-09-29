# ADR-002: Corpus-Aware Retrieval Strategy (Graduated RAG)

## Status

Accepted

## Date

2026-07-22

## Context

The framework needs retrieval capabilities so agents and human chats can answer questions like "have we written about this before?", "have we made a decision about X?", and "have we built something like this feature before?". These questions apply to two structurally different corpora:

1. **The framework's own corpus** — Knowledge Items and ADRs under `shared/knowledge/` and `docs/adrs/`. Small (~30-200 items expected long-term), stable, deliberately curated, prose-heavy.
2. **The installed project's corpus** — the codebase and documentation of any project that installs the framework. Potentially huge (thousands of source files, hundreds of feature docs), dynamic (changes daily), mixed prose and code.

One retrieval algorithm serves these poorly. Pure vector RAG is overkill for the framework corpus (which fits in a context window). It doesn't add over Claude Code's built-in code search for source. Pure lexical scales poorly for the semantic queries the installed-project corpus needs — e.g., the analyst's "have we built something like this feature before?" today runs via grep + mtime scanning of `docs/features/`, which misses paraphrases.

[ADR-001](ADR-001-adopt-rag-friendly-docs-structure.md) established the docs directory structure (`docs/features/`, `docs/adrs/`, `docs/patterns/`, `docs/runbooks/`) explicitly to be RAG-friendly. This ADR is the retrieval side of that story: given that structure exists, what actually retrieves against it?

Design pack guidance in `docs/aos/AOS_Governance_Design_Pack/06-LightRAG-Strategy.md` establishes that markdown stays canonical and retrieval is an optional index over it, but doesn't prescribe the algorithm.

The AOS migration plan Phase 3 Ops 3.3-3.4 needs this decision before implementation.

## Decision

Adopt a **graduated, corpus-aware retrieval strategy** with three implementations behind one adapter interface:

| Corpus | Retrieval | Rationale |
|---|---|---|
| Framework KIs + ADRs | LLM-as-retriever | Corpus fits in context; simplest thing that works; no index maintenance; no drift |
| Installed project's `docs/` (features, adrs, KIs) | BM25 via sqlite-fts5 | Prose retrieval, structured docs, mature, no ML, no drift |
| Installed project's feature-archive semantic similarity | Vector RAG via sqlite-vec | The "have we built this before?" query is inherently semantic |
| Installed project's source code | Lean on client (Claude Code Grep/Glob/Read); optional vector backend later | Modern LLM clients already do this well; only build if telemetry shows a gap |

All retrieval capabilities are packaged as **MCP tools inside the broad framework-MCP** (`saturday-mcp` per the mcp-expand scope decision recorded in `saturday-mcp/docs/prompts/mcp-expand.md`). Every MCP client (Claude Code, Cursor, Claude Desktop) gets the same retrieval against any installed project. No Claude-Code-specific skill for retrieval — MCP is the transport.

Retrieval always returns references to canonical markdown or source, never content copies. Retrieval hits are pointers, per the LightRAG-Strategy principle "always verify retrieved knowledge against markdown."

The `shared/rag/` adapter interface stays intentionally simple: `Retrieve(query, corpus) → []Reference`. Implementations swap without touching callers.

## Consequences

### What becomes easier

- Right tool for each corpus. The framework's small KI corpus doesn't pay embedding cost; installed-project source doesn't reinvent code search.
- Ships incrementally. The docs BM25 tool is the highest-leverage single addition (powers agent + chat queries about the actual project) and can ship in Milestone 1 of mcp-expand. Feature-archive vector search follows. Source retrieval is deferred indefinitely.
- Every MCP client benefits equally. Cursor + Claude Desktop + Claude Code all get the same tools against an installed project without per-client integration.
- ADR-001's prescriptive discipline (structured docs, DOMAIN_DICTIONARY, feature-doc contracts) directly amplifies retrieval quality. The framework's "GEO the code files" pillar becomes real, not just a phrase.

### What becomes harder

- Three retrieval implementations instead of one. The adapter interface must be genuinely clean to prevent the "three shapes" from becoming three tightly-coupled specialties.
- Testing must cover three retrieval paths, each with its own corpus fixture.
- The `.claude/rag/` index directory per installed project needs a rebuild story (install-time initial pass + `/reindex` skill for manual). Cache invalidation on KI/doc edits is a real concern for the vector tier.

### Trade-offs

- **LightRAG as future v2** — the design pack names LightRAG as the preferred vector backend, but it adds a Python runtime dependency to installs. Deferring to a v2 upgrade if sqlite-vec's simpler baseline proves insufficient in practice.
- **Judgment-only fitness function for retrieval quality** — retrieval quality is hard to measure without a test set that itself becomes a maintenance burden. Instead, use the AOS telemetry layer (Phase 1 landed) to record retrieval events (query, hits returned, hit chosen/dismissed). Review the log periodically for miss patterns. If a class of miss becomes chronic, that's the signal to graduate a corpus to the next retrieval tier.

  > **Amendment, 2026-09-01 (roadmap L3.9).** The mechanism above never existed. The AOS telemetry
  > layer recorded nothing — `.claude/telemetry/events.jsonl` had no verified writer — and a
  > `retrieval.queried` event type was never defined, let alone emitted. That layer is now retired,
  > so there is no log to review periodically and no miss pattern to find.
  >
  > The decision stands; what changes is an honest account of what backs it. Guardrail #7 permits a
  > judgment-only fitness function with a documented reason, and the documented reason is now this:
  > **retrieval quality is currently unmeasured**, and graduating a corpus to a higher tier is a
  > judgement call with no data behind it. Giving it data means building the retriever and its
  > emitter together — roadmap **L3.4** — not adding a row to a table. Until then, treat any claim
  > about retrieval quality in this repository as an opinion.

  > **Amendment, 2026-09-24 (roadmap L3.4).** The retriever and its emitter now exist together.
  > Every corpus search through `loom mcp serve` records a `retrieval.queried` event — corpus,
  > backend, hit count, hit or miss, top hit — as properties only: the query is a salted hash and a
  > length, never its text (guardrail #9). The fitness function is still judgment-only, but it now
  > has data to judge: a query hash that keeps missing is the signal this ADR named. Two decisions
  > above changed in building it. Vector retrieval is exact cosine search in pure Go, not
  > `sqlite-vec`, because release builds are `CGO_ENABLED=0` and the pure-Go sqlite driver cannot
  > load a native extension; `sqlite-vec` is exact brute-force search too, so no quality is lost at
  > this scale. And merged results are fused by reciprocal rank, not interleaved round-robin
  > (`shared/rag/retriever.interface.md` rule 6).
- **Mechanical CI check that could be added later**: assert every entry in `shared/memory-registry.json` has a `retrievalBackend` field valued from the enum `{lexical, llm-as-retriever, bm25, vector}` — no false positives, catches "someone added a source without deciding how it's retrieved."

## Related

- Supersedes: none
- Superseded by: none
- Related: [ADR-001](ADR-001-adopt-rag-friendly-docs-structure.md) established the structural side that this decision retrieves against
- Implements: `docs/aos/migration-plan.md` Phase 3 Ops 3.3 and 3.4
- References: `docs/aos/AOS_Governance_Design_Pack/06-LightRAG-Strategy.md`
