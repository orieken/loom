---
feature: "api-rate-limiting"
bounded_context: "UNRESOLVED — no Bounded Context in DOMAIN_DICTIONARY.md covers a running public API service"
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Context Manifest: Add rate limiting to the public API

## 1. Scope and Boundaries
- **Target Component**: Unknown — this repository is `ai-assistant-dot-files` (the Loom agent-orchestration
  framework: a Go CLI plus `shared/agents/`, `shared/skills/`, `shared/rules/` markdown). It contains no
  `src/` directory, no `/api/v1/` service, and no Redis-backed application code of any kind. A directory
  listing at repo root and a `Glob` for `src/**` both confirm this.
- **Relevant Layers**: Cannot be determined. The task describes an Interface Adapter (HTTP middleware) and
  Infrastructure (Redis client) change in a conventional web service — a layering this repository's own
  Clean Architecture guardrails (`shared/rules/architecture-guardrails.md`) would apply to *if* such a
  service existed here. It does not.
- **Bounded Context**: No match. `DOMAIN_DICTIONARY.md` §1 defines exactly five Bounded Contexts for this
  repository — Agent Orchestration, Craftsmanship Governance, Test Automation, Feature Delivery,
  Documentation Knowledge Base — none of which models "a public API with rate-limited HTTP endpoints."
  Forcing this task into one of the five would be a false mapping, so step 3's bounded-context assignment
  is left unresolved rather than guessed.

**This manifest cannot respond to the task as scoped, because its "Known context" names files and an
archived feature that do not exist in this repository.** Before any pinning, implementation, or budget
work can proceed, it needs confirmation of which repository actually contains `src/api/`, `src/cache/`,
and the `docs/features/auth-overhaul/` delivery.

## 2. Pinpoint Files (To Keep Open)
None pinned. Every file named in "Known context" was checked and does not exist:

- `src/api/middleware/auth.middleware.ts` — **MISSING**. No `src/` directory exists anywhere in this repo
  (`Glob src/**` → no results; repo root listing has no `src`).
- `src/cache/redis-client.ts` — **MISSING**, same check.
- Confirmed a second way: `mcp__loom__impact` (the repo graph) was called with both paths as seeds and
  returned a validation error — `"not in the repo graph: src/api/middleware/auth.middleware.ts,
  src/cache/redis-client.ts"` — rather than an empty dependents list. The graph treats an unknown path as
  an error, not a smaller answer, which is a second independent confirmation these files were never part
  of this repo's tracked source.

Pinning either path anyway (as the task's phrasing implies) would hand the next stage a manifest pointing
at code that isn't there.

## 3. Global Rules and Constraints
These are the real, applicable repo-wide rule files — relevant to any task here regardless of the scope
question above:
- [ARCHITECTURE_RULES.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/ARCHITECTURE_RULES.md)
- [DOMAIN_DICTIONARY.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/DOMAIN_DICTIONARY.md)
- [shared/rules/architecture-guardrails.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/shared/rules/architecture-guardrails.md) — would govern a Redis-backed middleware's layering (guardrail §1, §5, §6) *if* this task had a real target
- [shared/rules/approval-gates.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/shared/rules/approval-gates.md)

## 4. Knowledge Items & ADRs (To Load)
- **ADR-007 claim is false.** The task states "ADR-007 covers the Redis usage decision." The actual
  `docs/adrs/ADR-007-ui-test-location-and-artifact-contract.md` is titled "Where UI Tests Live, and What
  They Hand the Pipeline" — it is about `visual-qa-engineer`'s consumption of Playwright heatmaps and
  screenshot baselines, not Redis, not caching, not rate limiting. This is flagged here rather than linked
  as "required reading," because linking it as the task describes would misrepresent its content to any
  downstream agent that trusts this manifest without re-reading the ADR itself.
- No other ADR in `docs/adrs/` (ADR-001 through ADR-010, checked by listing) mentions Redis, caching, or
  rate limiting by title.
- **No Knowledge Item found.** `shared/knowledge/` and `docs/adrs/` were grepped case-insensitively for
  `redis` and `rate.limit` — zero matches. `mcp__loom__find_files` was also queried with "rate limit redis
  cache middleware" — the 12 results returned (token-optimization docs, fuzz-test skills, policy examples,
  etc.) are lexical false-positives on "limit"/"cache"-adjacent words, none is a rate-limiting or Redis
  pattern. There is nothing to load here.

## 5. Prior Deliveries in This Bounded Context
- `docs/features/auth-overhaul/` — **does not exist**. `Glob docs/features/*` lists only four archived
  deliveries: `2026-07-aos-foundations-and-frontmatter`, `context-engineering-framework`, `loom-codemap`,
  `token-optimization-strategies`. There is no `auth-overhaul`.
- Because step 3 could not resolve a Bounded Context for this task, the step-4 grep (`**Owning
  [Cc]ontext**` across `docs/features/*/analysis.md`) was still run for completeness rather than skipped.
  It found exactly one hit: `docs/features/context-engineering-framework/analysis.md` → "Owning context:
  Agent Orchestration." That delivery is about the context-engineering framework itself, not an API or
  caching feature, and is not relevant prior art here.
- No prior deliveries found in this bounded context — because this bounded context, as named by the task,
  does not exist in this repository's archive.

## 6. Prune Recommendations (To Close)
No files were open in this session to prune against bounded-context or change-surface rules — this was a
cold-start task with nothing already loaded.

## 7. Token Budget
- **Target agent tier**: developer (≤80%) of a 200k-token context window — the task is phrased as a
  concrete implementation change, not an analysis/design task, so this is the tier it would target *if*
  it had real files to pin.
- **Estimated total tokens**: _not computed — zero files were pinned in section 2, so there is nothing for
  `loom` to measure. Leave blank per the "leave blank" convention rather than reporting 0 as if it were a
  real measurement of a real file set._
- **Status**: _not computed, for the same reason._
- **Blast radius**: not computed. `mcp__loom__impact` was called on the two named seed paths (depth 2) and
  returned `not in the repo graph` rather than a dependents list — there is no graph answer to record, and
  no `blastRadius` block is included per the step-7 fallback ("if the repo-graph tools are unavailable or
  not_found, pin by Grep... omit blastRadius").

---

## Resolution needed before this manifest can be completed
This task's "Known context" section describes a different codebase than the one this agent is running
against. To build a real manifest, confirm one of:
1. The intended target is a **different repository** (one that actually has `src/api/`, `src/cache/`, and
   an `auth-overhaul` feature archive) — point the context-engineer at that repo.
2. The paths were illustrative/placeholder and the actual files live elsewhere in this repo — give the
   real paths.
3. This task is not yet buildable here at all (no service layer exists yet) — in which case the correct
   next step is a feature spec and `analyst` pass to decide *where* this code should live, not a context
   manifest for files that don't exist.
