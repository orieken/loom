# Framework Improvement Prompts

Self-contained agent handoff prompts for framework-level improvements to `ai-assistant-dot-files`. Each file is a fully-briefed prompt a fresh Claude Code chat or subagent can execute standalone.

Separate from `docs/aos/prompts/` (AOS-migration-specific) — these are general framework improvements not tied to the AOS phased migration.

## How to use

1. Open a fresh Claude Code chat or spawn a subagent.
2. Copy the entire target prompt file into the message.
3. The agent has everything needed: repo path, prior state, scope, guardrails, commit discipline, escalation criteria, report format.

For tasks that require a human (not a fireable agent prompt), see [../human-tasks.md](../human-tasks.md).

## Active Prompts

### Framework roadmap — next items (2026-09-15)

| File | Scope | Estimated size |
|---|---|---|
| [loom-next-items-2026-09-15.md](loom-next-items-2026-09-15.md) | Handoff after L3.38–L3.46 shipped. Verified repo state, the credible unblocked items, the working method that produced that stream, this repo's verification discipline, and five loose threads. Includes the trap that ~49 items *look* open because older milestones predate the SHIPPED convention | Medium — pick one item per session |


### Documentation cleanup (from docs/TODO.md audit — 2026-08-15)

Run in order where dependencies exist: `docs-cleanup-legacy-install-decision.md` before
`docs-cleanup-move-blueprints.md`. All others are independent and can run in any order or in
parallel.

| File | Scope | Estimated size |
|---|---|---|
| ~~[docs-cleanup-fix-roadmap-links.md](done/docs-cleanup-fix-roadmap-links.md)~~ — shipped `dd78f1d` | ~~Fix 5 broken Epic 69–73 table links in `docs/roadmap-2026-08-07.md` (missing `prompts/` prefix)~~ | ✓ Done |
| ~~[docs-cleanup-architecture-counts.md](done/docs-cleanup-architecture-counts.md)~~ — shipped `0c0a6c2` | ~~Refresh agent/skill/platform counts in `docs/ARCHITECTURE.md` (currently says 25/56, live is 39/69)~~ | ✓ Done |
| ~~[docs-cleanup-onboarding-counts.md](done/docs-cleanup-onboarding-counts.md)~~ — shipped `fbd67ce` | ~~Verify and fix stale counts in `docs/ONBOARDING.md`; replace hardcoded numbers with inventory pointers~~ | ✓ Done |
| ~~[docs-cleanup-expand-readme-tree.md](done/docs-cleanup-expand-readme-tree.md)~~ — shipped `ae40762` | ~~Add missing entries to the `docs/README.md` directory tree (THREAT_MODEL, human-tasks, roadmap, TODO, aos/, prompts/, runbooks/)~~ | ✓ Done |
| ~~[docs-cleanup-reconcile-human-tasks.md](done/docs-cleanup-reconcile-human-tasks.md)~~ — shipped `ae40762` | ~~Remove resolved decisions from `docs/human-tasks.md`; isolate saturday-mcp external entries~~ | ✓ Done |
| ~~[docs-cleanup-contributing-gate-language.md](done/docs-cleanup-contributing-gate-language.md)~~ — shipped `ae40762` | ~~Fix Gate #7 misapplication in `docs/CONTRIBUTING.md` "Adding a new rule" section~~ | ✓ Done |
| ~~[docs-cleanup-consolidate-docs-claude.md](done/docs-cleanup-consolidate-docs-claude.md)~~ — shipped `6fa621f` | ~~Merge unique content into root `CLAUDE.md`, then remove the divergent docs copy~~ | ✓ Done |
| ~~[docs-cleanup-consolidate-runbooks.md](done/docs-cleanup-consolidate-runbooks.md)~~ — shipped `c18e37f` | ~~Consolidate `docs/RUNBOOKS.md` into `docs/runbooks/README.md`; update consumers~~ | ✓ Done |
| ~~[docs-cleanup-threat-model-status.md](done/docs-cleanup-threat-model-status.md)~~ — shipped `3e8b76d` | ~~Annotate `docs/THREAT_MODEL.md` with Op 2 implementation status; update "Op 1 of 2" banner and candidate labels~~ | ✓ Done |
| ~~[docs-cleanup-legacy-install-decision.md](done/docs-cleanup-legacy-install-decision.md)~~ — shipped `ab6f0be` | ~~Determine if extensionless `install`/`uninstall` scripts are still supported; document or retire the path; write ADR~~ | ✓ Done |
| ~~[docs-cleanup-move-blueprints.md](done/docs-cleanup-move-blueprints.md)~~ — shipped `05e345b` | ~~Move 3 root blueprint `.md` files to `docs/blueprints/`; update all consumers (run after legacy-install-decision)~~ | ✓ Done |
| ~~[docs-cleanup-extend-drift-check.md](done/docs-cleanup-extend-drift-check.md)~~ — shipped `cceabe2` | ~~Extend `scripts/check-inventory-drift.sh` to detect stale counts in ARCHITECTURE.md and ONBOARDING.md~~ | ✓ Done |

### Cross-project setup + maintenance patterns

| File | Scope | Estimated size |
|---|---|---|
| ~~[install-framework-with-mcp-bridge.md](done/install-framework-with-mcp-bridge.md)~~ — shipped `46c745f` | ~~Install framework into a target project **AND** bridge selected framework tools (analyze_complexity, check_accessibility, search_ki, search_docs, etc.) into that project's existing MCP server. Follows Option B (copy source, keep MCP sovereign, adjust paths + monorepo `packagePath` filter for the target's layout). Reusable pattern for projects with Saturday-like structure but not using Saturday itself~~ | ✓ Done |
| [update-installed-framework.md](update-installed-framework.md) | Propagate framework updates into an already-installed target project. Auto-detects which of 4 install patterns (traditional symlink / bridge-copy / saturday-mcp-adopted / generated platform configs) apply — often multiple — and runs the correct update sequence for each. Includes contract-drift check for previously-delivered artifacts | Medium — Phase A detect + Phase B update per pattern + Phase C contract-drift check; commit count depends on pattern mix |
| ~~[add-model-tier-abstraction.md](done/add-model-tier-abstraction.md)~~ — shipped `02efc68` | ~~Add a portable `model_tier:` field to agent frontmatter (`light` / `default` / `heavy`) + a central `shared/model-defaults.yaml` mapping tier → concrete model per platform. Install script resolves the tier for the target platform (or strips + warns when the platform doesn't honor per-agent model). Unlocks cost-tier optimization without breaking Cursor/Copilot/Gemini installs~~ | ✓ Done |

### Phase 10 Roadmap Epics (from `docs/audits/framework-gap-audit-2026-07-25.md`)

Ten Epic-numbered handoffs drafted 2026-07-26 from that audit. Recommended execution order below matches the audit's priority ranking.

| Epic | File | Estimated size |
|---|---|---|
| ~~**53**~~ | ~~[epic-53-inventory-drift-check.md](done/epic-53-inventory-drift-check.md)~~ — shipped `41d9c27` | ✓ Done |
| ~~**42**~~ | ~~[epic-42-roo-code-cline-platform.md](done/epic-42-roo-code-cline-platform.md)~~ — shipped `0e47b2f`→`523d4e1` | ✓ Done |
| ~~**44**~~ | ~~[epic-44-jetbrains-exporter.md](done/epic-44-jetbrains-exporter.md)~~ — shipped `d047e69`→`3e953df` | ✓ Done |
| ~~**46**~~ | ~~[epic-46-visual-qa-engineer.md](done/epic-46-visual-qa-engineer.md)~~ — shipped `472c961`→`5d28b74` | ✓ Done |
| ~~**48**~~ | ~~[epic-48-iac-conventions.md](done/epic-48-iac-conventions.md)~~ — shipped `aa954e2` | ✓ Done |
| ~~**49**~~ | ~~[epic-49-mobile-conventions.md](done/epic-49-mobile-conventions.md)~~ — shipped `aea8172`→`0bc5107` | ✓ Done |
| ~~**50**~~ | ~~[epic-50-rust-conventions.md](done/epic-50-rust-conventions.md)~~ — shipped `e67653a` | ✓ Done |
| ~~**51**~~ | ~~[epic-51-enterprise-memory-sync.md](done/epic-51-enterprise-memory-sync.md)~~ — shipped `146e3bb`→`69d6c3d` | ✓ Done |
| ~~**55**~~ | ~~[epic-55-agent-eval-expansion.md](done/epic-55-agent-eval-expansion.md)~~ — shipped batch 1 + batch 2 + Phase C | ✓ Done |
| ~~**57**~~ | ~~[epic-57-context-manifest-fixtures.md](done/epic-57-context-manifest-fixtures.md)~~ — shipped | ✓ Done |

**Not drafted** — per-epic dispositions live in `docs/audits/framework-gap-audit-2026-07-31.md` §F5 (the 07-25 audit itself carries no inline reasons): Epics 52, 54, and 56 are subsumed by AOS Phase 3 (`docs/aos/prompts/phase-3-runtime.md`) and must not be drafted separately. Epics 43, 45, 47, and 58 were drafted 2026-07-31 — see the table below.

### Remaining standalone epics (drafted 2026-07-31, from `framework-gap-audit-2026-07-31.md` §F5)

Recommended execution order matches the 07-31 audit's priority ranking (47 first, 43 last).

| Epic | File | Estimated size |
|---|---|---|
| ~~**47**~~ | ~~[epic-47-ship-feature.md](done/epic-47-ship-feature.md)~~ — shipped `58a429c`→`f94e5c0` (incl. tool-validator fix) | ✓ Done |
| ~~**58**~~ | ~~[epic-58-documentation-auditor-automation.md](done/epic-58-documentation-auditor-automation.md)~~ — shipped `98159fe`→`ba34794` | ✓ Done |
| ~~**45**~~ | ~~[epic-45-refactor-engineer.md](done/epic-45-refactor-engineer.md)~~ — shipped `64a7b66`→`f72960e` | ✓ Done |
| ~~**43**~~ | ~~[epic-43-mcp-exporter.md](done/epic-43-mcp-exporter.md)~~ — shipped `85c2c8f`→`7b739c7` | ✓ Done |

### Project-as-RAG optimization pair (drafted 2026-07-31, from retrieval-optimization discussion)

Exploits the fact that the pipeline *generates* the installed project's corpus — enrich at write
time so every retrieval tier (today's lexical, ADR-002's future BM25/vector) inherits a better
corpus. 59 is independent and highest-leverage; 60 is best run after it. Neither depends on AOS
Phase 3.

| Epic | File | Estimated size |
|---|---|---|
| ~~**59**~~ | ~~[epic-59-retrieval-write-time-enrichment.md](done/epic-59-retrieval-write-time-enrichment.md)~~ — shipped `716718b`→`436ba7d` | ✓ Done |
| ~~**60**~~ | ~~[epic-60-retrieval-index-freshness-eval.md](done/epic-60-retrieval-index-freshness-eval.md)~~ — shipped `c43702a`→`7dcb129` | ✓ Done |

### Structural-gap epics (drafted 2026-07-31, from `framework-gap-audit-2026-07-31.md` §3b)

Eight gaps from the same-day structural review — not backlog items but blind spots ("what is the
framework missing"). Table order IS the priority order.

| Epic | File | Estimated size |
|---|---|---|
| ~~**61**~~ | ~~[epic-61-prompt-regression-harness.md](done/epic-61-prompt-regression-harness.md)~~ — shipped `8d07c99`→`3cd67b8` | ✓ Done |
| ~~**62**~~ | ~~[epic-62-gate-decision-cost-telemetry.md](done/epic-62-gate-decision-cost-telemetry.md)~~ — shipped `2b7ed81`→`e5218c8` | ✓ Done |
| ~~**63**~~ | ~~[epic-63-parallel-delivery-isolation.md](done/epic-63-parallel-delivery-isolation.md)~~ — shipped `18b9fd1`→`77ad560` | ✓ Done |
| ~~**64**~~ | ~~[epic-64-shipped-linter-configs.md](done/epic-64-shipped-linter-configs.md)~~ — shipped `2eb31cc`→`c245bcb` | ✓ Done |
| ~~**65**~~ | ~~[epic-65-framework-threat-model.md](done/epic-65-framework-threat-model.md)~~ — shipped `839b610`→`2699b03` | ✓ Done |
| ~~**66**~~ | ~~[epic-66-capability-inventory-lifecycle.md](done/epic-66-capability-inventory-lifecycle.md)~~ — shipped `b95df04`→`2168505` | ✓ Done |
| ~~**67**~~ | ~~[epic-67-production-feedback-bugfix-path.md](done/epic-67-production-feedback-bugfix-path.md)~~ — shipped `d9b802c` | ✓ Done |
| ~~**68**~~ | ~~[epic-68-install-version-marker.md](done/epic-68-install-version-marker.md)~~ — shipped `6ddb307`→`d64cc2f` | ✓ Done |

### 2026-08-07 Roadmap Epics (from `docs/audits/framework-audit-2026-08-07.md`)

Five forward-looking epics from the August 2026 audit §3 High-Value Recommended Features.
All §5 action plan items were already shipped as of v3.3.14 — these are new work.
See `docs/roadmap-2026-08-07.md` for the full roadmap with priority rationale.

| Epic | File | Estimated size |
|---|---|---|
| **69** | [epic-69-mcp-skill-bundler.md](epic-69-mcp-skill-bundler.md) — auto-generate MCP tool wrappers from `SKILL.md` frontmatter; `check-mcp-drift.sh` fitness function | M |
| **70** | [epic-70-health-check-autofix.md](epic-70-health-check-autofix.md) — `health-check --fix` + `install.sh --auto-sync` non-interactive remediation for deterministic failures | M |
| **71** | [epic-71-pipeline-tui.md](epic-71-pipeline-tui.md) — Terminal UI (Go bubbletea or Python rich) visualizing stage progress, active agent, duration, token spend | L |
| **72** | [epic-72-multi-model-fallback.md](epic-72-multi-model-fallback.md) — provider fallback chains in `shared/model-defaults.yaml`; `resolve-model-tier.py` chain output | M |
| **73** | [epic-73-policy-engine-completion.md](epic-73-policy-engine-completion.md) — operationalize AOS Policy Engine: `evaluate-policy.sh` + `.claude/policies/` + deliver-feature gate wiring | M |
| **74** | [epic-74-loom-cli.md](epic-74-loom-cli.md) — `loom` Go CLI: embed `shared/` into a standalone binary distributed via Homebrew tap + Winget; replaces `install.sh` as primary user-facing install path | L |
| ~~**74-A**~~ | ~~[epic-74-loom-phase-a-scaffold.md](done/epic-74-loom-phase-a-scaffold.md)~~ — shipped `a112795` — Phase A: Go module scaffold, `//go:embed`, Cobra CLI with stub subcommands | ✓ Done |
| ~~**74-B**~~ | ~~[epic-74-loom-phase-b-install.md](done/epic-74-loom-phase-b-install.md)~~ — shipped `b671079` — Phase B: `loom install` with platform detection, stack filtering, manifest, dry-run | ✓ Done |
| ~~**74-C**~~ | ~~[epic-74-loom-phase-c-subcommands.md](done/epic-74-loom-phase-c-subcommands.md)~~ — shipped `0bf4b5f`→`3a0c7f9` — Phase C: `loom uninstall`, `loom version` (ldflags), `loom health` | ✓ Done |
| **74-D** | [epic-74-loom-phase-d-release.md](epic-74-loom-phase-d-release.md) — Phase D: goreleaser config, GitHub Actions release workflow, Homebrew tap formula | M |
| **74-E** | [epic-74-loom-phase-e-docs.md](epic-74-loom-phase-e-docs.md) — Phase E: README + ONBOARDING updated for loom install path; cmd/loom/README.md subcommand reference | S |

### Distribution & Adoption (from `docs/roadmaps/BUILD-ROADMAP.md` D.1–D.5, drafted 2026-08-29)

One phased epic prompt operationalizing the PLATFORM — Distribution & Adoption workstream: MCP as
the portable capability surface, maturity levels (L1–L4) as a first-class install concept. Phase A
is unblocked today; Phases B–E carry hard roadmap blockers the prompt instructs the agent to verify
before starting.

| Epic | File | Estimated size |
|---|---|---|
| **88** | [epic-88-l35-episodic-memory.md](epic-88-l35-episodic-memory.md) — L3.5: a project-local sqlite store of what runs actually did, ingested from run-events.jsonl rather than co-written by the executor, with a loom memory query surface. Adopts the data epics 84–87 already produce; adds no emitter and no retriever corpus. Unblocks L4.4 and L3.13 | L |
| **87** | [epic-87-l216-policy-evaluator.md](epic-87-l216-policy-evaluator.md) — L2.16: a policy evaluator that actually runs — typed Go over the declared schema, no expression language, with the always-human gate list compiled in and rejected at load time. Evaluates and records every decision but deliberately does **not** auto-approve: closing the audit gap first, earning the right to honor a decision later. Unblocks L4.3 | M |
| **86** | [epic-86-l39-event-vocabulary.md](epic-86-l39-event-vocabulary.md) — L3.9: one Go enum as the source of truth for event types, with the JSON Schema and docs table generated from it and a drift test. Retires events.jsonl and event-recorder.md, which specified fifteen event types and emitted none. The nine unemitted types become a documented list pointing at the items that would build them | M |
| **85** | [epic-85-l45-correction-signal.md](epic-85-l45-correction-signal.md) — L4.5: collect the edited_then_approved signal the schema has always specified and nothing has ever emitted. Retains the artifact state a human was shown at a gate, diffs it at approval, and attributes the correction to the producing agent. Covers the sequence the schema describes, which L2.14 structurally could not see. Blocks L4.4 | M |
| **84** | [epic-84-l38-opentelemetry.md](epic-84-l38-opentelemetry.md) — L3.8: OpenTelemetry from the executor and the MCP server, with GenAI semantic conventions. Real per-stage token counts and cost taken from the claude CLI's own JSON envelope rather than computed here; the run-events timeline stays as the audit record beside the traces. Last of the roadmap's three highest-leverage items; unblocks L3.5, L3.9, L4.5, and L4.3 once L2.16 lands (the entry's claim on L4.6 was wrong and is corrected) | XL |
| **83** | [epic-83-l29-implementation-chain.md](epic-83-l29-implementation-chain.md) — L2.9 second cut: type implementation-notes, security-report, and qa-report; make the two semantic rules those contracts already declare (`Failed: 0`, Critical findings carry a fix) typed invariants; replace the two `summarize-artifact` call sites that read the already-typed analysis. Unblocks L2.10, L2.11, L2.18 | L |
| **82** | [epic-82-l217-review-loop.md](epic-82-l217-review-loop.md) — L2.17: the developer↔code-reviewer iteration as a declared span in plan data with a named condition and a real bound; typed review verdict replacing the literal-string parse; every iteration retained and digested; exhausting the bound halts at a gate for a human rather than failing or waving through | L |
| **81** | [epic-81-l30-route-from-analysis.md](epic-81-l30-route-from-analysis.md) — L3.0: a re-plan point after the analyst computes the route from typed analysis, records it as an artifact before `confirm-design` (so the human approves the route and editing it resets the gate), and marks skipped stages in run state before the developer starts. Review stages are never auto-skipped. Unblocks L3.1 | M |
| **80** | [epic-80-l214-gates-reset-on-edit.md](epic-80-l214-gates-reset-on-edit.md) — L2.14: approvals bind to the digests of every completed stage; an edit after approval invalidates it and the gate halts again; identical re-runs keep the approval; `loom state verify` reports INVALIDATED for the markdown pipeline (detection, not enforcement). Unblocks L4.5 | M |
| **75** | [epic-75-distribution-adoption.md](epic-75-distribution-adoption.md) — Phase A: `loom mcp serve` (D.1, unblocked) · Phase B: public embedding package (D.2, needs M0.3+L2.4) · Phase C: `loom init --level N` profiles + rules-corpus split (D.3) · Phase D: `loom health` maturity report (D.4) · Phase E: capability tools on the MCP surface (D.5, per-tool blockers — `validate_artifact` shipped structural-only `c0e8441`; the other three tools await L2.12/L3.9/L2.16) | XL total; Phase A alone is M |

## Completed Prompts (`docs/prompts/done/`)

| File | Scope | Shipped |
|---|---|---|
| [done/epic-79-l29-typed-graph-state.md](done/epic-79-l29-typed-graph-state.md) | L2.9 first cut: `internal/state/` typed structs for the analyst → architect hop; JSON Schema generated into `shared/schemas/pipeline/` by `cmd/gen-schemas` (drift caught by test); `StateKind`/`Consumes` as plan data; field-level projections; schema inlined into the stage prompt; markdown rendered as a *view* under the contract's filename with full retrieval frontmatter; `RequiresArchitect()` replacing `deliver-feature`'s routing on a heading that never existed | Shipped 2026-08-31 (`74155f1`…`3430594`) |
| [done/epic-78-l212-executor-owns-state.md](done/epic-78-l212-executor-owns-state.md) | L2.12: artifact digests computed and re-verified in Go, stale stages re-run with a cascade; run identity in state; `loom state record/verify/approve/show/timeline` so the markdown pipeline stops hashing its own artifacts; append-only `run-events.jsonl` giving events and timing a real owner | Shipped 2026-08-31 (`8441ffc`…`7e574c9`) |
| [done/epic-77-l213-gates-as-interrupts.md](done/epic-77-l213-gates-as-interrupts.md) | L2.13: gates as pre-stage barriers in the executor — `confirm-design`/`confirm-security`/`confirm-ship`; approval only via TTY prompt or `--resume --approve`, exit code 3 when halted non-interactively; provider output can never self-approve (signature test); `approval-gates.md` "Executor Enforcement" section | Shipped 2026-08-30 (`868c281`…`a971b2f`) |
| [done/epic-76-m04-executor-skeleton.md](done/epic-76-m04-executor-skeleton.md) | M0.1 + M0.2 + M0.4: ADR-006 "loom executes pipelines"; CI lint job, SHA pinning, coverage ratchet; executor core in `internal/orchestrator/` with mock provider; `loom run` + claude subprocess provider | Shipped 2026-08-29 (`ba78f21`, `cab0156`) |
| [done/epic-67-production-feedback-bugfix-path.md](done/epic-67-production-feedback-bugfix-path.md) | `docs/incidents/README.md` + `five-whys`/`on-call` closing step persisting incident records (Candidate Records format); `extract-lessons` mines incident-feature pairs ("which stage should have caught this?"); `shared/memory-registry.json` registers `docs/incidents/`; `shared/skills/deliver-bugfix/SKILL.md` (157 lines, 5-phase thin pipeline); `docs/patterns/bugfix-workflow.md`; `new-feature` bug fast-path routing | Shipped 2026-08-04 (`v3.3.14`) |
| [done/add-model-tier-abstraction.md](done/add-model-tier-abstraction.md) | `model_tier:` field added to all agent frontmatter (`light` / `default` / `heavy`); `shared/model-defaults.yaml` tier→model mapping; `model-tier-auditor` counter-agent; installer tier resolution | Shipped 2026-07-29 (`02efc68`) |
| [done/install-framework-with-mcp-bridge.md](done/install-framework-with-mcp-bridge.md) | Install framework into a target project + bridge framework tools into the project's existing MCP server (Option B: copy source, keep MCP sovereign, adjust paths + monorepo `packagePath` filter) | Shipped 2026-07-28 (`46c745f`) |
| [done/epic-45-refactor-engineer.md](done/epic-45-refactor-engineer.md) | `shared/agents/refactor-engineer.md` v1.0.0 (standalone, model_tier: default); `shared/contracts/refactoring-contract.md`; `shared/templates/refactoring-notes.template.md`; golden-file fixture + `test-agents.sh` update; platform configs regenerated (38→39 agents); `AGENT_REFERENCE.md` agents 28–39 built out | Shipped 2026-08-02 (`v3.3.13`) |
| [done/epic-43-mcp-exporter.md](done/epic-43-mcp-exporter.md) | `shared/mcp/` Go module: scope ruling (Phase A), module skeleton + cmd/mcp-server, 4 analyzers + BM25 adapter, 6 M1 tools, server wiring, `install.sh --with-mcp` flag, `shared/mcp/README.md` usage guide | Shipped 2026-08-02 (`v3.3.12`) |
| [done/epic-61-prompt-regression-harness.md](done/epic-61-prompt-regression-harness.md) | `scripts/run-agent-evals.sh` (428 lines): pattern checks + LLM generation via `claude -p` + Haiku rubric judge + regression diffing; `shared/evaluation/agent-eval-harness-design.md` design doc (4 rulings: runner/judge/cadence/record); `shared/evaluation/agent-evals/README.md` schema + delta logic; opt-in CI job behind `secrets.ANTHROPIC_API_KEY`; `tests/agents/README.md` headless-path section | Shipped 2026-08-04 (`v3.3.11`) |
| [done/epic-63-parallel-delivery-isolation.md](done/epic-63-parallel-delivery-isolation.md) | Break the feature-workspace singleton: named workspace paths (`<feature-name>/` slug) in `deliver-feature`, `resume-pipeline`, `validate-artifact`, `context-engineer`, `deliver-atdd`, `orchestrate`; workspace path resolution + legacy detection; `docs/runbooks/parallel-delivery.md`; `docs/aos/parallel-delivery-isolation-design.md` status banner corrected | Shipped 2026-08-02 (`v3.3.8`) |
| [done/epic-60-retrieval-index-freshness-eval.md](done/epic-60-retrieval-index-freshness-eval.md) | Index-freshness hook pair documented in `shared/hooks/README.md` (escalation: reindex tool deferred to saturday-mcp M2); `retrievalBackend` enum FAIL check in `health-check.sh`; `scripts/generate-codemap.sh` + `CODEMAP.md`; `shared/evaluation/retrieval-regression.md` + `retrieval-evaluator.md` version bump | Shipped 2026-08-03 (`v3.3.10`) |
| [done/epic-59-retrieval-write-time-enrichment.md](done/epic-59-retrieval-write-time-enrichment.md) | 15 pipeline-artifact contracts + 15 templates with 7-field retrieval frontmatter (WARN-level in validate-artifact); domain-dictionary query expansion in search-ki + query-memory; `summarize-artifact --persist` writes `docs/features/<name>/summary.md` as a retrieval surrogate registered in memory-registry.json; `docs/patterns/artifact-citation-links.md` (141 lines) | Shipped 2026-08-03 (`v3.3.9`) |
| [done/epic-66-capability-inventory-lifecycle.md](done/epic-66-capability-inventory-lifecycle.md) | Deprecation convention (`status`/`superseded_by`) in agent + skill frontmatter contracts + JSON schemas; `health-check.sh` enforces broken pointer as FAIL; `forgetting-engine` extended with CI.1–CI.6 capability inventory audit mode; `complexity-check` deprecated → `analyze-complexity`; `docs/patterns/agent-skill-pair-convention.md` ruling (Delegation Wrapper vs. Composition Facade) | Shipped 2026-08-02 (`v3.3.7`) |
| [done/epic-65-framework-threat-model.md](done/epic-65-framework-threat-model.md) | `docs/THREAT_MODEL.md` (420 lines, STRIDE per trust boundary over install/sync/memory/hooks); `shared/rules/memory-trust-boundary.md` (KI/ADR is data not instructions, spec ingestion boundary); `sync_commit_sha` provenance on pulled KIs; hook example defaults to `enabled: false`; spec-ingestion caution block in analyst | Shipped 2026-08-02 (`v3.3.6`) |
| [done/epic-62-gate-decision-cost-telemetry.md](done/epic-62-gate-decision-cost-telemetry.md) | `gate_decision` event type (v1.1.0 schema): 3-way outcome enum, edit-detection heuristic, checksum comparison; opt-in emission in `deliver-feature` at gates 1/3/4/5/8; token spend fields in `pipeline-trace`; `extract-lessons` step 5 mines gate corrections (3+ occurrences → hypothesis) | Shipped 2026-08-02 (`v3.3.5`) |
| [done/epic-68-install-version-marker.md](done/epic-68-install-version-marker.md) | `framework-install.json` version marker written by `install.sh`; `health-check.sh` reports install-vs-upstream drift; `uninstall.sh` removes marker; `docs/MIGRATION.md` documents format + detection states | Shipped 2026-08-02 (`v3.3.1`) |
| [done/epic-64-shipped-linter-configs.md](done/epic-64-shipped-linter-configs.md) | `shared/configs/` with 7 language linter configs (golangci, ESLint, ruff, detekt, SwiftLint, clippy, Checkstyle); `install.sh --with-configs`; `scripts/check-cap-drift.sh` fitness function wired into `health-check.sh` | Shipped 2026-08-02 (`v3.3.3`) |
| [done/epic-58-documentation-auditor-automation.md](done/epic-58-documentation-auditor-automation.md) | Hook example (`on-inventory-change-doc-audit.yaml`); scheduler example in `scheduler/SKILL.md`; `health-check.sh` WARN at 14-day staleness; `documentation-auditor.md` output convention + `AGENT_REFERENCE.md` entry updated | Shipped 2026-08-01 (`v3.3.2` era) |
| [done/epic-47-ship-feature.md](done/epic-47-ship-feature.md) | `ship-feature` skill: branch creation, Conventional Commit gate (#2), PR creation gate (#5), optional `--release` mode delegating to `release-manager`; wired as opt-in step 43 of `deliver-feature` | Shipped 2026-08-01 (`v3.3.2`), fix `v3.3.4` |
| [done/epic-57-context-manifest-fixtures.md](done/epic-57-context-manifest-fixtures.md) | 3 hand-authored `tests/fixtures/context-manifests/` fixtures (passing, warning-no-cuts, missing-status); `check-context-budget.sh` rewritten to scan both real manifests and fixtures, make missing-Status a FAIL instead of SKIP, and document expected fixture outcomes | Shipped 2026-07-31 |
| [done/epic-55-agent-eval-expansion.md](done/epic-55-agent-eval-expansion.md) | Golden-file fixture expansion from 5/38 to 32/38 agents (84%). Batch 1: 14 pipeline agents (developer, sre-engineer, tech-writer, devops-engineer, performance-engineer, data-engineer, accessibility-engineer, visual-qa-engineer, context-engineer, spec-writer, product-owner, release-manager, test-driven-developer, unit-tester). Batch 2: 13 counter/auditor agents (agent-evaluator, context-auditor, documentation-auditor, documentation-manager, knowledge-auditor, memory-auditor, model-tier-auditor, pattern-reviewer, privacy-auditor, prompt-evaluator, retrieval-evaluator, rule-auditor, tool-validator). Phase C: `test-agents.sh` contract_for_agent() expanded to all 33 covered agents; `health-check.sh` fixture-coverage enforcement section added (FAIL when non-deferred agent lacks fixture directory) | Shipped 2026-07-31 |
| [done/epic-51-enterprise-memory-sync.md](done/epic-51-enterprise-memory-sync.md) | ADR-003 (separate git repo model) + `scripts/sync-memory.sh` (pull/push with --confirm gate) + `install.sh --sync-memory` flag + `memory-registry.json` enterpriseSync stanza + README workflow docs + `health-check.sh` Memory Sync section | Shipped 2026-07-31 in commits `146e3bb`→`69d6c3d` |
| [done/epic-50-rust-conventions.md](done/epic-50-rust-conventions.md) | Add `shared/rules/rust-conventions.md` covering Clean Architecture layer separation, cargo + clippy as CI fitness function, clippy::cognitive_complexity capped at 6, thiserror/anyhow error strategy, tokio async runtime, #![forbid(unsafe_code)], proptest + rstest + mockall + criterion | Shipped 2026-07-30 in commit `e67653a` |
| [done/epic-49-mobile-conventions.md](done/epic-49-mobile-conventions.md) | Add `shared/rules/swift-conventions.md` (SwiftUI + Swift Concurrency, SwiftLint `< 7`, XCTest + Nimble + swift-snapshot-testing) and `shared/rules/kotlin-conventions.md` (Jetpack Compose + Coroutines, detekt `< 7`, JUnit 5 + MockK + Paparazzi). No agent wiring today — mobile features will pick up on first use | Shipped 2026-07-30 in commits `aea8172`→`0bc5107` |
| [done/epic-48-iac-conventions.md](done/epic-48-iac-conventions.md) | Add `shared/rules/iac-conventions.md` covering Terraform/OpenTofu, Dockerfile, Kubernetes/Helm, and GitHub Actions guardrails. Wired into `devops-engineer.md` pre-read block. Propagated to all platform generated configs | Shipped 2026-07-30 in commit `aa954e2` |
| [done/epic-46-visual-qa-engineer.md](done/epic-46-visual-qa-engineer.md) | Add `visual-qa-engineer` pipeline agent. Wraps `@orieken/saturday-ml-analyzer` (interaction heatmap cold-spot analysis) and Playwright `toHaveScreenshot()` (visual regression). Conditional: UI features with `heatmap-data/` or snapshot baselines. Adds contract, template, pipeline slot after qa-engineer (steps 28-29), workflow diagram, validate-artifact mapping | Shipped 2026-07-30 in commits `472c961`→`5d28b74` |
| [done/epic-44-jetbrains-exporter.md](done/epic-44-jetbrains-exporter.md) | Add JetBrains AI Assistant + Junie as a single platform entry. Generates `.aiassistant/rules/` (10 files, IDE mode hints) and `.junie/guidelines.md`. Confirmed Junie already reads root `AGENTS.md` so partial support existed — this adds the AI Assistant project-rules layer and explicit Junie path | Shipped 2026-07-30 in commits `d047e69`→`3e953df` |
| [done/epic-42-roo-code-cline-platform.md](done/epic-42-roo-code-cline-platform.md) | Add Roo Code (custom modes via `.roomodes` YAML, 37 agents mapped) and Cline (`.clinerules/` directory, 10 path-scoped files) as platform targets. Registry entries, config generators, parity checks, install detection, README table — full Phase A+B implementation | Shipped 2026-07-30 in commits `0e47b2f`→`523d4e1` |
| [done/epic-53-inventory-drift-check.md](done/epic-53-inventory-drift-check.md) | Add `scripts/check-inventory-drift.sh` — counts actual `shared/` files and greps authoritative prose docs for stated counts; wired into `health-check.sh` as WARN-level. Corrected 5 pre-existing drifts in README and AGENT_REFERENCE | Shipped 2026-07-30 in commit `41d9c27` |
| [done/add-mcp-patterns-directory.md](done/add-mcp-patterns-directory.md) | Extract framework-generic MCP tool patterns (retrievers, analyzers, walkutil, 6 M1 tools, response structs, registration pattern) from `saturday-mcp` into `shared/mcp-patterns/`, then re-point the bridge/update prompts at that directory. Broke the accidental coupling that forced downstream users to also clone saturday-monorepo | Shipped 2026-07-27 in commits `39747a5` → `349281c` (Ops A → D3 + Op C follow-up) |
| [done/write-blog-posts.md](done/write-blog-posts.md) | Draft dev.to + LinkedIn blog posts covering recent framework + agent developments (Posts 05–10) | Shipped 2026-07-25 in commits `1abd94f` → `47dccce` |
| [done/implement-automated-delivery-tier-a.md](done/implement-automated-delivery-tier-a.md) | IMPLEMENT interim Tier A auto-proceed & Tier B contract retries in `/deliver-feature` skill based on `.claude/delivery-policy.yaml` | Shipped 2026-07-25 in commit `25b1a50` |
| [done/automate-deliver-feature.md](done/automate-deliver-feature.md) | DESIGN automated `/deliver-feature` workflow (Tier A/B/C classification, policy schema, interim automation, design doc + implementation handoff prompt) | Shipped 2026-07-25 in commits `04ea74d` → `4d13163` |
| [done/capture-session-history.md](done/capture-session-history.md) | Capture AOS foundations + frontmatter + MCP retrofit session history via `/retrospective` and `/extract-lessons` | Shipped 2026-07-25 in commits `e0a48e8` → `61a17e7` |
| [done/framework-hygiene-sweep.md](done/framework-hygiene-sweep.md) | Eight small pending items: commit `.gitignore` mod, track blog drafts, delete redundant AOS zip, add KI template, adopt `done/` convention, investigate audits doc, fix 2 pre-existing health-check WARNs | Shipped 2026-07-24 in commits `5a4a35d` → `9717fca` |
| [done/add-frontmatter-contracts.md](done/add-frontmatter-contracts.md) | Add `shared/contracts/agent-frontmatter-contract.md` + `skill-frontmatter-contract.md` + `ki-frontmatter-contract.md` so `validate-artifact` can grep-check these | Shipped 2026-07-22 in commits `ae1e440` → `38b14a9` |
| [done/add-frontmatter-json-schemas.md](done/add-frontmatter-json-schemas.md) | Add `shared/schemas/agent-frontmatter.schema.json` etc. Enables IDE autocomplete + enum-value validation. Wire into VS Code / Cursor settings templates | Shipped 2026-07-22 in commits `fdfdd9b` → `a522e14` |

## Convention

Every prompt in this directory:
- Names the target repo path (`/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`)
- Restates that this repo IS the git repo — commits land here directly, not in a parent
- Lists the exact ops in scope with clear file paths
- Enumerates guardrails (commit per op, conventional commits, `git add` explicit paths only)
- Names escalation criteria (when to stop and ask)
- Requests a specific report format

When a prompt is executed and shipped, move it to `docs/prompts/done/` and update the table above.
