# Agent Reference: Roles and Counterbalances

Every agent in this framework produces an output that *something* checks — another agent's review, a
structural contract, a human approval gate, an aggregate metric measured after the fact, or (stated plainly
where true) nothing yet. This doc makes that explicit for all 40 agents, one at a time, instead of leaving
it scattered implicitly across `deliver-feature/SKILL.md`, `shared/contracts/`, and each agent's own file.

This is documentation of what already exists in v2 today — it does not introduce new agents or roles. Where
an agent has no real counterbalance, that's stated as a gap, not filled in with something invented to make
the table look complete. (A larger, formal "checks and balances" model — pairing every role with a dedicated
auditor counterpart — is being prototyped separately for v3/AOS; see `docs/aos/`. This doc is about what v2
actually has running today.)

## How to read "Counterbalance"

Five different *kinds* of check show up across these 40 agents, and they're not interchangeable:

| Kind | What it catches | Example |
|---|---|---|
| **Structural contract** | Missing/malformed sections in the artifact itself | `validate-artifact` against `shared/contracts/*.md` |
| **Downstream agent review** | Wrong or low-quality content, live, on this delivery | `code-reviewer` reviewing `developer`'s output |
| **Human approval gate** | Irreversible or consequential actions | `shared/rules/approval-gates.md`'s 8 gates |
| **Process-enforced gate** | The same, but the barrier is code rather than an instruction — nothing an agent returns can pass it | `loom run` halting at `confirm-design` before `developer` (roadmap L2.13) |
| **Aggregate/delayed metric** | Systemic drift across many deliveries, not visible in any one | `agent-scorecard`'s per-agent quality scores |

A structural contract can't catch bad judgment; a downstream review can't catch a slow, aggregate decline in
quality; a human gate only fires for the specific actions it names; and a process-enforced gate only covers
stages that actually run under the executor — for the markdown pipeline the same checkpoints remain
prompt-discipline. Most agents below are checked by more
than one kind — that's by design, not redundancy.

---

## What counts as a pipeline agent

**A pipeline agent is one the delivery plan invokes** — the fourteen stages `loom run` executes and
`deliver-feature` describes, from `context-engineer` to `devops-engineer`. That definition is tied to an
executable artifact (`DefaultDeliverFeaturePlan` in `internal/orchestrator/plan.go`) rather than to prose,
so it cannot drift silently.

Two consequences worth stating, because three documents previously disagreed about this:

- `spec-writer` and `product-owner` are **pre-pipeline**. They act on the spec before delivery starts, not
  on the feature workspace, and the delivery plan does not invoke them. They are documented first below
  because that is the order a human meets them.
- `visual-qa-engineer` **is** a pipeline agent — it is stage 11 of the plan — despite being added later
  than the rest. Its own section remains under "Late-Addition" below for continuity.

The executor's plan additionally contains an internal `router` stage (roadmap L3.0), which decides which
of the conditional stages a given feature needs. It is not an agent and is not counted here.

## Pipeline Agents (in `deliver-feature` order)

*(§1 and §2 below are the pre-pipeline agents described above; §3–§15 are the delivery plan's own.)*

### 1. `spec-writer`
**Role**: Interviews the user one question at a time to draft a feature spec, then immediately critiques its
own draft for downstream readiness (Write Mode → Review Mode) against every later agent's actual needs.
**Counterbalance**: Self-critique is built into its own process, not external — but `product-owner` reviews
the resulting spec next for value/scope, and `analyst` is the first agent to actually try to *use* the spec
technically; a spec that was too vague surfaces as a gap in `analysis.md`.
**Gap**: No agent double-checks whether spec-writer's own readiness critique was accurate — if it declared
READY prematurely, that's only caught downstream, not immediately.

### 2. `product-owner`
**Role**: Challenges whether a feature should be built at all — value, scope, simpler alternatives — before
any analysis or code happens. A DO NOT BUILD verdict blocks the pipeline.
**Counterbalance**: **Human approval gate** — a DO NOT BUILD or REDUCE SCOPE verdict requires explicit human
override to proceed, per its own guardrail.
**Gap**: Nothing checks whether product-owner's *own* pushback is well-calibrated over time (too aggressive,
killing legitimate work, vs. too lenient) — no aggregate metric exists for this agent the way `agent-scorecard`
tracks code-reviewer and security-reviewer.

### 3. `context-engineer`
**Role**: Pre-flight context optimizer — scopes the bounded context, prunes irrelevant files, surfaces
relevant KIs/ADRs and prior deliveries, estimates token budget, before analyst reasons independently.
**Counterbalance**: **Structural contract** (`shared/contracts/context-manifest-contract.md` via
`validate-artifact`, step 7) checks the manifest has all 7 required sections and never reports a token
budget without an estimate. Separately, the `context-audit` skill checks *after the fact* whether pinned
files were actually used — a different kind of check (waste analysis, not correctness).
**Gap**: None significant — this is one of the more thoroughly checked agents, deliberately, since everything
downstream depends on it.

### 4. `analyst`
**Role**: Turns a feature spec into acceptance criteria, task breakdown, data model/API changes, bounded
context mapping, and a definition of done — the first technical translation of the spec.
**Counterbalance**: **Structural contract** (`analysis-contract.md`) + a **human approval gate** (PAUSE, step
10) before any code is written. `architect`/`developer` downstream also surface gaps if the analysis was
wrong (e.g., a missing edge case shows up as rework).
**Gap**: None significant.

### 5. `architect`
**Role**: Structural decisions, fitness functions, layer boundaries, Team Topology fit — only invoked when a
feature actually needs structural judgment, not a rubber stamp on every feature.
**Counterbalance**: **Structural contract** (`architecture-contract.md`) + a **human approval gate** (PAUSE if
an RFC was written — expensive/permanent decisions require explicit acknowledgment) + `team-topology-check`
for Conway's-Law-shaped crossing mismatches. `code-reviewer` downstream also flags it if the implementation
didn't actually follow the architecture.
**Gap**: None significant.

### 6. `performance-engineer`
**Role**: Shift-left performance review of the *architecture* before implementation starts — idempotency,
timeouts, N+1 prevention, caching strategy.
**Counterbalance**: **Structural contract** (`performance-contract.md`, requires a Status line per risk
category). `qa-engineer` is the practical, downstream check on whether the mandated SLAs are actually met.
**Gap**: Nothing verifies performance-engineer's *estimates* were realistic (e.g., a caching recommendation
that turns out not to help) — no load-testing feedback loop exists yet.

### 7. `data-engineer`
**Role**: Schema design and zero-downtime (Expand/Contract) migrations for features touching the database.
**Counterbalance**: **Structural contract** (`data-engineering-contract.md`, requires a declared Phase) + the
`validate-migrations` skill (invoked directly by this agent, rejects destructive operations) + `devops-engineer`
independently invokes `validate-migrations` again before deploy.
**Gap**: None significant — migrations are one of the most heavily gated artifact types in the pipeline.

### 8. `developer`
**Role**: Implements the feature via TDD in an isolated worktree, following the analysis and architecture.
**Counterbalance**: **Structural contract** (`implementation-contract.md`) + **downstream agent review** from
`code-reviewer` (blocks on CHANGES REQUESTED, including an explicit "Test Design Review" of developer's own
TDD tests), `security-reviewer`, and `accessibility-engineer` — the most heavily reviewed handoff in the
pipeline by agent count.
**Gap**: None significant for the code itself. See `code-reviewer` below for a different, related gap.

### 9. `code-reviewer`
**Role**: Reviews the implementation against Clean Architecture, SOLID, Sandi Metz limits, and Fowler smells;
blocks the pipeline until APPROVED.
**Counterbalance**: **Structural contract** (`review-contract.md`, requires the literal `APPROVED`/`CHANGES
REQUESTED` string) + **aggregate metric** (`agent-scorecard`'s first-pass-acceptance-rate, flags if this
agent is systematically too lenient — rubber-stamping — or too strict, floor < 50%).
**Gap**: No agent re-reviews a single code-reviewer verdict *live* — the check is statistical and delayed
(next month's scorecard), not immediate. A rubber-stamped APPROVED on one specific delivery isn't caught
until the pattern shows up in aggregate.

### 10. `accessibility-engineer`
**Role**: Reviews UI changes for semantic HTML, WCAG compliance, keyboard navigation — fixes objective
violations directly. Only invoked when the feature touches UI.
**Counterbalance**: **Structural contract** (`accessibility-contract.md`, requires all 4 evaluation categories
present) + `qa-engineer` independently runs its own accessibility check (`[A11Y]` findings) as a second,
differently-sourced verification.
**Gap**: None significant — this is one of the few agents with a genuine second, independent check (qa-engineer's
own a11y pass) rather than just a structural contract.

### 11. `security-reviewer`
**Role**: STRIDE threat model of the implementation; fixes Critical/High findings directly rather than just
recommending them.
**Counterbalance**: **Structural contract** (`security-contract.md`) + **human approval gate** (explicit "fix
confirmed" required before QA starts on any Critical finding) + `qa-engineer` tests the specific security
scenarios security-reviewer calls out in "Notes for QA" + **aggregate metric** (`agent-scorecard`'s security
true-positive-rate proxy).
**Gap**: `agent-scorecard`'s own docs flag this explicitly — the TPR metric is a proxy, not a confirmed
true/false-positive rate, since there's no mechanism yet for a human or later agent to formally dispute a
finding after the fact.

### 12. `qa-engineer`
**Role**: Writes and runs tests covering every acceptance criterion and edge case; fixes bugs it finds.
**Counterbalance**: **Structural contract** (`qa-contract.md`, requires `Failed: 0`) + the `run-tests` skill
enforces the 85% coverage threshold mechanically, not just by qa-engineer's self-report.
**Gap**: Nothing independently reviews whether qa-engineer's tests are actually *meaningful* (verify real
behavior, not tautological assertions) the way `code-reviewer`'s "Test Design Review" does for developer's
TDD tests specifically — qa-engineer's own separate test suite has no equivalent second reviewer.

### 13. `sre-engineer`
**Role**: Observability review — defines SLIs, fixes high-cardinality logging, checks OTel span coverage and
PII hygiene.
**Counterbalance**: **Structural contract** (`observability-contract.md`).
**Gap**: Nothing verifies after deployment whether the SLIs sre-engineer defined were actually achievable or
meaningful in production — that feedback loop doesn't exist yet (it would need to compare defined SLIs
against real monitoring data, which is outside this framework's current scope).

### 14. `tech-writer`
**Role**: Updates all documentation for the delivered feature — CHANGELOG, README, ADRs, API docs.
**Counterbalance**: **Structural contract** (`docs-contract.md`) + tech-writer's own rule ("do not invent
behavior — only document what was actually implemented, verify with the code") is a self-verification
requirement, not an external check.
**Gap**: No agent fact-checks the resulting documentation against the actual implementation after the fact —
this relies entirely on tech-writer's own discipline. See `documentation-manager` below for a related,
partially-overlapping capability.

### 15. `devops-engineer`
**Role**: CI/CD, environment config, deployment artifacts — the final pipeline agent.
**Counterbalance**: **Structural contract** (`devops-contract.md`) + `validate-migrations` (invoked directly,
same as data-engineer) + **human approval gate** (Gate #8, "Deploying to Environment" — deploy requires
explicit "deploy" confirmation; Gate #2 covers the commit itself).
**Gap**: None significant — deploy is one of the most consequential actions in the whole pipeline and is
gated accordingly.

---

## Standalone / On-Demand Agents (not part of `deliver-feature`)

These aren't invoked automatically by the pipeline — they're triggered directly by a human or by a specific
condition (build time SLA breach, release request, etc.). None of them have a `shared/contracts/` entry,
since `validate-artifact` only gates pipeline handoffs.

### 16. `dependency-auditor`
**Role**: Audits the full dependency tree for vulnerabilities, license compliance, and unused packages.
**Counterbalance**: **Human approval gate** on *acting* on findings ("never auto-upgrade without explicit
user approval") — but this only gates the response, not the audit's own accuracy.
**Gap**: No agent or process double-checks the audit's findings themselves (e.g., a CVE dismissed as
inapplicable, or a license miscategorized). The report is trusted as read.

### 17. `release-manager`
**Role**: Analyzes git history since the last tag, applies semantic versioning from conventional commits,
drafts a release plan with rollback procedure.
**Counterbalance**: **Human approval gate** (its own rules: never tag without explicit "tag it"/"approve
release," never force-push, never skip CI) + its own checklist cross-checks that no open Critical/High
security findings exist before release, pulling from `security-reviewer`'s prior work.
**Gap**: None significant — this agent is unusually well-gated for a standalone tool.

### 18. `chaos-engineer`
**Role**: Designs and executes fault-injection experiments to verify the architect's resilience patterns
actually hold up.
**Counterbalance**: **Human approval gate** ("NEVER run chaos experiments against a live production database
without explicit multi-stage approval") for the risky *action*; its own report format requires checking
whether observability actually caught the injected fault, which indirectly validates sre-engineer's earlier
work too.
**Gap**: No dedicated reviewer checks the experiment *design* itself (is the hypothesis reasonable, is the
blast radius actually contained as claimed) before it runs — the approval gate covers the destructive action,
not the design's soundness.

### 19. `dx-engineer`
**Role**: Optimizes local development loop, build pipelines, flaky test quarantine.
**Counterbalance**: Its own guardrail requires measuring before/after impact quantifiably — a self-imposed
check, not external.
**Gap**: No agent reviews whether a DX fix (e.g., quarantining a flaky test) was the right call versus fixing
the underlying flakiness — this is a real tension (quarantine vs. fix) with no independent arbiter.

### 20. `finops-engineer`
**Role**: Reviews architecture/implementation for cost implications — treats cost as a fitness function
alongside latency and uptime.
**Counterbalance**: None formal — its own guardrail ("do not sacrifice resiliency or security to save
fractions of a cent") is self-imposed, not externally checked.
**Gap**: No agent or process verifies finops-engineer's cost estimates against actual cloud billing after
deployment — the "Cost Fitness Function" it proposes is a recommendation for a human to wire up, not
something this framework verifies itself.

### 21. `documentation-manager`
**Role** *(redesigned 2026-07-05, v1.0.0 -> v2.0.0)*: The ad-hoc-session counterpart to `promote-memory` —
extracts durable knowledge from a session that never went through `deliver-feature` (so no `retrospective.md`
exists for `promote-memory` to act on) and produces Candidate Records for human review, exactly like
`promote-memory` does for pipeline deliveries.
**Counterbalance**: **Human approval gate** — same as `promote-memory`, it never writes a KI, ADR, rule
change, or living-doc edit directly; it produces a Candidate Record (Source, Type, Evidence, Tags,
Expiration condition) and only applies the specific approved candidate(s) after explicit sign-off.
**Resolved**: this entry previously flagged a real, undocumented overlap with `memory-engineer`/
`promote-memory`/`extract-lessons`, and a dormant capability (`docs/GOTCHAS.md`, one of its four original
targets, never existed in this repo). Both are fixed now: gotchas route through `create-ki` like any other
KI (no separate file), and the boundary is explicit — `promote-memory` covers pipeline deliveries,
`extract-lessons` covers recurring cross-delivery patterns, `documentation-manager` covers everything else
(ad-hoc sessions with no `retrospective.md`). Deliberately still manual/on-demand, not hooked to run after
every session — see `docs/runbooks/context-engineering.md`'s Learning section for why that would be its own
kind of over-engineering.

### 22. `memory-auditor`
**Role**: Read-only counter-agent for the KI corpus — audits `shared/knowledge/` and optional
`.claude/knowledge/` entries for schema compliance, duplicate candidates, semantic overlap, and stale
metadata.
**Counterbalance**: Its own read-only tool boundary (`Read`, `Glob`, `Grep`) prevents it from acting on
findings directly; any merge, deletion, expiration, or registry edit still routes through a human or
`memory-engineer` with the normal approval gates.
**Gap**: Semantic duplicate detection is judgment-heavy. The agent can flag overlap candidates, but no
automated fitness function proves two KIs really encode the same reusable lesson without human review.

### 23. `modernization-supervisor`
**Role**: Coordinates three parallel modernization workstreams (dependency updates, pattern refactors, test
coverage) across a legacy codebase.
**Counterbalance**: Its own process requires running full-suite integration tests after coordinating the
three workstreams, and producing a risk-assessed merge strategy — an implicit **human approval gate** on the
merge itself (nothing in its own rules says it merges without asking).
**Gap**: No dedicated reviewer checks the *coordination* quality itself (did it actually prevent the 3
sub-workstreams from conflicting) beyond the integration test suite passing.

### 24. `api-test-generator`
**Role**: Generates Sunday Framework API test suites (Playwright + Vitest + Zod) from a spec or OpenAPI doc.
**Counterbalance**: If invoked as part of a full feature delivery, `code-reviewer`/`qa-engineer` would still
review the generated tests as part of that pipeline. If invoked standalone (its primary use case — "generate
API tests" on demand), there is none.
**Gap**: Standalone invocations have no formal check on the generated tests' quality beyond a human reading
the `api-test-report.md`.

### 25. `test-driven-developer`
**Role**: An autonomous, alternate red-green-refactor loop — explicitly authorized to iterate without asking
for permission between steps, working directly from user-provided acceptance criteria rather than through
`deliver-feature`.
**Counterbalance**: The test suite itself (mechanical — tests pass or they don't). As of v1.1.0, also a
`search-ki` lookup before test design and a `documentation-manager` recommendation (not auto-invoked) after
a substantial session — see the 2026-07-13 CHANGELOG entry.
**Gap — significant, and worth understanding clearly before choosing this over the full pipeline**: this
agent deliberately bypasses every other counterbalance the normal pipeline has. No `code-reviewer` catches a
SOLID violation, no `security-reviewer` catches a STRIDE-class vulnerability, no `accessibility-engineer`
catches a semantic-HTML violation — none of that review happens unless a human separately runs it after the
fact. This is a real, deliberate speed-for-safety tradeoff (autonomy without pausing), not an oversight — but
it should be a conscious choice, not a default one, given how much of this framework's value elsewhere comes
from exactly the reviews this agent skips. (This gap is about *review*, not *memory* — the memory-side gap,
starting cold with no KI lookup and never routing learnings back afterward, was closed in v1.1.0.)

### 26. `unit-tester`
**Role**: The mirror image of `test-driven-developer` — writes unit tests for existing code without
modifying it, either to raise coverage on already-trusted code or to build a Michael Feathers-style
characterization-test safety net before a legacy refactor or migration. Standalone, not gated behind
`deliver-feature`.
**Counterbalance**: Unlike `test-driven-developer`, this one isn't just a soft recommendation — the
`backfill-unit-tests` skill auto-chains a real `code-reviewer` pass against the new test files every time,
because (unlike `documentation-manager`, where most sessions produce nothing worth promoting) a code-quality
check on newly-written tests is cheap and useful every single time, not just sometimes.
**Gap**: Still no `security-reviewer` or `accessibility-engineer` pass — same review-chain bypass
`test-driven-developer` has, scoped down slightly since this agent only ever produces test files, never
production code. If a seam is genuinely required to make legacy code testable, that's explicitly *not*
something this agent can do unilaterally — it's gated behind `approval-gates.md` gate #6, reported and
held rather than performed.

### 27. `refactor-engineer`
**Role**: Large-scale or multi-target structural refactoring — complexity violations flagged by
`health-check.sh`, framework migrations, Boy Scout Rule debt from code-review, or an explicit modernization
sprint. Invokes `context-engineer` as Step 0 to scope the bounded context and surface ADR constraints
and prior retrospective lessons before touching code. Then builds a characterization-test safety net via
`unit-tester`, applies named Fowler operations (Extract Function, Replace Conditional with Polymorphism,
etc.) to lower cyclomatic complexity and eliminate duplication, verifies behavior preservation with a
passing test suite, and produces `refactoring-notes.md` validated by `refactoring-contract.md`.
**Counterbalance**: **Structural contract** (`refactoring-contract.md` + `validate-artifact`) guards
the output; `unit-tester` provides the behavioral safety net that makes the refactor safe. The "no
behavior added in the same run" rule is an attestation in the contract itself, not just convention.
**Gap**: No `code-reviewer` auto-chained after the refactor (unlike `backfill-unit-tests`). The contract
validates structure and attestation, but a human should review the diff before committing if the scope
is large — the complexity tool output is a mechanical check, not a substitute for a code-quality read.

---

## Late-Addition Pipeline Agent

`visual-qa-engineer` was added as part of Epic 46 (Saturday visual-testing expansion) after the Pipeline
Agents section above was finalized. It runs after `qa-engineer` in the `deliver-feature` pipeline on
UI-touching features with heatmap or screenshot baseline data, and is documented here to preserve the
existing numbering of agents 1–27.

### 28. `visual-qa-engineer`
**Role**: Extends `qa-engineer`'s functional coverage into visual and interaction dimensions. Analyzes
Saturday heatmaps (via `@orieken/saturday-ml-analyzer` on `heatmap-data/`) for cold spots on primary
journey elements and Playwright screenshot baselines for pixel-level regressions. Produces
`visual-qa-report.md`. Conditionally invoked — only when `heatmap-data/` or Playwright baselines exist;
outputs `UNCONFIGURED` and exits without blocking the pipeline when neither is present.
**Counterbalance**: **Structural contract** (`visual-qa-contract.md`) via `validate-artifact`. The
conditional-exit rule ("UNCONFIGURED if no heatmap data AND no baselines") prevents false-green reports on
projects that haven't adopted the Saturday heatmap fixture. `qa-engineer`'s report is the upstream signal
this agent extends, so gaps in functional coverage propagate into (and are sometimes surfaced by) the
visual layer.
**Gap**: Heatmap analysis requires the project to have adopted `@orieken/saturday-playwright-heatmap`. On
projects that haven't — the majority — `visual-qa-engineer` produces only UNCONFIGURED with no actionable
signal. No aggregate metric currently tracks whether its coverage scores are actually catching real visual
regressions over time.

---

## AOS Counter-Auditor Agents (v3.0 - v3.1)

The following 11 read-only counter-auditor agents implement the opposing-force checks in
`docs/aos/governance-pairs.md`. All 11 are `light` model tier, read-only (`Read`, `Glob`, `Grep`), and
never mutate files — each produces an audit findings report for a human or upstream agent to act on.
(`memory-auditor`, the counter to `memory-engineer`, is documented at §22 under Standalone Agents above.)

### 29. `context-auditor`
**Role**: Counter to `context-engineer`. Audits `.claude/feature-workspace/<feature-name>/context-manifest.md`
after a delivery — checks for pinned files never read by downstream agents (context bloat), broken
KI/ADR/source-file paths, and budget calculation inaccuracies.
**Counterbalance**: Read-only tool boundary (`Read`, `Glob`, `Grep`) prevents acting on findings directly;
fixes route through a human or `context-engineer`. `context-manifest-contract.md` via `validate-artifact`
gates the manifest's *structure* at creation time; `context-auditor` checks the *semantic quality* of what
was actually pinned — whether it helped or bloated downstream context windows.
**Gap**: Can only audit post-delivery. Findings arrive too late to affect the current feature's context
window — they are improvement data for the next delivery, not a live correction.

### 30. `knowledge-auditor`
**Role**: Counter to the `create-ki` skill. Audits newly authored Knowledge Items for frontmatter schema
compliance (against `ki-frontmatter.schema.json`), semantic duplication against the existing KI corpus, and
domain dictionary alignment.
**Counterbalance**: Read-only tool boundary + `ki-frontmatter.schema.json` as a deterministic structural
reference. Schema compliance is a mechanical check; semantic duplicate detection is judgment-heavy — the
agent flags candidates, humans decide.
**Gap**: Two KIs encoding the same reusable pattern in different words can slip through. The agent catches
structural overlap and keyword matches; purely conceptual duplicates require human review to confirm.

### 31. `prompt-evaluator`
**Role**: Counter to prompt authors (anyone editing `shared/agents/*.md` or `shared/skills/*/SKILL.md`).
Audits for prompt-engineering hygiene — fabricated URLs in examples, hardcoded secrets, un-decoupled
template examples that embed project-specific paths, and inconsistent voice across the agent's sections.
**Counterbalance**: Read-only tool boundary. `shared/rules/memory-trust-boundary.md` independently
constrains what agents can act on from KI/ADR content downstream — `prompt-evaluator` audits authoring
quality up-front so those downstream constraints don't need to rescue a poorly written prompt.
**Gap**: Prompt evaluation is judgment-heavy. No automated fitness function proves a prompt's examples are
fully decoupled or free of fabricated URLs — detection requires reading comprehension, not grep. A
`prompt-evaluator` finding is only as trustworthy as the run that produced it, which is not independently
checked.

### 32. `agent-evaluator`
**Role**: Promotes the `agent-eval` skill's golden-file evaluation logic into a dedicated agent persona.
Runs evaluations against `shared/agents/` frontmatter contracts and prompt behavior expectations using
fixture inputs from `tests/agents/<agent>/`, logging regression metrics to `shared/evaluation/`. Counter to
the entire agent authoring process — not a single upstream agent.
**Counterbalance**: Read-only tool boundary + shared fixture format with `tests/agents/` (the same inputs and
contract checks used by `scripts/test-agents.sh` and `scripts/run-agent-evals.sh`). The interactive skill
and the headless harness form a coherent regression safety net: `agent-evaluator` for spot-checks, the
harness for batch sweeps.
**Gap**: `agent-evaluator`'s grading quality is not independently verified — no meta-evaluator audits its
verdicts. Golden-file evals also only cover what fixture inputs exist; novel failure modes not covered by
any current fixture will be missed.

### 33. `rule-auditor`
**Role**: Counter to rule authors (anyone editing `shared/rules/*.md`). Audits for internal consistency —
contradictory constraints across files, dead path references (a rule cites a file that no longer exists on
disk), and un-indexed rule files that would be invisible to agents loading only the indexed set.
**Counterbalance**: Read-only tool boundary. `shared/rules/architecture-guardrails.md` documents which
constraints are HARD and cannot be overridden — `rule-auditor` can flag when a later rule *appears* to
contradict a hard constraint, surfacing the conflict for human resolution.
**Gap**: Cross-rule contradiction detection requires understanding intent. Two rules can appear contradictory
in wording but resolve clearly in practice (e.g., "NEVER use X" and "use X only at the adapter layer").
The agent flags apparent contradictions; humans resolve them.

### 34. `pattern-reviewer`
**Role**: Counter to pattern document authors (`docs/patterns/*.md`). Audits pattern docs for accuracy against
the current codebase — stale code snippets (class or function names that were renamed or removed), broken
file paths, and obsolete architectural references.
**Counterbalance**: Read-only tool boundary. Stale-snippet detection is partly deterministic: `Grep` checks
whether a named class or function still exists; `Read` verifies the signature hasn't changed. Path
existence checks are fully deterministic.
**Gap**: Catches renamed or removed artifacts better than semantically drifted ones. A method whose behavior
changed but kept its name won't be flagged by grep — the pattern doc could describe a no-longer-valid
behavior using a still-valid class name.

### 35. `tool-validator`
**Role**: Counter to skill authors (anyone editing `shared/skills/*/SKILL.md`). Audits skills for
standalone-mode declaration, hidden MCP dependencies (a skill claims `Read`/`Glob`/`Grep` but its body
pipes through an MCP tool not listed in `tools:`), frontmatter schema compliance, and valid parameter
declarations.
**Counterbalance**: Read-only tool boundary. Frontmatter schema (if defined in `shared/schemas/`) provides a
deterministic reference for structural checks; dependency detection is judgment-based, requiring a read of
the skill body compared against the declared `tools:` list.
**Gap**: A skill that hides an MCP dependency behind an example rather than a direct invocation can slip
through pattern matching. No automated fitness function enforces that the declared `tools:` list is
exhaustive.

### 36. `documentation-auditor`
**Role**: Counter to `tech-writer` and prose documentation authors. Audits `README.md`,
`docs/AGENT_REFERENCE.md`, and `docs/prompts/README.md` for staleness — stale counts, un-indexed agents,
and deprecated skill references. Writes findings to `docs/audits/doc-audit-YYYY-MM-DD.md`.
**Counterbalance**: Read-only tool boundary + `scripts/check-inventory-drift.sh` (CI script detecting
numeric count mismatches — the mechanical layer). `documentation-auditor` adds the semantic layer: un-indexed
agents and deprecated references that aren't just count mismatches. `health-check.sh` warns when the newest
doc-audit file is older than 14 days. Three automation paths: (a) on-change hook in `shared/hooks/examples/`;
(b) weekly cron via the `scheduler` skill; (c) freshness nudge from `health-check.sh`.
**Gap**: Catches presence vs. absence in reference docs, but not quality. An AGENT_REFERENCE entry that exists
but is factually wrong about the agent's current behavior won't be caught by count or index checks alone.

### 37. `retrieval-evaluator`
**Role**: Counter to retrieval skills and the RAG engine. Audits KI and ADR corpus retrievability using
ADR-002 telemetry and `memory-registry.json` — flags zero-match queries as missing-KI or bad-metadata
candidates, runs the approved regression set in `shared/evaluation/retrieval-regression.md`, and proposes
new regression cases from telemetry patterns.
**Counterbalance**: Read-only tool boundary + `shared/evaluation/retrieval-regression.md` as a structured,
versioned regression set (the retrieval equivalent of `tests/agents/` golden files — same philosophy applied
to retrieval quality). ADR-002 telemetry provides external signal, not just self-referential checks.
**Gap**: Only as good as the telemetry. On a new install with sparse query history, the evaluator has nothing
to evaluate beyond the static regression set. It catches known past failure modes; novel retrieval gaps
require real query volume to surface.

### 38. `privacy-auditor`
**Role**: Counter to `security-reviewer` and developers. Audits pipeline artifacts in
`.claude/feature-workspace/<feature-name>/` — *not source code* — for accidental PII in prompt examples,
hardcoded tokens or passwords in implementation notes, and data boundary leaks (real data referenced in
what should be synthetic context).
**Counterbalance**: Read-only tool boundary. Deliberate scope distinction from `security-reviewer`: that
agent audits *source code* for STRIDE threats; `privacy-auditor` audits *workspace artifacts* for PII and
credential contamination introduced during the delivery process — a different layer, different threat surface.
**Gap**: PII detection in free-text artifacts is pattern-matching plus judgment. Redacted, encoded, or hashed
PII can slip through. The agent detects obvious, un-obscured PII more reliably than deliberately or
accidentally obfuscated forms.

### 39. `model-tier-auditor`
**Role**: Counter to agent authors. Scans `shared/agents/*.md` for missing `model_tier` frontmatter fields,
invalid enum values (anything other than `light` / `default` / `heavy`), and tier assignments that mismatch
the agent's operational profile — e.g., a read-only counter agent claiming `heavy` when `light` is clearly
sufficient.
**Counterbalance**: Read-only tool boundary. `model_tier` is the portable tier declaration consumed by
`scripts/run-agent-evals.sh` and model-resolution logic. A missing field causes the harness to silently
fall back to `inherit`, using whichever model the user has configured rather than the author's intended
tier — `model-tier-auditor` catches that drift before a harness run encounters it silently.
**Gap**: Heuristic mismatch detection is judgment-heavy. The auditor can flag obvious mismatches (a Grep-only
counter agent claiming `heavy` tier), but boundary cases — when is `default` better served as `light`? —
require understanding the agent's actual inference needs, not just its tool list.

### 40. `exemplar-auditor`
**Role**: Counter to the exemplar set a project declares in `.claude/exemplars.yaml`. Checks each entry
against `shared/contracts/exemplar-contract.md` — the test still exists and runs, still carries its
exemplar and issue/AC annotations, still has a confirmed digest, and still demonstrates what the manifest
claims it demonstrates.
**Counterbalance**: Read-only tool boundary, and a different competence from the auditors around it. An
exemplar is a few-shot prompt delivered through the filesystem: every test written after it inherits its
pattern, so a stale exemplar is worse than none. `memory-auditor` sweeps the same manifest as a registry
source and would confirm it is well-formed — which a beautifully-registered vacuous test also is. This
agent asks the question that matters instead: would this test fail if the behavior it names were broken,
and would someone copying it learn what it promises?
**Gap**: It is read-only, so it cannot run the mutation that would settle vacuity mechanically — it answers
from control flow and assertions, and says UNCERTAIN plus the file it would need rather than guessing.
`backfill-unit-tests` step 6 runs the real mutation, in a throwaway worktree, and the two are complementary
rather than redundant.

---

## What this survey actually shows

Reading all 40 agents together, three patterns stand out:

1. **The 14 pipeline agents are well-checked** — the fourteen the delivery plan invokes, per "What counts as
   a pipeline agent" above, which includes `visual-qa-engineer` and excludes the pre-pipeline `spec-writer`
   and `product-owner`. Every one has at least a structural contract; most have a real downstream reviewer
   or a human approval gate too, and three now sit behind gates the executor enforces in code rather than
   by instruction. This is the part of the framework that has had the most iteration.
2. **The 12 standalone agents are inconsistently checked** — though less inconsistently than before
   `unit-tester` arrived with a real auto-chained reviewer rather than just a report a human might read.
   Some (`release-manager`, `data-engineer`-adjacent
   `devops-engineer`) are well-gated because they touch genuinely irreversible actions. Others
   (`finops-engineer`, `dx-engineer`, `dependency-auditor`) have no check beyond a human reading the report —
   which may be the right amount of ceremony for their risk level, or may not; that's a judgment call this
   doc surfaces rather than settles.
3. **`documentation-manager` vs. the memory-engineering skills was a real boundary problem, now resolved**
   (see its entry above — redesigned as the ad-hoc-session counterpart to `promote-memory`, same Candidate
   Record contract, `GOTCHAS.md` retired in favor of KIs). `test-driven-developer`'s bypass of the whole
   review chain is still a real, significant tradeoff, deliberately left as-is — worth a conscious choice
   each time it's reached for, not a default one.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
