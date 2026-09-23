# Domain Dictionary

This file is the single source of truth for the **Ubiquitous Language** in the ai-assistant-dot-files ecosystem. All agents, skills, rules, and documentation MUST use exactly these terms. Do not invent synonyms.

**Rule:** When a new concept is introduced (via a new agent, skill, or rule), it MUST be added here before merging.

---

## 1. Core Domains (Bounded Contexts)

- **Agent Orchestration**: Coordination of specialized AI agents (or personas, on lower-tier platforms) through a sequential delivery pipeline with human checkpoints. Canonical definitions live in `shared/`.
- **Craftsmanship Governance**: Enforcement of clean code, architecture, and testing standards across all AI assistants and generated code.
- **Test Automation**: Automated verification of software quality through the Saturday Framework (E2E/UI) and Sunday Framework (API).
- **Feature Delivery**: The end-to-end lifecycle of taking a feature spec through analysis, implementation, review, and deployment.
- **Documentation Knowledge Base**: Persistent, RAG-friendly documentation that serves as context for both agents and humans.

---

## 2. Entities (Identifiable Objects)

| Term | Domain | Description | Synonyms to AVOID |
|---|---|---|---|
| **Persona** | `Agent Orchestration` | A context frame that shapes an AI model's identity, tone, and focus areas. A persona has no tool access and no autonomous workflow. It is a component of an Agent. Used on Tier 2/3 platforms (Cursor, Copilot, Gemini) where native orchestration is unavailable. Canonical definitions live in `shared/agents/`. | `profile`, `role` (too generic), `character` |
| **Agent** | `Agent Orchestration` | A Persona enhanced with tool access, an autonomous process, and pipeline participation. An Agent can take actions, produce Artifacts, and hand off to the next Agent in a Pipeline. Used on Tier 1 platforms (Claude Code). Canonical definitions live in `shared/agents/`. | `bot`, `assistant`, `model` |
| **Capability Tier** | `Agent Orchestration` | Classification of an AI platform's support level. **Tier 1 (Full)**: agents, skills, rules, hooks, orchestration (Claude Code). **Tier 2 (Personas + Rules)**: persona-level context and rule files, no orchestration (Windsurf, GitHub Copilot — Copilot upgraded from Tier 3 in 2026-07 once its path-scoped `.github/instructions/*.instructions.md` support was confirmed). **Tier 3 (System Prompt)**: single instruction file only (OpenAI); Gemini/Antigravity is also Tier 3 by rules mechanism (single `AGENTS.md`, confirmed 2026-07-02 via live testing) but has real skill invocation on top, closer to Tier 1 on that one axis. **Cursor is a mixed profile, not a clean single tier** (confirmed 2026-07-06): `.cursor/agents/`/`.cursor/skills/` are genuinely Tier-1-equivalent (real subagent + skill loading, symlinked directly to `shared/`, zero drift) while `.cursor/rules/` remains Tier 2 (still fully inlined, no orchestration at that layer) — the granular `capabilities` object in `shared/platform-registry.json` is authoritative per capability; the single `tier` number is a rough label only. See `shared/platform-registry.json` for the full confirmed/unconfirmed breakdown. Defined in `shared/platform-registry.json`. | `level`, `grade` |
| **Skill** | `Agent Orchestration` | An executable, on-demand capability triggered by keywords or slash commands. Lives in `shared/skills/`. | `tool`, `command`, `action`, `plugin` |
| **Rule** | `Craftsmanship Governance` | A governance document that constrains agent behavior. Lives in `shared/rules/`. | `policy`, `guideline`, `standard` |
| **Team Type** | `Craftsmanship Governance` | Skelton & Pais's classification of a team's structural role: Stream-aligned, Platform, Enabling, or Complicated-subsystem. Registered per Bounded Context in `TEAM_TOPOLOGY.md`. | `team role` |
| **Interaction Mode** | `Craftsmanship Governance` | Skelton & Pais's classification of how two teams work together at a Bounded Context crossing: Collaboration, X-as-a-Service, or Facilitating. Registered per crossing in `TEAM_TOPOLOGY.md`; checked by `architect` and the `team-topology-check` skill. | `communication style` |
| **Agent Eval Case** | `Agent Orchestration` | A fixed input fixture plus a qualitative rubric used to check whether an agent's prompt still produces correct *reasoning* (not just the right section headings) after an edit. Lives in `tests/agents/<agent-name>/` (fixture + `expected-patterns.txt`, structural) and `eval-rubric.md` (qualitative), run via the `agent-eval` skill. Distinct from a unit test — the checker is an LLM reading the rubric, not a deterministic assertion. | `prompt test`, `golden file` (that term refers to the structural check only) |
| **Exemplar Test** | `Test Automation` | A real, executing test in a project's own suite, marked as the one to imitate when writing a new test of that language and level. Marked in place with a language-native annotation and declared in `.claude/exemplars.yaml`; audited by `exemplar-auditor` against `shared/contracts/exemplar-contract.md`. Distinct from an **Agent Eval Case**'s golden file, which is a recorded baseline compared *against* — an exemplar is a specimen copied *from*. | `golden test` (reserved — see Agent Eval Case), `sample test`, `reference test`, `example test` (too generic) |
| **Feature Spec** | `Feature Delivery` | A structured markdown document describing a work item. Lives in `features/`. | `ticket`, `story`, `issue`, `task` |
| **Pipeline** | `Feature Delivery` | The sequential chain of agents that processes a feature spec into shipped code. | `workflow`, `flow`, `process` |
| **Artifact** | `Feature Delivery` | A markdown document produced by an agent during pipeline execution. Persisted to `docs/features/<name>/`. Individual artifacts are legitimately named with a `-report.md` suffix per their content (`security-report.md`, `qa-report.md`, etc.) — that's a filename convention for a specific artifact type, not a synonym for the umbrella term "Artifact" itself. | `output`, `result` |
| **Feature Workspace** | `Feature Delivery` | The temporary working directory (`.claude/feature-workspace/`) used during pipeline execution. | `scratch`, `temp`, `staging` |
| **Context Manifest** | `Feature Delivery` | `context-engineer`'s output artifact: scope/boundaries, pinned files, global rules, KIs/ADRs, prior deliveries in the same Bounded Context, prune recommendations, and token budget. Lives at `.claude/feature-workspace/<feature-name>/context-manifest.md`, structurally validated by `shared/contracts/context-manifest-contract.md`. | `context file`, `manifest` (too generic alone) |
| **Pipeline State** | `Feature Delivery` | The resumability checkpoint file (`.claude/feature-workspace/<feature-name>/pipeline-state.json`) `deliver-feature` writes after every Checkpoint step — which agents completed, their artifact checksums, contract status. Read by `resume-pipeline` to continue an interrupted run. | `progress file`, `checkpoint` (too generic alone) |
| **Pipeline Trace** | `Feature Delivery` | The per-agent timing/status/iteration-count file (`pipeline-trace.json`, schema owned by the `pipeline-trace` skill) `deliver-feature` writes alongside `pipeline-state.json`. Read by `pipeline-retrospective` and `agent-scorecard` for cross-delivery trend analysis. Distinct from Pipeline State — one is for resuming a run, the other for analyzing many runs after the fact. | `log file`, `timing data` |
| **Run Trace** | `Feature Delivery` | The OpenTelemetry trace `loom run` emits for one run (`traces.jsonl` beside `run-state.json`, OTLP/JSON): a root span, a child per stage, and a grandchild per model invocation. Answers how long each stage took and what it cost. Distinct from the Run Event Timeline, which is the audit record of gates, digests and staleness and stays readable with no collector configured — and from Pipeline Trace, which is a model's own estimate. | `otel trace`, `spans`, `timing file` |
| **Reported Usage** | `Feature Delivery` | The token counts and dollar cost a provider states for one model invocation, recorded verbatim on the stage and on its span. Named to fix the distinction that matters: loom **reports** what the provider says it was charged and never **computes** a price, because a price table here would be wrong within a quarter while sounding authoritative. Absent Reported Usage is not zero usage — a provider that reported nothing is a different fact from one that reported zero. | `cost`, `token count`, `spend` |
| **Human Correction** | `Feature Delivery` | A human's edit to a stage's output at a gate, recorded by the executor with a unified diff and attributed to the **producing agent** rather than to the gate that caught it (`artifact.corrected`). Taken from the **rendered view**, which the pipeline never reads back — so a Human Correction is *advisory*: it is recorded, not adopted, and the next render overwrites the file. Editing a tracked artifact instead re-runs its stage, which is a different act. | `edit`, `fix`, `override` |
| **Policy Decision** | `Agent Orchestration` | What the delivery policies watching a gate collectively decided, evaluated by `internal/policy` from the run's own state and recorded as `policy.evaluated` plus an entry in `run-state.json`. A decision is **recorded, not honoured**: a matching `auto-approve` policy does not open the gate, and every record carries `honoured: false` until roadmap L2.19 changes that. A condition the run cannot answer resolves to *unknown*, never to true, and the decision names the fact it could not see. | `policy result`, `auto-approval` |
| **Episodic Store** | `Documentation Knowledge Base` | The project-local sqlite record of what runs actually did (`.claude/memory/episodes.db`): stages, timings, loop rounds, gate halts, human corrections, and reported cost. Distinct from the semantic memory of KIs and ADRs, which record what someone concluded rather than what happened. It is a **projection**, not the record — `run-state.json` and `run-events.jsonl` archived into the Feature Archive are the durable copy, and `loom memory ingest` rebuilds the store from them. | `run history`, `episodes.db`, `memory` (too generic alone) |
| **Feature Archive** | `Documentation Knowledge Base` | The permanent directory (`docs/features/<name>/`) where pipeline artifacts are persisted after delivery. | `output folder`, `results` |
| **Memory Registry** | `Documentation Knowledge Base` | The catalog of every durable memory source (KIs, ADRs, feature archive, lessons-learned, `DOMAIN_DICTIONARY.md`, `TEAM_TOPOLOGY.md`) with owner, portability, and retrieval-backend metadata. Lives at `shared/memory-registry.json`. Read by `search-ki`, `query-memory`, and `memory-engineer` before they act. | `memory index`, `knowledge catalog` |
| **Candidate Record** | `Documentation Knowledge Base` | A structured proposal (Source, Type, Evidence, Tags, Expiration condition) that a piece of captured knowledge should be promoted into a KI, ADR, rule change, living-doc update, or lesson. Produced by `promote-memory` (pipeline deliveries), `extract-lessons` (recurring cross-delivery patterns), or `documentation-manager` (ad-hoc sessions `promote-memory` never saw) — never written directly to `shared/knowledge/` without one. See `docs/runbooks/memory-engineering.md`'s Memory Contract. | `memory candidate`, `promotion candidate` |
| **Memory Sweep** | `Documentation Knowledge Base` | A periodic `memory-engineer` audit of the KI corpus for duplicates, overlaps, and expiration candidates, plus a `shared/memory-registry.json` accuracy check. Produces recommendations only — never deletes or merges a file without human approval. | `memory audit`, `KI cleanup` |
| **Blueprint Prompt** | `Test Automation` | A foundational prompt that establishes framework conventions for E2E or API test generation. | `template`, `boilerplate` |
| **Approval Gate** | `Craftsmanship Governance` | A mandatory human checkpoint before an irreversible action. Declares a reset condition: an edit to the approved work withdraws the approval. Enforced in code for an **Executor Gate**; prompt-discipline for the nine prose gates in `shared/rules/approval-gates.md`. | `confirmation`, `sign-off` |
| **Executor Gate** | `Agent Orchestration` | A pre-stage barrier the Go executor refuses to cross until run state records a human approval (`confirm-design`, `confirm-security`, `confirm-ship`). Approval arrives only through a CLI channel — nothing an agent returns can create one — and binds to the artifact digests it was given, so an edit resets it. | `process gate`, `hard gate` |
| **Run State** | `Agent Orchestration` | The executor-owned durable record of one delivery run (`.claude/feature-workspace/<feature>/run-state.json`): per-stage status and sequence, artifact digests computed in Go, and recorded approvals. Written by `loom run`, or by `loom state` for markdown-pipeline runs. Distinct from **Pipeline State**, the prompt-owned fallback. | `run-state.json`, `executor state` |
| **Delivery Route** | `Agent Orchestration` | The decision about which pipeline stages a run executes, computed from the typed analysis immediately after the analyst and recorded as one row per stage with the reason. Approved by a human at the design gate along with the design; editing it withdraws that approval. Review stages are never routed around. | `route`, `route.md` |
| **Review Loop** | `Agent Orchestration` | The bounded iteration in which the code-reviewer sends the developer back until it approves. Declared as a span of stages with a named condition and a maximum number of rounds; each round's artifacts are retained and digested. Exhausting the bound halts for a human rather than failing or proceeding. | `CHANGES REQUESTED loop`, `review iteration` |
| **Run Event Timeline** | `Agent Orchestration` | The append-only log of what happened during a run and when (`run-events.jsonl`): stage transitions, gate halts, approvals and their invalidation, and integrity failures, each timestamped by the process doing the work. Read with `loom state timeline`. Its event types are a Go enum from which `shared/schemas/telemetry/run-event.schema.json` and the run event types table are generated, so neither can drift from what the executor emits (roadmap L3.9). A local audit log: it stays readable with no collector configured, which is the property distinguishing it from a Run Trace. | `run-events.jsonl`, `event log` |
| **Fitness Function** | `Craftsmanship Governance` | An automated, measurable check that verifies an architectural property in CI. | `lint rule`, `check`, `validation` |

---

## 3. Value Objects (Immutable Concepts)

| Term | Domain | Description | Example |
|---|---|---|---|
| **Delivery Summary** | `Feature Delivery` | The final synthesis artifact produced by the orchestrator after all agents complete. | `docs/features/<name>/delivery-summary.md` |
| **Readiness Critique** | `Feature Delivery` | The spec-writer's structured evaluation of whether a feature spec is ready for the pipeline. | Verdict: READY or NEEDS WORK |
| **Human Checkpoint** | `Feature Delivery` | A pause point in the pipeline where explicit user confirmation is required before proceeding. | After analyst, after architect RFC |
| **Cyclomatic Complexity Threshold** | `Craftsmanship Governance` | The maximum allowed cyclomatic complexity per function: 7. | `complexity < 7` |
| **Coverage Threshold** | `Craftsmanship Governance` | The minimum required unit test coverage: 85%. | `coverage >= 85%` |

---

## 4. Domain Events

| Event Name | Domain | Triggered When |
|---|---|---|
| **SpecReady** | `Feature Delivery` | The spec-writer's readiness critique verdict is READY. |
| **AnalysisComplete** | `Feature Delivery` | The analyst produces `analysis.md` and the user confirms scope. |
| **ArchitectureApproved** | `Feature Delivery` | The architect's structural decisions are confirmed by the user. |
| **CodeReviewApproved** | `Feature Delivery` | The code-reviewer's verdict is APPROVED (no more feedback loops). |
| **SecurityCleared** | `Feature Delivery` | The security-reviewer finds no Critical findings, or all Critical findings are resolved. |
| **PipelineComplete** | `Feature Delivery` | All agents have produced their artifacts and the delivery summary is written. |
| **ArtifactsPersisted** | `Documentation Knowledge Base` | All pipeline artifacts are copied from feature workspace to `docs/features/<name>/`. |
| **ShippedToFriday** | `Feature Delivery` | The Cucumber JSON summary is POSTed to the Friday dashboard after user approval. |

---

## 5. Operations / Actions

- **Deliver**: Execute the full agent pipeline for a feature spec. Do not use "build", "run", or "process" when referring to pipeline execution.
- **Scaffold**: Deploy template files into a target project directory. Do not use "copy", "install", or "bootstrap" for this operation.
- **Persist**: Copy pipeline artifacts from the feature workspace to the permanent feature archive in `docs/features/`. Do not use "save", "export", or "archive" as verbs for this operation.
- **Critique**: The spec-writer's evaluation of a feature spec against agent readiness criteria. Do not use "review" (reserved for code-reviewer) or "audit" (reserved for dependency-auditor).
- **Profile**: The performance-engineer's diagnostic analysis of a slow system. Do not use "benchmark" (which implies measurement only, not diagnosis).

---

## 6. Framework Terms

### Saturday Framework (E2E / UI Testing)

| Term | Description | Synonyms to AVOID |
|---|---|---|
| **BaseSite** | Root orchestrator for a web application under test. | `App`, `Application` |
| **BasePage** | Represents a single page within a BaseSite. | `PageObject`, `Screen`, `View` |
| **BaseElement** | A reusable UI component abstraction within a single BasePage. | `Component`, `Widget` |
| **BasePartial** | A shared UI section (header, footer, global nav) that appears across multiple BasePages. Lives in a `partials/` subdirectory; deliberately does NOT follow the `FooPage` naming convention. Fills the gap `BaseElement` doesn't cover (cross-page reuse). | `SharedComponent`, `LayoutFragment`, `PartialPage` |
| **BaseFlow** | A multi-step user journey that spans multiple pages. | `Workflow`, `Scenario`, `Journey` |
| **SiteManager** | Manages cross-application test contexts. | `AppManager`, `ContextManager` |
| **TabManager** | Manages multi-tab browser contexts within a test. | `WindowManager`, `BrowserManager` |

### Sunday Framework (API Testing)

| Term | Description | Synonyms to AVOID |
|---|---|---|
| **BaseApiClient** | Abstract base for all domain-specific API clients. | `HttpClient`, `RestClient` |
| **IHttpAdapter** | Interface hiding HTTP implementation details from test logic. | `HttpService`, `RequestHandler` |
| **api fixture** | The custom Playwright fixture providing fluent API testing. | `client`, `request` |

---

*When designing a new agent, skill, or rule: if a new business concept is introduced, add it to this dictionary before merging.*
