# Architecture

This document explains how `shared/` becomes nine registered platform targets (covering ten named tools,
because the `jetbrains` entry covers both JetBrains AI Assistant and Junie), and how context flows through
the agent pipeline once a feature is being delivered. For "how do I add X," see
[CONTRIBUTING.md](CONTRIBUTING.md). For a narrative walkthrough, see the [README](../README.md). For what
each individual agent does and what checks its work, see [AGENT_REFERENCE.md](AGENT_REFERENCE.md).

---

## 1. The `shared/` Canonical Layer

Every agent, skill, and rule is authored exactly once, in `shared/`:

```
shared/
├── agents/          39 agents — .md, YAML frontmatter (name, description, tools, model, version)
├── skills/          69 skills — .md, YAML frontmatter (name, description, triggers)
├── rules/           architecture-guardrails.md, design-principles.md, approval-gates.md
├── contracts/       required-section contracts for pipeline agent handoffs (Epic 5)
├── knowledge/       portable Knowledge Items (KIs) — searchable via search-ki / query-memory
├── ARCHITECTURE_RULES.md
├── DOMAIN_DICTIONARY.md
├── TEAM_TOPOLOGY.md — Bounded Context -> team/type/interaction-mode registry (Skelton & Pais), checked by architect and team-topology-check
├── memory-registry.json — catalog of every durable memory source + retrieval backend, checked by search-ki/query-memory/memory-engineer
└── platform-registry.json
```

Nothing outside `shared/` is a source of truth. `.claude/agents/`, `.claude/skills/`, and `.claude/rules/`
are symlinks to their `shared/` equivalents (`ln -s ../shared/agents .claude/agents`, etc.) — editing through
the symlink and editing the canonical file are the same operation for Claude Code. Every other platform's
config is *generated*, not symlinked, because none of them can follow file references the way Claude Code
can (Cursor's own docs are explicit about this: `.mdc` rules must be fully self-contained).

### Why a single canonical layer
Before this structure existed, the same instructions were hand-copied into `.cursorrules`,
`copilot-instructions.md`, and `CLAUDE.md` independently, and they drifted every time one was edited without
the others. `scripts/check-parity.sh` exists specifically to catch that drift — it diffs every generated
config against `shared/` and fails if any platform is missing an agent, a rule concept, or has fallen out of
sync.

---

## 2. The Capability Tier System

Not every AI tool can do the same things. `shared/platform-registry.json` classifies each platform into one
of three tiers, and `DOMAIN_DICTIONARY.md` defines exactly what each tier means:

| Tier | Label | Capabilities | Platforms | Terminology |
|---|---|---|---|---|
| **1** | Full | Agents with tool access, autonomous multi-step process, pipeline participation, hooks | Claude Code | **Agent** |
| **2** | Personas + Rules | Persona-level context shaping + rule files, no native orchestration | Windsurf, GitHub Copilot | **Persona** |
| **2*** | Personas + Rules, agents/skills now Tier-1-equivalent | Real subagent + skill loading (confirmed 2026-07-06) via `.cursor/agents/`/`.cursor/skills/`, plus rule files (still inlined, no orchestration at the rules layer) | Cursor | **Agent** for `.cursor/agents/`, **Persona** for `.cursor/rules/` |
| **3** | System Prompt (rules); real skill invocation confirmed on top | Single rules file (`AGENTS.md`), plus genuine skill execution — not just description | Gemini/Antigravity, OpenAI/Codex | **Persona** |

Cursor is the one platform whose tier number alone no longer tells the whole story: it shipped native
Agent Skills (`.cursor/skills/*/SKILL.md`) and subagents (`.cursor/agents/*.md`) using the same open
standard `shared/agents/`/`shared/skills/` already follow (confirmed 2026-07-06 against
`cursor.com/docs/subagents`, `cursor.com/docs/skills`, and a live check in this repo — the analyst subagent
and search-ki skill both loaded and behaved correctly, not just generically). `install.sh` now symlinks
`.cursor/agents/`/`.cursor/skills/` directly, the same zero-drift mechanism Claude Code has always used —
see `shared/platform-registry.json`'s `capabilities` object, which is what's actually authoritative per
capability now, not the single `tier` number. Cursor Rules (`.mdc`) are unaffected by this — still fully
inlined, still no orchestration at that layer, still genuinely Tier 2. One real, permanent capability gap
versus Claude Code: Cursor subagents have no `tools:` allowlist field — they inherit *all* of the parent's
tools (MCP tools included) with only a coarse `readonly: true/false` available, so `shared/agents/*.md`'s
`tools:` frontmatter is simply ignored by Cursor's parser.

Copilot moved from Tier 3 to Tier 2 in 2026-07 after confirming (via GitHub's own docs) that it supports
path-scoped `.github/instructions/*.instructions.md` files alongside the repo-wide instructions file — the
same "multiple rule files, no orchestration" shape Windsurf already had.

Gemini/Antigravity was live-tested 2026-07-02 (see `tests/platform-verification/antigravity.md` and its
results file) rather than left on secondary-source guesswork: it reads `AGENTS.md` for rules (confirmed —
asking it to list approval gates returned an exact match against `shared/rules/approval-gates.md`), and it
genuinely *invokes* skills rather than just describing them (asking it to run `complexity-check` against a
fixture correctly applied the real thresholds). The framework's previous best guess —
`.gemini/antigravity/instructions.md` — was confirmed **not** read at all and has been removed. Skills
loaded from `~/.gemini/config/skills/` (the global root) in this test since project-level `.agents/skills/`
didn't exist yet at session start; that project-level path itself remains unconfirmed, not contradicted.

The distinction matters because it's enforced in the generated output, not just documented: `Persona` is a
context frame with no tool access and no autonomous workflow (see `DOMAIN_DICTIONARY.md`'s Entity table) —
Tier 2/3 configs consistently say "persona" in generated roster text (`collect_agent_roster()` in
`scripts/generate-configs.sh`), never "agent." Only Tier 1 gets the word "agent," because only Tier 1
actually runs multi-step orchestration with tool access.

### Generation strategy per tier
- **Tier 1 (Claude Code)**: symlink. `install.sh` creates `.claude/{agents,rules,skills}` -> `shared/`
  equivalents. Always current after a `git pull`; no generation step needed.
- **Cursor (mixed strategy)**: symlink for agents/skills, generate-inline for rules.
  - `install.sh`'s `install_cursor()` symlinks `.cursor/agents` -> `shared/agents` and `.cursor/skills` ->
    `shared/skills` directly — same zero-drift mechanism as Tier 1, confirmed working 2026-07-06. This
    retired the earlier `generate_cursor_personas()` workaround (built in Epic 11, before Cursor could do
    real skill/agent loading), which used to flatten each agent into a standalone `.cursor/rules/<name>.mdc`
    persona file.
  - Rules still can't follow file references (no evidence Cursor Rules support them, unlike agents/skills),
    so `generate-configs.sh` still generates 11 `.mdc` files for rules: `architecture.mdc`,
    `design-principles.mdc`, `agent-roster.mdc`, `approval-gates.mdc`, `testing.mdc`, `go-backend.mdc`,
    `vue-frontend.mdc`, plus `typescript-conventions.mdc`, `python-conventions.mdc`,
    `csharp-conventions.mdc`, `java-conventions.mdc` (Epic 31, 2026-07-07 — preferred packages/structure
    per language, sourced from `shared/rules/<language>-conventions.md`). `approval-gates.mdc` and
    `agent-roster.mdc` are the only `alwaysApply: true` files — the rest Auto Attach on a language-specific
    or broad source-file glob instead, since combined they'd otherwise blow well past Cursor's own
    recommended ~2,000-token always-apply budget. Cursor silently ignores a `.mdc` file with invalid
    frontmatter, so `generate_mdc()` is careful about exact YAML shape.
- **Tier 2 (Windsurf, GitHub Copilot)**: generate-inline, multi-file.
  - Windsurf gets one flat `.windsurfrules` (no per-file globs support in the legacy format).
  - Copilot gets the Tier-3-style `copilot-instructions.md` (roster + rules inlined) **plus** the same
    7 scoped `.github/instructions/*.instructions.md` files as Cursor's non-agent-roster `.mdc` set
    (`testing`, `go-backend`, `vue-frontend`, `typescript-conventions`, `python-conventions`,
    `csharp-conventions`, `java-conventions`), each with an `applyTo` frontmatter field (comma-separated
    glob string, not an array like Cursor's `globs`) — both coexist and combine per GitHub's docs.
- **Tier 3 (OpenAI)**: generate-inline, single file. `generate_tier3()` concatenates rules + craftsmanship
  section + persona roster into one instruction file.
- **Gemini/Antigravity**: generates root `AGENTS.md` only (the [agents.md](https://agents.md) cross-tool
  convention — confirmed read, see above). `install.sh` symlinks `shared/skills/` to
  `~/.gemini/config/skills/` on a `--global` install (confirmed global skills root) or to
  `.agents/skills/`/`shared/rules/` to `.agents/rules/` on a `--project` install (documented project-scope
  convention, not yet directly exercised by testing).

---

## 3. Context Flow (Six-Layer Taxonomy)

From `docs/runbooks/context-engineering.md`, the taxonomy every agent operates within, ordered from
permanent/static to ephemeral/dynamic:

| Layer | Name | Source | Lifetime |
|---|---|---|---|
| 1 | System Context | `CLAUDE.md`, `.cursorrules` | Session-long |
| 2 | Rule Context | `ARCHITECTURE_RULES.md`, `shared/rules/` | Session-long |
| 3 | Knowledge Context | `shared/knowledge/` KIs, `docs/adrs/` | Demand-driven |
| 4 | Task/Goal Context | Feature spec, `analysis.md` | Task-long |
| 5 | Historical Context | Thread history, `docs/features/` | Ephemeral |
| 6 | Runtime Context | Open files, tool outputs | Real-time |

`context-engineer` is the agent responsible for keeping Layers 3-6 high-signal before the rest of the
pipeline starts: it maps the task to a Bounded Context, auto-prunes files from unrelated contexts (Epic 17),
searches Layer 3 via `search-ki` before letting `analyst` reason independently (Proactive RAG), and estimates
a token budget per pipeline-agent tier (Analyst/Architect ≤60%, Developer ≤80%, Reviewers ≤40% of a
200k-token window).

### Context decay
An artifact 2+ pipeline phases old is never read in full — e.g. `qa-engineer` and `tech-writer` (Phase 3)
get part of `analysis.md` (Phase 1), not its full text, since `implementation-notes.md` already restates what
matters for their job. *Which* part depends on the pipeline: under `loom run` the analysis is typed state and
each stage receives a deterministic projection of the fields its contract declares (roadmap L2.10); under the
markdown pipeline each agent reads only its relevant sections, and `summarize-artifact` covers the eight
artifacts that are still untyped. This never applies to the artifact an agent is *immediately* reviewing.

### Subagent isolation
Spawning a subagent is a clean-slate context — it sees only its own definition plus the specific
artifact/task handed to it, never the orchestrator's full conversation history or other subagents' internal
reasoning. The orchestrator (`deliver-feature`) only ever consumes a subagent's final structured report. This
is why `shared/contracts/` exists: the report *is* the entire interface between agents, so its shape has to
be both complete and predictable.

---

## 4. The Pipeline's Own Observability

The pipeline is *specified* to instrument itself the way you'd instrument a production system.
Honest status first, artifact by artifact — some of this is now measured, and the rest is still a
model's own account of itself:

- **Measured** (written by the Go binary, timestamps from the clock): `run-state.json`,
  `run-events.jsonl`, and `traces.jsonl`. Stage transitions, gate decisions, and artifact digests
  are recorded by `loom run`, or by the `loom state` subcommands the markdown pipeline calls (M0.4,
  L2.13, L2.12). Stage durations are measured by the process doing the work (L3.8), not derived
  after the fact.
- **Reported Usage** (stated by the provider, recorded verbatim): per-stage token counts and dollar
  cost, taken from the claude CLI's own result envelope (L3.8). Loom computes no prices — a table in this
  repository would be wrong within a quarter while sounding authoritative.
- **Estimated** (written by the host platform's LLM following prompt instructions):
  `pipeline-trace.json`'s `budgetUtilization` and iteration counts, the scorecards, and the
  lessons-learned corpus. A duration recalled by a model is not a measurement.

OpenTelemetry emission has landed (L3.8): a run produces one trace — a root span, a child per
stage, a grandchild per model call carrying `gen_ai.*` usage — exported to a local OTLP/JSON file by
default and to a collector when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. `run-events.jsonl` remains a
separate thing on purpose: it is the audit log of gates, digests and staleness, and it stays
readable with no collector configured, which is a property an audit record needs and a trace does
not. See `docs/roadmaps/BUILD-ROADMAP.md` and ADR-006. The artifact set:

- **`run-events.jsonl`** (per feature, beside run state) — append-only event log written by the Go
  binary from both pipelines: stage transitions, gate halts and approvals, and integrity failures,
  each with a real timestamp. Read with `loom state timeline`. Its event types come from one Go
  enum, with the JSON Schema and the documentation table generated from it and a test that fails on
  drift (L3.9) — the framework previously kept two hand-maintained lists of event types and a
  recorder instructed to enforce one of them, which is how 60% of the specified surface ended up
  outside it.
- **Episodic store** (`.claude/memory/episodes.db`, project-local) — what past runs did, kept beyond
  the life of a feature workspace and queried with `loom memory` (L3.5). It collects nothing new:
  timings, loop rounds, gate halts, corrections, cost and routing reasons were already recorded per
  run and simply died with the workspace. A projection, not the record — `run-state.json` and
  `run-events.jsonl` are archived into `docs/features/<name>/`, so the history is in git and the
  store is rebuildable.
- **Policy Decisions** (in `run-state.json` and on the timeline) — what the policies watching each
  gate decided, evaluated in typed Go from the run's own state (L2.16). Recorded, not honoured: the
  executor halts at every gate regardless, so these say what *would* have happened. An authorization
  decision used to be resolved by a model reading YAML, with the kill-switch and the always-human
  list in the same prose it might misread.
- **Human Corrections** (in `run-state.json` and on the timeline, diffs under
  `.approved/<gate>/corrections/`) — what a person changed at a gate, attributed to the agent that
  produced it (L4.5). The framework has specified this as its highest-value learning signal since
  v3.0 and collected none of it until the executor could measure it. It is taken from the rendered
  view, which the pipeline does not read back, so a Human Correction is recorded rather than adopted.
- **`traces.jsonl`** (per feature, beside run state) — the Run Trace in OTLP/JSON,
  one complete request body per line, so a saved file replays into a collector unmodified (L3.8).
  Written by `loom run` unless `--no-telemetry` is passed. Token counts and cost also land on each
  stage in `run-state.json`, so `loom state show` answers what a run cost without a collector.
  `loom mcp serve` traces tool calls too, but writes no file — it is spawned by a host application
  and has no run to scope one to.
- **`pipeline-state.json`** (per feature, in `.claude/feature-workspace/`, persisted to `docs/features/<name>/`)
  — resumability: current phase, completed agents, artifact checksums. `resume-pipeline` reads this.
  Ownership has moved to the Go executor (L2.12). Under `loom run` the executor keeps its own
  `run-state.json`, hashes artifacts itself, and re-verifies them on resume. The markdown pipeline
  now records its checkpoints through `loom state` too — `deliver-feature` checks for the binary at
  Phase 0 — so this hand-written file is the explicit **fallback** for hosts without `loom`
  installed, not the default path.
- **`pipeline-trace.json`** (same location) — timing, status, iteration counts, and `budgetUtilization` per
  agent. `pipeline-trace` (single run) and `pipeline-retrospective` (cross-delivery trends) read this.
  Its timings remain model-written estimates. Measured wall-clock timing now exists alongside it in
  `run-events.jsonl` (L2.12) and in the trace (L3.8); folding this file into them is L3.5's episodic
  store, not a docs change.
- **`docs/agent-metrics/scorecard-YYYY-MM.md`** — monthly quality scores per agent (security TPR proxy,
  code-reviewer first-pass acceptance, analyst completeness, architect fitness-function coverage),
  trend-compared month over month.
- **`docs/lessons-learned/`** — cross-delivery pattern extraction; recurring findings get *drafted* as rule
  or prompt changes, never auto-applied (see `.claude/rules/approval-gates.md` Gate #7 — a rule change
  always requires explicit human sign-off).

---

## 5. Versioning and Testing the Agents Themselves

Agents are prompts, but prompts are code here: every agent has a `version:` field
(`shared/agents/CHANGELOG.md` tracks history), a pre-commit hook (`scripts/hooks/pre-commit`, opt-in) that
requires a version bump + changelog entry for any behavior change, and golden-file structural tests
(`tests/agents/`, run via `scripts/test-agents.sh`) for the five agents most likely to regress silently.
See [docs/runbooks/editing-agent-prompts.md](runbooks/editing-agent-prompts.md) for the full workflow.

---

## Directory Reference

```
shared/                          canonical source — see section 1
internal/
  orchestrator/                  executor (ADR-006, M0.4 + L2.13 + L2.12): plan, run loop,
                                   durable run-state.json (atomic writes), approval gates as
                                   process interrupts — a gated stage cannot start until run
                                   state records a human approval, and approvals bind to the
                                   digests they were given so an edit resets the gate — plus
                                   artifact digests
                                   computed and re-verified in Go, so an edited artifact stops
                                   counting as completed work, plus an append-only
                                   run-events.jsonl audit timeline written by both pipelines,
                                   a router that decides which stages a run needs from
                                   the typed analysis before the design gate (L3.0), and a
                                   bounded review loop the executor iterates and counts
                                   rather than a model repeating on its own output (L2.17)
  state/                         typed pipeline state (ADR-006, L2.9 first cut): Go structs for
                                   the analyst -> architect hop, JSON Schema generated from them
                                   into shared/schemas/pipeline/, field-level projections, and
                                   the markdown renderers that turn state back into a view
  provider/mock/                 deterministic scripted Provider for executor tests
scripts/
  generate-configs.sh            shared/ -> nine registered platform targets
  check-parity.sh                fitness function: configs match shared/
  test-agents.sh                 golden-file structural tests for agent prompts
  check-context-budget.sh        fitness function: no WARNING manifest without cut recommendations
  health-check.sh                symlinks, frontmatter, drift, contracts, changelog, KI validity
  check-agent-versions-ci.sh     CI equivalent of hooks/pre-commit (base-branch vs. PR-head, not staged vs. HEAD)
  hooks/pre-commit                opt-in: agent version bump + changelog gate
install.sh / uninstall.sh        --global | --project <path>, --copy, --platform, --dry-run
Makefile                          install, uninstall, generate, check, test-agents, health
.github/workflows/framework-ci.yml  check-parity, test-agents, health-check, agent-versions (PRs only)
tests/agents/                    fixtures + expected patterns for golden-file tests
docs/
  features/<name>/                every delivered feature's full pipeline artifact set
  adrs/                           Architecture Decision Records
  agent-metrics/                  monthly agent quality scorecards
  pipeline-retrospectives/        cross-delivery timing/iteration trend reports
  lessons-learned/                cross-delivery pattern extraction
  runbooks/                       operational guides
                                    parallel-delivery.md — running multiple features concurrently
```
