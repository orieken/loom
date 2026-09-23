# AOS Migration Guide

Companion doc to `docs/aos/migration-plan.md`. That file is the architectural plan; this file is the
operator-facing "what do I actually do" — one section per live phase, tracking what's available and
how to opt in.

**Status at v3.2.0**: Phases 1-3 shipped. Four AOS layers are now available:
telemetry (v3.0), governance counter agents + hooks (v3.1), RAG + orchestration runtime + Learning/Forgetting engines (v3.2). All are opt-in. Default install is unchanged from v2.x.

---

## Upgrading from Any Prior Version

**Nothing to do. All AOS additions are opt-in.**

A v2.x or v3.x install upgraded to v3.2.0 without invoking any AOS-specific capability behaves
identically to the prior version. `deliver-feature`, `validate-artifact`, `health-check`, and every
existing skill, agent, rule, contract, and blueprint work exactly as they did before.

Re-run `install.sh` (or `install.sh --full` if you want AOS layers pre-seeded) after pulling v3.2.0
to pick up new files. If you skip re-running `install.sh`, everything you had keeps working.

---

## Phase 1 (v3.0): Telemetry + First Counter Agent

### Opting into telemetry

**Design is live; no producer emits events by default yet.** See `shared/telemetry/README.md` for the
event schema and recorder skill. Telemetry emission is wired in Phase 3 (v3.2) via hooks.

### Opting into memory-auditor

Available now — invoke on demand:

```
> Use the memory-auditor agent to audit shared/knowledge/ and .claude/knowledge/.
```

What it checks: schema compliance, exact duplicates, semantic-overlap candidates, stale-metadata
candidates. Read-only — never modifies files, surfaces findings for human judgment.

---

## Phase 2 (v3.1): Governance Skeleton

### Counter Agents (all 15 pairs now live)

All counter agents are in `shared/agents/`. They are read-only, invocable on demand, and can be
triggered by `validate-artifact` if you configure it. Default: structural check only.

| Counter Agent | Audits |
|---|---|
| `memory-auditor` | KI schema, duplicates, staleness |
| `context-auditor` | Context manifests and context-engineer output |
| `knowledge-auditor` | KI frontmatter schema, semantic duplication |
| `prompt-evaluator` | Agent/skill prompt hygiene, no fabricated URLs |
| `agent-evaluator` | Agent frontmatter contracts, prompt behavior |
| `rule-auditor` | Rule internal consistency, dead paths |
| `pattern-reviewer` | Pattern docs accuracy against codebase |
| `tool-validator` | Skill frontmatter, standalone-mode declarations |
| `documentation-auditor` | README/ARCHITECTURE docs staleness |
| `retrieval-evaluator` | KI retrievability via memory-registry telemetry |
| `privacy-auditor` | PII in pipeline artifacts, hardcoded tokens |
| `security-reviewer` | STRIDE threat modeling (producer — already existed) |
| `model-tier-auditor` | Agent frontmatter model_tier declarations |

Invoke any counter agent directly: `> Use the <agent-name> agent to audit <scope>.`

### Hooks Layer

`shared/hooks/` contains the hook schema and example hooks. Hooks are the mechanism for wiring
counter agents to events automatically.

**To use hooks in your project:**
1. Create `.claude/hooks/` directory in your project.
2. Copy example hooks from `shared/hooks/examples/` (or from `shared/hooks/on-*.yaml`).
3. Set `enabled: true` on the hooks you want to activate.
4. Restart your AI tool — hooks are read at session start.

Example: enable `knowledge-auditor` on every KI write:
```yaml
# .claude/hooks/on-ki-created.yaml
version: "1.0"
hooks:
  - id: "knowledge-auditor-on-ki-created"
    event: "on-ki-created"
    enabled: true        # ← flip this
    action:
      type: "agent"
      target: "knowledge-auditor"
```

### validate-artifact with Auditor Invocation

`validate-artifact` can optionally invoke the corresponding counter agent after passing its
structural check. To enable:

```yaml
# .claude/delivery-policy.yaml
validateArtifactMode: "structural+audit"   # default is "structural"
```

When this is set, every `validate-artifact` call that passes structural validation also runs the
corresponding auditor (e.g., `knowledge-auditor` for KI artifacts). If the auditor returns findings,
they are surfaced as warnings; the pipeline proceeds unless findings are Critical severity.

---

## Phase 3 (v3.2): Runtime

### Install Mode: --base vs --full

```bash
./install.sh --project /path/to/my-app            # --base (default), same as before
./install.sh --project /path/to/my-app --full     # --full: also seeds AOS layers
```

`--full` seeds `.claude/hooks/` with example hooks (all disabled), links the RAG and orchestration
interface directories, and prints AOS next steps. All AOS capabilities remain opt-in even in `--full`
mode — no hook is enabled until you set `enabled: true`.

### RAG Layer (`/search-ki-semantic` + `query-memory --semantic`)

Semantic search over the framework KI corpus using the LLM-as-retriever adapter.

**When to use**: when `/search-ki` returns empty for a concept you believe is documented.

```
> /search-ki-semantic circuit breaker pattern for external HTTP calls
```

Or from `query-memory`:

```
> /query-memory --semantic what do we know about graceful degradation
```

The `--semantic` flag loads the full KI+ADR corpus into context and judges relevance holistically —
it catches paraphrases and conceptual matches that lexical search misses. See
`shared/rag/README.md` for the three-corpus model and `shared/rag/retriever.interface.md` for the
adapter contract.

### Learning Engine (Hook: `on-retrospective-written`)

The learning engine is now wired as an opt-in hook that fires after a retrospective is written.

**To activate:**
```yaml
# .claude/hooks/on-retrospective-written.yaml  (seeded by --full install)
hooks:
  - id: "learning-engine-on-retrospective"
    event: "on-retrospective-written"
    enabled: true        # ← flip this
```

What happens: after a retrospective lands in `docs/features/<name>/retrospective.md`, the learning
engine scans it for promotable patterns and produces a draft proposal in
`.claude/feature-workspace/proposed-lessons.md`. It **always pauses for human confirmation** before
writing to `docs/lessons-learned/`. The `draftOnly: true` guardrail is non-negotiable.

To run the learning engine manually: `/learning-engine` or `> run the learning-engine skill`.

### Forgetting Engine (Scheduled Monthly)

```yaml
# .claude/hooks/scheduled-monthly.yaml  (seeded by --full install)
hooks:
  - id: "forgetting-engine-monthly"
    event: "scheduled-monthly"
    enabled: true        # ← flip this
    schedule:
      cron: "0 9 1 * *"   # first of every month
```

Requires a cron runner (GitHub Actions, `/schedule` skill, `CronCreate`). The forgetting engine
scans `shared/knowledge/` and `docs/lessons-learned/` for items that are ADR-superseded, temporally
stale (> 6 months, no links), or reference removed framework concepts. Produces a tiered proposal —
never archives without explicit human confirmation.

To run manually: `/forgetting-engine`.

### Orchestration Runtime (`/orchestrate`)

The orchestration runtime wraps existing skills to add resumable checkpoints, parallel branches, and
automatic audit-after-producer behavior.

**Your existing `/deliver-feature` calls work identically.** `/orchestrate` is the opt-in path.

```
/orchestrate --workflow feature-delivery --spec features/my-feature.md
```

```
```

`--legacy` flag routes to the pre-workflow skill directly if a regression is introduced:
```
/orchestrate --legacy --workflow feature-delivery --spec features/my-feature.md
```

See `shared/orchestration/README.md` for the runtime overview and `shared/orchestration/interface.md`
for the Workflow plug-in contract.

### FeatureDeliveryWorkflow

One built-in workflow is defined as part of Phase 3 (Op 3.11); a second, a Red-Green-Refactor loop
(Op 3.12), was retired with ADR-009 on 2026-09-22:

- `shared/workflows/feature-delivery-workflow.md` — wraps `deliver-feature`

It adds the **audit-after-producer** composition pattern (Op 3.13): every stage that produces a
contract-bound artifact automatically invokes its corresponding counter agent before proceeding.
This is enabled by default in the workflow but can be disabled per-project:

```yaml
# .claude/delivery-policy.yaml
workflowAuditsEnabled: false   # disable audit-after-producer in workflow stages
```

---

## Phase 4 (v3.3): Policy Layer (Coming Next)

Policy files enable auto-approval on documented safe paths — e.g., "if the diff is a pure refactor
AND all fitness functions pass AND diff < 200 LOC, auto-proceed past the code-reviewer gate."

See `docs/aos/migration-plan.md` Phase 4 section for the planned scope.

---

## Quick Reference

| Capability | How to try it | Config to persist it |
|---|---|---|
| Semantic KI search | `/search-ki-semantic <query>` | None — invoke on demand |
| Semantic all-memory search | `/query-memory --semantic <query>` | None |
| Learning engine (manual) | `/learning-engine` | None |
| Learning engine (on retrospective) | Enable in `.claude/hooks/on-retrospective-written.yaml` | `enabled: true` |
| Forgetting engine (manual) | `/forgetting-engine` | None |
| Forgetting engine (monthly) | Configure cron + `.claude/hooks/scheduled-monthly.yaml` | `enabled: true` + cron |
| Orchestration runtime | `/orchestrate --workflow feature-delivery --spec <file>` | None required |
| Audit-after-producer | Automatic in workflow stages | Disable: `workflowAuditsEnabled: false` |
| AOS layers seeded at install | `install.sh --full` | Once per project |

---

## Related

- `docs/aos/migration-plan.md` — the architectural plan this guide operationalizes
- `docs/aos/governance-pairs.md` — all 15 governance pairs mapped to agents/skills
- `shared/agents/CHANGELOG.md` — v3.0, v3.1, v3.2 entries
- `shared/orchestration/README.md` — orchestration runtime opt-in guide
- `shared/rag/README.md` — three-corpus retrieval model

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
