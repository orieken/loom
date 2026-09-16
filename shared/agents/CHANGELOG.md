# Agent Changelog

Tracks version bumps for every agent in `shared/agents/`. Every prompt edit that changes agent *behavior*
(not just a typo or formatting fix) requires a version bump here, enforced by the pre-commit hook in
`scripts/hooks/pre-commit` (see that file's header comment for how to enable it — it's opt-in, not wired up
automatically for you).

## Versioning
Semantic-ish, not strict SemVer:
- **Patch** (1.0.x): wording/clarity fixes that don't change behavior.
- **Minor** (1.x.0): new process step, new output section, expanded guardrail — additive, backward compatible.
- **Major** (x.0.0): changed output contract (update the matching file in `shared/contracts/` too if one
  exists), removed/renamed a process step, or changed tool access.

## How to add an entry
When you bump an agent's `version:` frontmatter field, add a row under a new dated heading here in the same
commit — the pre-commit hook checks for exactly this.

## 2026-09-16 — a characterization net says what it does not cover

| Agent | Version | Change |
|---|---|---|
| unit-tester | 1.5.0 -> 1.6.0 | Minor: step 8 now requires a scope note **in the test file** in characterization mode — records-behavior-as-of date, an explicit `NOT COVERED` list (inputs never tried, env/locale/clock dependencies, unobserved side effects, and that correctness was never asserted), and the three behavior-carrying lines. Not required in coverage-backfill mode, where "does not encode intent" would be false. New `## Scope Note` section in the report points at where the note lives |

Two reasons it goes in the test file rather than the report. `.claude/feature-workspace/` is
gitignored, so a note written only to the report is deleted with the workspace and the net ships with
nothing recording its limits — the reader who needs it is looking at the test months later, not at the
report today. And step 8 already required naming three behavior-carrying lines while the output
template had **no section for them**: the instruction was orphaned, and relocating it to the scope note
gives it a home that survives.

Enforcement is `judgment-only` with a reason, marked in the agent under the
PRECONDITION / ENFORCED-BY convention: `health-check` runs against this repository and cannot see test
files in the projects this agent writes for, so there is nothing here to assert it against.

Found by the alignment audit (`docs/prompts/loom-alignment-todo-2026-09-16.md`, item `A5`) against
Level 2B's characterization scope note.

## 2026-09-15 — a mutant that never landed is not a passing test

| Agent | Version | Change |
|---|---|---|
| exemplar-auditor | 1.1.0 -> 1.2.0 | Minor: step 5 must now say which standard its vacuity answer meets. A YES reasoned from control flow is weaker than one demonstrated by mutation, and for the one test every later test is copied from that difference is worth stating rather than smoothing over. With no mutation evidence, recommend `backfill-unit-tests` step 6 instead of presenting the reasoned answer as settled |

`backfill-unit-tests` step 6 and `exemplar-contract.md`'s second disqualifier gained the same fix:
confirm the mutant landed (non-empty diff) before trusting a survival, and mutate what an assertion
depends on. Twice in two days a mutation failed to apply and the passing test read exactly like an
uncovered line.

## 2026-09-15 — the exemplar audit gets a cadence

| Agent | Version | Change |
|---|---|---|
| exemplar-auditor | 1.0.0 -> 1.1.0 | Minor: step 7 splits its output by how it was invoked. Ad hoc still writes `.claude/feature-workspace/exemplar-audit.md`; a scheduled run writes `docs/audits/exemplar-audit-YYYY-MM-DD.md`, dated and never overwritten, because an audit whose output is replaced each run cannot be told stale from fresh. `health-check` reads that date |

Wired into `shared/hooks/scheduled-monthly.yaml` as `exemplar-auditor-monthly`, disabled by default
like every entry in that file. Exemplars are deliberately not gated (L3.46), so this audit is the
only thing that notices a degraded one.

## 2026-09-14 — a suite has health numbers, not just a coverage number (roadmap L3.42)

New `docs/patterns/test-suite-health-metrics.md` and `shared/knowledge/flake-triage-taxonomy.md`.
Coverage is the one number that cannot detect what this framework's agents can produce: repairing a
vacuous test moves it by zero, retiring a dead test moves it down, and deleting an assertion leaves
the line covered.

| Agent | Version | Change |
|---|---|---|
| dx-engineer | 1.2.0 -> 1.3.0 | Minor: step 3 now says to cluster flaky tests by **cause** rather than by test, pointing at the taxonomy KI for the four categories and the evidence that separates them, and at the metrics pattern for the numbers that say whether triage worked. It is the only agent bound: these are suite-over-time numbers a lead reads, and `qa-engineer` works per feature |

The KI records a deliberate divergence from the source material rather than applying it silently:
the wider practice treats quarantine as a disposition an engineer applies, and here it is approval
gate #9 — an agent proposes, a human applies.

## 2026-09-14 — exemplar tests: the pattern an agent copies (roadmap L3.40)

New contract `shared/contracts/exemplar-contract.md`, manifest `.claude/exemplars.yaml`, and the
`test-exemplars` memory-registry source. Agents here write tests constantly and learned the house
pattern from prose; nothing showed them one good test. The framework already did this for its own
agents — `memory-auditor` is "the pattern exemplar every new counter agent should follow" — and had
never done it for tests.

| Agent | Version | Change |
|---|---|---|
| exemplar-auditor | — -> 1.0.0 | **New.** Thirteenth read-only counter agent. Audits each declared exemplar against the contract: still exists and runs, still annotated, digest still confirmed, and still demonstrates what it claims. The judgement is test craftsmanship, which `memory-auditor` has none of — a sweep checking schema and duplicates would pass a beautifully-registered bad test |
| developer | 1.2.0 -> 1.3.0 | Minor: consult the exemplar for the language and level before writing a test |
| qa-engineer | 1.5.0 -> 1.6.0 | Minor: same |
| dx-engineer | 1.1.0 -> 1.2.0 | Minor: same |
| refactor-engineer | 1.2.0 -> 1.3.0 | Minor: same |
| test-driven-developer | 1.4.0 -> 1.5.0 | Minor: same |
| unit-tester | 1.4.0 -> 1.5.0 | Minor: same |
| api-test-generator | 1.1.0 -> 1.2.0 | Minor: same |
| visual-qa-engineer | 1.1.0 -> 1.2.0 | Minor: same |

Not "golden": `DOMAIN_DICTIONARY.md` reserves that term for the structural check over
`tests/agents/*/actual-output.md`, and golden carries the golden-master sense — a recorded baseline
you compare *against*, the opposite instruction to *copy this*.

## 2026-09-13 — every test-writing agent is bound by the test repair contract (roadmap L3.39)

New rule `shared/rules/test-repair-contract.md` and approval gate #9 (Removing Test Coverage). Before
this, nothing in the framework stopped an agent making a suite green by deleting the assertion that
was failing — the only counter-language was one line in `qa-engineer`, binding one agent, enforced by
nothing.

| Agent | Version | Change |
|---|---|---|
| dx-engineer | 1.0.1 -> 1.1.0 | Minor, and the reason this item existed: step 4 said "Quarantine flaky tests", with Write and Edit and no gate — an unsupervised one-way door on regression signal. It now **proposes** a quarantine carrying owner, expiry, cause and resolving evidence, and a human applies it at gate #9. Also names the four flake categories, because a category-A flake is a product defect the test correctly found and quarantining it hides a real bug |
| developer | 1.1.0 -> 1.2.0 | Minor: bound by the test repair contract |
| qa-engineer | 1.4.0 -> 1.5.0 | Minor: bound by the test repair contract. Its existing "never skip a test just to make the suite green" line now has a rule behind it instead of standing alone |
| refactor-engineer | 1.1.0 -> 1.2.0 | Minor: bound by the test repair contract — a refactor that changes a test file has moved the net it was being verified against |
| test-driven-developer | 1.3.0 -> 1.4.0 | Minor: bound by the test repair contract |
| unit-tester | 1.3.0 -> 1.4.0 | Minor: bound by the test repair contract |
| api-test-generator | 1.0.1 -> 1.1.0 | Minor: bound by the test repair contract |
| visual-qa-engineer | 1.0.0 -> 1.1.0 | Minor: bound by the test repair contract |

## 2026-09-13 — the reviewer asks whether a test would fail (roadmap L3.41)

| Agent | Version | Change |
|---|---|---|
| code-reviewer | 1.1.0 -> 1.2.0 | Minor: new process step 7 and a **Test Evidence** criterion. For every test added or modified in the diff, answers "would this test fail if the behavior its name claims were broken?" as YES / NO / UNCERTAIN under the existing `## Test Design Review` heading. A `NO` names one of four types (vacuous, self-fulfilling, unreached, disabled) and requests changes; `UNCERTAIN` is advisory and never blocks, because a real assertion inside an unread custom matcher looks vacuous and a mock configured in a distant `beforeEach` looks legitimate. Scope is the diff's own tests, never a suite sweep. No contract change — the heading already existed and specified nothing |
| unit-tester | 1.2.0 -> 1.3.0 | Minor: a characterization net is not finished until mutation proves it. Names the stopping condition and points at `backfill-unit-tests`, which runs the mutation in a throwaway git worktree — this agent's "never modify source, full stop" rule is unchanged and stays absolute |

## 2026-08-31 — analysis reaches QA and tech-writer as a projection, not a summary

| Agent | Version | Change |
|---|---|---|
| qa-engineer | 1.3.0 -> 1.4.0 | Step 2 no longer calls `summarize-artifact` on `analysis.md`. Under `loom run` the acceptance criteria, edge cases, QA tasks, and definition of done arrive as a projection of typed analysis state — selected fields rather than a ~200-word LLM summary, so nothing is paraphrased or silently dropped. Without the executor, read the two relevant sections directly instead of the whole file (roadmap L2.9 / L2.10) |
| tech-writer | 1.2.0 -> 1.3.0 | Step 1 same change: feature summary, out-of-scope list, and its own task list arrive as a projection instead of a summary |

## 2026-08-20 — context-engineer wired as mandatory Step 0 in all delivery and refactoring pipelines

| Agent / Skill | Version | Change |
|---|---|---|
| refactor-engineer | 1.0.0 → 1.1.0 | Minor: context-engineer is now Step 0 of Phase 0. Invoked before scope baseline, produces `context-manifest.md` that pins the refactoring target files and surfaces ADR constraints and prior retrospective lessons. Guardrail added: never proceed to step 1 without `context-manifest.md`. Standalone escape hatch: note "context-debt" in `refactoring-notes.md` if context-engineer cannot run. |
| deliver-atdd | — | Step 6 added in Phase 0: invoke context-engineer → `context-manifest.md`; all subsequent steps renumbered 6–16 → 7–17. Phase 0 row added to delivery-summary table. Guardrail added: never skip context-engineer before qa-engineer writes scenarios. |
| deliver-bugfix | — | Phase 0 section added before Phase 1: invoke context-engineer → `context-manifest.md`. Comparison table row updated from Skipped → Required. `context-manifest.md` added to output artifacts. Guardrail added with context-debt fallback. |
| orchestrate | — | Validate-and-Load step 4 added: halts any `feature-delivery` or `refactoring` workflow whose first executable stage is not `context-engineer`. Guardrail added: cannot bypass via `--legacy`. |

## 2026-08-15 — Canonical runbook index

| Agent | Version | Change |
|---|---|---|
| documentation-manager | 2.0.1 -> 2.0.2 | Patch: living-document updates now target the canonical `docs/runbooks/README.md` index; `docs/RUNBOOKS.md` is a compatibility redirect. |

## 2026-08-04 — Epic 60: Retrieval Regression Set

| Agent | Version | Change |
|---|---|---|
| retrieval-evaluator | 1.1.0 | Added regression set runner: reads approved cases from shared/evaluation/retrieval-regression.md, simulates retrieval for each, reports PASS/FAIL. Added telemetry sweep for new retrieval.queried event type (pending schema v1.2.0). Updated description to reflect expanded scope. |

## 2026-08-02 — Epic 45: Refactor Engineer

| Agent | Version | Change |
|---|---|---|
| refactor-engineer | 1.0.0 | New agent — structural refactoring with Fowler operations, characterization-test safety net, and behavior-preservation attestation |

## 2026-08-01 — v3.3.0: AOS Policy Layer (Phase 4)

Fourth and final phase of the AOS migration. Phase 4 introduces the policy layer that makes
graduated automation possible. **A team on v3.2.0 that upgrades to v3.3.0 and does not create
any `.claude/policies/` files sees zero behavior change.** Default policy set is empty; all
approvals still require human confirmation. Policies are strictly opt-in per-project. This is
the backward-compatibility guarantee for the fifth and final time.

**Policy Layer (Ops 4.1-4.3)**
- `shared/policies/` scaffold: `README.md` (opt-in nature, audit-trail requirements, per-project
  storage), `policy-schema.md` (declarative matcher + condition + action format, gate eligibility
  table, conflict-resolution rules, telemetry event shapes), `examples/` (4 reference policies).
- `shared/policies/examples/auto-approve-refactor.policy.yaml`: Tier A auto-proceed on pure-refactor
  commits (code-reviewer APPROVED, no behavior change, fitness functions pass, diff < 200 LOC,
  no security/auth paths).
- `shared/policies/examples/auto-approve-doc-changes.policy.yaml`: auto-proceed on docs-only git
  commits (diffType docs-only, tests pass, diff < 500 lines). Targets Gate 2.
- `shared/policies/examples/auto-approve-test-additions.policy.yaml`: auto-proceed on fitness
  function wiring when the wired function is a new test (dry-run pass, all paths in test dirs,
  zero security criticals). Targets Gate 7.
- `shared/policies/examples/require-human-review-security.policy.yaml`: inversion policy —
  explicitly requires human on any file matching security/auth/credential paths regardless of
  other policies. Targets Gates 2 and 6. Demonstrates that `require-human` always beats
  `auto-approve` in conflict resolution.

**Policy Evaluator (Op 4.2)**
- `shared/orchestration/policy-evaluator.md`: spec for the evaluator that bridges `.claude/policies/`
  to `FeatureDeliveryWorkflow` stage boundaries. Algorithm: load → filter → evaluate → conflict-
  resolve → emit telemetry → return `proceed | halt | require-human`. Every decision emits a
  `policy.evaluated` telemetry event — no silent auto-approvals. Conflict resolution: `require-human`
  always beats `auto-approve`. Dry-run mode: `--dry-run-policies` validates policies against
  historical telemetry without mutating pipeline state.

**Approval Gates Classification (Op 4.5)**
- `shared/rules/approval-gates.md` updated: each of the 8 gates now annotated with policy
  eligibility (`Policy-eligible: Yes (Tier A)` or `No — Always Human`) and gate ID for the
  evaluator's matcher. Policy-eligible gates: 2 (`git-commit`), 6 (`out-of-boundary-write`),
  7 (`fitness-function-wiring`). Always-human gates: 1, 3, 4, 5, 8. New "Policy-Based Gate Type"
  section explains the v3.3 opt-in mechanism.

**Policy Authoring Guide (Op 4.4)**
- `docs/aos/policy-authoring-guide.md`: comprehensive operator guide covering the 8-gate
  eligibility table, schema quick-reference, first-policy walkthrough, inversion policy pattern,
  dry-run testing, emergency override (`policiesEnabled: false`), conflict resolution reference,
  audit trail requirements, and governance pair interaction (audits run before policy evaluation).

**Health Check Extension (Op 4.6)**
- `shared/skills/health-check/SKILL.md` extended with Step 5 (policy validation): parses each
  `.policy.yaml`, validates required fields, checks gate IDs against the eligible set, reports
  coverage map for the 3 eligible gates, detects conflicting policies, validates `action.type`
  values. Never fails on absence of policies. New "Policy Layer" section in health report output.

| Agent / Skill | Version | Change |
|---|---|---|
| `health-check` | 1.3.0 | Added Step 5: policy syntax validation, gate-ID eligibility check, coverage-conflict detection, policy layer section in output format. |

---

## 2026-08-01 — v3.2.0: AOS Runtime (Phase 3)

Third phase of the AOS migration. Phase 3 wires the runtime layers: RAG retrieval, orchestration,
Learning/Forgetting engines. All additions are opt-in. A team on v3.1.0 that does not invoke
`/orchestrate`, `--semantic` flags, Learning/Forgetting engines, or the `FeatureDeliveryWorkflow`/
`TDDWorkflow` sees **zero behavior change**.

**RAG Layer (Ops 3.1-3.4)**
- `shared/rag/` scaffold: three-corpus model (LLM-as-retriever / BM25 / vector) per ADR-002.
- `shared/skills/search-ki-semantic/` (new): full-corpus LLM-as-retriever for the KI+ADR corpus.
- `query-memory` updated: `--semantic` flag delegates KI/ADR portion to `search-ki-semantic`.
- Source retrieval deferred: `shared/rag/adapters/source-retrieval.deferred.md` documents the rationale.
- No re-work for installed-project BM25: shipped in `saturday-mcp` M1 (commit `5a47441`).

**Learning/Forgetting Engines Wired (Ops 3.5-3.6)**
- `shared/hooks/on-retrospective-written.yaml` (new): triggers `learning-engine` on retrospective write.
- `shared/hooks/scheduled-monthly.yaml` (new): monthly `forgetting-engine` + `memory-auditor` schedule.
- `learning-engine` SKILL.md formalized: invocation modes, duplicate-check step, explicit human-confirmation language.
- `forgetting-engine` SKILL.md formalized: staleness criteria, confidence tiers, archive-not-delete guarantee.

**Orchestration Runtime (Ops 3.7-3.10)**
- `shared/orchestration/` scaffold: README, interface.md (Workflow contract), pipeline-schema.md.
- `shared/skills/orchestrate/` (new): `/orchestrate` runtime entry point with `--legacy`, `--dry-run`, `--resume` flags.
- `install.sh`: `--base` (default, unchanged behavior) and `--full` (seeds AOS layers) modes.
- `docs/aos/migration-guide.md`: full v3.2 operator guide with per-layer opt-in steps.

**Trinity-Native Workflow Refactor (Ops 3.11-3.13)**
- `shared/workflows/feature-delivery-workflow.md` (new): `FeatureDeliveryWorkflow` — 14 named stages,
  resumable checkpoints, explicit Phase 4 policy boundaries, audit-after-producer on each contract artifact.
  `deliver-feature` skill is unchanged external entry point (v3.1 behavior preserved).
- `shared/workflows/tdd-workflow.md` (new): `TDDWorkflow` — 6-stage Red-Green-Refactor loop with
  named roles, coverage gate (≥ 85%), max-iteration halt, audit blocks on test-writing + refactor.
  `test-driven-developer` agent is unchanged external entry point (v1.3.0).
- `shared/orchestration/audit-composition-pattern.md` (new): producer→auditor mapping, retry protocol,
  config knob (`workflowAuditsEnabled: false` to disable per-project).

| Agent / Skill | Version | Change |
|---|---|---|
| `test-driven-developer` | 1.3.0 | Added TDDWorkflow reference and runtime invocation path (additive). |
| `search-ki-semantic` | 1.0.0 | New skill: LLM-as-retriever for framework KI corpus (Phase 3 Op 3.2). |
| `orchestrate` | 1.0.0 | New skill: /orchestrate runtime entry point (Phase 3 Op 3.8). |
| `learning-engine` | 1.1.0 | Formalized invocation modes, duplicate-check step, human-confirmation language. |
| `forgetting-engine` | 1.1.0 | Formalized staleness criteria, confidence tiers, archive-not-delete guarantee. |
| `deliver-feature` | *(no version bump — no behavior change)* | Added workflow reference comment. |

---

## 2026-07-30 — Epic 46: Visual QA Engineer

- **`visual-qa-engineer` 1.0.0**: Added new conditional pipeline agent. Runs after qa-engineer on UI-touching features that have `heatmap-data/` or Playwright snapshot baselines. Wraps `@orieken/saturday-ml-analyzer` for interaction coverage analysis and Playwright native `toHaveScreenshot()` for visual regression. Verdict: PASS | FAIL | UNCONFIGURED. An UNCONFIGURED verdict does not block the pipeline (heatmap instrumentation not yet adopted). Produces `visual-qa-report.md` validated by `shared/contracts/visual-qa-report-contract.md`.

## 2026-07-29 — Portable model_tier Abstraction

- **`model-tier-auditor` 1.0.0**: Added new read-only counter-agent auditing agent frontmatter for portable `model_tier` declarations (`light`, `default`, `heavy`).
- **`model_tier` Frontmatter Backfill**: Backfilled portable `model_tier` declarations with operational rationale comments across all 36 shared agents.

## 2026-07-25 — v3.1.0: AOS Governance Skeleton (Phase 2)

Second phase of the AOS (AI Operating System) migration described in `docs/aos/migration-plan.md`. Phase 2 is **purely additive**: every change is opt-in. A team upgrading to v3.1.0 without configuring `.claude/hooks/` or enabling auditor invocations sees zero behavior change from v3.0.0. That is the backward-compat guarantee this release commits to.

- **10 Counter-Auditor Agents**: Added `context-auditor`, `knowledge-auditor`, `prompt-evaluator`, `agent-evaluator`, `rule-auditor`, `pattern-reviewer`, `tool-validator`, `documentation-auditor`, `retrieval-evaluator`, `privacy-auditor`.
- **4 Opposing-Force Skill Pairs (7 skills)**: Added `memory-expansion` / `memory-compression`, `learning-engine` / `forgetting-engine`, `cost-optimizer` / `quality-optimizer`, and `scheduler`.
- **Event Interceptor Layer**: Created `shared/hooks/` with `hooks-schema.md` and example hook definitions (`on-artifact-write.yaml`, `on-validation-pass.yaml`, `on-ki-created.yaml`).
- **Opt-in Auditor Integration**: Updated `validate-artifact` to optionally invoke corresponding counter-auditor agents post-structural PASS when configured.
- **Health Check Extension**: Extended `health-check` skill to detect all 11 counter agents and validate hook schemas.

| Agent | Version | Change |
|---|---|---|
| `context-auditor` | 1.0.0 | Initial counter agent auditing `context-manifest.md` for pruning discipline, broken references, and token pressure accuracy. |
| `knowledge-auditor` | 1.0.0 | Initial counter agent auditing `create-ki` skill output for KI schema compliance, semantic overlap, and ubiquitous language. |
| `prompt-evaluator` | 1.0.0 | Initial counter agent auditing agent/skill prompts for prompt engineering hygiene, secret leakage, and template decoupling. |
| `agent-evaluator` | 1.0.0 | Initial counter agent promoting `agent-eval` skill logic into an agent persona, evaluating frontmatter contracts and quality scores. |
| `rule-auditor` | 1.0.0 | Initial counter agent auditing `shared/rules/*.md` for cross-rule contradictions, dead path references, and registry alignment. |
| `pattern-reviewer` | 1.0.0 | Initial counter agent auditing `docs/patterns/*.md` for code snippet accuracy and resolved file paths. |
| `tool-validator` | 1.0.0 | Initial counter agent auditing `shared/skills/*/SKILL.md` for standalone mode declarations and hidden script dependencies. |
| `documentation-auditor` | 1.0.0 | Initial counter agent auditing `README.md`, `docs/AGENT_REFERENCE.md`, and prose docs for accurate agent/skill counts. |
| `retrieval-evaluator` | 1.0.0 | Initial counter agent auditing KI + ADR retrievability per ADR-002 telemetry, identifying unmatched zero-hit queries. |
| `privacy-auditor` | 1.0.0 | Initial counter agent paired with `security-reviewer`, auditing workspace artifacts for secret leakage, PII, and data boundary leaks. |

---

## 2026-07-25 — AOS Phase 2 Op 2.1: 10 Counter-Auditor Agents

Adds 10 read-only counter agents under `shared/agents/` following the `memory-auditor` exemplar shape (Op 2.1 of `docs/aos/migration-plan.md`). Every agent is audit-only, reports findings for human review, and never mutates project state.

| Agent | Version | Change |
|---|---|---|
| `context-auditor` | 1.0.0 | Initial counter agent auditing `context-manifest.md` for pruning discipline, broken references, and token pressure accuracy. |
| `knowledge-auditor` | 1.0.0 | Initial counter agent auditing `create-ki` skill output for KI schema compliance, semantic overlap, and ubiquitous language. |
| `prompt-evaluator` | 1.0.0 | Initial counter agent auditing agent/skill prompts for prompt engineering hygiene, secret leakage, and template decoupling. |
| `agent-evaluator` | 1.0.0 | Initial counter agent promoting `agent-eval` skill logic into an agent persona, evaluating frontmatter contracts and quality scores. |
| `rule-auditor` | 1.0.0 | Initial counter agent auditing `shared/rules/*.md` for cross-rule contradictions, dead path references, and registry alignment. |
| `pattern-reviewer` | 1.0.0 | Initial counter agent auditing `docs/patterns/*.md` for code snippet accuracy and resolved file paths. |
| `tool-validator` | 1.0.0 | Initial counter agent auditing `shared/skills/*/SKILL.md` for standalone mode declarations and hidden script dependencies. |
| `documentation-auditor` | 1.0.0 | Initial counter agent auditing `README.md`, `docs/AGENT_REFERENCE.md`, and prose docs for accurate agent/skill counts. |
| `retrieval-evaluator` | 1.0.0 | Initial counter agent auditing KI + ADR retrievability per ADR-002 telemetry, identifying unmatched zero-hit queries. |
| `privacy-auditor` | 1.0.0 | Initial counter agent paired with `security-reviewer`, auditing workspace artifacts for secret leakage, PII, and data boundary leaks. |

---

## 2026-07-24 — Context-engineer changelog catch-up

Documentation-only catch-up for the existing `context-engineer` v2.2.0 prompt state. The agent file
already carried this version; this entry records it so `health-check.sh` no longer reports the version as
undocumented. No agent behavior changed in this edit.

| Agent | Version | Change |
|---|---|---|
| context-engineer | 2.2.0 | Changelog catch-up for the already-present proactive context-optimization prompt version. No prompt edit or behavior change in this documentation patch. |

---

## 2026-07-22 — v3.0.0: AOS foundations (Phase 1)

First landing of the AOS (AI Operating System) migration described in
`docs/aos/migration-plan.md`. Phase 1 is **purely additive**: every change here
is opt-in. A team upgrading a v2.x install to v3.0.0 without invoking any AOS-
specific capability behaves identically to v2.x. That is the backward-compat
guarantee this release commits to — see `docs/aos/migration-guide.md`.

### New
- `shared/telemetry/` — top-level layer for pipeline telemetry. Contains
  `README.md`, `event-schema.md` (agent.invoked, agent.completed,
  artifact.written, validation.passed, validation.failed), and the
  `event-recorder` skill (`event-recorder.md`) that appends events to
  `.claude/telemetry/events.jsonl`. No producer emits events by default —
  Phase 3 will wire that via hooks.
- `shared/evaluation/` — top-level layer for continuous evaluation specs.
  Contains `README.md` (continuous vs on-demand model) and the first spec,
  `pipeline-retrospective.md`, which points at the existing
  `shared/skills/pipeline-retrospective/SKILL.md` unchanged and documents its
  Phase 3 continuous-trigger contract.
- `shared/agents/memory-auditor.md` (v1.0.0) — the first AOS counter agent,
  paired with the `memory-engineer` skill (pair #2 in the AOS design pack's
  Governance Checks and Balances). Read-only (`Read, Glob, Grep`), reports
  schema failures, exact duplicates, semantic-overlap candidates, and stale-
  metadata candidates. Never modifies KIs. Exemplar for the 10 remaining
  audit-relationship counter agents Phase 2 will land.
- `docs/aos/migration-guide.md` — the "how to opt in" stub. Nothing forces
  adoption in v3.0.0.

### Updated
- `shared/skills/health-check/SKILL.md` — new "AOS Layers" section at the
  bottom of the health report inventories which AOS layers and counter
  agents are present. Absence of any AOS layer is never a failure —
  inventory only, consistent with the opt-in migration principle. Counter-
  agent detection filters out review-shaped producers (`code-reviewer`,
  `security-reviewer`, etc.) so only real AOS counter agents (paired per
  the 15 governance pairs) get counted. Skill has no frontmatter `version:`
  field today; not retroactively adding one just for this edit — kept
  consistent with every other skill.

### Unchanged
Every agent's `version:` in this changelog stayed exactly as it was in the
v2.x-era last row. Every existing skill file behaves identically. Every rule
under `shared/rules/` is untouched. Every contract, blueprint, and template
is untouched. `shared/memory-registry.json` is untouched (memory-auditor
reads it; it does not appear as a new source).

### The v3.0 backward-compat guarantee (verbatim)

> A v2.x install upgraded to v3.0 without opting into any AOS capability
> behaves identically to v2.x.

Verified via Op 1.7's identity check — see the migration plan for the exact
checks a human should run before publishing the v3.0.0 tag.

| Agent | Version | Change |
|---|---|---|
| memory-auditor | — -> 1.0.0 | **New agent.** First AOS counter agent, paired with the memory-engineer skill. Read-only auditor: schema compliance, exact + semantic duplicate detection, stale-metadata candidates. Never modifies KIs; produces findings for human/memory-engineer approval. Tools deliberately limited to Read, Glob, Grep. |

---

## 2026-07-15 — Testing taxonomy: FIRST, Three Laws, and annotation convention explicit

Surfaced by a direct question about whether the framework has a clear distinction between unit /
integration / acceptance / E2E tests, and whether tests should annotate their originating
issue/AC for traceability. Both real gaps: the framework had structural separation (Saturday = E2E,
Sunday = API, `test-driven-developer` = greenfield unit, `unit-tester` = existing-code unit) but no
doc naming the pyramid or its principles per level, and no in-test annotation convention (traceability
was report-time-only, evaporates the moment a test file gets renamed or moved).

The user's follow-up correctly pushed back on cargo-culting XP TDD: the design pressure of XP TDD comes
from epistemic role separation between the person writing the failing test and the person implementing
it. When a single agent does both, that gap collapses. Rather than pretend `test-driven-developer`
standalone produces the same design benefit as human XP TDD, this update states the honest scope: role-
separated (via `deliver-atdd`) preserves the design pressure; standalone is valuable for spec/
regression but relies on other mechanisms (complexity thresholds, SOLID, `code-reviewer`) for design
work. This is the same "state the tradeoff plainly rather than paper over it" approach
`docs/AGENT_REFERENCE.md` already uses for `test-driven-developer`'s review-chain bypass.

New: `docs/patterns/testing-pyramid.md` (the philosophy — five test levels, FIRST, Three Laws with the
role-separation scoping) and two new sections in `shared/rules/testing-conventions.md` (the
enforcement — Test Categories table mapping level to agent/framework, plus Test Annotation Convention
with per-language mechanisms — JSDoc for TS, docstring for pytest, `@Tag`/`@DisplayName` for JUnit,
`[Trait]` for xUnit, comment for Go, `@issue:...` tag for Gherkin).

| Agent | Version | Change |
|---|---|---|
| test-driven-developer | 1.1.0 -> 1.2.0 | **Minor**: cite Three Laws + FIRST explicitly (they were already the implicit discipline; naming them makes it teachable). Add honest scoping note about role separation — standalone use doesn't produce XP TDD's design benefit; that's a real tradeoff, not something to hide. New process step: annotate each test per the new Test Annotation Convention. Read `testing-conventions.md` in the preamble now that it has enforceable rules relevant to this agent. |
| unit-tester | 1.1.0 -> 1.2.0 | **Minor**: state explicitly that this agent does NOT follow the Three Laws (impossible — the code came first) but that its tests still satisfy FIRST as properties. Add annotation-convention citation to step 6, with a note that in characterization mode the "AC" being annotated is often the observed behavior itself, not a spec-defined AC. Read `testing-conventions.md` in the preamble. |
| qa-engineer | 1.1.1 -> 1.2.0 | **Minor**: add an intro paragraph making the multi-level scope explicit (integration / API contract / acceptance / E2E — this agent legitimately owns all four, and should be explicit about which level each test is at). Add annotation-convention citation to step 6 with per-format guidance (Gherkin gets `@issue:...` tag; scenario name IS the AC). Read `testing-conventions.md` in the preamble. |

---

## 2026-07-13 — unit-tester reads callers/dependents; backfill-unit-tests considers context-engineer

Follow-up to the same-day `unit-tester`/`backfill-unit-tests` addition, prompted by a direct question:
should context get cleaned up before either starts work? Answer landed as "not the full `context-engineer`
pass" (it's built around feature-spec-driven bounded-context mapping that doesn't exist for a bare
file/directory target) "but yes to a narrower version of the same need." Characterization mode specifically
depends on understanding a legacy target's callers and dependents, not just the target file — a caller that
only exercises the code under specific external state is invisible from the target file alone, and
`search-ki` doesn't cover it either (it finds documented gotchas, not undocumented call graphs).

| Agent | Version | Change |
|---|---|---|
| unit-tester | 1.0.0 -> 1.1.0 | **Minor**: expanded process step 3 — after reading the target, grep for its callers/importers and what it depends on, not just the target file in isolation. Most load-bearing in characterization mode; a single already-understood file being backfilled for coverage needs it less. |

`backfill-unit-tests` (no version — skills aren't versioned the way agents are): new process step 2 points
at `context-engineer`'s own proactive self-invocation trigger (3+ files, unfamiliar code) rather than
duplicating its logic — a directory/module-level characterization target should trigger it in practice, a
single already-familiar file usually shouldn't. Renumbered subsequent steps.

---

## 2026-07-13 — New agent: unit-tester (+ backfill-unit-tests skill)

A new gap, adjacent to but distinct from `test-driven-developer`: sometimes tests need to be added to code
that must *not* change — raising coverage on already-trusted code, or building a Michael Feathers-style
characterization-test safety net around legacy code before a refactor or migration. `qa-engineer` already
had the right guidance for this (its own "Testing Legacy Code" section), but only as a step gated behind
`deliver-feature`'s `analysis.md`/`implementation-notes.md` inputs — there was no way to point it at an
arbitrary existing file with no feature delivery underway.

Added `unit-tester`: same standalone-invocation style as `test-driven-developer`, but inverted — the code
doesn't change to satisfy the tests, the tests describe what the code already does. Stricter than
`qa-engineer` on one point: never modifies source, not even qa-engineer's narrow "except to fix a bug
found" exception. If the code genuinely can't be tested without a structural seam, that's treated as
`approval-gates.md` gate #6 (Writing Files out of Boundary) — reported and held for explicit approval, never
performed automatically.

Unlike `test-driven-developer`'s soft "recommend documentation-manager, don't auto-invoke" pattern (real
risk of wasted ceremony — most ad-hoc sessions produce nothing worth promoting), a code-quality pass over
newly-written tests has no equivalent "usually pointless" cost. So this one gets auto-chained instead of
just recommended: new skill `backfill-unit-tests` runs `unit-tester`, then automatically runs
`code-reviewer` against just the new test files (never the untouched source), producing one combined
report — same shape as `review-pr`, this repo's existing precedent for a thin orchestration skill that
coordinates agents without `deliver-feature`'s full checkpoint ceremony.

| Agent | Version | Change |
|---|---|---|
| unit-tester | — -> 1.0.0 | **New agent.** Standalone, writes/backfills unit tests for existing code without modifying it (coverage backfill or legacy characterization). Includes the same `search-ki` lookup + non-auto-invoked `documentation-manager` recommendation pattern just added to `test-driven-developer`. |

New skill: `shared/skills/backfill-unit-tests/SKILL.md` — coordinates `unit-tester` + `code-reviewer`, mirroring `review-pr`'s orchestration shape.

---

## 2026-07-13 — test-driven-developer gets memory awareness

`test-driven-developer` bypasses the whole `deliver-feature` pipeline by design (see
`docs/AGENT_REFERENCE.md` entry 24) — no `context-engineer` pass before it starts, and it's not part of
the `pipeline-trace`/retrospective-every-5 cadence after it finishes. That's a deliberate speed tradeoff
on the *review* axis (no code-reviewer/security-reviewer), but it left this agent silently cold on the
*memory* axis too: it never checked whether a relevant KI/ADR already existed, and nothing routed its
findings back into the memory system afterward. Closes that gap without reintroducing the ceremony the
agent exists to skip — a single cheap `search-ki` lookup going in, a recommendation (not an auto-trigger)
for `documentation-manager` coming out.

| Agent | Version | Change |
|---|---|---|
| test-driven-developer | 1.0.1 -> 1.1.0 | **Minor**: new process step 2 invokes `search-ki` for the feature's domain/keywords before test design (read-only, non-blocking — informs, doesn't gate). New "Knowledge Consulted" section in `tdd-report.md`'s output format. New rule: after a substantial session, recommend `documentation-manager` to the user rather than auto-invoking it, matching that agent's own "most sessions produce nothing durable enough to promote" discipline. |

---

## 2026-07-06 — Cursor native skills/agents compatibility (Epic 30, Phase 1)

Cursor shipped native Agent Skills (`.cursor/skills/*/SKILL.md`) and subagent (`.cursor/agents/*.md`)
support using the same open standard this repo's `shared/skills/`/`shared/agents/` already follow
(confirmed against `cursor.com/docs/skills` and `cursor.com/docs/subagents`). Scoping that integration
surfaced two prerequisite issues affecting all 24 agents, fixed together here since both touch the
same files in the same pass:

1. **`model: sonnet` → `model: inherit`**: every agent hardcoded a specific model regardless of what
   the user's own session was running. Both Claude Code and Cursor subagents default to `inherit` when
   the field is omitted and accept the literal keyword explicitly — confirmed via Cursor's own docs and
   a live Claude Code frontmatter check this session. `inherit` lets each subagent match whatever model
   the operator already chose for their session instead of forcing Sonnet unconditionally.
2. **Frontmatter preamble relocated**: 23 of 24 agents (every one except `documentation-manager`) had
   a "Read `.claude/rules/design-principles.md`..." instruction *before* their opening `---`, which is
   invisible to `health-check.sh`'s lenient grep-anywhere frontmatter check and tolerated by Claude
   Code's loader, but would very likely break Cursor's stricter parser once agents are symlinked
   directly (a planned later phase of this epic). Moved into the body as the agent's own first
   instruction, using canonical `shared/rules/` paths instead of the Claude-Code-only `.claude/rules/`
   prefix, so every file now starts with `---` on line 1.

| Agent | Version | Change |
|---|---|---|
| accessibility-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| analyst | 1.1.0 -> 1.1.1 | Patch: preamble relocated, model: inherit |
| api-test-generator | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| architect | 1.1.1 -> 1.1.2 | Patch: preamble relocated, model: inherit |
| chaos-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| code-reviewer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| context-engineer | 2.1.1 -> 2.1.2 | Patch: preamble relocated, model: inherit |
| data-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| dependency-auditor | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| developer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| devops-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| documentation-manager | 2.0.0 -> 2.0.1 | Patch: model: inherit (already had correct frontmatter placement from its 2026-07-05 rewrite -- no preamble to relocate) |
| dx-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| finops-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| modernization-supervisor | 1.0.0 -> 1.0.1 | Patch: preamble relocated (2-file variant: design-principles.md + ARCHITECTURE_RULES.md), model: inherit |
| performance-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| product-owner | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| qa-engineer | 1.1.0 -> 1.1.1 | Patch: preamble relocated, model: inherit |
| release-manager | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| security-reviewer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| spec-writer | 1.1.0 -> 1.1.1 | Patch: preamble relocated, model: inherit |
| sre-engineer | 1.0.0 -> 1.0.1 | Patch: preamble relocated, model: inherit |
| tech-writer | 1.1.0 -> 1.1.1 | Patch: preamble relocated, model: inherit |
| test-driven-developer | 1.0.0 -> 1.0.1 | Patch: preamble relocated (2-file variant: design-principles.md + ARCHITECTURE_RULES.md), model: inherit |

---

## 2026-07-05 — External audit fixes (api-generator portability, context-engineer casing tolerance)

| Agent | Version | Change |
|---|---|---|
| context-engineer | 2.1.0 -> 2.1.1 | **Patch**: an external audit found that step 6's Prior-Deliveries grep for `**Owning Context**` exact-matches, but the archived `docs/features/context-engineering-framework/analysis.md` uses `**Owning context**` (lowercase c) — a real historical drift that would silently miss that feature's retrospective. Fixed by making the lookup case-insensitive (documented in both the agent and `shared/skills/context-engineer/SKILL.md` twin) instead of retroactively editing the archived doc, since the feature archive is treated as an immutable historical record elsewhere in this framework. No output format change. |

Also (no agent version bump, non-agent fixes from the same external audit):
- `scripts/api-generator/index.ts`: removed hardcoded personal machine paths (`/Users/oscarrieken/...`) for
  Go/TS client output dirs — now CLI args or `API_GENERATOR_GO_DIR`/`API_GENERATOR_TS_DIR` env vars.
- `scripts/api-generator/package.json` + new `scripts/api-generator/README.md`: marked the tool explicitly
  experimental/unsupported (it has no tests and isn't wired into `scripts/ci-check.sh` or CI) instead of
  leaving that unstated.

---

## 2026-07-05 — documentation-manager narrowed to ad-hoc-session counterpart of promote-memory

| Agent | Version | Change |
|---|---|---|
| documentation-manager | 1.0.0 -> 2.0.0 | **Major**: changed output contract entirely. Previously wrote directly to `ARCHITECTURE.md`/`RUNBOOKS.md`/`GOTCHAS.md`/`ONBOARDING.md` with no review step -- an undocumented overlap with `memory-engineer`/`promote-memory`/`extract-lessons` (added later, in the Memory Engineering epic) that `docs/AGENT_REFERENCE.md` flagged explicitly. Redesigned as the ad-hoc-session counterpart to `promote-memory`: now produces Candidate Records (Source/Type/Evidence/Tags/Expiration) via the same Memory Contract, requires explicit human approval before any KI/ADR/rule/living-doc edit, and retires `GOTCHAS.md` as a target (gotchas are Knowledge Items now, via `create-ki`). Still manual/on-demand, not hooked to auto-run after every session. |

---

## 2026-07-04 — Memory Engineering epic (v2 scope, split from AOS/v3 prototyping)

| Agent | Version | Change |
|---|---|---|
| context-engineer | 2.0.0 -> 2.1.0 | New: Proactive RAG step now checks whether the task's question is KI/ADR-shaped (invoke `search-ki`, unchanged default) or broader (invoke the new `query-memory` skill instead, which also covers the feature archive and DOMAIN_DICTIONARY.md). Additive — existing behavior and output format unchanged. Applied identically to both the agent and its `shared/skills/context-engineer/SKILL.md` twin in the same edit, to avoid repeating the twin-drift bugs found across three independent audits this session |

---

## 2026-07-03 — Cross-agent audit fixes (independent review via docs/runbooks/self-audit-prompt.md)

| Agent | Version | Change |
|---|---|---|
| spec-writer | 1.0.0 -> 1.1.0 | Twin drift fix: the agent's Critique Report used emoji verdicts (`READY ✅ \| NEEDS WORK ⚠️`, `✅/⚠️` per row) while `shared/skills/spec-writer/SKILL.md` used plain text (`READY \| NEEDS WORK`, `PASS/FAIL`). Standardized both to plain text — matches the `PASS/FAIL` vocabulary used everywhere else in the framework (`validate-artifact`, contracts) and avoids emoji-rendering inconsistency across the 6 target platforms |
| architect | 1.1.0 -> 1.1.1 | Patch: removed a self-contradictory parenthetical ("read at step 3 as per instructions") on what is actually step 2 of its own process list — wording fix, no behavior change |

Also (no version bump — pure renames/config changes, not agent behavior changes):
- `modernization-swarm.md` -> `modernization-supervisor.md`, `test-driven-development-agent.md` ->
  `test-driven-developer.md`: filenames now match their own `name:` frontmatter field, like every other
  agent in `shared/agents/`.
- `context-engineer`'s skill twin (`shared/skills/context-engineer/SKILL.md`) had its Prune Recommendations
  bullet format aligned to match the agent's (proper `- [ ]` instead of backtick-wrapped `` `[ ]` ``, plus
  the reason column the skill was missing).

---

## 2026-07-03 — Context Engineering audit: contract + agent/skill heading realignment

| Agent | Version | Change |
|---|---|---|
| context-engineer | 1.4.0 -> 2.0.0 | **Major**: `shared/agents/context-engineer.md`'s Output Format headings (`## Scope & Boundaries`, `## Relevant Knowledge Items (KIs) & ADRs`, `## Pinpoint Files to Open...`, `## Pruning Checklist...`) had drifted from its own "standalone twin" in `shared/skills/context-engineer/SKILL.md` (`## 1. Scope and Boundaries` ... `## 7. Token Budget`) and was missing a `## 3. Global Rules and Constraints` section entirely. Realigned the agent's headings to match the skill's numbered format exactly, and added the missing section. This was found while adding `shared/contracts/context-manifest-contract.md` (see below) — the contract would have failed every real run against the agent's old headings. New contract added: `context-manifest.md` now gets the same `validate-artifact` structural gate every other pipeline artifact already had; wired into `deliver-feature` as new step 7 (renumbering all subsequent steps by one). |

---

## 2026-07-02 — Team Topologies alignment

| Agent | Version | Change |
|---|---|---|
| architect | 1.0.0 -> 1.1.0 | New "Team Topology Fit" sub-step under Strategic Domain Design: for any Context Crossing, invokes `team-topology-check` (new skill) against the new `TEAM_TOPOLOGY.md` registry to flag a stale Collaboration interaction mode or a bypassed Platform team — a Conway's-Law-shaped version of the existing Distributed Monolith anti-pattern check. New "Team Topology Fit" line added inside the already-required `## Bounded Context` section (no contract change needed — the heading itself is unchanged) and a new Anti-Pattern Check checklist item |

---

## 2026-07-02 — Epic 14 KI infrastructure

| Agent | Version | Change |
|---|---|---|
| context-engineer | 1.0.0 -> 1.1.0 | Step 5 (Proactive RAG) now invokes the `search-ki` skill instead of ad-hoc grepping `shared/knowledge/`, `.claude/knowledge/`, and `docs/adrs/` directly — additive, output format unchanged |

---

## 2026-07-02 — Epic 17 context decay and bounded-context pruning

| Agent | Version | Change |
|---|---|---|
| qa-engineer | 1.0.0 -> 1.1.0 | Step 2 now gets `analysis.md`'s acceptance criteria/edge cases via `summarize-artifact` instead of a full read (Context Decay — 2 phases old by this point) |
| tech-writer | 1.0.0 -> 1.1.0 | Step 1 now gets `analysis.md`'s scope via `summarize-artifact` instead of a full read (same reason) |
| context-engineer | 1.1.0 -> 1.2.0 | New step: auto-prune Pinpoint Files by bounded-context mapping (exclude other contexts' files unless the analysis explicitly flags a crossing) and by change surface (exclude infrastructure/migration files for UI-only tasks) |

---

## 2026-07-02 — Proactive self-invocation

| Agent | Version | Change |
|---|---|---|
| context-engineer | 1.2.0 -> 1.3.0 | Description now says "Use PROACTIVELY before starting any task that touches 3+ files, a new feature area, or unfamiliar code" instead of only firing on explicit request — closes the gap where context engineering only ever applied inside `deliver-feature`, never in ad-hoc sessions. Additive framing change, no process/output format change |

---

## 2026-07-02 — Cross-feature learning: same-bounded-context retrieval

| Agent | Version | Change |
|---|---|---|
| context-engineer | 1.3.0 -> 1.4.0 | New step: search `docs/features/*/analysis.md` for prior deliveries in the same Bounded Context (recency-independent) and surface their `retrospective.md` lessons in a new "Prior Deliveries in This Bounded Context" context-manifest.md section. Closes the gap where a same-area mistake from more than 3 deliveries ago was invisible to `analyst`'s recency-based feedback loop |
| analyst | 1.0.0 -> 1.1.0 | Step 5 (feedback loop) now treats context-manifest.md's "Prior Deliveries in This Bounded Context" as the primary, recency-independent same-area check, with the existing 3-most-recent-deliveries scan kept as a secondary check for general cross-cutting process trends |

---

## 2026-07-02 — Initial versioning rollout
All 24 agents in `shared/agents/` set to `1.0.0` — no prior version was tracked before this.

| Agent | Version | Change |
|---|---|---|
| accessibility-engineer | 1.0.0 | Initial version |
| analyst | 1.0.0 | Initial version |
| api-test-generator | 1.0.0 | Initial version |
| architect | 1.0.0 | Initial version |
| chaos-engineer | 1.0.0 | Initial version |
| code-reviewer | 1.0.0 | Initial version |
| context-engineer | 1.0.0 | Initial version |
| data-engineer | 1.0.0 | Initial version |
| dependency-auditor | 1.0.0 | Initial version |
| developer | 1.0.0 | Initial version |
| devops-engineer | 1.0.0 | Initial version |
| documentation-manager | 1.0.0 | Initial version |
| dx-engineer | 1.0.0 | Initial version |
| finops-engineer | 1.0.0 | Initial version |
| modernization-supervisor | 1.0.0 | Initial version |
| performance-engineer | 1.0.0 | Initial version |
| product-owner | 1.0.0 | Initial version |
| qa-engineer | 1.0.0 | Initial version |
| release-manager | 1.0.0 | Initial version |
| security-reviewer | 1.0.0 | Initial version |
| spec-writer | 1.0.0 | Initial version |
| sre-engineer | 1.0.0 | Initial version |
| tech-writer | 1.0.0 | Initial version |
| test-driven-developer | 1.0.0 | Initial version |
