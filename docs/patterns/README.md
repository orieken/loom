# Reusable Patterns

This directory documents the design and architectural patterns used across the ecosystem — the
structure, a concrete example, and the trade-offs each pattern brings, not just its name.

---

## Available Pattern Docs

- [gang-of-four-patterns.md](gang-of-four-patterns.md) — the 7 patterns in `CLAUDE.md`'s decision table
  (Factory, Builder, Strategy, Observer, Adapter, Decorator, Command) with the structure/example/
  trade-off detail that table doesn't have room for, plus 3 more (Template Method, Composite, State)
  named for recognition because this codebase's own mechanisms already use them, unnamed.
- [saturday-framework-patterns.md](saturday-framework-patterns.md) — the **Site-Centric Pattern** (named
  as its own entry) plus every component that implements it, organized top-down: `BaseSite` (the
  orchestrator), `BasePage` (the anatomy) with its sub-parts `BaseElement`, `Filters`, and `BasePartial`
  (shared cross-page UI like headers/footers/global nav — dedicated concept, not `BaseElement` overloaded),
  `BaseFlow` (multi-page journeys, with the Moist testing principle folded into its trade-offs), `Model`
  and `Factory` (test data flowing through everything, applying this framework's per-language factory
  conventions), plus `SiteManager` and `TabManager` as coordinators.
- [sunday-framework-patterns.md](sunday-framework-patterns.md) — the **Declarative API Client Pattern**
  (named as its own entry) plus every component that implements it, organized top-down: `BaseApiClient`
  (per-domain named operations), `IHttpAdapter` (the swappable HTTP layer), Fluent Matchers + Schema
  Validation (declarative assertions with Zod as both Model and validator), Resilience Primitives
  (`CircuitBreaker`, `ExponentialBackoffStrategy`), `Model` and `Factory` (Zod-typed request/response
  shapes and their producers), `Mock Server` (Test Doubles — route interception / provider stub — for
  isolating from real upstreams while exercising failure paths), and the **API Test Coverage Matrix**
  (per-HTTP-method baseline scenarios — `GET` → happy/not_found/network, `POST` →
  happy/server/validation, etc. — the language-agnostic single source of truth that
  `sunday-test-advisor` audits go-sunday specs against and future per-language advisors will reference).
- [clean-architecture-layers.md](clean-architecture-layers.md) — Domain, Use Case, Adapter, and
  Framework layers, expanding on the dependency-direction rule in
  `shared/rules/architecture-guardrails.md`.
- [domain-driven-design.md](domain-driven-design.md) — Entity, Value Object, Aggregate, Repository,
  Domain Service, Domain Event, Bounded Context, Context Map, Anti-Corruption Layer, Ubiquitous
  Language. Already actively practiced by `analyst.md`/`architect.md`/`DOMAIN_DICTIONARY.md`, just
  never consolidated as a reference until now.
- [enterprise-integration-patterns.md](enterprise-integration-patterns.md) — Message Channel,
  Content-Based Router, Message Translator, Publish-Subscribe Channel, Dead Letter Channel, Correlation
  Identifier, Saga. `architect.md` names Hohpe as an influence; several of this repo's own mechanisms
  (`pipeline-trace.json`, `deliver-feature`'s checkpointed pipeline) are concrete instances.
- [stability-patterns.md](stability-patterns.md) — Circuit Breaker, Bulkhead, Timeout, Fail Fast, Steady
  State (Michael Nygard, *Release It!*). `architect.md` names Nygard as an influence; the
  general/non-API-specific versions of these, complementing Sunday's API-specific Resilience Primitives.
- [twelve-factor-app.md](twelve-factor-app.md) — all 12 factors (Codebase through Admin Processes). The
  one axis nothing else here covers: how a service should actually run in production, not how its code
  is organized.
- [security-patterns.md](security-patterns.md) — STRIDE threat modeling, Secure by Default, Defense in
  Depth, Least Privilege, Paved Road / Golden Path. `security-reviewer.md` names Adam Shostack as an
  influence and already has a fully worked STRIDE table in its own contract.
- [observability-patterns.md](observability-patterns.md) — SLI, SLO, Error Budget, Low-Cardinality
  Logging, Structured Tracing, No PII in Telemetry. `sre-engineer.md`'s entire mandate; SLO/Error Budget
  are the one layer above the already-required SLI that wasn't named yet.
- [expand-contract-migrations.md](expand-contract-migrations.md) — the full pattern plus its own named
  approval gate, upgraded from "a rule that's referenced" to a documented pattern with the same
  Context/Structure/Example/Trade-offs treatment Circuit Breaker got in `stability-patterns.md`.
- [api-design-patterns.md](api-design-patterns.md) — Resource-Oriented Design, Idempotency Keys, Status
  Code Discipline, Pagination by Default, User Enumeration Prevention, Schema-First Contract. The
  `openapi` skill already enforces all of these per-endpoint; never written down as a standalone
  reference until now.
- [gate-enforcement.md](gate-enforcement.md) — how the nine approval gates are actually held: which
  three the executor refuses to start a stage without, which remain prompt discipline, what
  invalidates an approval, and what a policy decision records while nothing is auto-approved. The
  mechanism side of `shared/rules/approval-gates.md`, kept out of every stage's prompt (L3.19).
- [test-annotation.md](test-annotation.md) — the per-language syntax for tying a test to its issue
  and acceptance criterion, in six languages plus Gherkin, and the exemplar-test marking. The
  mechanics side of `shared/rules/testing-conventions.md`'s convention.
- [testing-pyramid.md](testing-pyramid.md) — the five test levels (Unit, Integration, API Contract,
- [test-suite-health-metrics.md](test-suite-health-metrics.md) — the four numbers that detect a suite
  getting greener and blinder at once (flake rate, time to diagnose, escaped defects, wall-clock), and
  why coverage is not one of them. Judgment-only: three of the four have no source loom can read.
  Acceptance, E2E/UI), FIRST principles for unit tests, and the Three Laws of TDD with an honest
  scoping note: XP TDD's design pressure only fully applies to agent-written code when there's role
  separation (as in `deliver-atdd`). For single-agent `test-driven-developer` use, tests are still
  valuable as executable spec and regression safety, but the design lever is elsewhere (complexity
  thresholds, SOLID, `code-reviewer`). References Saturday and Sunday patterns for the top two levels
  rather than restating them.
- [agent-skill-pair-convention.md](agent-skill-pair-convention.md) — when the same `name` appears in
  both `shared/agents/` and `shared/skills/`: two intentional sub-patterns (**Delegation Wrapper**
  where the skill is a thin routing shell that invokes the agent, and **Parallel Implementation**
  where agent and skill independently implement the same workflow for pipeline vs. ad-hoc contexts),
  plus rules for detecting and resolving accidental pairs. Confirmed pairs: `spec-writer` and
  `context-engineer`.
- [framework-meta-patterns.md](framework-meta-patterns.md) — **patterns novel to (or specific to) this
  framework itself**, for anyone building a similar multi-agent orchestration system rather than for
  teams *using* this framework. **Pipeline / Orchestration Pattern** (`deliver-feature`'s 14-agent
  sequence documented as a general pattern with the six load-bearing mechanisms named — sequential
  specialist stages, artifact handoff, contract validation, human checkpoints, checkpointed state,
  trace logging) and **Contract-First Agent Handoff** (`shared/contracts/` + `validate-artifact` as a
  named pattern with no established prior art for LLM-artifact contracts specifically). Kept in a
  separate file so the main catalog stays scoped to general software patterns. Written with v3
  planning in mind — the mechanisms named here should carry forward as the substrate whether or not
  the specific agent lineup does.

All terminology matches `shared/DOMAIN_DICTIONARY.md` exactly — that's the canonical definition source;
these docs go deeper into context, structure, and trade-offs, not redefine the terms.

---

## Contributing a Pattern

Each pattern entry follows this structure:

1. **Context** — when and why this pattern is used.
2. **Structure** — key abstractions, interfaces, and relationships.
3. **Example** — a concrete usage example, ideally from this codebase.
4. **Trade-offs** — what this pattern costs and what it buys.
5. **Related** — links to complementary or alternative patterns.

Group related patterns into one file by category (as the four docs above do) rather than one file per
pattern — most individual patterns are a few paragraphs, and splitting each into its own file adds
navigation overhead without adding content.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
