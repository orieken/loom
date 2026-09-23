# `loom` Build Roadmap — L2 → L4

**Status**: active build plan · **Framework version**: v3.3.14 · **Compiled**: 2026-08-29
· **Status markers last reconciled**: 2026-09-22

> **Reading the Problem statements.** Each item's "Problem" paragraph describes the state of the
> repository *when this roadmap was compiled*, in present tense. Items that have since shipped carry
> a **SHIPPED** line under their workstream header — read that first, because the Problem paragraph
> below it is deliberately preserved as the historical motivation, not as a current claim.
>
> Absence of a SHIPPED line means only that no one has reconciled it, not that the work is unbuilt.
>
> **Reconciled 2026-09-18**: the ten **Curriculum Alignment** items (L3.47–L3.56) were appended after
> shipping, each with its commit and what verifying its premise changed. That workstream's own header
> records the part worth carrying forward: six of eleven items were wrong as written, and three checks
> written during the work passed while testing less than they claimed. This reconciliation covers that
> stream only — items above it carry whatever status they last had.

> **First real end-to-end run: 2026-09-06.** Every item above M0.4 had been verified against
> `--provider mock` until then. One run with `--provider claude` — five stages, seven minutes,
> $3.57, 3.8M tokens — produced two new items (**L2.21**, **L3.16**), confirmed that L2.9's typed
> invariants stop a bad run on real data, and showed that the router takes a materially different
> path on a real analysis (10 of 12 stages, against the mock's 7). Mock-driven tests systematically
> under-exercise this pipeline; that is worth remembering when an item claims to be verified.
> Markers below were verified against the code on the date above; older milestones (M0.1–M0.3, L2.4,
> D.1–D.5) shipped earlier and are tracked in `docs/prompts/README.md`'s Completed Prompts table.

This is the single authoritative roadmap. It merges and supersedes:

| Source | Items | Disposition |
|---|---|---|
| [`agy.md`](agy.md) | 9 | Fully absorbed. Two items contributed material this plan did not have (host-IDE execution dependency; context-isolated reflexion). Two path citations corrected — see Appendix B. |
| [`maturity-todo-2026-08-29.md`](maturity-todo-2026-08-29.md) | 41 | Fully absorbed, re-sequenced into milestones with dependencies and acceptance criteria. |
| [`architectural-audit-2026-08-29.md`](architectural-audit-2026-08-29.md) | H1–H11 | Retained as the evidence document. Not superseded — read it for the *why*; read this for the *what next*. |

**45 items across 5 milestones** as compiled, plus the appended **PLATFORM — Distribution & Adoption**
workstream (D.1–D.5, appended 2026-08-29 from the distribution-strategy discussion — see
`docs/prompts/epic-75-distribution-adoption.md` for the executable handoff prompts). Every path
cited was verified to resolve on 2026-08-29.

---

## How to use this document

Each item carries six fields:

| Field | Meaning |
|---|---|
| **ID** | Stable reference (`M0.1`, `L2.4`, …). Never renumber — append instead. |
| **Workstream** | Which parallel track this belongs to. Items in different workstreams can proceed concurrently. |
| **Effort** | S = under a day · M = a few days · L = a week or two · XL = a month+ |
| **Blocked by / Blocks** | Hard dependency edges. Respect these; the sequencing is not advisory. |
| **Problem / Fix / Target Files** | As in the source documents. |
| **Done when** | A falsifiable acceptance criterion. If you cannot demonstrate it, the item is not done. |

Workstreams:

- **KERNEL** — the executor process: state, checkpoints, gates, retries, budget
- **TOOLS** — the MCP tool runtime: registry, validation, resilience, transport
- **MEMORY** — retrieval, episodic store, KI lifecycle
- **OBSERVE** — telemetry, evaluation, CI
- **PLATFORM** — provider abstraction, interop, distribution

---

## Critical path

```mermaid
graph LR
  M0.2[M0.2 Go in CI] --> M0.4[M0.4 Executor skeleton]
  M0.1[M0.1 Kernel ADR] --> M0.4
  M0.4 --> L2.9[L2.9 Typed state]
  M0.4 --> L2.13[L2.13 Gates as interrupts]
  M0.4 --> L3.8[L3.8 OTel emission]
  L2.9 --> L3.1[L3.1 Planner/Router]
  L2.9 --> L3.2[L3.2 Capability registry]
  L2.9 --> L2.11[L2.11 Semantic validation]
  L2.13 --> L4.5[L4.5 Correction signal]
  L3.8 --> L3.5[L3.5 Episodic memory]
  L3.8 --> L4.3[L4.3 Budget governor]
  L3.8 --> L4.4[L4.4 Prompt registry]
  L4.5 --> L4.4
  L3.2 --> L3.1
  L3.2 --> L4.9[L4.9 Agent cards]
```

**If only three things get built**: `L2.9` (typed state), `M0.4`+`L2.13` (executor owning gates and
retries), and `L3.8` (OTel emission). Those unblock roughly two-thirds of everything else.

---

# MILESTONE 0 — Foundations

Nothing else is safely buildable until these land. M0.2 in particular is one day of work and is the
highest-leverage item in the entire document.

### M0.1 — Decide and record what `loom` is
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: none · **Blocks**: M0.4

**SHIPPED** 2026-08-29 (epic 76, `9f6c90f` → `82a8c2d`) — ADR-006 "loom executes pipelines" is
Accepted. The README half of the done-when was reconciled in `77367be` (2026-08-31).

1. **Problem**: The repository is 52k lines of markdown specification and 8.9k lines of Go that only
   installs files. `README.md` and `docs/ARCHITECTURE.md` describe orchestration, telemetry, policy
   evaluation, and retrieval tiers as though implemented; all are prose with no executor. The
   ambiguity is load-bearing — it is why 20 items below target an `internal/orchestrator/` that does
   not exist, and why `agy.md` and `maturity-todo` both independently proposed "build a kernel"
   without either committing to it.
2. **Architectural Fix**: Write ADR-00N answering one question: does `loom` **execute** pipelines, or
   does it **validate and distribute** content a host runtime executes? Both are defensible products.
   Every item in M0.4 onward assumes the first. If the answer is the second, delete Milestones 1–4
   and reduce this to a content-quality roadmap — that is a legitimate outcome, but it must be chosen
   rather than drifted into.
3. **Target Files**: new `docs/adrs/`, `README.md`, `docs/ARCHITECTURE.md`
4. **Done when**: an accepted ADR exists and `README.md` no longer describes unimplemented subsystems
   in the present tense.

### M0.2 — Put the Go in CI and turn the framework's rules on itself
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: M0.4 · *(audit H9)*

**SHIPPED** 2026-08-29 (epic 76, `7c3e35d`) — golangci-lint job with `gocyclo` min-complexity 7 as
the build gate, SHA-pinned actions, coverage ratchet, and a fixture manifest that fails CI on a
deleted golden fixture.

1. **Problem**: `.github/workflows/framework-ci.yml` runs five bash/python scripts and **zero Go
   steps** — no `go build`, `go test`, `go vet`, `golangci-lint`. There is no `.golangci.yml`.
   `shared/mcp/` has **0.0% coverage in every package** against the non-negotiable 85% rule. The
   workflow has **no `permissions:` block** and pins `actions/checkout@v4` (mutable tag) across all
   six jobs — both violations of `iac-conventions.md`, which `loom-release.yml` gets right. And
   `scripts/test-agents.sh` reports "20 passed, 0 failed, 32 skipped" from **one** `actual-output.md`
   across 33 fixture dirs, because SKIP exits 0 by design.
2. **Architectural Fix**: Add build/test/vet/lint jobs for both modules. Write the `.golangci.yml`
   the framework mandates elsewhere, with `gocyclo` capped at 6. Add a coverage ratchet starting at
   today's real number. Add `permissions:` and SHA-pin every action. Make the agent suite fail on
   missing fixtures rather than passing. Then run the framework's own `verify_dependencies` and
   `analyze_complexity` against this repo — the first *will* fail on M0.3's finding, which is the
   point.
3. **Target Files**: `.github/workflows/framework-ci.yml`, new `.golangci.yml`,
   `scripts/test-agents.sh`, `scripts/ci-check.sh`, `Makefile`
4. **Done when**: CI fails on a deliberately introduced compile error, a deliberately introduced
   complexity-9 function, and a deliberately deleted test fixture.

### M0.3 — Fix the domain-layer dependency violation
**Workstream**: TOOLS · **Effort**: S · **Blocked by**: M0.2 · **Blocks**: L2.1 · *(audit H4)*

**SHIPPED** 2026-08-29 (`c67262e`) — the domain tool abstraction is transport-free. Since D.2,
`internal/domain` re-exports the public `github.com/orieken/loom/tools` package rather than importing
only stdlib; that package is itself pinned stdlib-only by `tools/deps_test.go`, and
`shared/mcp/internal/domain/deps_test.go` enforces the boundary transitively.

1. **Problem**: `shared/mcp/internal/domain/tool.go` — commented as "the framework's first-class
   abstraction for every capability" — imports `github.com/mark3labs/mcp-go/mcp` and
   `invopop/jsonschema`, and types its own signatures in them (`mcp.ToolInputSchema`,
   `mcp.CallToolRequest`, `*mcp.CallToolResult`). That is `architecture-guardrails.md` #1 violated in
   the one file defining the tool abstraction, in the framework that sells verifiable architecture.
2. **Architectural Fix**: Hexagonal port/adapter. Define transport-free
   `ToolRequest{Name string; Args map[string]any}` and
   `ToolResult{Content []ContentBlock; IsError bool; Err error}` in `domain`; move all `mcp.*`
   marshalling into a new `server/mcp_adapter.go`. `domain` imports zero third-party packages.
3. **Target Files**: `shared/mcp/internal/domain/tool.go`,
   `shared/mcp/internal/server/registration.go`, all 6 `shared/mcp/internal/tools/*_tool.go`
4. **Done when**: `go list -deps ./internal/domain` shows only stdlib, and the CI fitness function
   from M0.2 enforces it.

### M0.4 — Stand up the executor skeleton
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: M0.1, M0.2 · **Blocks**: L2.9, L2.12, L2.13, L2.14, L3.1, L3.3, L3.8, L4.1

**SHIPPED** 2026-08-29 (epic 76, `ba78f21` + `cab0156`) — `internal/orchestrator/` owns the run
loop and durable `run-state.json`; `loom run` executes the built-in plan via a claude subprocess
provider, with a mock provider for tests.

1. **Problem**: Both source roadmaps assume a kernel and neither builds one. `agy.md` item 1 names it
   ("a native tool orchestration kernel"); the maturity TODO targets `internal/orchestrator/*` in
   nine separate items. It does not exist. Today `loom` has **no execution engine at all** — it
   generates prompt configurations and relies entirely on host platforms (Claude Code, Cursor,
   Windsurf) to run agents, which ties framework resilience, retry semantics, and gate enforcement to
   proprietary IDE behavior the framework cannot observe or control.
2. **Architectural Fix**: A minimal Go executor that owns the run loop: load a plan, execute stages
   in order, persist state, invoke an agent via a provider adapter, and stop. No routing, no
   parallelism, no policy — those are later items that plug into it. Ship it running the existing
   linear `deliver-feature` sequence as a hardcoded default plan, so behavior is preserved while the
   substrate changes underneath.
3. **Target Files**: new `internal/orchestrator/` (executor, stage, plan), new `internal/provider/`,
   `cmd/loom/cmd/` (new `run` subcommand)
4. **Done when**: `loom run --spec features/<x>.md` executes at least three real stages end-to-end,
   writes state, and resumes correctly after `SIGINT`.

### M0.5 — Delete or compile `shared/mcp-patterns/go/`
**Workstream**: TOOLS · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(audit H10)*

**SHIPPED** 2026-09-02 (epic 89) — deleted. Twenty `//go:build ignore` files in no module,
compiled by nothing, presented as templates to copy. No Go file in the repository carries that tag
any more.

Deleting it left three porting guides pointing at nothing, and they now read
`shared/mcp/internal/` instead — the implementation that is actually compiled and tested on every
CI run, which is precisely the property the templates lacked. `shared/mcp-patterns/README.md`
documents `register.Frameworks` as the supported Go path, with `examples/embedding/` as a working
server that CI builds, so the embedding path breaking is a build failure rather than a copy
silently rotting.

1. **Problem**: ~1,200 lines of `//go:build ignore` copies of `shared/mcp/internal/`, in no Go
   module, referenced by no build, presented as the reference implementation teams should copy. All
   11 shared files have diverged from their originals (`retriever.go` by 186 diff lines,
   `bm25_retriever.go` by 106). It ships bugs downstream — including the concurrency defect its own
   comment documents at line 40 — and can never be compiled, tested, or kept honest.
2. **Architectural Fix**: Delete it. If a reference implementation is genuinely wanted, make it a
   compiled example module in the workspace with its own tests, so drift becomes a build failure.
   Copy-paste distribution of a Go library is the wrong mechanism when `register.FrameworkTools`
   already exists as a supported embedding path.
3. **Target Files**: `shared/mcp-patterns/go/**` (delete), `shared/mcp-patterns/README.md`,
   `shared/mcp/register/register.go`
4. **Done when**: no `//go:build ignore` file remains in the repo and `README.md` documents
   `register.FrameworkTools` as the supported integration path.

---

# MILESTONE 1 — Level 2: Coordinated Multi-Agent Systems

*Agents are task-specific, use tools reliably, and coordinate in deterministic workflows with
human-in-the-loop control.*

## Workstream: TOOLS — Tool Execution & Validation

### L2.1 — Enforce input schemas server-side instead of trusting the model
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: M0.3 · **Blocks**: L2.5

1. **Problem**: `InputSchema()` exists only to describe arguments *to the LLM*. Enforcement is
   unchecked type assertion — `parseComplexityArgs` does `args["projectPath"].(string)` and silently
   yields `""` on any non-string, then returns a generic error. A hallucinated argument shape
   produces a soft failure the model re-attempts blindly. No required-field check, no enum check, no
   bounds.
2. **Architectural Fix**: Validate every `Args` map against the tool's declared JSON Schema at the
   handler boundary *before* dispatch, returning a structured `ValidationError` naming the offending
   field and expected type — a machine-actionable repair signal, not prose.
   `github.com/santhosh-tekuri/jsonschema/v6` is **already in the dependency graph** as an indirect
   dep of `mcp-go`; promote it to direct.
3. **Target Files**: `shared/mcp/internal/server/registration.go`,
   `shared/mcp/internal/tools/schemas.go`, `shared/mcp/go.mod`
4. **Done when**: a malformed-argument call returns a field-level validation error, and a test
   asserts it for all six tools.

### L2.2 — Propagate `context.Context` and set per-tool deadlines
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: M0.3 · **Blocks**: L2.5

1. **Problem**: All six tools sign `Execute(_ context.Context, ...)` —
   `analyze_complexity_tool.go:54`, `check_accessibility_tool.go:52`,
   `check_ubiquitous_language_tool.go:50`, `search_docs_tool.go:64`, `search_ki_tool.go:56`,
   `verify_dependencies_tool.go:40`. Cancellation is discarded 100% of the time. A client
   disconnect, timeout, or user abort cannot stop an in-flight `filepath.Walk`.
   `go-conventions.md` mandates explicit timeouts; the tool layer has none.
2. **Architectural Fix**: Thread `ctx` through `Execute` → analyzer → walk, checking `ctx.Err()`
   inside every `filepath.WalkDir` callback. Add registration middleware applying
   `context.WithTimeout` from a per-tool budget in the registry entry.
3. **Target Files**: all 6 `shared/mcp/internal/tools/*_tool.go`,
   `shared/mcp/internal/analyzers/*.go`, `shared/mcp/internal/server/registration.go`
4. **Done when**: a cancelled context aborts an in-progress walk within 100ms, proven by test.

### L2.3 — Confine filesystem access to an explicit root
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: M0.3 · **Blocks**: none

1. **Problem**: `analyze_complexity` accepts an arbitrary `projectPath` from model-controlled
   arguments with no validation, no root confinement, and no symlink handling, then walks it
   unbounded and uncancellably. `projectPath: "/"` walks the disk. Same pattern in
   `check_accessibility`, `check_ubiquitous_language`, `verify_dependencies`, and `search_docs`
   (`docsPath`).
2. **Architectural Fix**: Adopt `os.Root` (available on `go 1.26.5`) for traversal-safe rooted FS
   access, with the root supplied by server config rather than tool arguments. Reject paths that
   escape it. Add a file-count and byte ceiling to abort runaway walks.
3. **Target Files**: `shared/mcp/internal/analyzers/walkutil.go`,
   `shared/mcp/internal/analyzers/*_analyzer.go`, `shared/mcp/internal/server/tool_provider.go`
4. **Done when**: `projectPath: "/"` and `projectPath: "../../etc"` are both rejected, with tests.

### L2.4 — Replace the hardcoded tool slice with a registry
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: M0.3 · **Blocks**: L2.2, L2.5, L3.2, L4.7

**SHIPPED** 2026-08-29 (`a68a23e`) — `domain.Registry` with per-tool timeout, retry class and
permission scope; adding a tool is one entry in `frameworkRegistrations`
(`shared/mcp/internal/server/tool_provider.go`), no edit to `handler.go`. The declared timeouts are
not yet enforced — that is L2.2.

1. **Problem**: `buildFrameworkTools()` is a slice literal returning six constructor calls. Adding a
   tool means editing and recompiling the handler. No discovery, no per-tool metadata (timeout,
   retry policy, permission scope, version), no enable/disable, and no way for a downstream project
   to contribute a tool without forking.
2. **Architectural Fix**: `map[string]ToolRegistration` where the registration carries the `Tool`,
   its timeout, retry class, and required permission scope. Populate via per-file `Register(name,
   reg)` or an explicit registry-builder consuming config. `register.FrameworkTools` becomes a
   registry merge rather than wholesale re-registration.
3. **Target Files**: `shared/mcp/internal/server/tool_provider.go`,
   `shared/mcp/internal/server/handler.go`, `shared/mcp/register/register.go`
4. **Done when**: a new tool is added by one `Register` call with no edit to `handler.go`.

### L2.5 — Introduce a typed failure taxonomy
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: L2.1, L2.2, L2.4 · **Blocks**: L2.6, L4.2

1. **Problem**: Every failure path collapses to `mcp.NewToolResultError(fmt.Sprintf(...))` with
   `err == nil` — a *successful* tool call carrying a prose error string. The caller cannot
   distinguish "bad argument, fix and retry" from "corpus missing, stop" from "transient I/O, back
   off." `search_docs` goes further, returning `Success: true, TotalHits: 0` with the failure reason
   smuggled into the `Query` field (`emptyResult`, line 101) — indistinguishable from a genuine
   zero-result search.
2. **Architectural Fix**:
   `ToolError{Kind: Validation|NotFound|Transient|Internal|Permission, Field, Message, Retryable bool}`
   serialized into a stable `error` envelope. Never encode failure state into a success payload.
   Orchestrator retry policy keys off `Kind`.
3. **Target Files**: `shared/mcp/internal/tools/responses.go`,
   `shared/mcp/internal/tools/search_docs_tool.go:101`, all `*_tool.go` error branches
4. **Done when**: no tool returns `Success: true` on a failure path, enforced by test.

### L2.6 — Add the resilience primitives the guardrails already mandate
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: L2.5 · **Blocks**: L4.7

1. **Problem**: `architecture-guardrails.md` #5 forbids hand-rolled retry loops and requires
   `CircuitBreaker` or `ExponentialBackoffStrategy`. Neither exists anywhere in the Go tree. The only
   retry logic in the framework is prose in `deliver-feature/SKILL.md` telling an LLM to count to
   three.
2. **Architectural Fix**: `sony/gobreaker` per tool and per downstream dependency, plus
   `cenkalti/backoff/v4` for `Transient`-classed failures, wired as registry middleware so no tool
   implements its own retry. Emit breaker state transitions as telemetry.
3. **Target Files**: new `shared/mcp/internal/server/middleware.go`,
   `shared/mcp/internal/server/registration.go`, `shared/mcp/go.mod`
4. **Done when**: a tool failing 5× consecutively opens its breaker and returns immediately, proven
   by test.

### L2.7 — Fix the per-query full-corpus re-index
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: L2.2 · **Blocks**: L3.7 · *(audit H2)*

1. **Problem**: `search_docs_tool.go:82` calls `EnsureIndex` inside `Execute`.
   `bm25_retriever.go:70` then walks the whole docs tree, `os.ReadFile`s every `.md`, and runs **one
   sqlite transaction per file** — no mtime check, no content hash, no dirty tracking. Every search
   is O(corpus) disk I/O plus O(n) transactions. `shared/mcp-patterns/go/tools/bm25_retriever.go:40`
   documents the rest: "EnsureIndex is not safe to call concurrently with itself" — and MCP servers
   field concurrent calls. Deleted docs are never evicted; `DELETE` only fires for re-inserted paths.
2. **Architectural Fix**: Move indexing out of the query path. Incremental index keyed on
   `(path, mtime, size)` or content hash in one batched transaction; a separate explicit
   `reindex_docs` tool plus optional `fsnotify` watcher; `sync.RWMutex` around writer access; a
   reconciliation sweep deleting rows whose paths no longer exist.
3. **Target Files**: `shared/mcp/internal/tools/bm25_retriever.go`,
   `shared/mcp/internal/tools/search_docs_tool.go`
4. **Done when**: a second identical query performs zero file reads, and a deleted doc disappears
   from results.

### L2.8 — Ship MCP over an authenticated network transport
**Workstream**: TOOLS · **Effort**: L · **Blocked by**: L2.4 · **Blocks**: L4.8

1. **Problem**: `cmd/mcp-server/main.go` calls `server.ServeStdio(s)` — stdio only. No streamable
   HTTP, no SSE, no authentication, no authorization, no tenancy, no per-caller rate limiting. The
   server is single-user, local-only, and has no notion of *who* is calling.
2. **Architectural Fix**: Streamable HTTP transport alongside stdio, with OAuth2/OIDC bearer
   validation, per-principal tool scoping enforced against the registry's permission field, and
   per-principal rate limits. Keep stdio for local dev.
3. **Target Files**: `shared/mcp/cmd/mcp-server/main.go`, `shared/mcp/internal/server/handler.go`,
   new `shared/mcp/internal/server/auth.go`
4. **Done when**: an unauthenticated HTTP call is rejected and a scoped token can reach only its
   permitted tools.

## Workstream: KERNEL — State Management

### L2.9 — Replace markdown-file state passing with a typed graph state
**Workstream**: KERNEL · **Effort**: XL · **Blocked by**: M0.4 · **Blocks**: L2.10, L2.11, L3.1, L3.2, L4.4

**SHIPPED (first cut)** 2026-08-31 (epic 79, `74155f1`…`3430594`) — `internal/state/` types the
analyst → architect hop: Go structs, JSON Schema generated into `shared/schemas/pipeline/`,
field-level projections, and markdown rendered as a view.

**SHIPPED (second cut)** 2026-09-01 (epic 83, `0b2d6c4`…) — the implementation chain:
`implementation-notes`, `security-report`, and `qa-report` are typed, joining `analysis`,
`architecture`, `route` (L3.0) and `review` (L2.17) for **seven typed artifacts**. Two contract
content rules became load-time invariants rather than greps — a non-zero `failed` test count and a
CRITICAL/HIGH security finding with no fix applied are now validation errors. `Stage.Consumes` became
plural and projections are keyed by `(consumer stage, upstream kind)`, since what a stage reads and
what it writes vary independently. **Eight of the fifteen artifacts still pass markdown** — the
end-of-pipeline reports (docs, devops, observability, accessibility, visual QA) and
`context-manifest`. Nothing evaluates a condition over those yet, which is why they were left.

1. **Problem**: There is no state object. Agents hand each other whole markdown documents on disk
   (`analysis.md`, `architecture-notes.md`, `implementation-notes.md`, … 15 artifacts). The only real
   delivery in the repo has a 15 KB `analysis.md`. Every downstream agent re-parses the full text;
   there is no field-level access, no size ceiling, no provenance, and no way to pass a value without
   passing a document.
2. **Architectural Fix**: A versioned `PipelineState` struct with per-stage typed sub-schemas (Go
   structs plus generated JSON Schema, or CUE as single source of truth). Markdown becomes a
   *rendered view* of state, not the transport. Agents receive a narrowly-scoped projection of the
   fields their contract declares, never the whole graph. `agy.md` proposed LangGraph or a custom
   FSM; a custom FSM is the right call here since the executor is Go and LangGraph would reintroduce
   a Python runtime dependency.
3. **Target Files**: `shared/orchestration/pipeline-schema.md` → generated; all 18
   `shared/contracts/*.md` → schemas; `shared/skills/deliver-feature/SKILL.md`; new `internal/state/`
4. **Done when**: two consecutive stages exchange data with no markdown file on the path, and a
   schema violation is a load-time error.

### L2.10 — Stop using an LLM as the context-compaction mechanism
**Workstream**: KERNEL · **Effort**: S (was M) · **Blocked by**: L2.9 · **Blocks**: none

**PARTLY ABSORBED** by epic 83 (2026-09-01). The two inter-stage call sites that existed were
replaced with deterministic projections of `AnalysisState`: `qa-engineer` (step 2) now receives
acceptance criteria, edge cases, QA tasks and the definition of done; `tech-writer` (step 1) receives
the summary, out-of-scope list and its own task list. Both agent files were edited and versioned.
Under `loom run` **no LLM call sits between the analysis and either agent**.

**What remains**:
1. The step 37a `--persist` retrieval surrogate in `deliver-feature/SKILL.md`, which is a *different*
   problem — the surrogate feeds `memory-registry`'s retrieval tier and has consumers of its own, so
   "replace it with field selection" is not the right fix and this item's done-when does not cover it.
   Deciding what the surrogate becomes is the open question here.
2. The markdown pipeline's fallback paths. Each edited agent keeps a no-executor path that reads the
   two relevant sections rather than the whole file — smaller context, but still a read the executor
   does by field selection. This closes only when those agents run under the executor.
3. Any call site introduced by typing the remaining eight artifacts.

1. **Problem**: Context decay is handled by `summarize-artifact --persist`, invoked at
   `deliver-feature/SKILL.md` step 37a — another LLM call producing a lossy ~200-word surrogate then
   indexed as a retrieval target. Compaction is nondeterministic, unverifiable, costs a model call,
   and silently drops whatever the summarizer deemed unimportant.
2. **Architectural Fix**: Deterministic projections. Each contract declares which fields downstream
   stages may read; the executor computes the projection by field selection, not summarization.
   Reserve LLM summarization for human-facing prose only, never machine handoff.
3. **Target Files**: `shared/skills/summarize-artifact/SKILL.md`,
   `shared/skills/deliver-feature/SKILL.md:139`, `shared/contracts/*.md`
4. **Done when**: no LLM call sits on the inter-stage data path.

### L2.11 — Make `validate-artifact` verify semantics, not heading presence
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.9 · **Blocks**: none

1. **Problem**: Contract validation checks that required `##` sections exist.
   `agent-scorecard/SKILL.md:29` confirms the depth: the analyst's "completeness score" is "fraction
   of required sections present **and** containing real content (not leftover `[...]` template
   placeholders)." That is a template-placeholder grep. A structurally perfect, semantically empty
   artifact passes every gate in the pipeline.
2. **Architectural Fix**: With typed state, validation becomes JSON Schema conformance plus
   declarative business rules — required cardinality, cross-field consistency, referential integrity
   against prior stages. Keep an LLM critic as an *additional* qualitative gate, never the structural
   one.
3. **Target Files**: `shared/skills/validate-artifact/SKILL.md`, `shared/contracts/*.md`,
   `shared/schemas/`
4. **Done when**: an artifact with all headings present but contradictory field values fails
   validation.

### L2.12 — Move `pipeline-state.json` ownership into the executor
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: M0.4 · **Blocks**: L2.14

**SHIPPED** 2026-08-31 (epic 78, `8441ffc`…`7e574c9`) — digests are computed and re-verified in
Go, an edited artifact demotes its stage and cascades, and `loom state record/verify/approve/show/
timeline` gives the markdown pipeline a way to record checkpoints without hashing its own work.
`run-events.jsonl` gives events and timing an owner.

1. **Problem**: The state file — including SHA-256 checksums used for tamper detection and gate-edit
   detection — is written *by the LLM following prose instructions* (`deliver-feature/SKILL.md`,
   "Checkpointing & Pipeline State"). A model computing and recording its own integrity hashes is not
   integrity. No `pipeline-state.json` exists anywhere in the repo, so this has never executed.
2. **Architectural Fix**: The executor owns the file: atomic write (temp + `os.Rename`), a schema
   version field, real `sha256` computed in Go, verification on resume in code rather than by
   instruction. Agents never write it.
3. **Target Files**: `shared/skills/deliver-feature/SKILL.md`,
   `shared/skills/resume-pipeline/SKILL.md`, new `internal/orchestrator/checkpoint.go`
4. **Done when**: hand-editing an artifact causes the executor to detect the mismatch and refuse to
   treat that stage as complete.

## Workstream: KERNEL — Human-in-the-Loop

### L2.13 — Implement gates as process interrupts, not prose
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: M0.4 · **Blocks**: L2.14, L4.5

**SHIPPED** 2026-08-30 (epic 77, `868c281`…`a971b2f`) — the executor refuses to start a gated
stage without a recorded human approval; approval arrives only via a TTY prompt or
`loom run --resume --approve <gate>`, and a halted run exits 3. Provider output cannot self-approve.
**Scope**: `loom run` only — the markdown pipeline and the other prose gates remain
prompt-discipline.

1. **Problem**: All eight gates in `approval-gates.md` are natural-language instructions ("user must
   say 'ship'"). The enforcement mechanism for an irreversible action — DB contract-phase `DROP`,
   deploy, external API mutation — is the model's willingness to comply with a paragraph. There is no
   code path that can physically prevent the action, and an LLM can hallucinate straight past a
   prompt-level guard.
2. **Architectural Fix**: The executor halts the process at gate boundaries, persists state, and
   yields to a real approval channel (CLI prompt, webhook, or queue). High-risk tool classes are
   declared in the tool registry and are *unreachable* without a signed approval token in the
   request. Enforcement lives below the model, not in it.
3. **Target Files**: `shared/rules/approval-gates.md`,
   `shared/skills/deliver-feature/SKILL.md:130-133`, new `internal/orchestrator/gate.go`, registry
   entries
4. **Done when**: an agent instructed to "skip the gate and deploy" cannot reach the deploy tool.

### L2.14 — Enforce "any edit resets the gate" in code
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.12, L2.13 · **Blocks**: L4.5

**SHIPPED** 2026-08-31 (epic 80, `27ac1f0`…`9db60ee`) — an approval binds to the digests of every
stage completed when it was given; an edit invalidates it, the run halts at that gate again, and the
invalidated record is kept for audit. Detection at verification rather than at the barrier, since a
re-run would otherwise overwrite the edit that caused it. `loom state verify` reports the same for
markdown-pipeline runs — detection, not enforcement.

1. **Problem**: Every gate in `approval-gates.md` declares "Reset condition: any edit to the pending
   artifact resets the gate." Nothing enforces it. The `gate_decision` telemetry spec describes
   checksum-diffing to detect `edited_then_approved` — but the checksum is computed by the model, the
   event is emitted by nobody, and no `events.jsonl` exists in the repository.
2. **Architectural Fix**: Approval binds to an artifact digest. The executor computes the digest at
   halt, issues a scoped approval token over it, and re-verifies at execution. Digest mismatch
   invalidates the token — a code-level check, not a remembered rule.
3. **Target Files**: `shared/rules/approval-gates.md`, `shared/telemetry/event-schema.md:112`,
   `internal/orchestrator/gate.go`
4. **Done when**: editing an artifact between approval and execution causes the execution to be
   refused.

### L2.15 — Make resume a real capability
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.12 · **Blocks**: none

1. **Problem**: `resume-pipeline/SKILL.md` implements three modes (resume, `--from-phase N`, per-agent
   rollback) entirely as instructions to an LLM to read state, recompute checksums, mark entries
   `"stale": true`, and jump to a numbered step in *another skill's* prose. There is no process to
   resume — the "pause" was only ever the model stopping. Steps are addressed by position in a
   hand-numbered 43-step list, so renumbering silently breaks every resume path.
2. **Architectural Fix**: Durable executor with content-addressed stage IDs (never ordinals), a real
   checkpoint store, and resume as a first-class operation replaying from persisted state. Rollback
   becomes state-graph surgery in code, not markdown restoration by instruction.
3. **Target Files**: `shared/skills/resume-pipeline/SKILL.md`,
   `shared/skills/deliver-feature/SKILL.md`, `shared/orchestration/interface.md`
4. **Done when**: `kill -9` mid-stage followed by `loom run --resume` continues from the last
   checkpoint with no duplicated work.

### L2.16 — Replace the LLM policy evaluator with a real one
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.13 (shipped) · **Blocks**: L4.3 · *(audit H7)*

**SHIPPED** 2026-09-02 (epic 87, `da5d829`…) — `internal/policy` loads, validates and evaluates
policies in typed Go. Every decision is recorded as `policy.evaluated` on the run event timeline and
in `run-state.json`; `loom run --dry-run-policies` replays them against a finished run.

**No expression language, against this item's own suggestion of CEL or Rego.** The condition
vocabulary is closed and small — nine fields over facts the executor already holds. CEL would buy
arbitrary boolean logic at the cost of a dependency, a cost-limiting surface, and a gate decision
written in a language most readers of a `.policy.yaml` will not know. Epic 82 reached the same
conclusion for loop conditions. Adding a check is now a code change with a test, which is the right
direction for a mechanism whose purpose is skipping human review.

**Nothing is auto-approved, deliberately.** A matching `auto-approve` policy is recorded as what
*would* have happened and the executor halts for a human anyway; every record carries
`honoured: false`. The first run that skips a barrier should not also be the first evidence the
evaluator decides what a human would. Honouring a decision is **L2.19**.

**Three prose controls became code.** The always-human list is a compiled constant and a policy
targeting one of those five gates fails at load, naming the gate — it used to be "silently
ignored", so someone writing a policy to auto-approve a deployment saw no error. `policiesEnabled:
false` now actually disables evaluation; it had been documented in three files and read by nothing.
And an unanswerable condition resolves to **unknown**, never true.

**What building it found.**

1. **The two gate vocabularies did not overlap.** Policy gate IDs named actions (`git-commit`,
   `deploy`); the executor's named stage progressions (`confirm-design`, `confirm-ship`). No valid
   policy could name a barrier `loom run` has, so evaluation would have run zero times forever. The
   executor's four gates are now policy-eligible. `confirm-ship` needs care in prose: it *precedes*
   the ship, commit and deploy gates rather than being one.
2. **The done-when's invalid YAML is worse than reported.** It names `policy-schema.md`'s example;
   `auto-approve-refactor.policy.yaml` — the example *this file* cites as its reference policy for
   `git-commit` — also had a duplicate `filePaths` key and had never been parsed. A lenient reader
   takes the second and discards the first, so its `**/security/**` exclusion would never have run.
3. **Five of nine condition fields have no source.** `diffLines`, `diffType`, `dryRunPass`,
   `fitnessFunction.allPass`, `codeReviewer.behaviorChange` are measured by nothing. Sourcing them
   is **L2.20**.

1. **Problem**: `policy-evaluator.md` specifies a condition language
   (`filePaths.noneMatch: "**/security/**"`, `diffLines.lessThan`, `not:`, `any:`) whose evaluator is
   a prompt. An authorization decision — *may this pipeline commit without a human* — is resolved by
   natural-language reasoning over YAML, with the kill-switch, the always-human list, and the
   conflict-resolution table all in the same prose the model may misread. `policy-schema.md`'s own
   worked example has a duplicate `diffType:` key in one map, which is invalid YAML — no parser has
   ever seen it.
2. **Architectural Fix**: `google/cel-go` or OPA/Rego. The condition schema maps near-directly onto
   CEL. The always-human gate list becomes a compiled constant. Add a policy unit-test harness and
   make `--dry-run-policies` actually execute.
3. **Target Files**: `shared/orchestration/policy-evaluator.md`, `shared/policies/policy-schema.md`,
   `shared/policies/examples/*.policy.yaml`, new `internal/policy/`
4. **Done when**: a policy targeting an always-human gate is rejected at load time, and the invalid
   YAML example above fails to parse.

### L2.19 — Honour a policy decision at a gate
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: L2.16 (shipped), L3.47 (shipped) · **Blocks**: none · *(raised 2026-09-02)*

> **Reachable, not ready — and still blocked.** *(corrected 2026-09-18)*
>
> L3.47 cleared one precondition: this item's safety argument is that an always-human gate "cannot
> be targeted at all", and until 2026-09-16 that held only by accident — gate #9 had no `GateID`, so
> a policy naming it was rejected as `unknown gate` rather than as always-human. That classification
> is now real and held by two tests.
>
> **A note added here on 2026-09-18 read that as "unblocked". It was wrong**, and this corrects it.
> The stop condition below — *do not build until real runs show the evaluator deciding what a human
> would* — is untouched by L3.47 and remains unmet.
>
> An experiment on 2026-09-18 tried to satisfy it cheaply, by dry-running a candidate policy against
> a recorded run. It could not:
> [`docs/audits/l219-policy-dry-run-2026-09-18.md`](../audits/l219-policy-dry-run-2026-09-18.md).
> No policy has ever been written in this repo, the only surviving run-state has `policyDecisions:
> ABSENT`, and **all four supposedly-sourced facts resolved UNKNOWN** — because the evaluator reads
> the typed stage documents under `<workspace>/state/`, which run archives do not retain. The facts
> existed during the run and were discarded when it was archived.
>
> So the evidence cannot be reconstructed from the nine recorded runs; it has to be gathered
> forward. Prerequisites: **(1) ~~retain typed stage state in run archives~~ — DONE, L3.57
> (`7c5183e`)**; **(3) ~~L2.20~~ — DONE**, the vocabulary is now honest and six fields resolve.
> What remains is **(2)**: write real policies and let L2.16 record decisions with
> `honoured: false` across several runs. That is the evidence this item's stop condition asks for,
> and nothing but real runs produces it.
>
> **The recording path itself is verified** (2026-09-21). A mock run carrying
> `require-human-on-critical-findings` through all three gates recorded two decisions: UNKNOWN at
> `confirm-design` naming the three facts it could not see, and **FALSE at `confirm-security` with
> every fact known** — the evaluator deciding on real state, recorded with `honoured: false`. So
> the plumbing is not what is missing; judgement data is. A mock's facts are scripted, so a mock
> run cannot show the evaluator agreeing with a human, which is the thing the stop condition asks
> about.
>
> Two traps found while doing it, now documented in `shared/policies/README.md`: **four of the six
> shipped examples watch gates `loom run` never halts at**, so following them accumulates nothing;
> and a policy at `confirm-design` can never see review, security or QA facts, because that gate
> precedes those stages. Watch `confirm-security` or `confirm-ship`.
>
> Note what (1) does *not* do: the nine already-recorded runs stay unanalysable, because their
> documents were discarded. The corpus starts empty and fills from the next run onward.
>
> **The corpus is now readable** (2026-09-21, `loom memory policies`). Prerequisite (2) was
> "let L2.16 record decisions across several runs" — and the evidence it names was **write-only**:
> decisions went into `policy_decisions` at ingest and nothing read them back, so the stop condition
> could only be judged by hand-reading archives. Worse, the ingest was lossy in the one dimension the
> judgement needs. It kept `gate, effect, honoured, at` and dropped each policy's own outcome and the
> facts it could not see, so a blind UNKNOWN and a considered UNKNOWN were indistinguishable in the
> store. And it recorded nothing about **what the human did at the same gate**, which is the other
> half of any comparison.
>
> Fixed: `policy_outcomes` and `gate_approvals` (schema v3, a rebuildable projection), and a query
> that reads each decision beside the approval that followed it. The command states its own limits —
> "agreed" is concurrence rather than an independent second opinion, because the human could see the
> decision, and `honoured` is expected to be 0 until this item ships.
>
> **The stop condition is unchanged and still unmet.** Nothing here honours a gate. This makes the
> question checkable from data instead of from memory; it does not answer it, and only real runs can.

1. **Problem**: L2.16 evaluates policies and records what they decided, then halts for a human
   anyway. The feature people actually want from policies — a gate that proceeds without a
   prompt — does not exist. That was deliberate: the first run to skip a barrier should not also be
   the first evidence the evaluator is correct.
2. **Architectural Fix**: `checkGate` honours a decision whose effect is `auto-approve`, recording
   `honoured: true` and an approval attributed to the policy rather than to a person. Gated behind
   the existing `policiesEnabled` switch, and never for an always-human gate, which cannot be
   targeted at all. The change is small; the evidence it needs is not.
3. **Target Files**: `internal/orchestrator/gate.go`, `policy_gate.go`, `shared/rules/approval-gates.md`
4. **Done when**: a run with a matching auto-approve policy proceeds without a prompt, the approval
   names the policy, and recorded history shows the same decisions before and after the change.

**Do not build this until real runs show the evaluator deciding what a human would.** That is the
entire reason L2.16 stopped short, and the records it writes are how the question gets answered.

### L2.20 — Source the condition facts nothing measures
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.16 (shipped) · **Blocks**: none · *(raised 2026-09-02)*

**SHIPPED** 2026-09-18 — `73e78a3`, `70da3df`, `e710f6f`. The done-when is met: every field the
vocabulary declares either resolves from run state or is gone.

**Three removed rather than sourced.** `diffType` declared an OPEN set (`docs-only`,
`test-additions`, …) — with no defined list of values it could not be implemented or tested, and
`filePaths.allMatch` already expresses it precisely. `dryRunPass` and `fitnessFunction.allPass`
need CI results the executor never sees. A policy naming a removed field now fails to load.

**Two sourced.** `codeReviewer.behaviorChange` (code-reviewer 2.2.0) is a self-report, so an
`auto-approve` policy naming it **fails to load** — including nested under `not:`/`any:`. It may
be read by `require-human`, `auto-reject` and `escalate`, where the worst case is a human looking
at something that did not need it. `codeReviewer.verdict` stays unrestricted deliberately: the
verdict IS the review's output, not the reviewer grading its own significance.

`diffLines` is measured against the commit the run started from, and **recorded at the gate**
rather than recomputed — `PolicyContextFor` promises a dry-run and a live evaluation cannot
disagree about what was visible. The first attempt read git live and left every dry-run blind.

**The specified baseline could not be built.** Diffing against the starting tree *digest* is
impossible: `Digest()` is a SHA-256 of `git diff HEAD` plus status, a one-way hash. The starting
*commit* is the reachable equivalent and covers committed and uncommitted work in one number.

**What this does not do.** L2.20 makes the vocabulary honest; it does not make policies useful.
L2.19 still needs real runs recording decisions — see its entry.

1. **Problem**: `policy-schema.md` declares nine condition fields; four have a source in run state.
   `diffLines`, `diffType`, `dryRunPass`, `fitnessFunction.allPass` and
   `codeReviewer.behaviorChange` are measured by nothing, so three of the five shipped example
   policies can never evaluate — they report **unknown**, correctly and uselessly.
2. **Architectural Fix**: Each fact needs a real source, and they are not the same kind of work.
   Diff size and type mean the executor learning to run `git diff`, which it does not do today and
   which raises its own question (diff against what — the run's start, the branch point, HEAD?).
   `fitnessFunction.allPass` and `dryRunPass` mean capturing CI results the executor never sees.
   `codeReviewer.behaviorChange` is a field the review contract could simply declare, and is the
   cheap one.
3. **Target Files**: `internal/orchestrator/policy_gate.go`, `internal/state/review_state.go`,
   new diff source, `shared/policies/README.md`
4. **Done when**: every field `policy-schema.md` declares either resolves from run state or is
   removed from the schema — a vocabulary should not describe questions nothing can answer.

---

## Workstream: PLATFORM — Distribution & Adoption

*Appended 2026-08-29. Strategy: MCP becomes the portable capability surface (executable behavior on
every MCP-speaking host); the dotfile/markdown export becomes the Level 1 convenience layer. loom
ships both a standalone server (`loom mcp serve`) and a semver'd embedding package, from one module.
The maturity ladder (L1→L4) becomes a first-class install concept rather than a roadmap-only idea.
These items constitute loom's **public API** — several are adoptable by external teams before the
orchestration kernel exists, which is why they sit in Milestone 1 despite spanning levels.*

### D.1 — Fold the MCP server into the `loom` binary as `loom mcp serve`
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: none · **Blocks**: D.3, D.5

**SHIPPED** 2026-08-29 (epic 75 Phase A, `248a742`) — `loom mcp serve`, tested in
`cmd/loom/cmd/mcp_serve_test.go`; `.goreleaser.yaml` builds the single `loom` binary. The `brew
install` half of the done-when was not re-verified in the 2026-09-22 reconciliation.

1. **Problem**: The MCP server is a separate module with its own entrypoint
   (`shared/mcp/cmd/mcp-server/main.go`, own `go.mod`), while the distributed binary is `cmd/loom/`.
   Teams adopting via `brew install orieken/tap/loom` get the installer but not the server — the
   framework's only *executable* capabilities require a second, unpublished build. Two artifacts,
   one tap entry.
2. **Architectural Fix**: Add a `loom mcp serve` Cobra subcommand that starts the server over stdio
   (network transport arrives with L2.8). Either merge the `shared/mcp` module into the root module
   or add a `go.work`/`replace` so one `goreleaser` build embeds both. The standalone
   `mcp-server` binary is kept building through one release cycle, then removed.
3. **Target Files**: `cmd/loom/cmd/` (new `mcp.go`, `mcp_serve.go`), `go.mod`,
   `shared/mcp/cmd/mcp-server/main.go`, `shared/mcp/register/register.go`, `.goreleaser` config
4. **Done when**: `brew install`ed `loom mcp serve` responds to an MCP `tools/list` over stdio with
   all six framework tools, and the release pipeline ships exactly one binary per platform.

### D.2 — Publish the embedding API as a semver'd public package
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: M0.3, L2.4 · **Blocks**: D.5

**SHIPPED** 2026-08-29 (epic 75 Phase B, `f597c8d`; example tidied `d23e5cd`) — public `tools/`
package, stdlib-only by `tools/deps_test.go`; `examples/embedding` is built by `framework-ci.yml`.

1. **Problem**: `register.FrameworkTools` is the right seam for "use loom's tools in your own MCP
   server," but it takes `*server.MCPServer` from `mark3labs/mcp-go` — embedding it welds every
   consumer to loom's transitive mcp-go version, and the `internal/` packages behind it are
   correctly unimportable but leave no public `Tool` contract for third parties to implement.
   "Others can extend" currently means "fork the repo."
2. **Architectural Fix**: After M0.3's port/adapter split, expose a public package (e.g.
   `github.com/orieken/loom/tools`) containing the transport-free `Tool` interface, the
   `ToolRegistration` type (timeout, retry class, permission scope — from L2.4), and a
   `Registry.Merge` API. `register.FrameworkTools` becomes a thin compatibility wrapper. Tag and
   semver the module; document the compatibility promise. Extension = implement the interface +
   one `Register` call, compile-time Go embedding first (typed, simple); a subprocess/plugin
   mechanism only if demand materializes.
3. **Target Files**: new `tools/` public package, `shared/mcp/register/register.go`,
   `shared/mcp/internal/server/tool_provider.go`, `shared/mcp/README.md`
4. **Done when**: an out-of-repo example project registers a custom tool against the public package
   without importing anything under `internal/` or any `mcp.*` type, and CI builds that example.

### D.3 — Maturity-level install profiles: `loom init --level N`
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: D.1 · **Blocks**: D.4 · *(audit H10 context tax)*

**SHIPPED** 2026-08-29 (epic 75 Phase C, `762457c`) — `shared/levels.yaml` with a documented
core-bundle byte ceiling (lowered by L3.19 on 2026-09-22).

1. **Problem**: The maturity ladder exists in this roadmap but not in the product. `loom install`
   drops the full corpus — 40 agents, 70 skills, every language convention — on every project, so a
   Level 1 team pays the full context tax (C.1's ~20k-token problem) for capabilities three levels
   above where they are. There is no way to adopt loom incrementally, which is the entire pitch.
2. **Architectural Fix**: Define level profiles as data (`shared/levels.yaml`): **L1** = core rules
   (guardrails, gates, trust boundary — the small always-on set) + agents/skills as prompts;
   **L2** = + `loom mcp serve` config and workflow YAML + executor when M0.4 lands; **L3** = +
   planner/parallelism/retrievers; **L4** = + reflexion/budget/prompt-registry layers. Split the
   rules corpus into an always-loaded core (~200 lines) and on-demand modules to make L1 cheap.
   `loom init --level N` (and `loom install --level N`) selects the bundle; default remains
   current behavior until profiles stabilize.
3. **Target Files**: new `shared/levels.yaml`, `cmd/loom/cmd/install_options.go`,
   `cmd/loom/internal/platform/`, `shared/rules/` (core/on-demand split), `README.md`
4. **Done when**: a fresh `loom init --level 1` installs the core bundle only, measured injected
   context is under a documented token ceiling, and `--level 2` adds exactly the L2 delta.

### D.4 — Teach `loom health` to report maturity level
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: D.3 · **Blocks**: none

**SHIPPED** 2026-08-29 (epic 75 Phase D, `8991d0b`) — `loom health` infers maturity level;
`TestInferMaturityAcrossAllLevels` covers all four.

1. **Problem**: "Help teams graduate from Level 1 to 2 to 3" has no instrument. Nothing tells a
   team what level they are at, what evidence supports that, or what specifically is missing for
   the next level. Adoption progress is vibes.
2. **Architectural Fix**: Extend `loom health` with a level assessment derived from mechanical
   checks against the D.3 profiles: which bundle is installed, is the MCP server configured and
   answering, do workflow definitions exist, is telemetry present, is the executor in use.
   Output: current level, the passing evidence, and a checklist of gaps to the next level. Never
   report a level whose enforcement layer isn't actually installed and answering — documentation
   alone does not confer a level.
3. **Target Files**: `cmd/loom/cmd/health_checks.go`, `cmd/loom/cmd/health_run.go`,
   `cmd/loom/cmd/health_output.go`, `shared/levels.yaml`
4. **Done when**: `loom health` on a fresh L1 install prints "Level 1" with a concrete L2 gap list,
   and unit tests cover the level-inference rules for all four levels.

### D.5 — Grow the MCP surface from lint tools to framework capabilities
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: D.1, D.2, L2.9 (state-read tools only) · **Blocks**: none

**PARTIAL** — `validate_artifact` shipped structural-only (`c0e8441`, 2026-08-29). The other three
tools were waiting on L2.12, L3.9 and L2.16, **all of which have since shipped**, so `pipeline_state`
(the half of the done-when still missing) is now unblocked. Status verified 2026-09-22.

1. **Problem**: The server exposes six introspective lint/search tools. The framework's actual
   capabilities — artifact contract validation, pipeline state, telemetry queries, policy
   evaluation — are prose-only, so a non-Claude host (or a team's own agent runtime) can adopt
   loom's *linting* but none of its *process*. The portable surface undersells the framework.
2. **Architectural Fix**: Add tools as their code-backed implementations land, never ahead of them:
   `validate_artifact` (structural contract checks, with L2.11), `pipeline_state` (read-only, after
   L2.12 gives state a single owner), `query_telemetry` (read-only over `events.jsonl`, after L3.9
   fixes the schema), `evaluate_policy` (after L2.16 makes evaluation real). Read-only tools first;
   anything mutating pipeline state stays exclusive to the executor. The scope boundary from the
   distribution strategy holds: **MCP exposes tools and resources; the orchestration kernel is
   `loom run` acting as an MCP client (L4.7), never a tool someone else calls** — do not let the
   pipeline itself become a tool call.
3. **Target Files**: `shared/mcp/internal/tools/` (new tools), registry entries per L2.4,
   `shared/mcp/README.md`
4. **Done when**: an MCP host with no loom markdown installed can validate an artifact against a
   contract and read pipeline state for a run, via tool calls alone.

### L2.17 — Bring the developer↔code-reviewer loop under the executor
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: M0.4 · **Blocks**: none · *(raised 2026-08-31, epic 80 review)*

**SHIPPED** 2026-08-31 (epic 82, `0aafaf1`…) — the loop is a span declared in plan data with a
named condition over a typed review verdict and a bound of three rounds. Every round is retained
and digested; exhausting the bound halts at `confirm-unresolved-review` for a human. The markdown
pipeline's step 21, previously unbounded, now states the same bound. **Not** wired to the Tier B
contract-retry loop — the mechanism generalises to it, and that work is now **L2.18**, which has to
put validation under the executor first.

1. **Problem**: `deliver-feature/SKILL.md` steps 18–21 describe an *iteration*: code-reviewer returns
   CHANGES REQUESTED, the current `implementation-notes.md` and `code-review-report.md` are copied to
   `.history/`, and the pipeline repeats from step 18 "until APPROVED and structurally valid". The
   same shape appears in the Tier B contract-retry loop (`maxContractRetries`, default 3) at every
   validate-artifact step. None of it is executed by anything: the loop condition, the bound, the
   history backup, and the decision to stop are all instructions an LLM is asked to follow about its
   own prior output. The executor cannot help — `Plan` is a linear list of stages with no notion of
   a cycle, so `loom run` invokes the developer exactly once and the code-reviewer's verdict changes
   nothing. This is the largest remaining piece of the pipeline that exists only as prose.
2. **Architectural Fix**: A bounded loop as plan data — a stage (or stage group) declaring
   `repeat_until` with a machine-checkable condition and a `max_iterations`, evaluated by the
   executor rather than the model. Iterations are recorded in run state (the `Sequence` field from
   L2.12 already distinguishes a re-run from a new step), each iteration's artifacts are retained
   rather than overwritten, and exhausting the bound is a halt with a clear reason, never a silent
   pass. The review verdict must become a typed field a condition can read (L2.9 for the review
   artifact), not prose a model re-reads.
3. **Target Files**: `internal/orchestrator/plan.go` (loop declaration), new
   `internal/orchestrator/loop.go`, `internal/state/` (typed review verdict),
   `shared/skills/deliver-feature/SKILL.md:99-104`, `shared/contracts/review-contract.md`
4. **Done when**: a code-reviewer stage returning CHANGES REQUESTED causes `loom run` to re-invoke
   the developer with the review findings and stop after a declared bound, with every iteration
   visible in run state — and no prose instruction anywhere in the path.

### L2.18 — Run contract validation under the executor, and bound its retries
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: L2.17 (shipped), L2.11 · **Blocks**: none · *(raised 2026-08-31)*

1. **Problem**: `deliver-feature` calls `validate-artifact` between every contract-bound handoff and
   wraps it in a Tier B retry loop — "apply Tier B retry loop up to `maxContractRetries`" appears at
   a dozen steps. None of it executes: `validate-artifact` is a skill a model runs, the retry count
   is a number a model is asked to remember, and the executor has no idea any of it happened. L2.17
   built a bounded loop for exactly this shape and deliberately did not wire it here, because the
   prerequisite is larger than the wiring: **validation itself does not run under the executor at
   all**. A run can pass every contract gate without the executor knowing a gate exists.
2. **Architectural Fix**: Make validation an executor stage — an internal stage like the router
   (L3.0), evaluating a contract against a typed artifact — then declare `agent → validate` as a
   bounded loop with the L2.17 mechanism, condition `validation-passed`, bound `maxContractRetries`.
   Exhausting it halts at a gate, as the review loop does. For typed artifacts this is schema
   conformance the executor already performs at stage output; the work is the stages still on
   markdown, and how a failure's reasons reach the producing agent's next attempt (a projection,
   as with review findings).
3. **Target Files**: `internal/orchestrator/` (validation as an internal stage, loop declaration),
   `shared/skills/validate-artifact/SKILL.md`, `shared/skills/deliver-feature/SKILL.md` (the dozen
   Tier B call sites), `shared/contracts/*.md`
4. **Done when**: a stage producing a contract-violating artifact is re-invoked with the violations,
   bounded, and the run halts for a human when the bound is reached — with no prose instruction in
   that path.
5. **Note on ordering**: this is worth doing *after* L2.11 gives validation something semantic to
   check. Wiring a bounded retry loop around a heading-presence grep would mechanise a check that
   a structurally perfect, semantically empty artifact already passes.

---

# MILESTONE 2 — Level 3: Autonomous Orchestration Layer

*Dynamic routing, cross-domain collaboration, minimal human intervention, dedicated governance.*

## Workstream: KERNEL — Dynamic Routing

### L3.0 — Compute the route from the analysis, before the design gate
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.9 (first cut), L2.12, L2.13, L2.14 — all shipped · **Blocks**: L3.1 · *(raised 2026-08-31)*

**SHIPPED** 2026-08-31 (epic 81, `a4f458a`…) — an executor-internal `router` stage computes the
route from typed analysis after the analyst, records one decision and reason per stage, and marks
skips before the developer runs. The route is an artifact, so approving `confirm-design` binds it
and editing it resets that approval. Two findings changed the design: a gate now survives its stage
being routed out (skipping devops was silently deleting the ship checkpoint), and a reroute clears
an earlier skip so work can come back.

1. **Problem**: `loom run` executes all fourteen stages unconditionally. The markdown pipeline does
   better — six of its stages are conditional — but the conditions are prose an LLM evaluates about
   an artifact it just read, and a skipped stage leaves nothing durable saying *why*. Neither
   pipeline can answer "is devops running on this feature, and if not, why not?" before it gets
   there. L3.1 answers this with a planner selecting over a capability registry (L3.2), which is the
   right end state and a long way off; almost every condition the pipeline actually needs is
   already a fact in typed `AnalysisState`.
2. **Architectural Fix**: A **re-plan point** after the analyst: a fixed prologue
   (`context-engineer`, `analyst`) runs, then the executor computes the route from typed analysis
   via predicates in Go — `RequiresArchitect()` (shipped, epic 79) and its siblings — and records it
   as a typed **route artifact**: one row per stage, included or skipped, with the reason. Skipped
   stages enter run state as `SKIPPED` immediately, so the whole shape of the run is visible before
   the second stage finishes. The route is an artifact like any other, so it is digest-recorded, and
   because it completes before `confirm-design`, L2.14 binds it: **the human approves the route
   along with the design, and editing the route resets that gate.** Forcing a stage back in is
   therefore a supported, attributed, gate-bound act rather than a workaround.
   **Skippability is an allow-list in plan data**: the review stages (`code-reviewer`,
   `security-reviewer`) are never auto-skipped, because the cost of wrongly skipping a review is
   asymmetric with the cost of running one unnecessarily. A human may still skip them by editing the
   route, which the gate then makes them re-approve.
3. **Target Files**: `internal/state/route.go` (typed route + predicates), `internal/orchestrator/plan.go`
   (`Skippable`, re-plan point), `internal/orchestrator/state.go` (`StageStatusSkipped`, skip
   reason), `shared/skills/deliver-feature/SKILL.md` (steps 12–29 conditionals reference the same
   predicates), `cmd/loom/README.md`
4. **Done when**: a feature with no infrastructure work skips `devops-engineer` by a recorded route
   decision — visible in `loom state show` and on the timeline before the developer stage starts —
   and hand-editing the route invalidates the `confirm-design` approval.
5. **Known limit**: a route computed from the analysis can be wrong about work the analysis did not
   foresee — the developer touching UI files a spec never mentioned. Re-planning mid-run is a cycle,
   which `Plan` cannot express; that is **L2.17**'s mechanism, and the two should land in that order
   or be designed together.

### L3.1 — Build a Planner/Router node
*(Scoped alongside **L3.0**, which computes the route from typed analysis with predicates in Go and
needs no registry. L3.1 is the general form: a planner selecting over declared agent capabilities,
able to route to agents it was never hardcoded to know about.)*
**Workstream**: KERNEL · **Effort**: XL · **Blocked by**: L2.9, L3.2 · **Blocks**: L4.2

1. **Problem**: There is no routing anywhere in the codebase. `deliver-feature/SKILL.md` is a
   hand-numbered 43-step list. Branching is static prose conditionals evaluated by reading markdown
   (steps 12/14/16). `pipeline-schema.md:118-128` defines the entire condition language as "simple
   dot-path equality checks" — `"feature.hasUI == true"`, `"analysis.architecturalFlags != 'None'"` —
   explicitly "no loops, no function calls, no side effects." Nothing lets an agent decide who runs
   next.
2. **Architectural Fix**: Split the executor into a graph runtime plus a `Planner` node that emits
   the next node ID (or a sub-graph) from typed state. Conditionals become CEL predicates over state
   fields, not dot-path string comparisons over documents. Keep the current linear pipeline as one
   registered *default plan*, so existing behavior is a special case of the router rather than a
   parallel code path.
3. **Target Files**: `shared/skills/deliver-feature/SKILL.md`, `shared/skills/orchestrate/SKILL.md`,
   `shared/orchestration/pipeline-schema.md`, new `internal/orchestrator/planner.go`
4. **Done when**: a spec with no UI skips the accessibility stage via planner decision, not a
   hardcoded conditional, and the decision is visible in the trace.

### L3.2 — Publish a machine-readable agent capability registry
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: L2.9 · **Blocks**: L3.1, L4.8

1. **Problem**: A router needs something to route *to*. There are 40 agents as markdown prose in
   `shared/agents/`. Their frontmatter carries `name`, `description`, `tools`, `model_tier`,
   `version` — no declared inputs, no declared outputs, no preconditions, no postconditions, no
   cost/latency class. `agent-frontmatter.schema.json` sets `additionalProperties: false`, so none of
   that can be added without a schema change.
2. **Architectural Fix**: Extend the frontmatter contract with `consumes: [state fields]`,
   `produces: [state fields]`, `preconditions: [CEL]`, `cost_class`. Generate `agent-registry.json`
   at build time; the planner selects over it. This also makes the capability catalog auditable and
   diffable.
3. **Target Files**: `shared/schemas/agent-frontmatter.schema.json`,
   `shared/contracts/agent-frontmatter-contract.md`, all 40 `shared/agents/*.md`,
   `scripts/generate-configs.sh`
4. **Done when**: `agent-registry.json` is generated in CI and an agent declaring a `produces` field
   no other agent `consumes` is flagged.

### L3.3 — Implement real parallelism
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: M0.4 · **Blocks**: none

1. **Problem**: `pipeline-schema.md:132` documents `sequential-simulation` as the default and defines
   it as "the LLM invokes parallel stages sequentially but treats them as logically parallel." The
   headline parallel-branch feature ships disabled by definition. `orchestrate/SKILL.md` step 6 asks
   the model to "collect adjacent `parallel: true` stages into a group" by reading YAML it cannot
   execute.
2. **Architectural Fix**: Real fan-out/join in the executor — `errgroup` with bounded concurrency,
   per-branch isolated state scopes, deterministic merge at the join with declared conflict
   resolution. Delete `sequential-simulation`; a fake concurrency mode is worse than none.
3. **Target Files**: `shared/orchestration/interface.md`,
   `shared/orchestration/pipeline-schema.md:130-134`, `shared/skills/orchestrate/SKILL.md`,
   `shared/workflows/*.md`, new `internal/orchestrator/parallel.go`
4. **Done when**: security-reviewer and accessibility-engineer complete in wall-clock time closer to
   `max(a,b)` than `a+b`.

## Workstream: MEMORY

### L3.4 — Implement the retriever backends that are currently markdown
**Workstream**: MEMORY · **Effort**: L · **Blocked by**: L2.7 · **Blocks**: L3.5, ADR-002's fitness function

**Also owns the `retrieval.queried` emitter** (added 2026-09-01, epic 86). ADR-002 declared
retrieval quality a judgment-only fitness function whose evidence would be a telemetry log of
retrieval events. That log never existed, the event type was never defined, and the layer was
retired in L3.9 — so retrieval quality is currently **unmeasured**, and graduating a corpus to a
higher tier is a judgement with no data behind it. Emitting `retrieval.queried` is not a row in a
table: it means a retriever that runs as code and can report what it was asked and what it returned,
which is this item. `shared/evaluation/retrieval-regression.md` waits on the same thing.

1. **Problem**: `shared/rag/retriever.interface.md` is a well-specified contract — references not
   content, bounded top-K, corpus isolation, no side effects. Then three of its four adapters
   (`llm-as-retriever.md`, `vector.md`, `source-retrieval.deferred.md`) are prose files. Only BM25
   exists in code. No vector store, no embedding pipeline, no graph.
   `shared/knowledge/ollama-local-embeddings.md` is documentation, not an implementation.
2. **Architectural Fix**: Implement `Retriever` as a real Go interface with BM25 refactored to
   satisfy it, then add a vector adapter with a pluggable embedding provider (local Ollama or hosted,
   behind an interface). Use **`sqlite-vec`**, not the `sqlite-vss` named in `agy.md` — vss is
   deprecated in favor of vec. Hybrid retrieval = reciprocal-rank fusion, not the round-robin
   interleave the interface doc currently prescribes.
3. **Target Files**: `shared/rag/retriever.interface.md`, `shared/rag/adapters/vector.md`,
   `shared/mcp/internal/tools/retriever.go`, `shared/mcp/internal/tools/bm25_retriever.go`
4. **Done when**: a conceptual paraphrase query that BM25 misses is answered by the vector adapter.

### L3.5 — Add episodic memory
**Workstream**: MEMORY · **Effort**: L · **Blocked by**: L3.8 (shipped) · **Blocks**: L4.4, L4.6

**SHIPPED** 2026-09-02 (epic 88, `9e3ce12`…) — `internal/memory` is a project-local sqlite store of
what runs actually did, with `loom memory` to query it. Ingest is automatic at the end of every run
and rebuildable from the archive.

**It collects nothing new, and that is the finding.** This item's Problem says nothing can learn
from execution. That stopped being true two epics ago without anyone reconciling it: usage (L3.8),
corrections with diffs (L4.5), policy decisions (L2.16), loop iterations (L2.17), routing reasons
(L3.0) and measured stage timings (M0.4, L2.12) are all produced per run — and all died with the
feature workspace. This epic is a store and a query surface over data that already existed.

**Three structural choices.**

1. **Ingest, not co-writing.** The executor is untouched. A database inside the run loop adds a
   locking failure mode that could disturb a delivery and duplicates a record already written
   reliably. Ingest is idempotent, so re-ingesting is a no-op and any past run can be imported.
2. **The records are now archived.** `run-state.json` and `run-events.jsonl` lived only in the
   temporary workspace and died on cleanup; they are persisted into `docs/features/<name>/`
   alongside the artifacts they describe. That archived JSONL is the durable record and the database
   is a projection of it — which is why `.claude/memory/` is gitignored and why deleting the store
   is recoverable rather than a loss.
3. **Project-local.** A global store would pool data from every repository loom runs in, including
   client code, into one file outside those repositories. That is a privacy posture change and not
   one a memory epic gets to make as a side effect.

**Deliberately not built**: the `episodic` CorpusID and any retriever adapter. Its adapters are
markdown specs with no running backends and the planner that would consume it is **L3.1**, so adding
an interface entry nothing implements is the defect epics 84–87 kept cleaning up. **L3.4** adds the
corpus when there is a retriever to add it to.

**The done-when was ambiguous and is now pinned.** "Retried more than twice" reads as iteration
count > 2 — at least three attempts — and the query's output states that, because the other reading
is equally defensible and a reader should not have to guess which produced the rows.

1. **Problem**: "Organizational memory" is semantic only: markdown KIs plus ADRs. There is no
   episodic store — no record of what was attempted, what failed, what a retry changed, or what a
   human corrected. `pipeline-trace.json` is specified to hold that and **does not exist anywhere in
   the repo**. Consequently nothing can learn from execution, which blocks all of Milestone 3.
2. **Architectural Fix**: An append-only run store (sqlite) keyed by `run_id`/`stage_id` capturing
   inputs, outputs, tool calls, retries, gate decisions, and human edits. *(The human-edit half now
   exists per run and should be **adopted rather than reinvented**: epic 85 records
   `artifact.corrected` in run state and on `run-events.jsonl` with a retained diff. This item's job
   for it is durability across runs and queryability, not detection — that is built.)* Index it as a retrievable
   corpus so the planner can condition on "how did this go last time."
3. **Target Files**: `shared/skills/pipeline-trace/SKILL.md`,
   `shared/rag/retriever.interface.md` (the `episodic` corpus — **left to L3.4**), new
   `internal/memory/`
   *(`shared/telemetry/event-schema.md` was here; it is deleted. The event vocabulary this item
   stores is now generated from `internal/orchestrator/vocabulary.go` into
   `shared/schemas/telemetry/` — L3.9 shipped, so this item **inherits a vocabulary rather than
   needing to invent one**, and adding an episodic event type means adding it to that enum, where
   the fitness function will require it to be documented.)*
4. **Done when**: "show me every run where code-reviewer retried more than twice" is answerable by
   query.

### L3.6 — Generate the memory registry; add eviction
**Workstream**: MEMORY · **Effort**: M · **Blocked by**: L3.4 · **Blocks**: L4.6

1. **Problem**: `shared/memory-registry.json` is hand-maintained. KI curation is four separate LLM
   skills — `memory-engineer`, `memory-compression`, `memory-expansion`, `forgetting-engine` —
   performing garbage collection by natural language, each requiring a human approval pass. No index,
   no recency decay in code, no automatic eviction, and no measure of whether any KI was ever
   retrieved and used.
2. **Architectural Fix**: Generate the registry from frontmatter at build time; validate in CI. Track
   retrieval hit-counts and last-used from the retriever, and drive staleness/eviction from that
   signal rather than an LLM's monthly judgement. Keep human approval for deletion; automate the
   *detection*.
3. **Target Files**: `shared/memory-registry.json`, `shared/skills/memory-engineer/SKILL.md`,
   `shared/skills/forgetting-engine/SKILL.md`, `scripts/health-check.sh`
4. **Done when**: a KI never retrieved in 6 months is flagged automatically with no LLM call.

### L3.7 — Structurally separate retrieved content from instructions
**Workstream**: MEMORY · **Effort**: M · **Blocked by**: L3.4 · **Blocks**: none

1. **Problem**: KIs are read whole into the context window. `shared/rules/memory-trust-boundary.md`
   correctly identifies synced org KIs (`sync_source` frontmatter, ADR-003 pull) as an injection
   vector — and then mitigates it by *asking the model in a prompt* to treat other prompt text as
   data. The defense occupies the same channel as the attack. `sync-memory.sh` validates frontmatter
   schema only; body content enters agent context unaudited.
2. **Architectural Fix**: Retrieved content is delivered in a structurally distinct, clearly
   delimited channel with provenance metadata attached, never concatenated into the instruction
   region. Add a deterministic pre-ingestion scanner in `sync-memory.sh` for imperative-override
   patterns, so untrusted bodies are flagged before reaching any model. Prompt-level caution remains
   as defense-in-depth, not the primary control.
3. **Target Files**: `shared/rules/memory-trust-boundary.md`, `scripts/sync-memory.sh`,
   `shared/mcp/internal/tools/search_ki_tool.go`, `shared/agents/analyst.md`
4. **Done when**: a KI containing "ignore your previous instructions" is flagged by
   `sync-memory.sh` before it can be pulled.

## Workstream: OBSERVE — Governance & Auditing

### L3.8 — Emit OpenTelemetry with GenAI semantic conventions
**Workstream**: OBSERVE · **Effort**: L · **Blocked by**: M0.4 · **Blocks**: L3.5, L3.9, L4.3 (with L2.16), L4.5

**SHIPPED** 2026-09-01 (epic 84, `1031891`…`71538ae`) — `internal/telemetry/` emits OTel traces from
the executor and the MCP server. A run produces one trace: a root run span, a child per stage, and a
grandchild per model invocation carrying `gen_ai.*` usage. Token counts and cost are **reported by
the claude CLI's own JSON envelope**, never computed here, and land in run state as well as the
trace — so `loom state show` and the run summary answer "what did this cost" with no collector
configured. Network export is opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT`; a local OTLP/JSON
`traces.jsonl` is written per run by default, because the reason to make export opt-in is egress and
a file beside `run-state.json` has none.

Guardrail #8 is now structural rather than reviewed: the `Tracer` interface lives with its consumer
in `internal/orchestrator`, `internal/telemetry` implements it, and an import-graph test asserts
that `internal/state`, `internal/orchestrator`, `shared/mcp/internal/domain` and `tools` reach no
OpenTelemetry package — with a companion test that fails if `internal/telemetry` ever stops
importing one, so the check cannot quietly become vacuous.

**Two honest limits.** MCP trace propagation is best-effort: the chain is `loom run` → `claude -p` →
`loom mcp serve`, loom spawns only the first hop, and MCP's protocol carries no trace context, so a
`TRACEPARENT` environment variable is the only channel. When it survives, a tool call lands under
its stage; when it does not, the call starts a clean trace of its own. And the envelope field names
are unverified against the live CLI — see **L3.15**.

**Blocks, corrected.** This entry previously claimed L4.6, which lists `L3.6, L3.10, L4.4` as its
own blockers and never named L3.8; the header's dependency graph reaches L4.4 through L3.5 and L4.5
rather than directly. Four items are directly unblocked, one of them only partly: **L3.5** and
**L4.5** are now fully unblocked, **L3.9** is unblocked, and **L4.3** still waits on **L2.16**
though its usage signal now exists.

1. **Problem**: `event-schema.md` has no `token_count`, no `cost`, no `trace_id`, no `span_id`, and
   no parent/child correlation — only a `pipeline_id` convention in free-form `metadata`.
   `duration_ms` and `pipeline-trace.json`'s `durationSeconds`/`budgetUtilization` are nominally
   recorded by an LLM that cannot measure elapsed time. No OTel is emitted anywhere, despite
   `architecture-guardrails.md` #8 mandating it from the adapter layer and `testing-conventions.md`
   requiring it on every BDD scenario. "What did this pipeline cost" is currently unanswerable.
2. **Architectural Fix**: OTel SDK in the executor and the MCP server. Spans per stage and per tool
   call using GenAI semconv (`gen_ai.operation.name`, `gen_ai.usage.input_tokens`,
   `gen_ai.usage.output_tokens`, `gen_ai.request.model`), tool-call payloads as span attributes with
   a size cap and secret redaction, real wall-clock latency, real trace propagation across handoffs.
   OTLP export; keep `events.jsonl` as a local file exporter for offline mode.
3. **Target Files**: `shared/telemetry/event-schema.md`, `shared/telemetry/event-recorder.md`
   (delete), `shared/mcp/internal/logging/logger.go`, new `internal/telemetry/`
4. **Done when**: a completed run produces a single trace with per-stage token counts and a total
   cost figure.

### L3.9 — Resolve the telemetry schema contradiction and generate the schema
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L3.8 (shipped) · **Blocks**: none · *(audit H6)*

**SHIPPED** 2026-09-01 (epic 86, `d4df34a`…`b134dc2`) — one Go enum in
`internal/orchestrator/vocabulary.go` is the source of truth; `shared/schemas/telemetry/`'s JSON
Schema and event-types table are generated from it by `go run ./cmd/gen-schemas`, with a drift test.
`shared/telemetry/event-recorder.md` and `event-schema.md` are deleted, and every live instruction
that pointed at `.claude/telemetry/events.jsonl` is redirected.

**The done-when was amended, and made stricter.** It read: *"all 15 event types are in one enum and
adding a 16th without documenting it fails to compile."* That count was wrong — it assumed all
fifteen were real. Fourteen are emitted and in the enum; the other nine were specified across the
spec files and emitted by nothing. Meeting the letter would have meant putting types that fire from
nowhere into the source of truth, which is the trap this item exists to close. So the enum holds
**only what is emitted**, the nine live in one table in `shared/telemetry/README.md` naming the item
that would build each emitter, and the fitness function parses the constants out of the source with
`go/ast` rather than trusting a list — a registry that enumerated itself would pass happily while a
new constant sat undocumented beside it.

**Where the enum lives, against this item's own target-files line.** It stays in
`internal/orchestrator`, not the proposed `internal/telemetry/events.go`: epic 84's guardrail
fitness function forbids the orchestrator importing the telemetry adapter, and the enum belongs
where the emitter is. That line predates the boundary.

**What building it found.** The premise "emitted by nothing" was not quite right — `deliver-feature`
instructed emission at seven points, and three other files added more. Prompt-discipline
instructions, never verified, and a v3.0.0 release check recorded the file was never created; but
instructions, and one of them backed a stated guarantee. `approval-gates.md` claimed a policy-based
gate emits `policy.evaluated` for every decision, so there are no silent auto-approvals. Nothing
recorded those decisions and nothing ever had. That file now states the gap rather than losing it:
the requirement is unchanged, no audit trail exists, and **L2.16** is what would produce one. See
also the ADR-002 amendment — its judgment-only fitness function named the same non-existent layer as
its evidence.

One boundary from epic 84 that this item kept: **traces are not events**. The run's OTel trace and
`run-events.jsonl` answer different questions — timing and cost versus gates, digests and staleness
— and folding them together would cost the audit log its independence from an exporter being
configured.

1. **Problem**: `event-recorder.md` instructs: "**Never** invent a new `event_type` — refuse if the
   caller passes one not documented." `event-schema.md` documents **six** types. Nine more are
   specified as emitted across the spec files: `policy.evaluated`, `policy.conflict`,
   `policy.skipped` (policy-evaluator.md), `audit.fail`, `audit.retry`, `audit.halt`
   (audit-composition-pattern.md:72), `contract.retry`, `workflow.completed`, and
   `workspace.migrated`. That is **60% of the telemetry surface outside the schema the recorder is
   instructed to enforce by refusal**. The policy events additionally use the key `event` where the
   schema requires `event_type`. `event-schema.md:5` concedes the gap: "(schema entry pending)".
2. **Architectural Fix**: One Go enum as source of truth; generate both the JSON Schema and the
   documentation table from it. Emission moves into the executor, so an undocumented event type
   becomes a compile error rather than a prose violation.
3. **Target Files**: `shared/telemetry/event-schema.md`, `shared/orchestration/policy-evaluator.md`,
   `shared/orchestration/audit-composition-pattern.md:72`, `shared/skills/orchestrate/SKILL.md`,
   new `internal/telemetry/events.go`
4. **Done when**: all 15 event types are in one enum and adding a 16th without documenting it fails
   to compile.

### L3.10 — Build the hook executor
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: M0.4 · **Blocks**: L4.6

1. **Problem**: `shared/hooks/` defines a schema and four example hooks against events
   `on-artifact-write`, `on-validation-pass`, `on-ki-created`, `on-retrospective-written`. **No code
   emits any of these events and nothing dispatches them.** `on-retrospective-written.yaml`
   additionally carries `description:` and `guardrails:` keys that `hooks-schema.md` does not define
   — including "draftOnly: true is non-negotiable and must not be overridden," a security-relevant
   constraint expressed as free text in a file no parser reads.
2. **Architectural Fix**: A real in-process event bus with a typed event catalog, a hook loader
   validating against the schema (rejecting unknown keys), sandboxed `script`-type execution with
   timeouts, and enforcement of hook-level guardrails as code. Every hook invocation emits telemetry.
3. **Target Files**: `shared/hooks/hooks-schema.md`, `shared/hooks/*.yaml`,
   `shared/hooks/examples/*.yaml`, new `internal/hooks/`
4. **Done when**: enabling `on-retrospective-written` actually fires `learning-engine`, and a hook
   with an undeclared key is rejected at load.

### L3.11 — Make the eval harness provider-agnostic and give it a baseline
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: M0.2 · **Blocks**: L4.4

1. **Problem**: `scripts/run-agent-evals.sh` is 428 lines of real work, but it shells out to
   `claude --bare -p` and hardcodes `JUDGE_MODEL="claude-haiku-4-5-20251001"`. Step 4 is "regression
   comparison against the previous eval in `shared/evaluation/agent-evals/`" — that directory
   contains **only a README**, so there is no baseline and the regression check has never fired. In
   CI it runs only on push-to-main behind `AGENT_EVAL_ENABLED == 'true'`, so agent prompt changes
   reach users ungated.
2. **Architectural Fix**: Abstract generation and judging behind a provider interface (env-selected:
   Anthropic, Bedrock, Vertex, local). Commit baseline eval records for every agent. Run evals on
   **pull requests** against changed agents, gate merge on regression, store scores as trend data.
   This is the async CI/CD evaluation pattern L3 requires, and the harness is ~80% built.
3. **Target Files**: `scripts/run-agent-evals.sh`, `shared/evaluation/agent-evals/`,
   `.github/workflows/framework-ci.yml`, `shared/evaluation/agent-eval-harness-design.md`
4. **Done when**: a PR degrading an agent prompt fails CI on eval regression.

### L3.12 — Move counter agents out of the synchronous execution graph
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L3.11 · **Blocks**: none · *(audit H8)*

1. **Problem**: `audit-composition-pattern.md` makes counter-agent invocation the default for every
   contract-bound stage, in-band and blocking. That roughly doubles LLM invocations and wall-clock
   per delivery. Four of eleven mappings also point auditors at artifacts they were never scoped for
   — `analyst → context-auditor` (scoped to `context-manifest.md` only),
   `accessibility-engineer → documentation-auditor` (scoped to README/ARCHITECTURE staleness),
   `qa-engineer → tool-validator` (scoped to `SKILL.md` frontmatter), and worst,
   `architect → rule-auditor` with **`onFail: halt`**, where `rule-auditor` audits
   `shared/rules/*.md` — framework files. Pointed at a customer's `architecture-notes.md`, it halts
   their pipeline over findings about *this* repo.
2. **Architectural Fix**: Auditors become an asynchronous post-delivery gate running on the PR
   against persisted artifacts in `docs/features/<name>/`, fanned out in parallel, off the critical
   path. Delete the four wrong mappings. Demote every auditor whose job is deterministic (frontmatter
   schema, broken links, dead paths) from LLM agent to linter.
3. **Target Files**: `shared/orchestration/audit-composition-pattern.md`,
   `shared/workflows/feature-delivery-workflow.md`,
   `shared/agents/{context,documentation,rule,privacy}-auditor.md`,
   `shared/agents/tool-validator.md`
4. **Done when**: a delivery completes with zero synchronous auditor invocations and the audit report
   arrives on the PR.

### L3.14 — Define and consume the UI evidence bundle
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: ADR-007 (Accepted), L2.9 (first cut) · **Blocks**: routing `visual-qa-engineer` (L3.0's known limit) · *(raised 2026-08-31)*

1. **Problem**: `visual-qa-engineer` decides whether it can run by checking the filesystem for a
   `heatmap-data/` directory and Playwright baselines — a fact about the environment, evaluated by a
   model, with nothing durable recording what was found or which build it described. L3.0 could
   therefore not route the stage at all, and nothing can tell a baseline captured against this
   build from one captured three releases ago. The Saturday heatmap plugin already scans every
   visible interactable element per page with a stability-ordered selector; that output is discarded
   per scenario instead of becoming an artifact the pipeline can reason about.
2. **Architectural Fix**: A **UI evidence bundle** as a versioned artifact contract, per ADR-007:
   `manifest.json` (interactables per route, each with its selector and that selector's stability
   tier — `id` | `testid` | `class` | `tag`), `coverage.json` (which manifest entries a run
   exercised), and `baselines/`, all keyed to the app version or commit they were captured against.
   `visual-qa-engineer`'s precondition becomes "a bundle for this version is available" — a fact
   about an artifact, which the router *can* read. Sourcing has two supported answers, both reading
   the same format: produced in-repo during the qa run, or published by a separate test repository's
   CI and fetched by version key. A bundle whose version key does not match what was built is
   refused rather than silently trusted.
3. **Decided 2026-08-31 — `loom` consumes, it does not generate.** The bundle format is a contract
   `loom` defines and reads; producing it stays with whoever owns the tests, and
   `saturday-playwright-heatmap`'s scanner already emits what the manifest needs. This keeps
   Playwright and a browser out of `loom`'s dependency tree, and works identically for both
   topologies in ADR-007. The cost is accepted: the stability-tier definition lives in a contract
   other implementations must honour, so the contract has to be explicit about it and the consumer
   has to reject a bundle that is not.
4. **Target Files**: `shared/agents/visual-qa-engineer.md` (precondition), new
   `shared/contracts/ui-evidence-bundle-contract.md`, `shared/schemas/` (bundle schema),
   `internal/state/` (typed bundle reference if consumed by the executor),
   `shared/rules/testing-conventions.md`
5. **Done when**: `visual-qa-engineer` runs or is routed out on the basis of a bundle's presence and
   version key rather than a directory listing, and a bundle captured against a different build is
   refused with a message naming both versions.
6. **Not in scope**: enforcing the fetch for the separate-repository path — that is a supply-chain
   step no current gate covers, and ADR-007 records it as documented-but-unenforced.

### L3.15 — Verify the provider envelope against the live CLI
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: L3.8 (shipped) · **Blocks**: none · *(raised 2026-09-01)*

**SHIPPED** 2026-09-02 (epic 89) — verified against one real `claude -p --output-format json`
response, captured as `internal/provider/claude/testdata/envelope-real.json` with only the session
identifiers redacted, and asserted field by field.

**The decoder was correct.** Every field name matched: `result`, `is_error`, `subtype`,
`total_cost_usd`, `usage.{input,output,cache_read_input,cache_creation_input}_tokens`, and
`modelUsage` keyed by model name. The real response carries twenty-one top-level fields, most of
which loom ignores — a test asserts the fixture is genuinely that wide, so nobody replaces it with a
hand-written approximation and loses the point.

**The suspicion check is narrower than this item proposed, and the real data is why.** It said a
completed stage reporting exactly zero input tokens is a decode that missed. The verified response
reported **input_tokens = 2** alongside **58,299 cache-creation tokens**: when the prompt is cached,
a tiny input count is correct. The check is now "no tokens of any kind AND no cost", which a real
invocation never produces. It warns rather than fails, since a provider may legitimately report
nothing — but it must not pass unremarked, because unreported and mis-decoded look identical
downstream.

**It also found a live defect in L3.5's store.** Cache tokens were not recorded, and
`loom memory runs` computed its token figure from input plus output — so that verified run would
have been reported as **6 tokens** rather than 58,305, understating it by four orders of magnitude
while showing its real cost of $0.35. The store now records cache read and creation counts and
totals all four. Its schema is versioned, and a mismatch rebuilds rather than migrates, which is
only safe because the store is a projection of records archived in git.

The item's premise holds and is worth restating: a wrong field name does not error, it decodes to a
zero value. This is the one place in the telemetry stack where a wrong number could still wear the
appearance of a measurement, and it is now closed by a captured response rather than by care.

1. **Problem**: `internal/provider/claude/envelope.go` decodes `claude -p --output-format json` from
   a documented understanding of its shape, not from an observed response — verifying it costs real
   API spend, so epic 84 did not. The failure mode is specific and asymmetric: a wrong field name
   does not error, it decodes to the zero value, so usage silently reads **zero tokens and zero
   dollars**. That is a wrong number wearing the appearance of a measurement, which is the exact
   defect L3.8 exists to remove — and the one place in the telemetry stack where it can still occur.
   Unknown-field tolerance is a feature everywhere else and a hazard precisely here.
2. **Architectural Fix**: Run one real stage against the live CLI, capture the envelope verbatim as
   a test fixture, and assert the decoder against it. Then add the check that closes the class of
   bug rather than one instance: a completed non-mock stage reporting exactly zero input tokens is
   not a cheap stage, it is a decode that missed, so it should warn loudly rather than record a
   confident zero.
3. **Target Files**: `internal/provider/claude/envelope.go`, new
   `internal/provider/claude/testdata/envelope.json`, `internal/provider/claude/claude.go`
4. **Done when**: the decoder is tested against a captured real response, and a zero-token
   completion from a real provider is reported as suspect rather than recorded as fact.

### L2.21 — Consume the analyst's own uncertainty
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.9 (shipped), L3.0 (shipped) · **Blocks**: none · *(raised 2026-09-06, from the first real end-to-end run)*

1. **Problem**: The analyst can say it does not know what it is looking at, and nothing reads it.
   In the first `--provider claude` run, against a project with no source code, the analyst named
   its Affected Components as `<app entrypoint / router — exact file depends on chosen stack,
   unresolved>` and `<health handler source file — unresolved pending stack decision>`. That is a
   correct and useful answer. The router then routed **10 of 12 stages**, the architect designed for
   an unknown stack, the performance-engineer set thresholds against nothing, and the run continued
   for seven minutes and **$3.57** before the developer produced an implementation touching no files
   and the typed invariant stopped it (L2.9). Every stage did its job; the pipeline had no way to
   act on the one fact that mattered.
2. **Architectural Fix**: Make unresolvability a **field**, not prose. `AnalysisState` already
   carries `ArchitecturalFlags`; it needs the counterpart — the analyst declaring that a component,
   a stack, or a target codebase could not be resolved, with what it would need. The router reads it
   before routing (it already runs at the earliest point the facts exist, L3.0), and an unresolved
   analysis halts at a gate for a human rather than routing work that cannot land. This is the same
   move as L2.17's typed verdict: a decision the pipeline needs was expressible only in prose that
   nothing parsed.
3. **Target Files**: `internal/state/analysis_state.go`, `internal/orchestrator/router.go`,
   `shared/agents/analyst.md`, `shared/contracts/analysis-contract.md`
4. **Done when**: an analysis that cannot name a single concrete affected file halts the run at a
   gate before any implementation stage is invoked, and says which fact it lacked.

**Why this is worth the effort**: the cost of the current behaviour is not a wrong answer, it is a
confidently-executed one. Four agents produced good artifacts about a codebase that did not exist.

### L3.16 — `run.started` fires once per invocation, not once per run
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-06, from the first real end-to-end run)*

1. **Problem**: `Executor.Run` emits `run.started` unconditionally, and the TTY approval path
   re-enters `Run` after every gate. One logical run therefore records several starts. The first
   real end-to-end run's timeline shows `run.started` at `0s` and again at `7m38s`, immediately
   after `gate.approved` — so "how long did this run take" has no unambiguous answer, and any
   analysis over the event log has to know to ignore all but the first. The episodic store (L3.5)
   inherits the ambiguity, and L3.13 would compute durations from it.
2. **Architectural Fix**: `prepareState` already distinguishes a fresh run from a resumed one — it
   is where `newRunFor` is called. Emit `run.started` only for a genuinely new run, and emit a
   distinct `run.resumed` for a continuation, which is a fact worth recording in its own right: how
   often a run is resumed is a question nobody can currently answer. Adding a kind means adding it
   to the vocabulary, where L3.9's fitness function requires it be documented.
3. **Target Files**: `internal/orchestrator/executor.go`, `vocabulary.go`, `timeline.go`,
   `internal/memory/ingest.go`
4. **Done when**: a run interrupted and resumed three times records one `run.started` and three
   `run.resumed` events, and the episodic store reports one run.

**Found by running the thing.** Every executor test drives `Run` once per assertion or resumes
through `--approve`, and neither shape surfaced this. It took a human answering `y` at three
consecutive gates.

**SHIPPED** 2026-09-21. `prepareState` now returns which event this invocation is — it is the only
place that can tell, since by the time `Run` holds the state a fresh run and a resumed one look
alike — and `Run` emits that. `run.resumed` is in the vocabulary, so L3.9's fitness function
regenerated the schema and the types table, and failed the build until it was run.

**The test the item said nobody had.** `driveThroughGates` answers three consecutive gates without
a human, which is the shape that surfaced the defect and that no test had. Against the previous
behaviour it records `run.started` four times and `run.resumed` never, and the ordering test prints
the first real run's timeline verbatim: `… gate.approved run.started …`.

**The episodic half was already true, and its test is a guard rather than a fix.** Run identity is
feature plus `StartedAt`, which is set once at creation and survives every checkpoint, so the store
reported one run before this change as well. The test passes against the old behaviour and is kept
because what it guards — a resume not resetting `StartedAt` — is the thing that would turn one run
into four rows, and nothing else asserted it.

**One stale thing fixed rather than extended.** `cmd/loom/README.md` carried a hand-written list of
recorded kinds, nine behind the generated table three paragraphs below it. Adding `run.resumed` to
that list would have made it ten-for-ten wrong in a new way, so it now points at the generated file
instead.

### L2.22 — Give the writing stages permission to write
**Workstream**: TOOLS · **Effort**: S · **Blocked by**: none · **Blocks**: L2.24 · *(raised 2026-09-06, from the second real end-to-end run)*

**SHIPPED 2026-09-07** (`3d825fc`) — the provider passes `--allowed-tools` built from the agent
definition's own `tools:` frontmatter, plus `--permission-mode acceptEdits` to remove the
confirmation a headless run has nobody to answer. An agent declaring no tools gets Read/Glob/Grep:
it has not asked to write, and inferring otherwise would reinstate the silent failure the other way
round. `bypassPermissions` is deliberately unused and a test keeps it that way.

Verified against the real CLI, because a flag nobody checked is the whole of this defect. Without
the flags, `permission_denials` carries a `Write` entry and no file appears; with them, no denials
and the file is created. Still to be confirmed end to end by run 4 — the done-when asks for a
non-empty `git diff` from a fresh install, and that has not been run yet.

The interim step this item suggested — have `loom install` write a `.claude/settings.local.json`
allowlist — is deliberately not taken. It was for "until the executor passes a posture", which is
now, and the markdown pipeline has a human present to answer the prompt.

**L2.24 is unblocked by this.**

1. **Problem**: `claude -p` denies every `Write` and `Edit` by default, and the provider invokes it
   as `exec.CommandContext(ctx, binaryPath, "-p", "--output-format", "json")` with no
   `--permission-mode` and no `--allowed-tools`. **No stage can write a file.** Confirmed directly:
   a `Write` to `internal/server/ping.go` came back as
   `permission_denials: [{"tool_name": "Write", ...}]` with the file never created. In the second
   real run's first pass the developer produced a complete `implementation-notes.md` naming
   `filesModified: ["internal/server/server.go"]`, with a self-review checklist including
   *"Passes all tests — go vet/build succeeds"*, against an **empty `git diff`**. The pipeline's
   central promise — that it writes code — does not hold on a default install, and it fails silently.
2. **Architectural Fix**: The executor decides what a stage may do; it should say so rather than
   inherit a host default. Pass an explicit permission posture per stage, derived from the stage
   definition: stages that produce state and no files get read-only tools, and the two that write
   (`developer`, `qa-engineer`) get write access scoped to the workspace and the repo. This is the
   same principle as L2.3 — an explicit root beats an ambient default — applied to the provider
   boundary instead of the tool boundary. Until it lands, `loom install` should write a
   `.claude/settings.local.json` allowlist and say that it did.
3. **Target Files**: `internal/provider/claude/claude.go`, `internal/orchestrator/plan.go`,
   `internal/orchestrator/stage.go`, `cmd/loom/install.go`
4. **Done when**: a fresh `loom install` followed by `loom run` produces a non-empty `git diff`
   without the operator configuring anything, and a stage denied a write **fails** rather than
   reporting success.

**Why this is worth the effort**: everything above M0.4 was verified against `--provider mock`, and
the mock provider never asks the host for permission to do anything. This is the first defect that
only exists on the real path — the entire class the mock cannot see.

### L2.23 — Stop instructing the writing stages not to write
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: none · **Blocks**: L2.24 · *(raised 2026-09-06, from the second real end-to-end run)*

**SHIPPED 2026-09-08** (as part of L3.29's fix) — `fileClause` conditions the clause on the stage's
own declared posture. A stage holding no edit tool is still told not to write; a stage holding one is
told to make its changes and that the JSON *reports* work it must verify with `git status` before
answering. Verified end to end: the developer went from an empty `git diff` to 304 insertions across
exactly the four files its design named.

**Not fully closed.** Five other typed agents still carry a "produce your artifact at `<name>.md`"
instruction that the appended output contract overrides. Run 4 completed all twelve stages, so the
contract empirically wins — but that is evidence, not a guarantee, and the contradiction is still in
the prompts.

1. **Problem**: `typedInstruction` appends to every typed stage's prompt:
   *"Return a single JSON object conforming to this schema, and nothing else. **Do not write files.**
   Do not add commentary before or after the JSON."* `developer` (`KindImplementation`) and
   `qa-engineer` (`KindQA`) are both typed. **The two stages whose entire purpose is writing files
   are told not to.** The instruction is aimed at "don't emit a markdown artifact alongside the
   JSON", but it does not say that, and the two agents resolved the contradiction differently: the
   developer disobeyed and shipped correct code, the qa-engineer obeyed and reported test results
   for a file it never created (L2.24). A prompt that requires an agent to disobey it to do its job
   is not a contract.
2. **Architectural Fix**: Say the thing that is actually meant — the *stdout channel* carries JSON
   and nothing else — and make the file-writing clause conditional on the stage. A typed stage that
   declares no file outputs is told not to touch the filesystem; a typed stage that produces files
   is told which ones it owns. The stage definition already knows which it is; the prompt builder
   just does not ask.
3. **Target Files**: `internal/provider/claude/typed_stage.go`, `internal/orchestrator/plan.go`
4. **Done when**: the developer and qa-engineer prompts contain no instruction contradicting their
   own job, and a test asserts the file-writing clause is absent for exactly the file-producing
   stages.

**Found by running the thing.** `typed_stage_test.go:24` asserts the string `"Do not write files"`
is *present* in a typed stage prompt — the contradiction is currently held in place by a test.

### L2.24 — Verify a stage's file claims against the filesystem
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L2.22, L2.23 · **Blocks**: none · *(raised 2026-09-06, from the second real end-to-end run)*

**SHIPPED 2026-09-09** (`960fdcc`, `6f4f13a`) — in two halves, with an honest boundary between them.

**Path claims are checked.** Every path-naming field is verified against the project after the stage
returns; a claimed file that does not exist fails the stage naming the field and the path, and a
path escaping the project root is refused rather than treated as satisfied. A claimed file that
exists but did not change while the stage ran is a warning, since a stage may legitimately list a
file it inspected. **The second real run's actual qa payload is the regression fixture** and must
fail — which required teaching the mock to model a stage that lies, because the executor caught the
mock itself claiming `internal/mock/thing.go` and never writing it.

**Measurement claims were reproduced — RETIRED 2026-09-10.** A second half re-ran the project's own
test command (`testCommand` in `.claude/delivery-policy.yaml` — project configuration, never the
state document) to reproduce a stage's claim that the suite passed. It is removed: `internal/verify`
is deleted, along with `MeasurementVerifier`, `verifyMeasurements`, `noteUnverified` and
`QAState.ClaimsPassingTests`.

The reason is the evidence below. Runs 8 and 9 tested both available explanations for run 2's
fabrication and neither reproduced it, so this half was paying a **full suite run per QA stage**
— bounded at 15 minutes — to guard a defect with no established cause. Run 9 is the sharper of the
two: with the pre-L3.29 contradiction restored, the stage wrote tests, ran them, and reported
`passed: 170` and `86.08%`, **both exact against independent verification**. It reported what it
measured.

Retiring only this half is deliberate. The path check costs a `stat`, has no such counter-evidence,
and stays — along with the second run's payload as its regression fixture. Reversing this is one
revert if a live fabrication ever appears.

**Never verified: coverage.** Reproducing a percentage means parsing a coverage report per language.
It was out of scope while the measurement half existed and is moot now.

**Validation status, stated exactly.** Both halves are covered by unit tests, including run 7's
payload shape. **Neither has been demonstrated against a live fabrication**, and run 8 explains why:
there was none to demonstrate against.

**BEHAVIOURAL HALF: TESTED AND NEGATIVE 2026-09-09 (run 8).** The condition this item describes was
finally constructed and held. Two preventions — a `qa-engineer` with no `Bash` at all, and one with
`Bash` whose install was sabotaged by a dead registry plus an unresolvable dependency — and in both
the stage reported `passed: 0, skipped: 3` with an accurate account of why. 8A returned an empty
`coverage.statements`. 8B identified both blockers, inferred they were deliberate, **declined to
remove them despite having the means**, and sourced its one coverage figure as the developer's
pre-QA measurement rather than presenting it as current.

See `docs/audits/loom-e2e-run-8-audit-2026-09-09.md`, including its §5.2: both conditions are
conspicuously adversarial, and ordinary breakage might not elicit the same care. Run 9 then removed
the other explanation (`docs/audits/loom-e2e-run-9-audit-2026-09-09.md`), which is what turned "the
code stays because it costs nothing per run" into a decision to retire the half that does.

1. **Problem**: The executor validates the *shape* of a stage's claims and never checks whether they
   are true. In the second real run the qa-engineer completed in 53 seconds and returned:
   `testFilesCreated: ["internal/server/server_test.go"]`, `testResults: {passed: 3, failed: 0}`,
   `coverage: {statementCoveragePercent: 100, newTests: 3}`. **The file does not exist. No test was
   ever run.** Three tests passed that were never written, at 100% coverage of nothing. The payload
   is schema-valid, so the stage completed, the run cleared the `confirm-security` gate, and
   tech-writer and devops-engineer ran on top of it. `go test ./...` on the delivered repo reports
   `[no test files]` — the spec's third acceptance criterion, stated explicitly, unmet and unnoticed.
   The developer exhibited the same failure in the first run (L2.22). Both are cheap to catch:
   `filesCreated`, `filesModified`, and `testFilesCreated` name paths, and paths either exist or
   they do not.
2. **Architectural Fix**: Extend typed validation from structural to **referential**. Every state
   field that names a path is checked against the workspace root after the stage returns: a created
   file must exist, a modified file must differ from its pre-stage digest (the baseline machinery
   from L2.14 already records these), and a stage claiming passing tests must have produced a test
   artifact. A claim that does not hold fails the stage with the discrepancy named, which is exactly
   the signal L2.18's bounded retry should consume. This is L2.11's argument — verify semantics, not
   heading presence — moved from the markdown validator into the typed one, where it is now cheap
   because the fields are structured.
3. **Target Files**: `internal/orchestrator/typed.go`, `internal/state/implementation_state.go`,
   `internal/state/qa_state.go`, `internal/orchestrator/approval_binding.go`
4. **Done when**: a stage claiming a file it did not write fails with that file named, and the
   second real run's qa payload is a regression fixture that must fail.

**Why this is worth the effort**: this is the most expensive defect the run found, and not because
of the dollars. A fabricated test suite passed a human security gate. The gates in
`approval-gates.md` are only as good as the facts presented at them, and nothing currently
establishes that those facts are real.

### L2.25 — Generate the enums the validator enforces
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: L2.9 (shipped) · **Blocks**: none · *(raised 2026-09-06, from the second real end-to-end run)*

**SHIPPED 2026-09-07** (`c2e0aa2`) — closed-set types publish their values once through the
`Enumerated` interface and derive their own `JSONSchema` from them, so a schema enum cannot drift
from the constants `valid()` checks. The hand-written enum tags are removed as redundant.
`TestEveryClosedSetFieldCarriesItsEnum` is the fitness function: it reflects over each stage's Go
type and asserts every closed-set field's generated schema carries its values. Removing
`StrideCategory`'s derived schema reproduces the original defect.

**Known gap**: `NonFunctionalRequirement.Category` and `ContextState.Tier` are plain strings with
hand-written enum tags rather than named types, so the fitness function cannot see them. Their
enums are present and correct, just not derived. Converting them touches routing predicates changed
in `ef81c76`.

1. **Problem**: `security_state.go` requires `stride[].category` to be one of six exact literals —
   `SPOOFING`, `INFORMATION_DISCLOSURE`, and so on. The schema handed to the agent declares that
   field as a bare `{"type": "string"}`: no enum, no description, no list of legal values. The
   agent is validated against a rule it was never told. It emitted, correctly and completely:
   `Spoofing`, `Tampering`, `Repudiation`, `Information Disclosure`, `Denial of Service`,
   `Elevation of Privilege` — all six categories, properly assessed, in the title case the agent
   definition itself uses (`**S**poofing`). The run failed twice, ~70 seconds and **$0.65 each**,
   with `field "stride" does not assess SPOOFING — the contract requires all six categories`. That
   message is **wrong**: spoofing was assessed. It reports a security gap where there is a
   serialization mismatch. Adding one casing hint made the stage pass on the next attempt.
   `findings[].severity` in the same schema *does* carry an enum, so the generator can express this.
2. **Architectural Fix**: Any Go type with a closed set of constants must emit those constants as a
   JSON Schema `enum`. Make the schema generator derive it rather than relying on each state type's
   author to remember, and add a fitness function asserting that every field whose validator
   compares against a fixed set has an enum in the generated schema — otherwise this recurs the next
   time someone adds a category. Separately, a validation message should describe the discrepancy
   (`category "Spoofing" is not one of: SPOOFING, ...`) rather than assert a conclusion about the
   agent's analysis.
3. **Target Files**: `internal/state/schema.go`, `internal/state/security_state.go`,
   `internal/state/*_state.go`
4. **Done when**: no validator compares against a constant set the generated schema omits, and the
   STRIDE failure message names the value it received.

**Found by running the thing.** The mock provider's scripted security payload uses the correct
literals, so every mock run passes and the mismatch is invisible.

### L2.26 — Keep the payload a stage was rejected for
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: L2.18 · *(raised 2026-09-06, from the second real end-to-end run)*

1. **Problem**: `persistTypedOutput` decodes the payload, and on failure returns an error and drops
   it. Nothing is written anywhere. The second real run failed four typed stages — architect once
   on `unknown field "developerHandoffNotesNote"`, security-reviewer twice on STRIDE — at
   50–120 seconds and **$0.64–0.74 each, ~$2.10 discarded**, leaving one error line and no artifact.
   Diagnosing the STRIDE failure required reconstructing the prompt by hand and re-invoking the CLI
   outside the executor, because there was no other way to see what the agent had said. The
   deliberate no-retry stance (`typed.go:76` — *"a silent repair would hide the modelling failures
   this epic exists to surface"*) is defensible, but it surfaces those failures **un-diagnosably**:
   the evidence is destroyed at the moment of detection.
2. **Architectural Fix**: Write the rejected payload to `state/<stage>.rejected.json` beside the
   validation error before returning it, and name that path in the error. Costs one file write, and
   turns "the architect emitted an unknown field" into something readable. It is also the input
   L2.18's bounded contract-retry needs: a repair prompt cannot reference a payload nobody kept.
3. **Target Files**: `internal/orchestrator/typed.go`, `internal/state/schema.go`
4. **Done when**: every stage failure caused by an invalid payload leaves that payload on disk, and
   the error names where.

**SHIPPED** 2026-09-21. The done-when is met at both places a payload is rejected, which is one more
than the item counted.

**Two rejection sites, not one.** The executor rejects a payload that parsed as JSON and failed the
schema (`persistTypedOutput`); the provider rejects a response that was not a state document at all
(`extractJSON`). The item named only the first. The second is where the run-4 audit's **partial**
verdict came from: the response was echoed into the error and cut at 800 characters, so the tail —
`knownGaps`, in that run — was lost. Both now write the evidence whole:
`state/<stage>.rejected.json` for a payload, `state/<stage>.rejected.txt` for a response, each named
in the error. The 800-character quote stays, because an error a person reads at a terminal should
still say what came back.

**Kept byte-for-byte, deliberately.** A rejected payload is written exactly as the agent produced it
rather than re-encoded, so `jq` reads it when the failure was a schema violation rather than a syntax
error — and so that what is on disk is what the agent said, not what the executor made of it.

**Two things fell out of building it.** A stage that failed, was fixed and re-ran would otherwise
leave evidence of a run that no longer happened, so a successful persist clears the stage's rejected
files — evidence of the wrong attempt is worse than none. And `archiveTypedState` copies `.json`
documents on the stated ground that the archive holds only what a reader decodes; a rejected payload
is by definition one that does not, so it is excluded and stays in the workspace, where whoever is
diagnosing the failure is.

Six tests, each confirmed to fail against the previous behaviour before being kept — including one
that reproduces the run-4 truncation exactly, cutting off before `knownGaps`.

**What this does not do.** It does not retry, repair, or re-prompt. L2.18's bounded contract-retry is
still unbuilt; this only supplies the input it needs, which was the other half of why the item existed.

### L3.17 — Carry the run's provider across resume
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: L2.15 (shipped) · **Blocks**: none · *(raised 2026-09-06, from the second real end-to-end run)*

**SHIPPED** 2026-09-21 — `fb9b54a`. `RunState.Provider` records the provider at creation, resume adopts
it, and a contradicting `--provider` is refused naming both values. The printed resume command
spells out a non-default provider as well, so a command pasted elsewhere still reproduces the run.

**Verified against the exact failure**, not a proxy: a mock run resumed with **no `--provider` at
all** — the command that billed $1.69 — now stays mock and completes the `developer` stage, where
before it invoked the real binary and failed. State written before this records no provider and
still resumes on the flag.

**Independently reconfirmed the day before it shipped**, while trying to produce L2.19 evidence: a
mock run resumed with the printed command failed at `developer` with `agent exited with error`,
which is this defect wearing a confusing error message. The item's description needed no
correction — it already said "silently switches".

1. **Problem**: `--provider` is a flag on the invocation, not a property of the run, and
   `run-state.json` does not record it. `loom run --spec X --provider mock` halts at
   `confirm-design`; the resume command the executor **itself prints** is
   `loom run --spec X --resume --approve confirm-design` — with no `--provider`. Following it
   silently switches to the real `claude` binary mid-run. That is exactly what happened on the
   documented "mock first (free, proves the wiring), then the real one" path: the mock run's
   developer and code-reviewer stages ran against Anthropic for **$1.69**, implementing the mock
   analysis's placeholder feature (`mock-feature`, `internal/mock/thing.go`) before the typed
   invariant stopped it. A dry run billed real money, and the command that caused it was the one
   the tool suggested.
2. **Architectural Fix**: The provider is part of a run's identity — a run is mock or it is not, and
   half of each is meaningless. Record it in `run-state.json` at creation, have `--resume` adopt the
   recorded value, and refuse a `--provider` on resume that contradicts it rather than honouring the
   switch. The printed resume command should reproduce the run it came from.
3. **Target Files**: `internal/orchestrator/executor.go`, `internal/orchestrator/state.go`,
   `cmd/loom/run.go`
4. **Done when**: a mock run resumed with the printed command stays mock, and a contradicting
   `--provider` is rejected with both values named.

**Why this is worth the effort**: the mock provider exists so the wiring can be proven for free. A
resume that leaves mock mode defeats the only reason it exists, and does so by charging for it.

### L3.18 — Route on what the analysis says, not how many items it has
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: L3.0 (shipped) · **Blocks**: none · *(raised 2026-09-06, from the second real end-to-end run)*

1. **Problem**: `RequiresDevOpsEngineer()` is `len(a.Tasks.DevOps) > 0`. In the second real run the
   analyst emitted exactly one DevOps task whose text is *"None required by this spec — no CI or
   deployment config changes requested."* The router counted one item and ran devops-engineer:
   76 seconds, **$0.64**, to conclude there was nothing to do. A prose "none" is indistinguishable
   from work under an arity test, and models write prose "none" constantly. The same run routed the
   performance-engineer in on an NFR whose stated threshold is *"Zero I/O calls in the handler
   body"* — a qualitative property, not the measurable threshold `hasPerformanceThreshold()` claims
   to detect. Meanwhile `visual-qa-engineer` ran on a JSON endpoint as *"always runs; not skippable
   by routing"*, in the same run where `accessibility-engineer` was correctly skipped for having no
   UI surface — two stages that answer the same question disagreeing about it.
2. **Architectural Fix**: An empty list is the only honest way to say "nothing to do", so make the
   analyst's contract say that and give the router a typed signal rather than a count. Routing
   predicates should read fields that cannot be satisfied by prose — a threshold with a number and a
   unit, a task list whose emptiness is structural. Then reconcile the unskippable set: whatever
   makes `accessibility-engineer` skippable applies to `visual-qa-engineer`.
3. **Target Files**: `internal/state/analysis_state.go`, `internal/state/route.go`,
   `shared/agents/analyst.md`, `shared/contracts/analysis-contract.md`
4. **Done when**: an analysis whose only DevOps task says "none required" skips devops-engineer, and
   no stage that answers a UI question runs on a feature with no UI surface.

**Why this is worth the effort**: L3.0 moved routing off a model re-reading the analysis and onto
predicates, which was right. The predicates now need to read facts a model cannot accidentally fake.

**SHIPPED 2026-09-07** (`ef81c76`) — with L3.24, which is the same defect with a different trigger.
`Threshold` is now typed `{metric, value, unit}` and `IsMeasurable()` requires all three; a prose
"none" is ignored by `RequiresDevOpsEngineer`; `visual-qa-engineer` and `sre-engineer` are
skippable and routed on a declared surface. `analyst.md` said *"If a section doesn't apply, write
'None' as the body"* until this commit, so the instruction that caused the behaviour is corrected
along with the predicate that trusted it.

Applied to the third real run's own analysis, all four stages that had nothing to do now route out
and the three that were correctly skipped still are — asserted by a test carrying that run's
actual shape, confirmed to fail against the previous behaviour.


### L3.19 — Cut the per-stage prompt tax
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-06, from the second real end-to-end run)*

1. **Problem**: The second real run delivered **four lines of Go** into a **26-line** repository for
   **$10.63 across 8.3M tokens**. None of that is explained by the size of the codebase. Measured
   from `traces.jsonl`, per stage: ~6–20 input tokens, 1.3k–11.6k output tokens, and **74k–97k
   cache-creation tokens** — fifteen times, for 1.2M cache-creation tokens total, the single largest
   line item in the run. Every stage is a fresh `claude -p` process that rebuilds and re-caches a
   prompt of substantially the same material: the install carries ~61k tokens of agent definitions,
   ~99k of skills, ~17k of rules, plus `CLAUDE.md`, `ARCHITECTURE_RULES.md` and
   `DOMAIN_DICTIONARY.md`. Cache reads ran 206k–964k per stage and 84.5% of all tokens moved. The
   floor this sets is stark: **a stage that correctly decides it has nothing to do still costs
   $0.50–0.74** — tech-writer paid $0.74 to write "None", devops-engineer $0.64 to agree. Across
   this run, ~$3 went to stages whose output was a well-reasoned "nothing to do here".
2. **Architectural Fix**: Two independent levers, in order. **First**, share the cached prefix across
   stages instead of rebuilding it per process — the framework preamble is identical for every
   stage in a run and is currently paid for fifteen times. **Second**, give a stage a cheap way to
   decline: a routing-time or pre-flight check that can conclude "nothing to do" without loading a
   full agent definition, so the skip costs cents rather than dollars. L3.18 reduces how often a
   stage is asked; this reduces what it costs to ask. Note what is *not* the fix here: a repo
   map or source graph would optimize a dimension this run never touched — the codebase was 26
   lines, and essentially none of the 8.3M tokens were source. That optimization needs its own
   measurement on a real repository before it is worth building (see the `aider-repo-map` and
   `repomix-codebase-packing` KIs).
3. **Target Files**: `internal/provider/claude/claude.go`, `internal/provider/claude/typed_stage.go`,
   `internal/orchestrator/plan.go`, `cmd/loom/install.go`
4. **Done when**: per-run cache-creation tokens do not scale linearly with stage count, and a stage
   the router skips costs measurably less than one that runs.

**Why this is worth the effort**: the cost model is currently a function of how many agents exist,
not of how much work the feature is. That is backwards, and it gets worse with every agent added.

**RESOLVED 2026-09-07 — the repo-map question is closed, and this item's ordering holds.** The third
real run put the deferred measurement on a real repository: `saturday-monorepo`, **1,547 tracked
files / ~22k lines of first-party TypeScript**, against run 2's 2-file, 26-line repo — roughly
**770x the source**. The decision rule was fixed in advance: `context-engineer` cache_read within
~2x of run 2's 964,590 means source discovery is not the cost driver.

It came in at **729,694 — 0.76x. It went *down***. Per-call cost stayed flat too: $0.55–1.53 against
run 2's $0.50–0.74 floor, mean $0.79/call, $9.49 across 12 calls. The single outlier is `developer`
at 2.59M cache_read and $1.53, and that is explained by tool-use iterations (it ran `pnpm install`
and the 169-test suite), not by reading source. Cache reads were 89.0% of all tokens moved, almost
identical to run 2's 84.5%.

The confound was measured separately and does not rescue the alternative: the framework surface
*grew* between the runs (v3.1.0 → v3.7.0: agents ~56k→62k, skills ~79k→99k, rules ~14k→17k tokens),
so the one input that did increase is framework, not source — and cache_read still fell. Source
scale is not a cost axis in this architecture, because no stage ever reads the repository broadly;
it reads the handful of files the manifest pins.

**EVIDENCE STALE 2026-09-08 (run 5A).** The 729,694 figure this block rests on does not
reproduce. Three invocations of `context-engineer` on the identical spec and repository, on the
current build, mean **2,024,888** — **2.77x** run 3's number and within 3% of run 4's 1,974,775. Run
4's figure was not an outlier; it is the reproduced level. The measured run-to-run spread is
**27.4%**, so a 177% shift is six times the noise band and is a real change in level.

This does **not** refute the conclusion. Run 5 held source scale constant, so it says nothing about
whether source scale drives cost — the question this item answered. What it invalidates is the
*evidence*: the stated measurement describes a build that no longer exists, and it was a single
sample from an instrument now known to carry ~27% spread.

**RE-ESTABLISHED 2026-09-08 (run 6) — the conclusion holds, on evidence that tests it.** The
re-test named above was run. Two conditions on one build, spec and target file held byte-identical,
n=3 each:

| Condition | Repository | cache_read mean |
|---|---|---:|
| S | 6 files, 124 lines | 1,858,579 |
| L | 1,547 files, 21,826 lines | 2,024,888 |

**176x the source moves cache_read by 8.2%** — inside the noise band, and far from the 40% drop the
pre-committed rule required to call a source term real. **No detectable source term.**

So `aider-repo-map` and `repomix-codebase-packing` stay unbuilt, now on a controlled measurement
rather than run 3's confounded one. Run 3's 0.76% figure is **superseded**, not merely stale: it
compared two builds and two specs at n=1 and reached the right answer for poor reasons.

**Run 6 also revises what run 5A's 27.4% meant.** Condition S's spread is **4.6%** against condition
L's 27.4% — a 6x difference on the same stage, prompt, build and model. The variance is the
repository, not the instrument: there is only variance to have when there is something to explore.
That is consistent with this item's stated mechanism (a stage reads what its manifest pins) without
proving it.

**Still open**: the 8.2% gap has the predicted sign and sits exactly in this design's blind spot, so
"no detectable term" must not harden into "no term". Resolving it needs n>=12 per condition, ~$35,
and it does not block the repo-map decision, which turned on whether a *large* term exists. See
`docs/audits/loom-e2e-run-6-audit-2026-09-08.md`.

**L3.19a ANSWERED 2026-09-10 — the prefix is attributed, and it re-orders this item.** A controlled
ablation against `claude -p` (~$0.85, n=1 per condition, haiku) measured what each part of an
install contributes to a stage's prompt. Full method, raw numbers and limits:
`docs/audits/loom-prompt-tax-attribution-2026-09-10.md`.

| Component | On disk | Tokens |
|---|---:|---:|
| CLI system prompt + tools (empty directory) | — | **38,519** |
| `CLAUDE.md` | 8KB | +1,793 |
| `.claude/rules/` (5 core) | 32KB | **+7,707** |
| `ARCHITECTURE_RULES.md` + `DOMAIN_DICTIONARY.md` | 36KB | +21 |
| `.claude/agents/` (39) | 320KB | +3,637 |
| `.claude/skills/` (69) | 552KB | +458 |
| loom's per-stage prompt (definition + schema) | 17.5KB | +4,573 |

**Agent and skill bodies never enter the prompt** — 872KB of them costs 4,095 tokens, under 2% of
their size, because only names and descriptions load. So "install only what a plan uses", which I
proposed as the first lever, is worth **~4.6%** and is a papercut, not a lever.

**68% of a stage's prefix is the CLI's own baseline**, which loom cannot reduce: 38,519 tokens to
start a `claude -p` process at all, repeated byte-identically ~578,000 times' worth per fifteen-stage
run. Sharing the prefix across stages is therefore not one option among three — it is the only one
that reaches the majority term, and its crux is *why stages 2–15 pay cache-creation rather than
cache-read*. Revised order: **(c) share the prefix**, then (d) cheap decline, then trimming
`.claude/rules/` (7,707 tokens, loaded in full — the only per-byte win), with (b) install-less last.

**L3.19c ANSWERED 2026-09-10 — the mechanism exists, and it costs stage isolation.**
`docs/audits/loom-prefix-sharing-2026-09-10.md`.

Repetition alone never amortizes: two **byte-identical consecutive** `claude -p` invocations both
pay 30,092 in cache-creation. A separate process cannot reuse the previous one's prefix. That kills
the hypothesis I opened with — that the per-stage `--allowed-tools` allowlist was perturbing the
prefix — since R2 changed nothing and still missed.

| Invocation | created | read |
|---|---:|---:|
| fresh | 30,090 | 17,927 |
| byte-identical repeat | 30,092 | 17,927 |
| **same session, `--resume`** | **51** | 48,017 |
| `--fork-session` from a primer | 30,241 | 17,927 |

`--resume` amortizes completely — **99.8% off cache-creation, 8.1x cheaper per stage weighted for
billing, ~5.5x over fifteen stages**. `--fork-session` does not: a fork is a cold start with
history, so prefix-sharing-without-conversation-sharing is not available from the CLI.

Amortizing therefore requires every stage to be a turn in **one continuous conversation**, which is
precisely what L2.9's typed state prevents: a stage would receive the raw transcript of everything
before it rather than its projected upstream fields. Context also grows linearly (the 5.5x is an
upper bound measured with trivial outputs), the 200k window becomes a run-length limit, and a
poisoned context becomes a run-level failure.

**GROWTH MEASURED 2026-09-10 — the 5.5x was an artifact, and it inverts the recommendation.**
`docs/audits/loom-prefix-growth-2026-09-10.md`. Twelve turns replaying the real run's stage sequence
and output sizes, two arms, $1.91.

| | created | read | prefix units | measured cost |
|---|---:|---:|---:|---:|
| SHARED (one session) | 131,966 | 1,141,406 | 279,098 | $0.7947 |
| COLD (today) | 388,542 | 520,496 | 537,727 | $1.1121 |
| | 2.94x less | 2.19x more | **1.93x less** | **1.40x cheaper** |

The upper bound was three times too high because read grows (turn 2: 50,979 → turn 12: **142,321**)
and each turn's own content is cached at the 1.25x rate (6,407–12,343 per turn, against ~50 with
trivial replies). **Read reaches 71% of a 200k window by turn 12**, so a fifteen-stage shared
session does not fit.

Confound recorded: SHARED produced 47% more output from identical prompts, so the 1.40x end-to-end
figure is indicative and only the 1.93x prefix-unit ratio is defensible.

**So one-session-per-run is a bad trade** — stage isolation, bounded context and per-stage failure
containment, for under 2x on the prefix term. **And a direct-API provider gets better**: its stages
are separate conversations sharing only a cached prefix, so they pay no accumulation penalty and
stay near the 8.1x per-stage figure. Revised ranking: **(1) direct-API provider with explicit
`cache_control`** (overlaps L4.8, now the only structural option worth the work), (2) trim
`.claude/rules/`, (3) reduce stage count — L3.24's lever, since a stage that does not run pays
neither prefix nor output — (4) install less.

**Corrected 2026-09-22 — this paragraph twice said output dominates the bill, and in the
architecture that ships it does not.** That framing came from the growth audit's own summary line and
was wrong for the COLD arm, which is what production runs. At the 5x output weight every Claude model
bills at, COLD's cache creation alone is **485,678** input-equivalents against output's **282,655**,
and the whole prefix term is **1.90x** output. Output would need to bill at 8.6x to overtake creation
and 9.5x to overtake the prefix, so the finding does not turn on the model. Output dominates only in
the SHARED arm — that is, only *after* a fix has already removed most of the prefix.

The ranking is unchanged, and the reasoning behind item (3) was repaired rather than kept: cutting a
stage wins because it removes that stage's prefix *and* its output, not because output was the bigger
term. What does change is the case for item (1): the prefix is roughly two-thirds of billable units
today, so the term a direct-API provider reaches is the larger half of the bill rather than the
smaller. Detail and the cost-column caveat: §3 of
[`loom-prefix-growth-2026-09-10.md`](../audits/loom-prefix-growth-2026-09-10.md).

~~Four options are costed in the audit. **Recommended: selective merging** — share a session only
among stages where isolation is not load-bearing — **plus trimming `.claude/rules/`** (7,707 tokens,
loaded in full on every stage). A direct-API provider with explicit `cache_control` keeps isolation
*and* the sharing and is the right end state, but it means implementing the agent loop the CLI
provides, and overlaps L4.8.~~ *(superseded by the growth measurement above.)* **This needs a design
decision before any code.**

A confound was caught mid-measurement and is recorded rather than buried: the first pass showed
agents and skills costing zero because the user's global `~/.claude` already held all of them, so
the baseline was never bare. The corrected run installs the same surface under non-colliding names
and reads the marginal cost.

**LEVER 2 TAKEN 2026-09-22 — `.claude/rules/` trimmed, and the ceiling came down with it.** The
core-rules bundle went **30,794 → 22,672 bytes**, ~2,030 tokens off every stage of every run
(~30k per fifteen-stage run), and `coreRulesCeilingBytes` moved 31,000 → 23,500 so the room is not
lent back.

**Nothing was deleted and no constraint was weakened.** Two sections moved to `docs/patterns/`,
following the split this repo already uses between `testing-conventions.md` and `testing-pyramid.md`:

| Moved | From | Tokens | Why it is not a rule |
|---|---|---:|---|
| Executor Enforcement + Policy-Based Gate Type | `approval-gates.md` | ~1,591 | Go internals, `--approve`, exit code 3, roadmap status. **51% of the most expensive core rule**, and an agent cannot act on a line of it |
| Test Annotation Convention mechanics | `testing-conventions.md` | ~1,054 | Six languages of syntax examples. A test is written in one language; twelve of fifteen stages write none |

The nine gates are untouched — a test asserts all nine headings survive, and the moved text was
verified present verbatim in its new home. Each rule keeps the normative statement and a pointer.

**The saving is locked by the fitness function that already existed.** `TestCoreRulesBundleStaysUnderCeiling`
enforces `shared/levels.yaml`, so lowering the ceiling in the same commit is what makes this a floor
rather than a one-off. Leaving 31,000 standing would have bought 8,300 bytes to drift back into —
which is precisely how the 5.5KB of drift that file records went unnoticed.

**One thing caught while doing it**, worth naming because it is the same defect as the drift above.
The first ceiling was set against 27,090 bytes — the **six**-rule always-set `generate-configs.sh`
emits — while the test sums the **five** paths in the `core-rules` bundle, 22,672. Two legitimate
lists, one wrong number in a comment. Corrected, and the comment now says which set it measures.

**What this does not do: it does not meet the done-when.** Trimming lowers the constant; it does not
break the linear scaling with stage count. That still needs lever (1), the direct-API provider with
explicit `cache_control`, which overlaps L4.8 and remains a design decision. Levers (3) and (4) are
unchanged.

**Therefore**: `aider-repo-map` and `repomix-codebase-packing` should **not** be built. They optimize
source-discovery cost, which this measurement shows is near zero, and they would add a per-run
indexing pass to a system whose spend is ~90% prompt-prefix re-caching. The two levers named in
§2 above — sharing the cached prefix across stages, and making a decline cheap — remain the only
ones the evidence supports, and L3.24 adds a third: not asking the stage at all.

### L3.20 — Papercuts from the second real run
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-06)*

**SHIPPED** 2026-09-21 — `5ee57c6`. The done-when asked that each be fixed or explicitly declined.
Four fixed; **#3 was already resolved by L3.32** (`c0ca216`, 2026-09-08) and is declined rather
than fixed twice — that item decided deliberately, on evidence from run 4, that a gate on a
routed-out stage still halts, and fixed the two narrower defects underneath. Re-fixing it here
would have reversed a decision made on evidence.

Two are worth noting beyond the fix. **#1** destroyed diagnostic information rather than merely
omitting it: the cause was in a buffer the function already held, and the empty `stderr:` suffix
read as "the process said nothing" when it had said plenty. **#5** was a convention asserted by a
generated artifact and implemented by nothing — the tech writer's own report cited a directory
that held two files.

Verified on a mock run for a feature deliberately named differently from the mock payload:
`route.md` reads `health-endpoint`, and the archive holds all nine artifacts where it held none.

Five small defects, each individually trivial, grouped so none is lost.

1. **A failing CLI reports nothing.** The architect's first failure was
   `exit status 1 — stderr: ` with an empty stderr. Under `--output-format json` the CLI writes its
   error to **stdout**, which the provider parses as an envelope and discards on failure. The actual
   cause (a usage limit) was unrecoverable from the run record. → `internal/provider/claude/claude.go`
2. **`route.md` is titled from the analyst payload, not the run.** The mock run's route document is
   headed `# Delivery Route: mock-feature` for a run whose feature is `health-endpoint`.
   → `internal/state/render.go`
3. **`confirm-ship` gates a stage the router may have skipped.** In the mock run devops-engineer was
   routed out, and the run still halted for approval before it. A gate guarding nothing still asks a
   human. → `internal/orchestrator/plan.go`
4. **Untyped artifacts keep their code fence.** `tech-writer.md` was persisted wrapped in a
   ```` ```markdown ```` fence, because untyped stages write the model's stdout verbatim.
   → `internal/orchestrator/executor.go`
5. **The feature archive holds no artifacts.** `docs/features/health-endpoint/` received only
   `run-state.json` and `run-events.jsonl`; every artifact stayed in `.claude/feature-workspace/`.
   The tech-writer's own report asserts the record "is already captured by the pipeline artifacts
   persisted under `docs/features/health-endpoint/`" — a documented convention that the executor
   does not implement. → `internal/orchestrator/executor.go`, `shared/skills/deliver-feature/SKILL.md`

**Done when**: each is fixed or explicitly declined in this list.

### L3.21 — `extractJSON` rejects a valid state document preceded by one sentence
**Workstream**: PLATFORM · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07, from the third real end-to-end run)*

**SHIPPED 2026-09-07** (`c19bd64`) — the last fenced block holding an object wins. The *last*
specifically, because a schema example an agent quotes back precedes its real answer and never
follows it, which a test pins. A block holding something else (a shell command after the answer) is
passed over rather than fatal; a response with no JSON object still fails.

1. **Problem**: The third real run **died at `qa-engineer`** with
   `agent did not return a JSON state document — got: Now producing the final QA state JSON.` The
   agent's JSON was complete, valid and schema-conformant; it was preceded by a single sentence of
   prose before the fence. `extractJSON` accepts raw JSON, and `unfence` accepts a response that is
   *entirely* one fenced block (`strings.HasPrefix(text, "```")`), so a leading sentence fails the
   prefix test and the run halts. The function's own comment concedes the case — "models fence their
   output as a formatting habit, and failing a run over that would report a reflex as a modelling
   error" — but the accommodation stops one sentence short of the same reflex. Worse, it is
   **nondeterministic**: an unmodified `--resume` re-ran the identical stage and it parsed
   first try. A run therefore dies or survives on whether the model prepended a sentence, and the
   failed attempt still bills (**$0.69** here, see L3.22). Note that the prompt already forbids this:
   `typed_stage.go:29` instructs *"Do not write files. Do not add commentary before or after the
   JSON."* The model disregarded both halves in the same run — it wrote files (L2.23) and it added a
   preamble — so the instruction is not load-bearing and the parser cannot assume it is.
   → `internal/provider/claude/typed_stage.go:29,66-92`
2. **Architectural Fix**: Accept a state document that is preceded by prose, while keeping the
   property the current strictness exists to protect — not accidentally adopting a schema example
   the agent quoted back. Taking the **last** fenced block in the response holds that property
   (a quoted example precedes the real answer, never follows it) and costs one line. Reject only
   when there is no fence and no leading `{`.
3. **Target files**: `internal/provider/claude/typed_stage.go`
4. **Done when**: a response of `<prose>\n\n```json\n{...}\n```` parses, a response containing a
   quoted schema example followed by a real state document resolves to the latter, and both are
   held by a test.

**Why it matters**: this is the only defect in the run that stopped the pipeline, and it stopped it
for a reason that has nothing to do with the work being done. A flaky run-killer is worse than a
deterministic one — it cannot be reproduced on demand, so it gets rediscovered rather than fixed.

### L3.22 — The run summary under-reports what the run actually cost
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07)*

**SHIPPED 2026-09-07** (`0927b82`) — a stage record accumulates every attempt rather than being
overwritten by the last. **Recurred and was re-fixed 2026-09-08** (`a3ebaee`); see the note below.

1. **Problem**: The third run's completion line and `loom memory runs` both report **$8.7978**.
   Summing `loom.usage.cost_usd` across every `generate_content` span in `traces.jsonl` gives
   **$9.49** across 12 calls. The $0.69 difference is exactly the `qa-engineer` attempt that failed
   to parse (L3.21) — spend that was billed and is recorded in the traces, but is excluded from the
   figure the operator is shown and from the figure persisted to the memory store. The error is
   silent and always in the same direction: retries and failures are free in the summary and not
   free on the invoice. The gap scales with how badly a run goes, which is precisely when the number
   is being read.
2. **Architectural Fix**: Total usage over every provider call the run made, not every call that
   succeeded. A failed attempt is a line item, not an absence.
3. **Target files**: `internal/orchestrator/executor.go`, `internal/memory/` (run record)
4. **Done when**: a run containing a failed stage reports a total equal to the sum of its trace
   spans, and a test asserts the two agree.

**RECURRED AND RE-FIXED 2026-09-08** (`a3ebaee`). Run 4's own cost data carried the same
under-report through a second door. The executor reported **$20.2718** for a run whose
`generate_content` spans total **$21.5106**; the $1.2389 delta is exactly the first `developer`
attempt — the record run 4 deleted by hand to re-run that stage, because loom has no rollback
command (run 4 §9.1).

The first fix made a stage record sum its own attempts, which stopped a retry overwriting a failure,
and left the total *derived* from the records. Removing a record therefore still removed its spend.
Money spent is a fact about the run, not a property of a record someone may delete, so it is now
accumulated on `RunState` as the run goes; the derived sum remains the fallback for states written
before the field existed.

**Why it matters**: L3.13 wants to derive quality metrics from execution, and already warns that a
run reporting zero cost reported *nothing* rather than costing nothing. This is the same class of
error one level up — a cost model that hides retry spend will systematically under-price exactly the
agents that need retrying most.

### L3.23 — `install` replaces committed project files with writable symlinks into a shared cache
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07)*

**SUPERSEDED 2026-09-07 by L3.26.** This entry records one symptom — two named project documents
replaced by symlinks into a writable shared cache. Reproducing it minimally showed the cause is
general: install's unit is the directory, so it destroys *any* pre-existing agent or skill content,
and `--copy` does not avoid it. Track the work in L3.26; this stays for the provenance.

1. **Problem**: `loom install --target .` replaced **124 committed files** in the clone —
   `.claude/agents`, `.claude/skills`, `.claude/rules`, plus `ARCHITECTURE_RULES.md` and
   `DOMAIN_DICTIONARY.md` — with symlinks into `~/Library/Caches/loom/v3.7.0/shared/`. It backed
   each up and printed that it had, but **after the fact**: there was no prompt, and no warning that
   the targets were tracked files with local content. `git status` went from clean to 124 deletions.
   Two consequences, one latent and one immediate. The **latent** one: the symlinked files are
   writable and shared by every project installed from that cache, so an agent that edits
   `DOMAIN_DICTIONARY.md` corrupts the framework for all of them. This is not hypothetical — the
   analyst emitted Developer Task 4, *"Add 'ConsoleLogger', 'captured log entry', and 'log entry
   type' to DOMAIN_DICTIONARY.md"*. The developer declined to do it, so the cache survived this run
   on the agent's judgment rather than on any property of the system. The **immediate** one: this
   project's own `DOMAIN_DICTIONARY.md` (13,363 bytes of its actual ubiquitous language) was
   shadowed by the framework's generic 18,530-byte default, while `design-principles.md` §6 requires
   every domain term to match that file. The install silently swapped the thing the rules are
   checked against.
2. **Architectural Fix**: Three separable pieces. (a) Detect that a target is tracked and locally
   modified, and require confirmation before replacing it — the approval-gates rule already covers
   "writing files out of boundary"; this is that gate, unwired. (b) Copy, or symlink read-only, any
   file an agent is permitted to edit; the shared cache must not be reachable through a project's
   working tree by a writable path. (c) Never shadow a project-authored `DOMAIN_DICTIONARY.md` or
   `ARCHITECTURE_RULES.md` — these are project content, not framework content, and `install`
   already knows how to skip (`skipped CLAUDE.md (already exists)`).
3. **Target files**: `internal/install/`, `cmd/loom/install.go`
4. **Done when**: installing over a dirty or tracked file prompts before acting; no path inside a
   project resolves to a writable file in the shared cache; and a project-authored dictionary
   survives an install.

**Why it matters**: every finding in this run was measured against agents the install put there, and
the install quietly changed what the project's own rules mean. A tool that rewrites 124 tracked files
without asking is one agent's good judgment away from corrupting every project on the machine.

### L3.24 — Two UI-only stages are marked non-skippable, and one boilerplate NFR routes in two more
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07)*

1. **Problem**: The feature under test was a **three-line synchronous array filter** with no UI, no
   I/O and no network. The router skipped `data-engineer`, `accessibility-engineer` and
   `devops-engineer` correctly — and then ran four stages that had nothing to do, for **$2.63**:
   - `visual-qa-engineer` ($0.55, 87s) — routed in as *"always runs; not skippable by routing"*,
     and reported `UNCONFIGURED`, *"no visual QA surface exists to evaluate"*. Note the
     contradiction: `accessibility-engineer` was skipped with the reason *"no accessibility
     requirement, so the analysis describes no UI surface"*. The same fact skips one UI-only agent
     and cannot skip the other.
   - `sre-engineer` ($0.63, 79s) — concluded there is no availability or latency SLI for an
     in-process test utility.
   - `architect` ($0.75) and `performance-engineer` ($0.70, 161s) — both routed in on a single
     boilerplate NFR line (*"O(n) ... no I/O"*) matching *"a performance requirement carries a
     measurable threshold"*. The performance report then answered **"Not applicable"** to all four
     of its own risk categories. This is L3.18's failure mode with a different trigger: L3.18 counts
     list items, this counts the mere presence of an NFR sentence the analyst writes every time.
2. **Architectural Fix**: (a) Make `visual-qa-engineer` and `sre-engineer` routable on the same
   evidence that already skips `accessibility-engineer` — a UI surface and a served runtime surface
   respectively; "always runs" is not a property either one earns. (b) Route `architect` and
   `performance-engineer` on a threshold that is *actually measurable* (a number, a budget, an SLO),
   not on the existence of an NFR heading.
3. **Target files**: `internal/state/` (routing predicates), `internal/orchestrator/plan.go`
4. **Done when**: this exact spec routes in neither UI stage nor `performance-engineer`, and a
   feature with a real latency budget still routes `performance-engineer` in.

**Why it matters**: L3.19 measured the floor — a stage that does nothing still costs $0.55–0.88. This
item is the other half: the cheapest stage is the one never asked to run. Over a quarter of this run's
spend — $2.63 of $9.49, 27.7% — went to four correct, well-written reports that said "not
applicable".

**SHIPPED 2026-09-07** (`ef81c76`), with L3.18. See that entry for the mechanism.

**What this does not cover.** ADR-007 holds that `visual-qa-engineer`'s real precondition is an
available UI evidence bundle for the built version, and records that as accepted-but-unimplemented.
This item does not implement it. It applies the necessary condition knowable from the analysis
today — a feature with no UI surface can never produce a bundle — which is strictly narrower than
always-runs and strictly wider than the bundle check, so it cannot skip a run the bundle check
would have kept. The bundle-availability half remains ADR-007's to deliver.

**What it should save.** $2.63 on a run of the third run's shape, at no added per-run cost. That
figure is a projection from one run and should be checked against a real one — and the repeat runs
under §9.1 of the run-3 audit are the right vehicle, since without a variance estimate a
before/after comparison cannot distinguish the saving from noise.

### L3.25 — `context-engineer` reports a token budget that is ~7x under, with arithmetic
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07)*

1. **Problem**: `context-engineer.md` states *"Recomputed precisely: 84 + 164 + 34 + 10 + 376 + 238
   ≈ **906 tokens**"*, then *"≈ **1,350 tokens total**"*, then *"Status: OK (≈1,350 tokens is ~1.1%
   of the Analyst tier budget — no cuts needed)"*. The real total is **~9,100 tokens**. Every term
   is derived from a ~2-tokens-per-line rule that holds for nothing in the list:
   `ARCHITECTURE_RULES.md` is 188 lines / 14,949 bytes — **~3,737 tokens, counted as 376**;
   `DOMAIN_DICTIONARY.md` 119 lines / 18,530 bytes — **~4,632 tokens, counted as 238**. The
   presentation is the problem as much as the number: "Recomputed precisely", a per-file breakdown,
   a percentage, and a Status line, all resting on a per-line rate that is wrong by an order of
   magnitude for prose. A budget that is 7x under will report OK right up to the point it overflows.
2. **Architectural Fix**: Estimate from **bytes** (`bytes/4`), not lines, and have the agent read
   file sizes rather than infer them from line counts. Better, compute the estimate in the executor
   from the files the manifest pins and hand it to the agent — this is arithmetic over known
   quantities, and there is no reason a model is doing it.
3. **Target files**: `shared/agents/context-engineer.md`, `internal/orchestrator/executor.go`
4. **Done when**: the manifest's estimate for a known file set is within 20% of a real token count,
   and the estimate is produced by the executor rather than asserted by the agent.

**Why it matters**: this is the run's clearest instance of the category that has been most valuable
in all three runs — not a wrong answer, but a **confidently** wrong one, dressed in enough supporting
detail that a reader has no reason to check it. The `context-engineer` exists to protect the context
budget; the number it reports that budget with is the one number in the run nothing verifies.

**SHIPPED 2026-09-07** (`bf302c8`) — `context-engineer` is now a typed stage producing
`ContextState`. It pins files and names a tier; the executor measures the pinned files from disk
(`bytes/4`), sums them, compares against the tier ceiling and writes the budget in. Anything an
agent puts there is discarded. Measured through a real run, `ARCHITECTURE_RULES.md` returns **3,737
tokens** against the manifest's claimed **376** — the audit's figure, reproduced.

The prompt, template and guardrail that asked for the estimate are corrected alongside the code that
trusted it, since the instruction was the cause and not a bystander.

**Honest scope on the accuracy claim.** This item's done-when said "within 20% of a real token
count". That is **not** what is asserted, because this repository has no tokenizer and nothing here
was compared against one. What is asserted is that the estimate is `bytes/4` over the real byte
counts, exactly, and that it is no longer wrong by an order of magnitude for prose — which is the
defect that was actually observed. `bytes/4` remains an estimate and will be wrong for content that
tokenizes unusually (minified files, dense CJK, long base64). Closing the remaining gap needs a real
tokenizer, and claiming 20% without one would repeat the mistake this item is about.

Two things fell out of building it. A pinned path that escapes the project root, names a directory,
or cannot be read is reported as unmeasurable rather than counted as zero. And a budget that fits
but could not see every file reports **INCOMPLETE**, not OK — a confident status resting on a
knowably short total is the same defect one level down.

### L3.26 — `install` clobbers agent and skill files it did not create
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-07, supersedes L3.23)*

**SHIPPED 2026-09-07** (`e9ed447`, `7340cd4`) — ownership is recorded per path in the manifest and
install resolves the four cases below; a directory source expands into one entry per file. A project
holding its own agent, skill and `DOMAIN_DICTIONARY.md` now survives an install with every byte
intact, no backups written, and `git status` clean apart from loom's own paths. Four tests assert it,
each confirmed to fail against the previous behaviour before being kept.

Cache files are read-only. That is not precautionary: while testing this, appending one line to a
linked `.claude/agents/analyst.md` wrote through the symlink and modified the copy shared by every
project on the machine — the corruption L3.23 §9.7 could only argue was possible. Cache directories
stay writable so the cache remains evictable. `ARCHITECTURE_RULES.md` and `DOMAIN_DICTIONARY.md` are
copied only when absent.

**Still open, and not part of this fix**: the four unimplemented level bundle actions described
under "Related" below. Until they exist, levels 2–4 install `.mcp.json` and documentation only.

L3.23 recorded one symptom — `DOMAIN_DICTIONARY.md` replaced by a symlink into a writable shared
cache. This is the general defect behind it, reproduced in a minimal case rather than inferred from
the run.

1. **Problem**: The unit of installation is the **directory**, not the file.
   `internal/platform/claude.go:7-8` maps `shared/agents` -> `.claude/agents` as one atom, and
   `Writer.Install` replaces whatever occupies a destination. A project that already has its own
   agents or skills loses all of them:

   ```
   $ echo "..." > .claude/agents/our-team-reviewer.md      # committed
   $ echo "..." > .claude/skills/my-deploy/SKILL.md        # committed
   $ loom install --target . --platform claude-code --level 1
     backed up .claude/agents -> agents.bak.1788807242968860000
     linked    .claude/agents -> ~/Library/Caches/loom/v3.7.0/shared/agents
     backed up .claude/skills -> skills.bak.1788807242969205000
     linked    .claude/skills -> ~/Library/Caches/loom/v3.7.0/shared/skills
   $ git status --short
    D .claude/agents/our-team-reviewer.md
    D .claude/skills/my-deploy/SKILL.md
   ```

   Three aggravating factors:
   - **`--copy` does not avoid it.** Same `backup` -> `replace` path (`fs/install.go:19-24`). There
     is currently no safe install mode, so L3.23's implied mitigation is not available.
   - **The manifest cannot tell ours from theirs.** `.loom-manifest.json` records paths at directory
     granularity (`.claude/agents`), so `loom uninstall` is blind in the same way in reverse.
   - **The cache is writable and shared.** Cache files are `0644` under
     `~/Library/Caches/loom/<version>/shared/`, shared by every project installed from it — the
     L3.23 near-miss.

   The blast radius is the whole adoption story: the projects most worth installing into are the
   ones that already have agent content, and those are exactly the ones this destroys.

2. **Architectural Fix**: **Own files explicitly; never touch anything unowned.** The manifest
   records every installed path with the content hash loom wrote, and install resolves four cases:

   | Destination state | Action |
   |---|---|
   | absent | install; record path + hash |
   | present, not in manifest | **skip and warn** — foreign, never backed up, never replaced |
   | present, in manifest, hash matches | ours and unmodified -> update |
   | present, in manifest, hash differs | user edited our file -> skip and warn; `--force` overrides |

   The last row is what makes the 1 -> 2 -> 3 upgrade path safe, which is the property "install into
   a project at any adoption level" actually requires.

   **Namespacing is not uniformly available — verified 2026-09-07, and it constrains the design.**
   Agents *are* discovered recursively (`.claude/agents/loom/analyst.md` is found), but a subagent's
   identity is its frontmatter `name`, not its path — so nesting prevents file collision and **not**
   name collision. Skills are worse: discovery is **not** recursive, a skill must sit at exactly
   `.claude/skills/<name>/SKILL.md`, and its invocation name *is* the directory name. So
   `.claude/skills/loom/deliver-feature/` would simply not load. Conclusion: per-file ownership is
   the mechanism, not namespacing. Name collisions are **detected and reported**, not prevented by
   layout. Ownership granularity therefore differs by kind — the file for agents and rules, the
   skill directory for skills, because that directory is the identity unit.

   Two further changes fall out of the same principle:
   - `ARCHITECTURE_RULES.md` and `DOMAIN_DICTIONARY.md` are **project** documents —
     `design-principles.md` §6 requires every domain term to match the latter. Symlinking them into
     a shared cache silently changes what the rules are checked against. They become
     `CopyIfMissing`, as `CLAUDE.md` already is (`claude.go:58`).
   - Cache contents become read-only (`0444` files, `0555` directories), which closes L3.23's
     latent corruption path outright instead of relying on an agent declining to write.

3. **Target files**: `cmd/loom/internal/manifest/`, `cmd/loom/internal/fs/`,
   `cmd/loom/internal/platform/`, `cmd/loom/cmd/install_levels.go`, `cmd/loom/cmd/uninstall_run.go`
4. **Done when**: a project with pre-existing `.claude/agents/our-team-reviewer.md` and
   `.claude/skills/my-deploy/SKILL.md` survives an install with both files intact, no `.bak`
   directories created, and `git status` clean apart from paths loom recorded as its own — asserted
   by a test, not by inspection.

**Why it matters**: this is the one defect that makes the framework unsafe to adopt incrementally.
Every other item on this roadmap improves a run; this one decides whether a team with existing AI
tooling can run it at all. It is also the second instance of the L3.23 pattern — a destructive
default that announces itself only after the fact, and whose damage was survivable by luck.

**Related: `--level` gates far less than its name implies.** Measured 2026-09-07, correcting an
earlier claim in this entry that `--level` defaults to "the maximum":

- **Every level installs all 39 agents and all 69 skills.** The platform install writes those
  unconditionally, before any level bundle is considered. `--level` narrows exactly one thing —
  rules, from 14 to the 5 core ones plus whatever `--stack` adds. So "level 1: foundational prompts,
  minimal footprint" is not what level 1 does.
- **Levels 2–4 install documentation and `.mcp.json`, and nothing else.** Four bundles (`executor`,
  `telemetry-stream`, `policy-engine`, `eval-loop`) declare an `action` that
  `installBundleAction` does not implement; only `mcp-config` exists. Everything else at those
  levels is `docsOnly`.
- **`shared/levels.yaml`'s `landed:` list had gone stale**, so those bundles were reported as
  *"requires roadmap item M0.4, which has not landed"* for M0.4, L3.9 and L2.16 — all three shipped
  between 2026-08-29 and 2026-09-02. Corrected in this commit; the install outcome is unchanged
  because the actions are unimplemented either way, but the message is now true. The file's own
  comment already required updating `landed` in the shipping commit, so the fix is discipline, not
  design.

Sequencing follows from that: changing the `--level` default is not the useful next move, because
the levels barely differentiate yet. Implementing the four bundle actions is, and until they exist
"supports levels 1–4" should be stated as "installs level 1, registers the MCP server at level 2,
and ships level 3–4 documentation".

### L3.27 — Give a pipeline a definition, so a project can have more than one
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: none · **Blocks**: a plan-editing TUI · *(raised 2026-09-07)*

**SHIPPED 2026-09-08.** `loom run --plan <name>` executes a plan defined in
`.claude/plans/<name>.yaml` (project-local) or `shared/plans/<name>.yaml` (installed), and
`shared/plans/deliver-bugfix.yaml` ships as the worked example — 8 stages against
deliver-feature's 15.

**The design decision, and it is the whole item**: a plan **selects and orders** stages from
`orchestrator.BuiltInStages()`. It does not define them. Gate, typed state kind, upstream reads,
skippability and timeout are all inherited and cannot be restated, so a plan cannot drop a gate,
un-type a contract, or make a routed stage unconditional. Two roadmap items paid for that
restriction: L3.24 measured an always-runs stage on a feature it could not serve at $0.55–0.88 a
time, and L2.17 bounded the review loop because prose said "repeat until APPROVED". A format
letting each project restate either would hand both bills back per-project. Loops are **named**,
not defined, for the same reason.

Validation rejects, each naming the offending line: an unknown stage, an unknown loop, a stage
whose `Consumes` upstream the plan omits, a loop spanning stages the plan lacks or ordered
backwards, the built-in plan's name, an unsupported version, and — the L3.24 preserver — **a
routable stage in a plan with no `router`**, which would otherwise always run with nothing
reporting the route that was never computed.

Upstreams are checked for **presence, not order**. `developer` consumes `code-reviewer`, which runs
after it, because on a second loop round the developer reads the findings that sent it back;
requiring upstreams to appear earlier would reject the built-in plan.

The built-in plan **stays in Go** and remains the default: it is the pipeline every run has
exercised, and putting it behind the loader on day one would make a malformed embed break every run
rather than only the custom ones. `TestTheBuiltInPlanIsExpressibleInTheFormat` generates the YAML
from the built-in plan's own stage order and asserts a byte-equal round trip, so the done-when holds
and adding a stage never requires editing the test.

**Not built**: new stages, project-defined routing predicates, and custom loop bounds. Those were
considered and declined — see the design decision above. A plan-editing TUI remains an adoption
decision (`bubbletea`/`huh`) on top of this, not an extension of it; `go.mod` still has no TUI stack.

1. **Problem**: There is exactly one pipeline and it is a Go function.
   `DefaultDeliverFeaturePlan()` builds the stage list, the gates, the typed-state kinds, the
   `Consumes` edges, the skippable set and the loop bound in code, and `selectPlan` rejects every
   other name:

   ```
   unknown plan %q — only %q exists today (custom plans are a later roadmap item)
   ```

   That error has been honest since it was written, and it is now the limiting factor. A team that
   wants a shorter pipeline for a bugfix, a longer one for a migration, or the same one minus a
   stage they do not staff has no way to say so short of editing Go and rebuilding. The framework
   ships fifteen agents and one arrangement of them.

2. **Architectural Fix**: A serialized plan — YAML alongside the other `.claude/` content — loaded
   and validated the way `shared/levels.yaml` already is, with `DefaultDeliverFeaturePlan()` becoming
   the built-in default rather than the only option. What the format has to carry is already fixed by
   what `Plan` holds today: stage order, agent, gate, state kind, `Consumes` edges, skippability,
   timeout, and loop bounds.

   Two properties the format must not lose, both of which cost real money to learn:
   - **Skippability is not free-form.** L3.24 showed that a stage marked always-runs on a feature it
     cannot serve costs $0.55–0.88 to say so. A custom plan declaring its own stages must also
     declare what evidence routes each one in, or every custom plan re-earns L3.24 privately.
   - **A loop needs a bound.** L2.17 put a bound on the review loop precisely because prose said
     "repeat until APPROVED". A plan format that lets someone write an unbounded loop hands that
     failure back to every project that writes one.

3. **Target files**: `internal/orchestrator/plan.go`, `cmd/loom/cmd/run.go`, a new plan loader
   package, `shared/plans/`
4. **Done when**: `loom run --plan <name>` executes a plan defined in a file, an invalid plan is
   rejected with the line that is wrong, and the built-in deliver-feature plan is expressible in the
   format without special-casing.

**Why it matters**: this is the prerequisite for every "custom workflow" conversation, including a
TUI for assembling one. A YAML plan is diffable, reviewable in a PR, testable, and shareable across
projects; a plan assembled only through a UI is none of those. Note also that the CLI is cobra and
`go-isatty` today — there is no TUI stack in `go.mod` — so a plan editor is an adoption decision
(bubbletea/huh) on top of this item, not an extension of something already present. Building the
format first also avoids designing it through a form, which is how a format ends up shaped by a
widget.

### L3.30 — A read-only stage modified source, through Bash
**Workstream**: TOOLS · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, from run 4)*

**SHIPPED 2026-09-08** (`28ac6d3`) — the posture half. L3.36 is the other half.

1. **Problem**: `accessibility-engineer` declares `tools: Read, Glob, Grep, Bash` — no edit tool —
   and modified `handlers.go`, saying so in its own report. `--allowed-tools` (L2.22) is an allowlist
   over named tools, not a write barrier: any stage holding Bash can write through a heredoc or
   `sed -i`, and six of the plan's stages hold Bash. "Read-only stage" was a property nothing
   enforced and nothing checked. `security-reviewer`, same posture, did not edit — so this is stage
   behaviour, not an inevitable consequence of granting Bash.
2. **Fix**: the executor fingerprints the working tree around each stage and records any that
   changed it without declaring write access, printing them at the end of the run. It **records
   rather than fails**: the edits were correct and improved the code, so the defect is that nothing
   noticed, not that it happened. Enforcing would mean removing Bash from six reviewing stages that
   use it to run checks, which costs more than the defect.
3. **Detail that matters**: the digest hashes content, not status. `git status --porcelain` alone
   would have missed this entirely — the edit was to a file the developer had already modified, so
   its status never changed, only its bytes did. A test pins that case.
4. Also corrects the comment on `writeTools`, which run 4 falsified: it claimed a stage declaring
   Bash without an edit tool "declares it to run checks, not to author code", which is true about
   what the declaration MEANS and false as the guarantee about behaviour it was written as.

### L3.31 — QA reports one package's coverage as the feature's
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, from run 4)*

**SHIPPED 2026-09-08** (`74283e9`).

1. **Problem**: `qa-engineer` reported `statementCoveragePercent: 89.7` with no qualifier. Real and
   correctly measured — for `internal/runs`. The feature also spanned `internal/httpserver`, holding
   the handlers, the pagination link and the page rendering, at **43.1%**. Neither the lower figure
   nor the word "package" appeared anywhere, and `testing-conventions.md` makes coverage >= 85%
   CRITICAL, so a reader concluded the feature cleared a bar most of its new surface did not.
   Nothing was fabricated: a real measurement of the wrong scope, presented unqualified.
2. **Fix**: a bare float cannot carry a scope, so coverage is a list of named units and the rendered
   report leads with the lowest — the number a coverage bar is judged against. `qa-engineer.md` is
   corrected alongside the schema, with these figures in it: the instruction said to ensure coverage
   meets 85% and never said whose.

### L3.32 — A gate halts on a stage the router already skipped
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, from run 4)*

**SHIPPED 2026-09-08** (`c0ca216`).

1. **Problem**: `confirm-ship` halted on `devops-engineer`, which the router had routed out.
   Approving it proved the stage still does not run, so routing is not bypassed and L3.24's saving
   is real — the audit downgraded this to INFO-to-LOW on that evidence. Two narrower defects
   survived: the halt stamped its own clock over the settled record, producing one whose `StartedAt`
   was **eight hours after** its `FinishedAt`; and nothing told the human the gated stage would not
   run.
2. **Fix**: a settled stage keeps the times it actually ran, and the halt carries the skip reason so
   the CLI can say the gate guards no work. **The gate still halts, deliberately** — `advance()`
   checks the gate before the settled check so a human checkpoint cannot vanish because routing
   removed the stage behind it.

### L3.33 — The architecture schema under-specifies what the validator enforces
**Workstream**: KERNEL · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, from run 4)*

**SHIPPED 2026-09-08.**

1. **Problem**: `validateDecision` requires `fitness` unless the decision is flagged
   `judgmentOnly`. The generated schema said none of it — `required` listed only `decision` and
   `rationale`, there was no `if`/`then` or `dependentRequired`, the real rule appeared solely in
   one field's prose description, and `judgmentOnly` (the escape hatch) carried **no description at
   all**. Run 4's Experiment A adds one method to one class, so the architect reasonably had a
   decision with no meaningful fitness function, omitted it, and had no way to learn that
   `judgmentOnly: true` was how to say so. **$2.52 spent, run abandoned at stage 4 of 12** — and
   with L3.18's incomplete filter (fixed separately in `a96b6aa`) this is why Experiment A produced
   no cost or variance data at all.
2. **Fix**: the conditional is declared once as data in `internal/state/conditional.go`. The
   validator reads its error message from that declaration and the generator emits it into the
   schema as an `anyOf` — either `fitness` is present, or `judgmentOnly` is present and `true`.
   One statement, two consumers, no room to drift. The escape hatch is now documented, since a flag
   nothing describes cannot be used.
3. **Done when**: a document using the escape hatch the schema offers is accepted by the validator,
   and one with neither is still refused naming the hatch. Both asserted.

**The third instance of one defect.** L2.25 (STRIDE enum), L3.28 (`schemaVersion`), L3.33 (this) are
all "a constraint the validator enforces and the schema does not communicate", and all three were
invisible to mocks because mocks build valid state in Go.
`TestEveryConditionalRequirementReachesTheSchema` is the fitness function for the third shape.

**Measured limitation, worth recording.** L3.35's model-boundary test does **not** catch this. It
was run with L3.33 reintroduced and **passed**: `fitness` only binds when the architect has a
decision without one, and against L3.35's trivial spec it wrote one for every decision — the same
reason run 4's Experiment B passed where A failed. L3.35 catches *always-binding* contract defects;
*conditionally-binding* ones need the unit test. Both files now say so.

### L3.35 — Test the model boundary, not the mock behind it
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, from run 4)*

**SHIPPED 2026-09-08** (`internal/provider/claude/contract_integration_test.go`).

1. **Problem**: run 4's audit §12.6 — four of its seven findings were invisible to the test suite,
   which was green before the run and green after it. Neither state predicted anything. Three had
   one shape: a property asserted in the framework and verified against mocks, which does not hold
   when a real model is asked. **L3.28** ($2.09, two runs dead at stage 1), **L3.33** (halted
   Experiment A at stage 4), **L3.29** (developer reported success against an empty `git diff`).
   A mock cannot find any of them: it builds state in Go, where every constant is correct by
   construction and no instruction is obeyed or disobeyed. The gap was never coverage — the tests
   were on the wrong side of the boundary.
2. **Fix**: a contract test that asks a **real model** for each typed stage's document and asserts
   the validator accepts it, plus one that asserts the developer actually writes a file. The
   assertion is the validator itself rather than a restated field list, because the defect class is
   "the schema says something weaker than the validator enforces" and any restatement would drift
   from it exactly as the schemas did.
3. **Cost control**: excluded from `go test ./...` by a build tag **and** gated on `LOOM_INTEGRATION`,
   because a suite that silently spends money is worse than no suite. ~$0.50–1.50 per stage; run one
   kind while iterating.
4. **Verified as an instrument, not just as a test**: reintroducing L3.28 makes it fail in 23
   seconds with the production error verbatim — `field "schemaVersion" is 1, this build supports 2`
   — for about $0.50, against the $2.09 and two dead runs it cost to learn the same thing from a
   real pipeline.

**What it does not prove**: that a stage's output is *correct*, only that it conforms. A model can
return a schema-valid analysis that is nonsense and this passes. It is also n=1 per run against a
nondeterministic system, so a pass is evidence and not proof. Both limits are stated in the file.

**Why this is worth the effort**: run 4's cheapest correct response was never seven fixes. Every
mock-verified claim in this repository is now one command away from being checked at the boundary
that matters, and the remaining ones have not been audited — §12.3 of that run's audit says so
explicitly.

### L3.36 — Nothing re-reviews what the post-review stages write
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-08, split from L3.30)*

**SHIPPED** 2026-09-18 — `757fa54`. The done-when is met: a run whose tree changed after
`code-reviewer` approved now says so, as a run-end warning and on the stage record.

**Candidate 1 of the three, chosen deliberately.** It records the divergence and does not prevent
it — re-reviewing the delta and moving the review later both exceed this item's own done-when and
are a larger, separate decision. The restraint is the point: `sre-engineer` did nothing wrong in
run 4, and the defect is that a human could not tell.

**L3.50 made this urgent rather than moot.** The posture check exempts stages that declare a write
tool — correctly, because it asks a different question. It caught run 4 only because
`accessibility-engineer`'s declaration was *wrong*; making the declaration honest removed the
incidental catch, and every post-review stage now declares `Write`, so that check no longer notices
this case at all. Detection now lives in a check that knows about the review boundary.

The boundary anchors on the reviewing stage's typed **kind**, not its ID, so renaming or
duplicating that stage still anchors. Both run-end warnings also moved out from behind the usage
early-return, which had made them reachable only when the provider reported tokens.

> **The mechanism question is answered; the policy question is what remains** (L3.50, 2026-09-16).
> This item asked how `accessibility-engineer` — "declares write tools? **no**" — changed production
> source. It was *instructed* to ("Fix violations directly whenever possible"), `Bash` was the only
> channel it had, and that channel is invisible to anything reading the `tools` field. Its tools now
> declare `Write` and `Edit`, so the divergence is at least visible.
>
> What is left is the decision, and L3.50 deliberately did not make it: **should a post-review stage
> modify a tree `code-reviewer` already approved?** Stripping those agents' write channel there would
> have decided this by the back door. The three candidates below are unchanged.

1. **Problem**: `code-reviewer` returned `APPROVED` against a **312-insertion** tree in run 4. The
   tree that ended the run was **343**. Three stages modified it afterwards:

   | Stage | Declares write tools? | Changed | Re-reviewed? |
   |---|---|---|---|
   | `accessibility-engineer` | no | production source (+12) | no |
   | `qa-engineer` | yes | test files only | no — its own remit |
   | `sre-engineer` | **yes** | production source (+19/-8) | no |

   The posture half of L3.30 is fixed (`28ac6d3`) and covers only the first row. This is the
   structural half, and it implicates a stage that **did nothing wrong**: `sre-engineer` is entitled
   to write, its `slog` instrumentation was correct and low-cardinality, and the suite still passed.

   The plan orders `code-reviewer` before four stages that can modify source. Nothing re-reviews,
   and **no artifact records that the approved tree and the shipped tree differ** — neither the fact
   nor its size appears anywhere in run state.

2. **Architectural Fix**: not obvious, which is why this is separate rather than bolted onto a
   check. Candidates, none costed:
   - Record the divergence without acting on it — cheapest, and at least makes it visible.
   - Re-run `code-reviewer` over the delta when post-review stages changed production source, which
     costs a review invocation on most runs.
   - Move the review later in the plan, which trades this problem for a longer feedback loop and
     makes the design gate approve less.

   The executor now fingerprints the tree around every stage (L3.30), so the mechanism to detect the
   divergence exists; what to *do* about it is the open question.

3. **Target files**: `internal/orchestrator/`, `internal/orchestrator/plan.go`
4. **Done when**: a run whose tree changed after `code-reviewer` approved says so in an artifact a
   human reads.

**Why it matters**: "review sees what ships" is a property the pipeline sells. Run 4's audit rates
this arguable at §12.4 — the unreviewed edits were correct, so the finding rests on process rather
than outcome — and it is recorded as a defect for exactly that reason: the run got a good result
from a mechanism that does not guarantee one.

### L3.37 — A form for composing pipelines
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: L3.27 (shipped) · **Blocks**: none · *(raised 2026-09-09)*

**SHIPPED 2026-09-09** (`6785221`).

1. **Problem**: L3.27 gave a pipeline a definition; nothing made the definition discoverable. A team
   had to know the YAML by heart, and had no way to see which plans a project could run.
2. **What shipped**: `loom plan list` and `loom plan show <name>`, which work anywhere including CI,
   and `loom plan new`, a `huh` form. A plan file that does not parse is listed as **BROKEN with its
   error** rather than omitted.
3. **The design decision**: the form gathers a name and a stage set and **does not validate**. What
   it renders goes through `planfile.Parse` before it reaches disk, so a composed plan obeys exactly
   the rules a hand-written one does and a rule added to the loader covers this command for free. A
   form with its own idea of what is legal is how the two drift. A test composes a plan with a
   missing upstream and asserts the refusal carries the loader's words.
4. **Deselect, do not reorder.** Stages are offered in the built-in plan's order, all selected.
   Reordering is a text edit — the built-in order is the only one this command could offer without
   inventing a second source of truth for which order is right, and "the same pipeline minus the
   stages we do not staff" is the case L3.27 came from. Each option carries its gate and whether it
   is routable, since that is the reason to keep a stage and is invisible in a list of bare names.
5. **`--accessible`** swaps the full-screen TUI for plain prompts: for screen readers, and because
   it is the only mode that works in a terminal which does not answer the capability queries
   (OSC 11, cursor position) the full-screen renderer blocks on.

**Two defects found by driving the real form, not by reading it.** `validatePlanName` trimmed before
checking while the raw value became the filename, so a name with a trailing space produced
`my-plan .yaml` — observed in a PTY, not theorised. And the write path now validates too, because
`--name` skips the prompt the form's validator runs in.

**Dependency added**: `charmbracelet/huh`. The CLI was cobra and `go-isatty` before this; a TUI stack
was the adoption decision L3.27 named, and it is taken here rather than assumed.

**Not built**: reordering stages, and editing an existing plan. Both are text edits on a format
designed to be edited by hand, and neither is worth a form until someone finds the text edit
insufficient.

### L3.13 — Derive agent quality metrics from execution
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L3.5 (shipped), L3.8 (shipped) · **Blocks**: none

**UNBLOCKED** 2026-09-02. Both blockers shipped, and the store already answers two of the four
metrics this item wants with measured data: how often an agent needed more than one attempt
(`loom memory retries`) and how often a human corrected its output (`loom memory corrections`).
Neither is an LLM reading persisted markdown.

Two properties to inherit rather than re-derive. A correction was **recorded, not adopted** (L4.5),
so it is evidence an agent's output needed fixing and not that anything shipped fixed. And a run
reporting zero cost reported *nothing*, which is not the same as costing nothing — a scorecard that
conflates those produces a confident wrong number.

1. **Problem**: `agent-scorecard/SKILL.md` scores four metrics computed by an LLM reading persisted
   artifacts. Nothing measures retry count, human-edit rate, gate rejection rate, latency, or cost,
   because none are recorded. The scorecard self-describes one metric as "directional, not exact,
   until that dispute-tracking mechanism exists."
2. **Architectural Fix**: Compute metrics from the episodic store and OTel spans: retries per stage,
   `edited_then_approved` rate per producer, gate rejection rate, p50/p95 latency, tokens and cost
   per stage. The LLM writes the *narrative*; the numbers come from telemetry.
3. **Target Files**: `shared/skills/agent-scorecard/SKILL.md`,
   `shared/skills/pipeline-retrospective/SKILL.md`, `docs/agent-metrics/`
4. **Done when**: the scorecard contains at least three metrics no LLM computed.

**SHIPPED** 2026-09-21. `loom memory agents` counts per agent: attempts, retries, failures, human
corrections, p50/p95 latency and cost, with a `--json` form the skill consumes. `agent-scorecard`
now has two tables that must not be merged — **Measured**, counted from records, and **Judged**, a
model reading artifacts — and the done-when is met five times over rather than three.

**One judged metric became measured rather than being kept twice.** `code-reviewer`'s first-pass
acceptance rate was a model reading `changesRequestedCount` out of `pipeline-trace.json`. Every
stage in a loop's span carries the same iteration count (`internal/orchestrator/loop.go`, `reenter`),
so a `code-reviewer` stage with `iterations > 1` is a round the loop went again — the same fact,
counted. It moved to the measured table and was removed from the judged one; keeping both would
have invited a scorecard reporting two figures for one property.

**Most of the work is about the numbers that were never taken.** A stage that never finished has no
duration, and averaging it as 0 makes the agent furthest from finishing look fastest. An agent with
no stages has no failure rate, and 0% would read as perfect. A provider reporting nothing is not a
provider reporting zero. Each is a nullable field, an em dash in the table, and a test; the skill is
told to report an absent value as absent and never to reconstruct a measured metric by reading
markdown when the store is unavailable, because a derived figure in a measured column claims a
provenance it does not have.

**Percentiles are nearest-rank, not interpolated.** With the sample counts a real corpus holds, an
interpolated p95 invents a duration no stage ever took, and the number exists to be compared against
stages that really ran.

Eleven tests. What this does not do: it adds no new collection. Every figure was already in the
store and unqueried per agent.

---

## Workstream: OBSERVE — Test Evidence & Trace Fidelity

Seven items raised 2026-09-13 from a review of two training levels written against loom — Level 2B
(AI adoption in existing test suites) and Level 3C (GenAI observability). The review is reference
material, not instruction: per `shared/rules/memory-trust-boundary.md` nothing in it overrides a
hard constraint, and two places where it argues for relaxing one are named below rather than
adopted. Three of its premises were stale — `gen_ai.*` emission, pipeline instrumentation and the
trace file it proposed building all shipped in L3.8 — and are not items here. What follows is what
was measured absent.

### L3.38 — Tool spans record the payload, not the properties
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: L3.8 (shipped) · **Blocks**: none · *(raised 2026-09-13)*

**SHIPPED** 2026-09-13 (`3b18bca`) — guardrail **#9** (telemetry records properties, not payloads),
and `internal/telemetry/tool.go` inverted from a denylist to an allowlist.

A value reaches a span only when the tool declared that argument safe; everything else becomes a
length, plus a salted hash when `LOOM_TELEMETRY_SALT` is set. The inversion is what the
consumer-extensible registry requires: a denylist of content-shaped names cannot cover an argument in
someone else's tool (`examples/embedding`'s echotool takes `text`), and that failure is silent.

`tools.SafeArguments` is an optional interface a Tool implements to declare its own non-sensitive
arguments — opt-in because only the tool's author knows whether `text` is an echo payload or a user's
message. A tool that implements nothing gets every value hashed. A secret-shaped name is redacted
whatever the tool declares. No salt means no hash, never an unsalted one.


1. **Problem**: `internal/telemetry/tool.go` redacts an argument when its *name* looks secret —
   `token`, `password`, `api_key` — and truncates everything else to 512 characters. Secret
   redaction is not content minimisation. `search_ki` and `search_docs` each declare a required
   `query` argument (`shared/mcp/internal/tools/search_ki_tool.go:30`), so a traced run exports
   `loom.tool.arg.query` verbatim, and `loom.tool.result` carries up to 512 characters of whichever
   KI body came back. Guardrail #8 constrains where instrumentation may live and says nothing about
   what it may carry; `docs/patterns/observability-patterns.md`'s "No PII or Secrets in Telemetry"
   states the rule as prose that nothing enforces. The default exporter writes a local file, which
   bounds this to the machine — until `OTEL_EXPORTER_OTLP_ENDPOINT` is set, which is the documented
   path and the reason export was made opt-in.
2. **Architectural Fix**: a second denylist, over content-shaped argument *names* (`query`, `text`,
   `content`, `prompt`, `message`, `input`, `body`), recording a salted hash and a length instead of
   the value; and a result attribute carrying size and status rather than a preview. Promote the
   rule to `architecture-guardrails.md` **#9**, so it binds the project code these agents write and
   not only loom's own emitter.
3. **Target files**: `internal/telemetry/tool.go`, `shared/rules/architecture-guardrails.md`,
   `docs/patterns/observability-patterns.md`
4. **Done when**: a tool call whose argument is named `query` produces a span carrying no substring
   of that query, and a test in `internal/telemetry` fails if the denylist loses an entry.

**Not built**: a content-capture escape hatch, and replay testing. Both need captured prompt text.
Shipping a constraint and its exemption in the same increment is how the constraint fails to
establish, and the training that teaches replay flags the same collision itself.

### L3.39 — Nothing constrains what an agent may change when repairing a test
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: none · **Blocks**: L3.40 · *(raised 2026-09-13)*

**SHIPPED** 2026-09-13 (`0479b56`, fitness function `7a78e8c`) —
`shared/rules/test-repair-contract.md`, approval gate **#9** (Removing Test Coverage), and nine
agents bound.

The contract states what a repair MAY change (selectors, fixed sleeps, an expected value *with source
evidence*) and MAY NOT (remove or weaken an assertion, add a construct converting failure to pass,
skip, quarantine, retire, move a CI threshold). Every repair states what the test could catch before
and after — the load-bearing clause, because that sentence is hard to write honestly about a deleted
assertion.

Gate #9 is scoped to deliberate coverage removal and is Always Human. Assertion removal is forbidden
outright rather than gated: a rule that says no is cheaper than a halt, and a gate there would fire
on every legitimate test rewrite.

`dx-engineer` 1.1.0 is why this existed — step 4 said "Quarantine flaky tests", with Write and Edit
and no gate. It now proposes one carrying owner, expiry, cause and resolving evidence.

Timeout widening requires the measured p95 and why the old value was wrong, with **no fixed
percentage**: the source material's 25% is admitted arbitrary and exists to force a number into the
conversation, which the evidence requirement does without hardcoding a threshold.

`health-check` fails when any of eight pinned test-writing agents drops the contract reference.


1. **Problem**: an agent can make a suite green by deleting the assertion that was failing, and no
   rule, gate or check in this repository forbids it. The entire counter-statement is one line —
   `shared/agents/qa-engineer.md:118`, "Never skip a test just to make the suite green" — which
   binds one agent and is enforced by nothing. `shared/agents/dx-engineer.md:27` runs the other way:
   it instructs the agent to *"Quarantine flaky tests"*, with `Write` and `Edit` in its tool list,
   no owner, no expiry, no cause, and no gate. Removing regression signal is a one-way door and
   belongs with the other eight.
2. **Architectural Fix**: `shared/rules/test-repair-contract.md` — a MAY / MAY NOT list over
   test-file edits (selectors and waits yes; assertion removal, a `try/catch` that converts a
   failure into a pass, skip, and quarantine no), plus a required statement of what the test could
   catch before the repair and what it can catch after. Referenced by every agent holding
   test-write authority. A ninth approval gate covers skip and quarantine; `dx-engineer` proposes a
   quarantine rather than applying one.
3. **Target files**: `shared/rules/test-repair-contract.md` (new), `shared/rules/approval-gates.md`,
   `shared/agents/dx-engineer.md`, `shared/agents/qa-engineer.md`, `shared/agents/developer.md`,
   `shared/agents/refactor-engineer.md`, `scripts/health-check.sh`
4. **Done when**: an agent with test-write authority that does not reference the contract fails
   `health-check.sh`.

**The fitness function is the drift check, not the diff.** A CI grep counting removed `expect(`
against added ones is the cheap diagnostic the source material recommends, and it is warn-only
here: it is language-specific and it flags a legitimate rewrite exactly as loudly as a deletion.
The deterministic check is that every test-touching agent carries the contract — the same shape as
`scripts/check-cap-drift.sh`, and gate #7 applies to wiring it.

### L3.40 — Exemplar tests: the pattern an agent copies, kept honest
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: L3.39, L3.41 · **Blocks**: none · *(raised 2026-09-13)*

**SHIPPED (concept scope)** 2026-09-14 — `shared/contracts/exemplar-contract.md`,
`.claude/exemplars.yaml`, the `test-exemplars` registry source, the `exemplar-auditor` counter agent,
and the eight test-writing agents told to consult the exemplar for their language and level.
**The executor barrier is deliberately not in it — see L3.46.**

**What the scope split bought.** §9 of this item left protection open, and it stayed open: marking a
test as an exemplar does not yet stop an agent editing it. Gating reverses `posture.go`'s explicit
observe-don't-gate stance, and that reversal deserves its own argument rather than riding inside a
large increment — particularly before anyone knows how often an exemplar legitimately changes, which
is exactly what the audit trail will say.

**loom declares three of its own**, against the earlier "consumer projects only" instinct. A
mechanism with no live users is one nobody has verified: the manifest, the annotation convention and
the health-check agreement were all exercised against real files rather than a fixture. The set is
bounded by what this repository actually tests — Go at unit and integration, nothing invented for
languages it does not use.

**Two costs paid, as predicted.** Adding a fortieth agent cascaded further than the item guessed:
config regeneration, a golden-file fixture with a real baseline, inventory counts in four documents,
and a Go test pinning the agent count. And the exemplar section in `testing-conventions.md` pushed
the core rules bundle 40 bytes past the ceiling raised only the day before — which is the tight
margin working as intended. The fix was not another raise: the per-language mark table moved into
the contract, where detail belongs, leaving a pointer in the rule every agent loads.

**What is enforced, and what is not.** `health-check` asserts the manifest and the annotations agree
in both directions, and the mechanical disqualifiers are checkable. Whether an exemplar is *good*
stays judgment — `exemplar-auditor` reads it against the contract and reports, and nothing verifies
that judgment was made honestly. The contract says so rather than implying the auditor settles it.

1. **Problem**: every agent here that writes a test — `qa-engineer`, `test-driven-developer`,
   `unit-tester`, `api-test-generator` — learns the house pattern from prose.
   `testing-conventions.md` states rules; `docs/patterns/testing-pyramid.md` states philosophy;
   neither shows one good test. The framework already solved this for its own agents — "
   `shared/agents/memory-auditor.md` — the **pattern exemplar** every new counter agent should
   follow" (`docs/aos/prompts/phase-2-governance.md:24`) — and never did it for tests. An **Exemplar
   Test** is a real, executing test in a project's own suite, marked as the one to imitate.
2. **Not "golden", deliberately.** `DOMAIN_DICTIONARY.md` lists `golden file` under Synonyms to
   AVOID on the **Agent Eval Case** row, reserving it for the structural check over
   `tests/agents/*/actual-output.md`. Golden also carries the golden-master sense throughout — a
   recorded baseline to *compare against*, which is the opposite instruction to *copy this*.
   Exemplar is this repository's own word for the role and collides with nothing.
3. **They live in the project's suite, in place.** Not copied into `.claude/exemplars/`. An exemplar
   that the real suite does not execute stops being evidence the moment it stops passing, and
   nothing would notice. The mark travels with the test; only the *index* lives elsewhere.
4. **Marked twice, and the disagreement is the check.** Language-native annotation per
   `testing-conventions.md`'s Test Annotation Convention (`@exemplar` in a JSDoc block,
   `@Tag("exemplar")`, `[Trait("Exemplar", …)]`, a pytest marker, a `# exemplar` comment, a
   `@exemplar` Gherkin tag — the same mechanism as `@issue`/`@ac`), plus an entry in a project-local
   manifest. Two sources of truth is the obvious objection; the answer is that `health-check` asserts
   them equal in *both* directions — every manifest entry's file carries the annotation, every
   annotated file appears in the manifest. Checked redundancy, not drift.
5. **Tracked as memory.** A new `shared/memory-registry.json` source — `test-exemplars`, type
   `Exemplar Test`, `retrievalBackend: lexical` — pointing at `.claude/exemplars.json`. Two
   consequences to get right: the path **must** be added to `optionalPaths`, or `health-check.sh:442`
   fails on every repository that has no exemplars, this one included; and this becomes the first
   registry source whose content is source code rather than markdown, which the `lexical` backend
   describes as "tag/domain pre-filter over frontmatter, then full-body read" — a test file has no
   frontmatter, so the manifest entry carries the metadata the pre-filter needs (language, level,
   what the test demonstrates) and the file supplies the body.
6. **Per-language, bounded by the repository.** Each entry declares `language` and test `level`
   (unit / integration / api-contract / acceptance / e2e). Coverage is required only for the
   (language, level) pairs a project actually has tests for — a Go service is never asked for a
   Kotlin exemplar, and loom ships none of its own. The eight language `shared/rules/*-conventions.md`
   files describe the conventions; the exemplars are the consumer project's own proof it follows them.
7. **How an exemplar is checked to still be one** — three layers, honest about which are decidable:

   | Layer | Catches | Deterministic |
   |---|---|---|
   | Digest recorded in the manifest | The file *changed* | Yes — the L2.14 artifact-digest mechanism, applied to a second thing |
   | Mechanical disqualifiers | It no longer meets the contract, checkably | Yes |
   | `exemplar-auditor` counter agent | It is no longer good craft | **No** — judgment, findings for a human |

   The mechanical disqualifiers, each reusing machinery that exists: the test does not run or is not
   in the suite (it proves nothing); `@issue`/`@ac` annotation missing (`testing-conventions.md`
   requires it of every test, and an exemplar violating the convention it demonstrates is the worst
   case); cyclomatic complexity ≥ 7 (`analyze-complexity`); filename off the
   `name.type.extension` convention; and **vacuity** — mutate the code under test, and if no
   assertion in the exemplar fails, it is disqualified automatically. That last one is why **L3.41
   blocks this item**: an exemplar that cannot fail teaches every test written after it to be
   equally hollow, and it is the single check most worth having before any of this ships.

8. **The auditor is a new counter agent, and that cost is real.** `exemplar-auditor`, read-only,
   findings for human review, matching the twelve existing counter agents. The cheaper alternative —
   extending `memory-auditor`, since exemplars become a registry source it already sweeps — is
   rejected because the judgment wanted here is test craftsmanship and `memory-auditor` has none;
   a sweep that checks schema and duplicates would pass a beautifully-registered bad test. If the
   thirteenth agent is judged too expensive, extend `code-reviewer` instead, which has the competence
   and lacks the read-only posture.
9. **Protection is a separate question, left open.** An exemplar probably also wants the "no agent
   may weaken this" property L3.39 defines — it is the one place the two concepts genuinely meet.
   This item does not assume it: the drift *detection* here is a digest and a flag, not a barrier,
   and whether an exemplar edit should halt a run is a decision to make once the audit exists and
   has said how often exemplars legitimately change.
10. **Target files**: `.claude/exemplars.json` (new, project-local),
    `shared/contracts/exemplar-contract.md` (new — the family of `ki-frontmatter-contract.md`),
    `shared/memory-registry.json`, `shared/rules/testing-conventions.md`, `DOMAIN_DICTIONARY.md`,
    `shared/agents/exemplar-auditor.md` (new), `scripts/health-check.sh`
11. **Done when**: an agent writing a new test in a project with exemplars reads the exemplar for
    that language and level before writing, and `health-check` fails when a registered exemplar has
    lost its annotation, gone missing from the suite, or drifted from its recorded digest without
    an audit since.

**What this does not do.** It does not make an exemplar *correct* — an auditor reading a test
against a craftsmanship contract is the same class of judgment as `code-reviewer` reading a diff,
and no check verifies that judgment was made honestly. The deterministic half is that an exemplar
runs, asserts something that can fail, carries its annotations, and has not silently changed. Good
taste stays human, and the contract should say so rather than implying the auditor settles it.

### L3.41 — Nothing asks whether a test would fail
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: L3.40 · *(raised 2026-09-13)*

**SHIPPED** 2026-09-13 — `code-reviewer` 1.2.0 gains process step 7 and a **Test Evidence** criterion;
`unit-tester` 1.3.0 and `backfill-unit-tests` gain the mutation stopping condition.

**The gap was narrower than this item claimed, and the correction is the useful part.**
`## Test Design Review` was *already* a required section in `shared/contracts/review-contract.md`,
already in the template, and already a field in `review.schema.json`. What it contained was one
bracketed prompt — "[Are tests verifying behaviors instead of implementation details?]" — and
`code-reviewer.md` listed no criterion for it anywhere. So this shipped as **giving an enforced
section something to say**, not as adding a section: no contract change, no schema regeneration, no
fixture rework, and the S estimate held.

**Three decisions worth keeping.** `UNCERTAIN` is advisory and never blocks — the agent is
confidently wrong in both directions here (a real assertion inside an unread custom matcher looks
vacuous; a self-fulfilling mock configured in a distant `beforeEach` looks legitimate), and both are
failures of what it could see rather than of reasoning, so it names the file it would need instead
of guessing. A `NO` must name one of the four types, because "this test is weak" is not an
actionable blocking finding and the contract requires `CHANGES_REQUESTED` to carry one. And scope is
the diff's own tests — a suite sweep is unbounded the moment a shared helper changes.

**Where the mutation check runs, and why there.** `backfill-unit-tests` step 6 mutates three
behavior-carrying lines in a **throwaway git worktree** and confirms each breaks at least one test.
It belongs to the skill rather than to `unit-tester` because that agent may never modify source,
full stop — and a worktree keeps that rule absolute instead of granting it a
revert-afterwards exception that a crashed run would leave behind as mutated source. A surviving
mutation is blocking: either the test that catches it gets written, or the gap is recorded in the
report's NOT COVERED list.

**Still judgment, and the item said so.** No check verifies the reviewer answered honestly — the
reading half is the same class of judgment as any other review criterion. What is now mechanical is
the mutation half, and it is the half L3.40 depends on.

1. **Problem**: `code-reviewer` reviews test code for style, naming and structure. It never asks the
   one question separating a test from a decoration: *would this fail if the behaviour it names were
   broken?* A test asserting `toBeDefined()` on a function that always returns an object passes
   review, passes CI, and covers nothing. The word "vacuous" appears twice in this repository, both
   times about a check on loom's own import graph, never about a test.
2. **Architectural Fix**: one review section in `code-reviewer.md` — YES / NO / UNCERTAIN per test
   touched, naming the vacuity type when NO (vacuous, self-fulfilling, unreached, disabled) — and
   the mutation stopping-condition made explicit in `backfill-unit-tests`: change three lines of the
   target, and each must fail at least one of the new tests, or the net is not a net.
3. **Target files**: `shared/agents/code-reviewer.md`, `shared/skills/backfill-unit-tests/SKILL.md`,
   `shared/agents/unit-tester.md`
4. **Done when**: a review of a diff containing a vacuous assertion names it.

**Judgment-only, and the reason is worth keeping.** The reading half is a judgement a model makes
about a diff, and no check verifies it was made honestly. The mutation half *is* mechanical wherever
a mutation tool exists — and loom already practises precisely this on itself: L3.8 ships a companion
test that fails if `internal/telemetry` ever stops importing OpenTelemetry, "so the check cannot
quietly become vacuous." The framework demands this of its own fitness functions and has never asked
it of a test.

### L3.42 — A suite has no health metrics, only a coverage number
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-13)*

**SHIPPED** 2026-09-14 — `docs/patterns/test-suite-health-metrics.md`,
`shared/knowledge/flake-triage-taxonomy.md`, and one reference from `dx-engineer` 1.3.0.

**Judgment-only, as this item said, and the doc says so in its own text.** Three of the four numbers
have no source loom can read: escaped defects live in a bug tracker, time to diagnose in ticket
timestamps, flake rate in CI history. Only wall-clock is reachable, and one of four is not a metric
set. Claiming a check would repeat what ADR-002 records — a judgment-only fitness function citing a
layer that did not exist. The doc also names the cheapest honest way to start capturing escaped
defects (one line per delivery in `retrospective.md`) and deliberately does not build it.

**The KI records a divergence rather than applying it silently.** The wider practice treats
quarantine as a disposition an engineer applies; here it is approval gate #9, so an agent proposes
one with owner, expiry, cause and resolving evidence and a human applies it. That is what
`memory-trust-boundary.md` asks for when source material and a framework rule disagree — surface the
conflict, do not let a retrieved KI quietly contradict a gate.

**One KI, not two, and only one agent bound.** The taxonomy is operational: an agent triaging
failures acts on it, so it belongs in the retrievable corpus. The four numbers are for a human
reporting upward across a quarter and nothing an agent does mid-task depends on them — so they live
in the pattern doc, which is reachable by direct reference rather than retrieval, since
`docs/patterns/` is not a memory-registry source. `dx-engineer` is bound because build and flake
health is its remit; `qa-engineer` works per feature and has no use for a ninety-day trend.

**Not merged into `pipeline-retrospective` or `agent-scorecard`**, as this item required. Those
measure the pipeline — which agent is slowest, which loops most. These measure the suite. Merging
them produces one dashboard where flake rate sits beside code-reviewer p95 and neither means
anything.

1. **Problem**: `CLAUDE.md` gates on coverage ≥ 85% and `run-tests` enforces it. Coverage is the one
   number that cannot detect the failure these agents are capable of producing: repairing a vacuous
   test moves it by zero, retiring a dead test moves it *down*, and deleting an assertion leaves the
   line covered. Nothing here names flake rate, time to diagnose, escaped defects or suite
   wall-clock, and nothing names the pair — flake rate falling while escaped defects rise — that
   identifies a suite being made green by deleting evidence.
2. **Architectural Fix**: `docs/patterns/test-suite-health-metrics.md`, plus a KI for the flake
   taxonomy: four categories, the evidence distinguishing them, and four dispositions each carrying
   owner, expiry and the evidence that would resolve it.
3. **Target files**: `docs/patterns/test-suite-health-metrics.md` (new),
   `shared/knowledge/flake-triage-taxonomy.md` (new), `shared/memory-registry.json`
4. **Done when**: nothing. Judgment-only — and the doc says so in its own text.

**Why judgment-only, explicitly.** Three of the four numbers have no source loom can reach: escaped
defects live in a bug tracker, time to diagnose in ticket timestamps, flake rate in CI history. A
metric the framework cannot measure is a pattern doc, not a fitness function; claiming otherwise is
how ADR-002's judgment-only fitness function ended up citing a layer that did not exist.

**Not a skill, and not merged into an existing one.** The taxonomy wants a skill and cannot have one
yet: clustering ninety days of failures needs ninety days of failures in machine-readable form, and
nothing ingests CI history. A skill whose first step is "paste your CI JSON" is a prompt with
frontmatter. Revisit if ingestion ever lands. Equally, these are *suite* metrics —
`pipeline-retrospective` and `agent-scorecard` measure the *pipeline*, and merging them yields one
dashboard where flake rate sits beside code-reviewer p95 and neither means anything.

### L3.43 — The attributes a run does not record
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: L3.8 (shipped) · **Blocks**: L3.44 · *(raised 2026-09-13)*

**SHIPPED** 2026-09-14 — `gen_ai.response.model`, `gen_ai.response.finish_reasons`,
`loom.provider.terminal_reason`, and `loom.loop.<id>.terminated_by` on the run span.

**The premise was wrong in a way worth recording.** This item said `response.model` was missing. It
was not missing — it was **mislabelled**. `orchestrator.Usage.Model` is read from the envelope's
`modelUsage` keys, which is the model that *answered*, and `tracer.go` emitted it under
`gen_ai.request.model`. That is worse than absent: a reader comparing the attribute against a pinned
model would see the served model and conclude there had been no substitution. Renamed, and
`gen_ai.request.model` is now emitted by nothing.

**The honest limit, stated rather than engineered around.** loom passes no `--model`, so it never
expresses a request and has no source for a request model. Recording what served is therefore all
this can do: the trace says which model answered, never whether it was the one you wanted. Detecting
substitution needs an expressed intent, which is a change to how loom selects models rather than a
telemetry fix, and it was deliberately not made here.

**L3.15's capture settled the uncertainty this item inherited.** The real envelope carries
`stop_reason: "end_turn"` and `terminal_reason: "completed"` — so both attributes have a verified
source rather than an assumed one. They answer different questions and are namespaced apart: a
completion cut off at the token ceiling and one that finished its thought both terminate with
`completed`.

**A second defect found while building it.** `internal/telemetry/otlpjson.go` had no `ArrayValue`
case. Its comment was candid — slice attributes fall back to their string form, "and nothing this
package emits uses a slice attribute today" — and this item made that sentence false, since the
GenAI convention specifies `finish_reasons` as a list. A tool reading the convention would have
failed on a flattened string. The encoder now emits a real `ArrayValue`.

**Loop termination.** `converged` / `gate_approved` / `round_limit` / `error`, keyed by loop ID on
the run span. `gate_approved` was not in this item's plan: reading `closeLoop` showed a loop can also
be settled by a human having already approved its gate, which is a different fact from converging and
deserved its own name. The key count is bounded by the loops a plan declares, so cardinality stays
low however many rounds run. Both paths are tested through the real executor, not the tracer alone.

1. **Problem**: `internal/telemetry/tracer.go:48` records `gen_ai.request.model` — the model asked
   for — and never `gen_ai.response.model`, the model that answered. A provider serving something
   other than what was pinned is invisible. Also absent: `gen_ai.response.finish_reasons`, so a
   truncated completion is indistinguishable from a complete one; and any record of *why* a loop
   ended — L2.17 bounds the developer↔code-reviewer loop at three rounds, and a run that hit the
   bound looks in the trace exactly like one that converged.
2. **Architectural Fix**: emit `gen_ai.response.model` and `gen_ai.response.finish_reasons` wherever
   the provider envelope carries them — **L3.15** is the item that verifies those field names
   against the live CLI, and this inherits its uncertainty rather than assuming past it — plus a
   `loom.loop.terminated_by` on the stage span: `converged` / `round_limit` / `gate_halt` / `error`.
3. **Target files**: `internal/telemetry/tracer.go`, `internal/orchestrator/executor.go`,
   `internal/provider/`
4. **Done when**: a run that exhausts the review loop says so on a span, and a test fails if a
   required attribute stops being emitted.

That last clause is the source material's rule for instrumentation and this repository's rule for
fitness functions, and they are the same rule: a span that stops being emitted turns every assertion
about it green.

### L3.44 — Trace-shape assertions on loom's own runs
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L3.43 · **Blocks**: none · *(raised 2026-09-13)*

**SHIPPED** 2026-09-14 — `internal/orchestrator/shape.go` (an exported `ShapeRecorder` that
implements `Tracer`), the committed baseline at
`internal/orchestrator/testdata/shape-deliver-feature.json`, and three tests.

**The done-when, verified rather than asserted**: a stage added to the built-in plan was inserted,
the test failed naming it, and the baseline was restored. The failure prints the whole committed and
current shape plus the exact `-update` command, so a shape change arrives in review as a diff.

**What the built-in plan's shape actually is**: eight of fifteen stages run — the router skips seven
— the review loop converges, and seven model calls are made (eight stages minus the internal
router). That last number is the one a routing regression moves, and nothing recorded it before.

**The token-budget assertion this item called for was dropped, deliberately.** The mock reports the
usage a fixture hands it, so a ceiling over a mock run asserts the fixture rather than anything the
system decided — it would pass forever and catch nothing, which is exactly the vacuous assertion
L3.41 now has `code-reviewer` reject. Cost regression needs a real-provider run, which is a
different kind of test and not this one.

**Two limits, both in the code's own comment.** A shape records what RAN: a stage the router skips
is settled before its span opens and appears nowhere, so a change in *why* a stage was skipped, with
the executed set otherwise identical, is invisible here — the route in run state is where that
question belongs. And a mock run exercises the mock path; the first real run showed the mock
under-exercises this pipeline, so this catches structural drift, not routing on a real analysis.

**The `-update` flag exists, with the agent-goldens discipline attached**: hand-edited JSON drifts
from what the run emits, so the flag is the honest tool — and what stops it becoming
regenerate-until-green is that a failure prints both shapes and the commit that moves the baseline
has to say why. A second test asserts two identical runs produce identical shapes, so a baseline can
never contain something that varies run to run.

**One dead field removed before shipping.** `StageStep` briefly carried a `SkipReason`, written from
the span outcome — until the generated baseline showed eight stages rather than fifteen and
`IsStageSettled` turned out to short-circuit before a skipped stage's span opens. An unpopulated
field in a committed baseline would have read as "nothing was skipped for a reason".

1. **Problem**: nothing asserts the shape of a run. A router change that adds two stages, a retry
   storm that triples cost, a loop terminating by bound rather than convergence — each passes every
   test here so long as the artifacts validate. `agent-eval` grades one agent's output;
   `pipeline-retrospective` trends timings across deliveries; neither asserts that a known input
   produces a known trajectory.
2. **Architectural Fix**: extract a shape from a `--provider mock` run — stage sequence, stage count,
   `terminated_by`, required spans present, total tokens under a ceiling — commit it, assert it.
   Durations, trace IDs, span IDs and timestamps are excluded by construction: they change every run
   and are not the thing being protected.
3. **Target files**: `internal/telemetry/`, `internal/orchestrator/`, `tests/`
4. **Done when**: adding a stage to the built-in plan fails a shape assertion until the baseline is
   updated in the same commit.

**Two honest limits.** A mock-run shape protects the mock path, and the first real run showed the
mock systematically under-exercises this pipeline — 10 of 12 stages against the mock's 7 — so this
catches structural drift, not routing on a real analysis. And the failure mode to design against is
the baseline that moves with the code: a shape regenerated until green asserts only that current
behaviour is current behaviour. The discipline that holds is the one already used for agent goldens
— the diff shows old shape and new, and the commit says why the change was intended.

### L3.45 — Climb back to the coverage floor, and find out why it slid
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-14)*

**SHIPPED** 2026-09-14 — coverage back to **66.3%** (from 64.7% before this work began), the floor
raised to match, and the reporting defect fixed.

**The second defect was the point, and it is closed.** The ratchet and the embedding-example build
now carry `if: always()`. The ratchet always **reports** the number and **gates** only when the test
step passed: `go test` writes a coverage profile even when tests fail, so the number is always
available, but coverage across a failing run is not comparable to a clean one and failing on it
would be noise stacked on a failure that already has a cause. It prints `NOT GATED` and says why.
Twelve days of drift went unseen because a step that never ran also never said so.

**What the tests cover, and why these.** `internal/orchestrator`, as this item specified — the
executor holds the invariants the framework sells. `WouldInvalidateApprovals` had no test at all,
and the CLI calls it before recording an approval so it never writes one the same command is about
to destroy. Also covered: the policy dry-run context, the route and loop callbacks that tell a human
what is about to happen before a gate asks them to approve it, and the gate-halt message.

**Every new test was mutation-verified**, per `backfill-unit-tests` step 6 — raising a coverage
number with tests nobody has shown can fail is exactly what
`docs/patterns/test-suite-health-metrics.md` was written to name. Two findings from doing it:

- One mutation **survived**. The dry-run test does not fail when `cloneForInspection` is removed,
  because `WouldInvalidateApprovals` loads fresh state and never saves it, so an in-memory demotion
  is discarded and unobservable. The test does fail when the check **persists** what it inspected,
  which is the failure that matters. The comment now says precisely that instead of claiming the
  broader property — the clone is defence against a future caller that persists.
- One mutant was **ineffective and looked like a passing test**: a deferred mutation of a local in a
  function with an unnamed return never reaches the caller. Mutation testing needs its own mutants
  checked, or a bad mutant reads as a covered line.

**One test premise was wrong and the code was right**, which is worth recording given how often that
has been the other way round this week: a policy-context test asserted a review verdict would be
visible after the loop exhausted. It is not, because the run halts at `confirm-unresolved-review`
with `code-reviewer` in `WAITING_APPROVAL` — a stage that has settled nothing. The corrected pair
now asserts both directions, including that an unsettled stage contributes no facts, since an absent
fact resolves a policy check to UNKNOWN and a guessed one resolves it to a decision nobody made.

1. **Problem**: the ratchet floor was the measured 66.4% on 2026-09-02. By `a5ae65b` combined
   statement coverage was **64.7%** and nobody knew, because from 2026-09-10 the `go test` step
   failed first — the stage-timeout defect fixed in `ca8b9b2` — so the ratchet step never ran. A
   gate downstream of a failing gate reports nothing, and the thing it guards drifts in silence for
   as long as the first one stays red. The floor was re-baselined to **65.5** on 2026-09-14 by a
   human decision; this item is the climb back.
2. **Where the untested code is**, measured rather than assumed — 290 functions at zero coverage:

   | Package | Functions below 100% / total |
   |---|---|
   | `cmd/loom/cmd` | 133 / 246 |
   | `internal/orchestrator` | 91 / 205 |
   | `shared/mcp/internal/tools` | 64 / 93 |
   | `internal/state` | 53 / 201 |
   | `cmd/loom/internal/fs` | 50 / 58 |
   | `cmd/loom/internal/platform` | 45 / 62 |
   | `shared/mcp/internal/analyzers` | 41 / 52 |

   `internal/orchestrator` is the one to start with: it is the executor, it holds the gate and loop
   invariants the framework sells, and it is second-largest. `cmd/loom/cmd` is the biggest number
   and the least valuable per test — CLI wiring, mostly exercised end-to-end already.
3. **Architectural Fix**: raise coverage with `backfill-unit-tests` against the ranked list above,
   raising `COVERAGE_FLOOR` to each new measured value as it goes. Characterization mode, not
   coverage theatre — L3.41's mutation stopping condition applies, and a test that cannot fail
   raises the number while lowering the signal, which is precisely the failure `test-suite-health-metrics`
   (L3.42) exists to name.
4. **The second defect is the more important one.** A gate that silently stops reporting because an
   earlier step failed is the same class as an unemitted span turning every assertion about it green
   (L3.43's closing note). Either the ratchet runs independently of the test step's exit status, or
   something reports that it did not run. Fixing only the coverage number leaves the blind spot that
   let it slide for twelve days.
5. **Target files**: `.github/workflows/framework-ci.yml`, `internal/orchestrator/`,
   `shared/mcp/internal/tools/`, `internal/state/`
6. **Done when**: combined statement coverage is back above 66.3%, and a run whose tests fail still
   says what coverage was — or says out loud that it does not know.

**Why the floor moved at all.** `test-repair-contract.md` forbids an agent changing a CI gating
threshold to accommodate a failure, and that rule held here: the agent measured, reported that the
drift predated its own work, and stopped. A human made the call. Recording that because the contract
working as intended is easier to see in an example than in its own prose.


### L3.46 — Decide whether an exemplar edit halts a run
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L3.40 (shipped) · **Blocks**: none · *(raised 2026-09-14)*

**SHIPPED as a decision** 2026-09-15 — **exemplars are not gated**, recorded in
`shared/contracts/exemplar-contract.md` with the measurement behind it. No barrier was built.

This item required deciding with change-frequency evidence rather than before it, and said the digest
flag would produce that number. It did not have to be gathered prospectively: the exemplars are real
files with real history, so the question was answerable the same day by measuring backwards.

| | |
|---|---|
| Commits touching an exemplar's file | 10 |
| …that created the exemplar | 3 |
| …that added the exemplar annotation | 1 |
| **…that changed the file without touching the exemplar** | **4** |
| **…that changed an exemplar's body after it was declared** | **0** |

**The manifest addresses exemplars by file path**, so a barrier can only fire on file changes. On
this evidence it would have halted four runs for edits to neighbouring functions and none for an
actual exemplar change. A gate that halts wrongly every time it fires is one people learn to approve
without reading — worse than no gate, because it also spends the credibility of the gates that do
matter.

**The posture question this item framed turned out to be secondary.** It expected the argument to be
about reversing `posture.go`'s observe-don't-gate stance. The evidence moved it to granularity: at
file addressing there is nothing worth gating, and function-level addressing was already rejected in
L3.40 because finding function boundaries across six languages is worse than the duplication it
removes. The posture debate is still there to have, and it is not blocking anything.

**The workflow proved itself while this shipped.** The digest warning fired on `envelope_test.go` —
because `6a66be3` changed `TestDecoderMatchesARealResponse`, a different function in the same file.
`git log -L` confirmed the exemplar's own body untouched since creation, so the digest was
re-recorded. Warning → confirm → re-record, exactly the loop the design intends, and a live instance
of the false-positive class the decision rests on.

**What would reverse this**: an exemplar actually edited in a way that degraded it. That is the
signal to watch, and the digest warning is what surfaces it. Its text now says plainly that a file
digest cannot distinguish an exemplar edit from a neighbouring one — an honest noisy signal beats a
silent one, and an unexplained noisy signal gets muted.

1. **Problem**: an exemplar is declared, annotated, registered and audited — and an agent can still
   edit one. L3.40 shipped detection (a digest, and a flag when it changes), not protection. The
   question it deferred is whether editing an exemplar should halt the run.
2. **The mechanism already exists**, which is why this is a decision rather than a build: L2.24's
   `WorkTree.ChangedPaths()` lists every changed path, L3.30 already consumes it after every stage,
   and the exemplar manifest is a path list. Matching one against the other is small.
3. **What makes it a real argument**: `internal/orchestrator/posture.go:37-40` says the check
   "observes a stage rather than gating one", deliberately — L3.30's finding was that the unreviewed
   edits were *correct*. A barrier here reverses that posture for one class of file. That may well be
   right, since an exemplar an agent can quietly edit is not protected; but it is a different
   philosophy of enforcement from the one beside it, and the two should be reconciled out loud.
4. **Evidence to gather first**: how often does an exemplar legitimately change? The digest flag
   L3.40 ships produces exactly that number. Decide with it rather than before it.
5. **If it gates**, it is approval gate #9's shape, not a new one — a human deliberately accepting a
   change to something the project declared load-bearing.
6. **Target files**: `internal/orchestrator/posture.go`, `shared/rules/approval-gates.md`,
   `shared/contracts/exemplar-contract.md`
7. **Done when**: either a stage that edits an exemplar halts the run at a gate, or the contract
   records the decision not to gate and why — with the change-frequency evidence either way.


---

## Workstream: OBSERVE — Test Value over Test Ritual

Five items raised 2026-09-22 from a discussion of whether agent TDD earns its cost. Two ADRs carry
the decisions, both Accepted 2026-09-22: **ADR-008** defines "tested" by what the tests can catch, measured on the change, and
**ADR-009** retires the unit-level TDD ritual while keeping acceptance tests written from the spec.
The order is load-bearing: **L3.58–L3.60 build the replacement checks before L3.61 removes anything**,
so there is no window in which neither the ritual nor its replacement is in force.

Two facts shaped these items. The `loom` module's coverage floor is 66.3%, so a whole-codebase 85%
cannot gate here — the diff can. And this repository produced three checks in one week that passed
while testing less than they claimed (C1, A7, the C.1 drift script); every item below is proved red
before it gates.

### L3.58 — Gate coverage on the changed code at 85%
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: none · **Blocks**: L3.61 · *(raised 2026-09-22, ADR-008)*

1. **Problem**: The 85% rule is a whole-codebase number. Here it is a 66.3% ratchet described as
   aspirational; for an agent it is the easiest target to pad. New code is held to nothing specific.
2. **Architectural Fix**: Measure the fraction of executable statements added or modified by the change
   that the suite executes, against the merge base, and fail below 85%. Keep the existing whole-module
   ratchet as a separate one-way floor. Generated files and `_test.go` are excluded by an explicit list,
   not a pattern that can quietly grow. On a direct push to `main` the base is the previous commit,
   stated in the step so nobody mistakes it for PR behaviour.
3. **Target Files**: `.github/workflows/framework-ci.yml` (Go job), new `scripts/check-diff-coverage.sh`
   (or a Go helper under `tools/`), `scripts/ci-check.sh`
4. **Done when**: a change adding an untested exported function fails CI naming the uncovered
   statements; the same change with a test passes; the whole-module ratchet still runs and is unchanged.

### L3.59 — Mutation-test the changed code
**Workstream**: OBSERVE · **Effort**: L · **Blocked by**: none · **Blocks**: L3.61 · *(raised 2026-09-22, ADR-008)*

1. **Problem**: Mutation proof exists only as a manual step in `backfill-unit-tests` (step 6), run by a
   model, for characterization nets. Nothing measures whether new tests can detect a fault in new code.
2. **Architectural Fix**: Spike first — pick the Go tool by trying it on this module, not from a list.
   Candidates: `gremlins` (go-gremlins), `go-mutesting`; confirm maintenance and diff-scoping before
   adopting. Run it on the changed statements only. **Report-only** until a baseline exists across
   several changes; then set the floor from the measurement and ratchet it like coverage. Mutants that
   fail to apply or compile are listed separately and never counted as killed. Record the candidate tool
   per language in each `*-conventions.md` (StrykerJS, Stryker.NET, mutmut, PIT for Java and Kotlin,
   cargo-mutants, muter for Swift) as candidates to verify, not decisions.
3. **Target Files**: `.github/workflows/framework-ci.yml`, new `scripts/check-diff-mutation.sh`,
   `shared/rules/*-conventions.md`, `shared/skills/backfill-unit-tests/SKILL.md` (step 6 can call the
   same runner), `shared/skills/run-tests/SKILL.md`
4. **Done when**: a change whose new test executes but does not depend on a changed line reports that
   line's mutant as surviving; a mutant that does not apply is reported as not applied, never as killed;
   the floor is recorded with the measurements it came from.

### L3.60 — Reject tests that cannot fail
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: L3.61 · *(raised 2026-09-22, ADR-008)*

**SHIPPED** 2026-09-22 — `internal/testlint` walks the module and fails any test with no path to
`t.Error`/`t.Errorf`/`t.Fatal`/`t.Fatalf`/`t.Fail`/`t.FailNow`, following subtest closures and helpers
handed the testing value, across packages and import aliases, through recursion. The fitness function
is `internal/testlint/repository_test.go`; `permitted` is empty — all 600 tests in the module can fail.
A second test fails on a pin that no longer matches, so a fixed test cannot leave a dead excuse behind.

**What verifying the premise changed.** The "only asserts no error" half of the item is **not
decidable in plain Go** and was dropped. A survey flagged 22 tests whose every failure sat under
`if err != nil`, and each one inspected was a real assertion: `os.Stat(archived)` failing *is* "the
file was not archived", `json.Unmarshal` failing *is* "the output does not parse". No AST rule tells
that apart from "the code under test did not error". The repository uses no testify, so the
`NoError`-only form that is decidable has nothing to run on. Weak-but-asserting tests are **L3.59's**
job. ADR-008's third clause overstates what can be rejected mechanically; its fitness-function
section (a test function with no assertion) is what shipped.

**Proved red, and the proof is kept.** A fixture module under `internal/testlint/testdata/` pins
every edge — nine tests that must be flagged, nine that must pass, `TestMain` and a lower-case name
that are not tests — and the fixture test asserts the exact flagged set. Seven mutants of the checker
were run; **two first attempts proved nothing**: M2 (accept any receiver's `Error`) did not compile,
and its compiling rewrite *survived*, because the logger fixture called `slog.Default().Error` — a
call, not a named receiver. The fixture now uses a named logger and all seven are killed. The aliased-
import case was likewise vacuous until a helper that cannot fail was reached through the alias. Also
proved on the real module: a planted assertion-free test and a stale pin both fail. Deliberately
lenient: a method, or a helper outside the module, handed `t` is assumed able to fail.

1. **Problem**: A test with no assertion — or whose only check is that an error is nil — passes
   coverage and review alike. `code-reviewer`'s "would it fail?" catches it only when someone looks.
2. **Architectural Fix**: An AST check over `_test.go` files, in the style of
   `internal/state/untyped_test.go`: every `Test*` function must reach a failing call (`t.Error*`,
   `t.Fatal*`, `testify` `assert`/`require`, or a helper that does). A test whose only assertions are
   `NoError`/`NotNil` is flagged. Allowlist by name, with a reason, pinned by a test that forces the
   justification to widen it — the same pattern as the safe-argument allowlist.
3. **Target Files**: new `internal/testlint/` (or a `_test.go` fitness function at module root)
4. **Done when**: a planted assertion-free test fails the check, a planted `NoError`-only test is
   flagged, and the current suite passes with an allowlist whose every entry states why.

### L3.61 — Retire the unit-level TDD ritual and `test-driven-developer`
**Workstream**: PLATFORM · **Effort**: M · **Blocked by**: L3.58, L3.59 (report-only is enough), L3.60 · **Blocks**: none · *(raised 2026-09-22, ADR-009)*

1. **Problem**: "ALWAYS practice TDD — Red-Green-Refactor" is a rule whose reason does not hold for an
   agent that writes both sides, and the mechanisms contradict each other: `developer.md` says both
   "write the failing test first" and "Do NOT write test files"; `TDDWorkflow` gives RED to
   `unit-tester`, which does not follow the Three Laws, and audits it with `tool-validator`, which
   audits skill files. Nothing fails today on a reference to a removed agent or workflow.
2. **Architectural Fix**: (a) Add a check that fails on any reference to an agent, skill or workflow
   that does not exist, proved red against a planted reference. (b) Replace the TDD rule in
   `testing-conventions.md` and `CLAUDE.md` with ADR-008's definition of done; keep the Three Laws in
   `testing-pyramid.md` as a technique. (c) `developer` owns its unit tests; remove the contradicting
   line. (d) Remove `test-driven-developer` outright — no deprecation release, no alias (decided
   2026-09-22) — with a CHANGELOG entry and release notes naming `developer` as the replacement. (e) Remove `tdd-workflow.md` and
   its flat `tdd-state.json`, which also closes C.2's second finding. (f) `deliver-atdd` Phase 3 invokes
   `developer`. (g) Regenerate configs; the drift check from C.1 fails until this is done.
3. **Target Files**: `shared/rules/testing-conventions.md`, `CLAUDE.md`, `shared/agents/developer.md`,
   `shared/agents/test-driven-developer.md`, `shared/workflows/tdd-workflow.md`,
   `shared/skills/{deliver-atdd,orchestrate,bootstrap-project,backfill-unit-tests,run-tests}/SKILL.md`,
   `shared/blueprints/*.md`, `shared/orchestration/*.md`, `docs/patterns/testing-pyramid.md`,
   `cmd/loom/internal/platform/content.go`, generated configs, `shared/agents/CHANGELOG.md`
4. **Done when**: the dangling-reference check is green with no exceptions added; no rule requires
   Red-Green-Refactor; `developer.md` has one answer to who writes unit tests; the Training repo has a
   backlog item to teach ADR-008's definition of done in place of agent TDD — the curriculum follows
   the framework.

### L3.62 — Keep acceptance authoring blind to the implementation
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-22, ADR-009)*

1. **Problem**: The independence worth keeping is that acceptance tests come from the acceptance
   criteria, not the code. `deliver-atdd` gets this by order — scenarios are written before
   implementation. In `deliver-feature`, `qa-engineer` runs after `developer` and reads
   `implementation-notes.md`, so its tests can describe what was built rather than what was asked for.
   Unverified: which of `qa-engineer`'s current inputs under `loom run` include implementation artifacts.
2. **Architectural Fix**: First establish the fact — list each stage input `qa-engineer` receives under
   `loom run`. If the acceptance-scenario step can see implementation artifacts, split the step: author
   scenarios from the `AnalysisState` projection (L2.10's `QAAcceptanceInput` already exists) before or
   independently of the implementation, then automate them after. Under the markdown pipeline this is
   judgment-only and says so.
3. **Target Files**: `internal/state/projection.go`, `internal/orchestrator/` (plan and stage inputs),
   `shared/agents/qa-engineer.md`, `shared/skills/deliver-feature/SKILL.md`
4. **Done when**: a test asserts that the scenario-authoring stage's input contains no implementation
   artifact, proved red by adding one.

---

## Workstream: OBSERVE — Curriculum Alignment

Eleven items: ten raised 2026-09-15 from a **two-way alignment audit** of loom against the Zero to Agent
SDET curriculum: every major principle checked against what loom actually *enforces*, not what it
documents. The audit and its running record live in
[`docs/prompts/loom-alignment-todo-2026-09-16.md`](../prompts/loom-alignment-todo-2026-09-16.md).

Two findings about the audit itself are worth more than any single item. **Six of the eleven items
were wrong as written** — right about where to look every time, wrong about the fix roughly half the
time — so each entry below records what verifying the premise changed. And **three checks written
during the work passed while testing less than they claimed**, each caught only by deliberately
breaking it. Prove red; a green new check is evidence of nothing.

All eleven shipped between 2026-09-16 and 2026-09-18. `health-check` went 321 → 367 passing, 0
failing. **L3.57 was not in the audit** — it was found by trying to satisfy L2.19's stop condition
and discovering the evidence had been discarded at archive time, which is the kind of finding only
an experiment produces.

### L3.47 — Gate #9 is always human in code, not only in prose
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: **L2.19** · *(raised 2026-09-15)*

**SHIPPED** 2026-09-16 — `2859f59`. `GateTestRemoval` and an `alwaysHuman()` entry in
`internal/policy/gate.go`, plus the stale "eight gates" comment and `approval-gates.md`'s
self-contradicting "those five gates" (its own table marked six).

**Why it blocked L2.19**: that item rests its safety argument on always-human gates being
unreachable. Gate #9 was unreachable only *by accident* — it had no `GateID`, so a policy naming it
was rejected as **`unknown gate`**, the message a typo gets. The hazard was the repair: that message
invites whoever reconciles the two lists to add gate #9 to `eligible()`, and L2.19 is the item that
would then consume it. A test removal auto-approved by policy is the one-way door gate #9 exists to
hold.

**Verified**: removing the `alwaysHuman()` entry made both tests fail with the `unknown gate`
message — the finding verbatim. `TestRemovingTestCoverageIsRejectedAsAlwaysHumanNotAsUnknown`
asserts the rejection is *not* `unknown gate`, which is the distinction that matters.

### L3.48 — Assert the approval-gate rule and the code agree
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: L3.47 · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-16 — `f7aaf29`, `health-check.sh` section `7d`. This is the check whose absence
let L3.47's drift happen in silence. Parses every `### N.` gate and its Policy-eligible line from
the rule, then cross-checks against `gate.go`.

**The gate-number → `GateID` map is pinned, not parsed**: only three gates carry a `Policy gate ID:`
line, and adding one to the other six costs ~180 bytes of a core rule with 206 left. Same trade as
`TEST_WRITING_AGENTS` above it.

**Verified** by four mutants, each restored. **New failure mode worth knowing**: adding a tenth gate
to `approval-gates.md` now fails the build until it is pinned — deliberate, but a new way for a rule
edit to break CI.

### L3.49 — A characterization net says what it does not cover
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-16 — `e4862ba`, `unit-tester` 1.5.0 → 1.6.0. Step 8 requires a scope note in
characterization mode: records-behavior-as-of date, an explicit `NOT COVERED` list, and the three
behavior-carrying lines.

**The item's own fix was wrong.** It proposed a `## Not Covered` section in the report — but
`.claude/feature-workspace/` is gitignored, so a note written only there dies with the workspace and
the net ships to the repository recording nothing. It goes in the **test file**.

**Second defect the audit missed**: step 8 already required naming three behavior-carrying lines
while the output template had **no section for them** — an orphaned instruction. They now live in
the scope note, which is also what `backfill-unit-tests` step 6 mutates.

**Enforcement**: judgment-only with a reason — `health-check` cannot see test files in the projects
this agent writes for.

### L3.50 — `tools` describes what an agent can actually do
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-16 — `b6823f3` (seven agents to 2.0.0) and `0255eb7` (`health-check` section
`7e`).

**The diagnosis was backwards in the audit.** It read as over-permission — "read-only reviewers hold
`Bash`". The opposite is true: **all seven are required to produce a markdown artifact and none
declared `Write`**, so `Bash` was the undeclared write channel the markdown pipeline runs on.
Dropping it would have broken the pipeline. Three go further and are instructed to modify source
with tools they do not hold. Both reviewers have carried exactly `Read, Glob, Grep, Bash` since
their first commit — `Write` was never removed, it was never there.

**This closes L3.36's mechanism question** — see that item.

`Write` declared on all seven, `Edit` on the three told to fix source (removing their write channel
would have decided L3.36 by the back door), `Bash` dropped from the five with no execution use.
**No prompt behavior changed.** The contract, pattern doc and schema stopped claiming `tools`
"enforces capability boundaries", which was false for all seven.

### L3.51 — A review must answer whether its tests would fail
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-16 — `634b4ba`, `code-reviewer` 2.0.0 → 2.1.0.

`review-contract.md` always listed `## Test Design Review` as required and `validate-artifact`
checks the heading — but `ReviewState.TestDesignReview` was `omitempty` and absent from `Validate()`,
**and an empty list renders as the word "None"** (`render.go:110-118`). So a run under the executor
could emit a report that satisfied the markdown contract and answered nothing: the vacuous-assertion
shape L3.41 exists to reject, arriving one level up.

**The conditional variant was checked and dropped**: requiring it only when the diff touched tests
is not available to a pure `Validate()` on `ReviewState`, and `FilesModified` is agent self-reported
rather than measured. Required unconditionally, which is what the contract already said.

### L3.52 — Inner layers import only inward
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-17 — `8613861`, `internal/orchestrator/layers_test.go`. Guardrail #1's fitness
function, generalizing `boundary_test.go`'s transitive `go list` pattern. `state`, `policy`,
`worktree` reach nothing; `orchestrator` reaches `state` and `policy` only. Adapters deliberately
unpinned.

**Equality, not a ceiling** — a stale pin fails too, which is how a guardrail stops becoming
decoration.

**Honest scope**: the Go compiler already rejects any inversion that closes an import cycle (the
first mutant proved the compiler, not the test). What it cannot catch is an inner layer importing a
leaf-shaped adapter, or reaching one through a helper — the transitive mutant is what justifies the
test existing.

### L3.53 — Least privilege on egress is a separate question
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L3.50 · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-17 — `79abb17`, `security-reviewer` 2.0.0 → 2.1.0, `health-check` section `7f`.

Least privilege on *data* is not least privilege on *egress*, and the framework only had the first.
`security-patterns.md` grouped `Write`/`Edit`/`Bash` as one "can cause damage" axis — right about
damage, silent about disclosure. The new pattern names the trap the data axis lacks: **egress does
not require a tool that looks like egress.** A rendered markdown image URL fetches for whoever
controls it, with no network call in the code.

**Verified**: no agent declares `WebFetch` or `WebSearch`, so the entire egress surface is `Bash` —
both axes at once.

**`7f` caught its own first version.** The regex required "read-only counter agent"; `memory-auditor`
says "Read-only counter to the memory-engineer skill", so it was silently excluded while the section
printed **all PASS over 12 of 13 agents**. It now carries a coverage floor, and shrinking the derived
set fails loudly.

### L3.54 — The quarantine-expiry job is the project's, and loom says so
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-17 — `f089936`.

**The item proposed a `health-check` section or a `scheduled-monthly.yaml` entry; the answer was
neither.** A quarantine lives in the test file of the project whose suite is quarantined.
`health-check.sh` runs against this repository; `loom health` verifies an *installation*. Neither
reads a user's tests, so neither can see an expiry.

**It also settled what `scheduled-monthly.yaml` is.** `shared/hooks/README.md` always said nothing
dispatches hooks — the executor is **L3.10** — but both hook files contradicted it ("Enable
individually per project need", "set `enabled: true` to activate"). There is no switch. Both headers
now say `enabled:` is the project's runner's flag, not loom's.

`flake-triage-taxonomy.md` now says the job is the project's to build, why loom cannot, and ships a
copyable ~20-line one. **Verified by running it** against an expired quarantine, one dated 2027, and
a clean file: it flagged exactly the expired one and exited 1, then exited 0 once removed.

### L3.55 — Bound the `any`/`interface{}` rule to what it can honestly forbid
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-17 — `91777a0`, `internal/state/untyped_test.go`.

`go-conventions.md` said "NEVER use `any` or `interface{}`" while loom's own Go used them 49 times.
A rule violated that often knowingly teaches that rules here are decorative.

**The audit's count of 68 was wrong.** It matched English prose — error strings, the policy YAML key
named `any`, and the embedded TypeScript rule text in `generated_rules.go:32` ("never use raw any
types"). The rule's own words counted as a violation of itself.

**None of the 49 is the defect the rule targets**: JSON Schema literals (24), marshal/reflect helpers
(8), heterogeneous dispatch (9), MCP wire maps (5), variadic `slog` (3). `StageSchema.subject` exists
to *feed* a fitness function (L2.25); the logger mirrors `slog`'s own signature.

The rule now permits four named boundaries and forbids the thing it means. The fitness function
asserts only the clause judgement cannot soften: **an untyped value must not travel inward.**

**Its first version was vacuous and passed** — it matched only `*ast.Ident`, so it saw `any` but not
`interface{}`, the spelling actually in the code, which made both pinned exceptions dead code.

### L3.56 — Name the two disciplines loom relies on but cannot check
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: L3.50, L3.53 · **Blocks**: none · *(raised 2026-09-15)*

**SHIPPED** 2026-09-18 — `20c5260`. Both judgment-only, marked with a reason.

**"Could not reproduce" is not a closure state.** It describes one attempt, and closing on it turns a
defect into a rumour. The evidence to settle most of these already existed and was never pointed at
the question: the run's shape, the route with per-stage skip reasons, loop termination,
`gen_ai.response.model`, `loom memory runs/retries/corrections`, trace IDs.
`observability-patterns.md` now tabulates which surface answers which question, then states three
norms — a report without a run reference gets one question rather than a triage debate; when the
evidence is genuinely absent the instrumentation gap *is* the finding; and a diagnosis says what it
ruled out.

**Tool scoping is two problems.** For tools you define, narrow the parameter to a value — loom's
seven MCP tools take no language and cause no outbound effect. For tools you consume, you only
choose who holds them: `Bash` comes from the host platform's vocabulary and loom cannot narrow it.
So the control there is which agents hold it (L3.50, L3.53). That is a real control and a **weaker**
one than a narrow tool, and `security-patterns.md` says so rather than implying otherwise.

### L3.57 — Archive the typed stage documents with the run
**Workstream**: OBSERVE · **Effort**: S · **Blocked by**: none · **Blocks**: **L2.19** · *(raised 2026-09-18)*

**SHIPPED** 2026-09-18 — `7c5183e`, `internal/memory/workspace.go`.

`ArchiveRecords` copied `run-state.json` and `run-events.jsonl`. Those record that a stage completed
and which *kind* of document it produced; the document holds the review verdict, the security
findings, the test results and the changed paths — everything `gateContext` reads. So every finished
run was unanalysable for exactly the questions its records exist to answer.

**Found by the L2.19 experiment**, which is the only reason it surfaced: four sourceable policy facts
resolved UNKNOWN against a recorded run. `completedStageOfKind` succeeded and `ReadFile` failed, and
an absent fact is indistinguishable from one that never existed. See
[`docs/audits/l219-policy-dry-run-2026-09-18.md`](../audits/l219-policy-dry-run-2026-09-18.md).

**Verified end to end**: a mock delivery through the first gate archives `state/analyst.json`,
`state/context-engineer.json` and `state/router.json` beside the two files. Before, neither.

**This is prerequisite 1 of 3 for L2.19** and does nothing for the nine runs already recorded —
their documents are gone. Evidence starts accumulating from the next run.

---

# MILESTONE 3 — Level 4: Self-Learning Agentic Ecosystems

*Self-reflection, continuous learning, adaptation to novel constraints without code deployment.*

## Workstream: KERNEL — Self-Reflection & Error Recovery

### L4.1 — Bounded Reflexion with isolated context
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: M0.4 · **Blocks**: L4.4 · *(audit H1)*

1. **Problem**: Two defects compound. First, `deliver-feature/SKILL.md:102` (step 21): on
   `CHANGES REQUESTED`, "repeat from step 18 **until APPROVED**." The text explicitly scopes
   `maxContractRetries` to the *structural* check and calls the qualitative verdict "independent of
   structural check" — so the one loop that actually oscillates has no ceiling, no backoff, no
   progress detector, and no cost budget. That is `architecture-guardrails.md` #5 violated in the
   flagship pipeline. Second, where a bound does exist (`maxRetries: 3` at
   `pipeline-schema.md:26,36`), the retry protocol at `audit-composition-pattern.md:68-72` re-invokes
   the producer with **only the latest findings** appended — no isolation from the failed attempt's
   framing, and no accumulation across attempts. Retrying a degraded context with the same framing is
   how a hallucination spiral starts.
2. **Architectural Fix**: A bounded Reflexion loop in the executor combining both fixes. Max
   attempts; per-attempt output hashing with a no-progress detector (two semantically near-identical
   attempts ⇒ escalate, never retry). On failure, spawn a **fresh, isolated** analysis session whose
   only job is to diagnose the error and write a structured `Reflection` record — preventing context
   contamination from the failed attempt. Attempt N+1 receives *accumulated* reflections, not just
   the latest report.
3. **Target Files**: `shared/skills/deliver-feature/SKILL.md:102`,
   `shared/orchestration/audit-composition-pattern.md:68-72`,
   `shared/orchestration/pipeline-schema.md:26`, new `internal/orchestrator/reflexion.go`
4. **Done when**: an agent producing near-identical output twice escalates instead of retrying, and
   the escalation cites both attempts.

### L4.2 — Error taxonomy and recovery strategies for novel failures
**Workstream**: KERNEL · **Effort**: L · **Blocked by**: L2.5, L3.1 · **Blocks**: none

1. **Problem**: The entire error-handling policy is one line in `orchestrate/SKILL.md`: "On any
   unhandled error in a stage: checkpoint current state, surface the error, and stop." No
   classification, no recovery-strategy selection, no fallback, no degraded mode, no escalation
   ladder. Every novel error is a full stop requiring a human — the definition of *not* L4.
2. **Architectural Fix**: Error taxonomy (`Transient | Contract | Capability | Resource | Novel`)
   with per-class strategies: retry-with-backoff, re-plan via the router, decompose-and-retry,
   substitute agent, escalate. Unclassified errors route to a reflection step that attempts
   classification before escalating, and the outcome is recorded so the taxonomy improves.
3. **Target Files**: `shared/skills/orchestrate/SKILL.md`, `shared/orchestration/interface.md`,
   new `internal/orchestrator/recovery.go`
4. **Done when**: a transient tool failure recovers without human intervention and a capability gap
   triggers a re-plan rather than a halt.

### L4.3 — Global budget governor
**Workstream**: KERNEL · **Effort**: M · **Blocked by**: L3.8 (shipped), L2.16 (shipped) · **Blocks**: none

**UNBLOCKED** 2026-09-02. It inherits both halves it was waiting for: per-stage token counts and
cost on every `StageRecord` with `RunState.TotalUsage()` summing them (epic 84), and a policy
evaluator that runs as code with a working kill-switch (epic 87). Neither needs a metrics pipeline
or a collector — a governor reads the same numbers `loom state show` prints.

One property to inherit rather than re-litigate: a budget check that cannot see a number must not
treat it as zero. Epic 87's evaluator resolves an unanswerable condition to **unknown**, and a
governor deciding to halt on absent usage data has the same shape of problem.

1. **Problem**: No token ceiling, no dollar ceiling, no wall-clock ceiling, no per-run cap of any
   kind. `maxDiffLines` and `maxContractRetries` in `.claude/delivery-policy.yaml` are the only
   numeric limits and neither bounds spend. Combined with the unbounded loop in L4.1 and doubled
   invocations from synchronous audits, a single stuck delivery burns until a human notices.
2. **Architectural Fix**: A `BudgetGovernor` seeded per run (tokens, dollars, wall-clock, tool
   calls), decremented from real OTel usage data, enforcing soft-warn then hard-halt. *(As of epic
   84 the usage signal exists and is readable in-process: every stage's reported tokens and cost are
   on its `StageRecord`, and `RunState.TotalUsage()` sums them. A governor needs no metrics pipeline
   and no collector — it reads the same numbers `loom state show` prints. L2.16 remains the real
   blocker.)* Budget
   exhaustion is a first-class terminal state that checkpoints cleanly and reports what was consumed
   where.
3. **Target Files**: `.claude/delivery-policy.yaml` schema,
   `shared/orchestration/policy-evaluator.md`, new `internal/orchestrator/budget.go`
4. **Done when**: a run configured with a $1 budget halts cleanly at $1 and reports per-stage spend.

## Workstream: OBSERVE — Prompt & Tool Evolution

### L4.4 — Prompt registry with eval-gated promotion
**Workstream**: OBSERVE · **Effort**: XL · **Blocked by**: L3.5, L3.11, L4.1, L4.5 (shipped) · **Blocks**: L4.6

**Inherits from L3.5** (epic 88): that corrective signal is now durable and queryable across runs
rather than per-workspace. `loom memory corrections` ranks agents by how often a human corrected
them, which is the recurrence bar this item needs — one correction is an anecdote, and the store is
what turns a set of anecdotes into a count. `loom memory retries` does the same for stages that
needed several attempts.

**Inherits from L4.5** (epic 85): a labelled corrective signal that already exists per run —
`artifact.corrected` entries in run state and on the timeline, each naming the producing agent and
carrying a unified diff of what a human thought the output should have said. That is the training
signal this item's prompt-variant generation was specified to need, and it no longer has to be
inferred from gate ownership.

Two properties to design around rather than discover. The correction is **advisory**: the human's
text was recorded, not adopted, so the delivered artifact does not contain it. And one correction is
one person's judgement — promotion must stay eval-gated and recurrence-based, which is what this item
is for.

1. **Problem**: "Self-learning" today is `learning-engine` scanning retrospectives and writing a
   markdown proposal to `.claude/feature-workspace/proposed-lessons.md` for a human to approve into
   `docs/lessons-learned/`. It never touches an agent prompt. Agent prompts are static markdown
   edited by hand, versioned by hand, enforced by `scripts/check-agent-versions-ci.sh` requiring a
   semver bump. There is no mechanism — not even a manual one — for the system to propose, test, and
   adopt a prompt change.
2. **Architectural Fix**: Prompts become versioned artifacts in a registry with multiple live
   variants per agent. A candidate variant is generated from accumulated reflections (L4.1) and
   correction signals (L4.5), evaluated by the harness (L3.11) against committed baselines, and
   promoted only on a statistically meaningful win. Champion/challenger with automatic rollback on
   regression.
   > **Conflict resolved**: `agy.md` proposed a background meta-learning agent that "rewrite[s] agent
   > `.md` prompt templates or tool schemas programmatically." This plan deliberately gates that
   > behind evaluation rather than allowing direct autonomous rewriting. Unmediated self-modification
   > of prompts with no eval gate and no rollback is how a fleet silently degrades. The autonomy
   > `agy.md` wanted is preserved — generation is automatic — but promotion requires a measured win.
3. **Target Files**: `shared/agents/*.md` → registry-backed, `scripts/run-agent-evals.sh`,
   `scripts/check-agent-versions-ci.sh`, `shared/skills/learning-engine/SKILL.md`,
   new `internal/prompts/registry.go`
4. **Done when**: a challenger variant beats champion on the eval suite and is promoted with no human
   edit to a `.md` file.

### L4.5 — Capture the human-correction signal the schema already describes
**Workstream**: OBSERVE · **Effort**: M · **Blocked by**: L2.13, L2.14, L3.8 — all shipped · **Blocks**: L4.4

**SHIPPED** 2026-09-01 (epic 85, `b818edb`…) — the executor retains what a human was shown at each
gate, diffs it at approval, and records `artifact.corrected` on `run-events.jsonl` and in run state,
attributed to the **producing stage and agent** with a unified diff under
`.approved/<gate>/corrections/`. Visible in `loom state show` and `loom state timeline`.

**What building it changed about the design.** The signal is taken from the **rendered view**, not
the tracked artifact, and that is the finding rather than a compromise. Two facts forced it:

1. A human edit to a tracked artifact **does not survive**. L2.12 marks the stage STALE and re-runs
   it, and the agent's fresh output overwrites the human's text.
2. For the seven typed artifacts the tracked file is `state/<stage>.json`, while `analysis.md` is a
   derived view that is deliberately not digest-tracked (L2.9). So the file a person would actually
   open produced **no signal at all**, and hand-editing the JSON produced a signal about an edit the
   executor immediately discarded.

The property that makes a view safe to edit is what makes it the right channel: the executor never
reads it back, so a human can write a correction there with nothing to corrupt and nothing to
discard. A view edit is **advisory** — the pipeline does not adopt it, and the next render overwrites
the file. What survives is the record, which is what a learning signal needs.

Two invariants keep the signal from becoming fiction, both held by tests: reporting a correction
refreshes the baseline so nothing is reported twice, and the executor refreshes any baseline holding
a stage whose output *it* just wrote — without the second, a stage re-running after an edit would
have its own second attempt attributed to a person.

**Not done here, deliberately**: `events.jsonl`, the `gate_decision` event type and its emitter
(**L3.9**); the episodic store this belongs in long-term (**L3.5**); anything that consumes the
signal to change an agent (**L4.4**); and emission from the markdown pipeline, whose checksums are
the model's own.

1. **Problem**: `event-schema.md:131` defines `edited_then_approved` — "the human edited the artifact
   (checksum changed between gate-presented and gate-approved) … the corrective-signal case
   `extract-lessons` and `retrospective` mine." This is the single highest-value learning signal in
   the design, it is precisely specified, and **it is emitted by nothing**. No `events.jsonl` exists.
   The framework describes its own reward signal and never collects it.
2. **Architectural Fix**: Executor-side digest comparison at every gate (already required by L2.14),
   emitting `edited_then_approved` with a structured diff of what the human changed. Store in
   episodic memory as labelled training signal keyed to producing agent and stage. Feed into
   prompt-variant generation.
3. **Target Files**: `shared/telemetry/event-schema.md:112-144`,
   `shared/skills/deliver-feature/SKILL.md`, `shared/skills/extract-lessons/SKILL.md`,
   `internal/orchestrator/gate.go`
4. **Done when**: editing an artifact at a gate produces a stored diff attributable to the producing
   agent.

### L4.6 — Close the loop: lessons must reach prompts automatically
**Workstream**: MEMORY · **Effort**: L · **Blocked by**: L3.6, L3.10, L4.4 · **Blocks**: none

1. **Problem**: `learning-engine` → `promote-memory` → `create-ki` → `memory-engineer` →
   `forgetting-engine` is a five-skill chain terminating in a markdown file a human must read.
   Nothing consumes lessons-learned at runtime except a human deciding to edit a rule file.
   `forgetting-engine`'s expiry pass is itself an LLM invoked monthly by a hook that has no executor.
2. **Architectural Fix**: Lessons become structured records with a scope (`agent | stage | global`),
   injected as retrieved context to the relevant agent at run time via the retriever rather than
   copied into prompt bodies by hand. Promotion to a permanent prompt change happens only through the
   eval-gated registry (L4.4). Retrieval hit-count drives expiry, replacing the LLM forgetting pass.
3. **Target Files**: `shared/skills/learning-engine/SKILL.md`,
   `shared/skills/promote-memory/SKILL.md`, `shared/skills/forgetting-engine/SKILL.md`,
   `shared/rag/retriever.interface.md`
4. **Done when**: a lesson recorded in run N measurably changes agent behavior in run N+1 with no
   human edit.

## Workstream: PLATFORM — Open Ecosystem Integration

### L4.7 — Build an MCP client runtime
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: L2.4, L2.6 · **Blocks**: L4.8

1. **Problem**: `cmd/mcp-server/main.go` and `register/register.go` expose framework tools *outward*.
   There is no MCP client anywhere in the codebase. `loom` cannot consume a third-party MCP server;
   it delegates that entirely to the host IDE. The framework's own agents cannot use external tools
   except through whatever the host provides — there is a tool *export*, not a tool ecosystem.
2. **Architectural Fix**: An MCP client runtime in the executor: server discovery from config,
   capability negotiation, tool namespacing to avoid collisions, per-server auth, timeouts, circuit
   breakers (reusing L2.6). External MCP tools populate the same registry as native ones, so agents
   cannot tell the difference and `tools:` declarations become portable. The Router (L3.1) can then
   delegate sub-tasks to third-party servers.
3. **Target Files**: new `internal/mcp/client.go`, `shared/mcp/internal/server/tool_provider.go`,
   `shared/schemas/agent-frontmatter.schema.json`
4. **Done when**: a loom agent successfully calls a tool on a third-party MCP server with no host IDE
   involved.

### L4.8 — Break the Anthropic lock in the agent contract
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: L3.2, L4.7 · **Blocks**: none · *(audit H5)*

1. **Problem**: `agent-frontmatter.schema.json` declares `tools` as a closed regex over Claude Code's
   built-in names — **the framework's own MCP tools `analyze_complexity`, `search_ki`, and
   `search_docs` are literally unrepresentable in agent frontmatter.** `model` is
   `^(inherit|claude-[a-z0-9-]+)$`, so a non-Anthropic model is a schema violation.
   `shared/model-defaults.yaml` confirms the consequence: `claude_code` has real tier mappings;
   **all six other platforms are `null` across all three tiers**, with `roo_code` and `cline` marked
   "TBD — depends on Epic 42 landing." The framework ships installers for nine platforms and portable
   model selection works on one.
2. **Architectural Fix**: Replace the tool regex with a namespaced identifier pattern
   (`builtin:Read`, `mcp:<server>/<tool>`) validated against the generated tool registry. Replace the
   `model` regex with `model_tier` plus a provider-resolution table, and add a provider adapter layer
   so `light|default|heavy` resolves on Bedrock/Vertex/OpenAI/local. Fill in or explicitly deprecate
   the `null` rows.
3. **Target Files**: `shared/schemas/agent-frontmatter.schema.json`, `shared/model-defaults.yaml`,
   `shared/platform-registry.json`, `scripts/resolve-model-tier.py`, all 40 `shared/agents/*.md`
4. **Done when**: an agent declares an MCP tool and a non-Anthropic model, and both validate.

### L4.9 — Publish agent cards for external interop
**Workstream**: PLATFORM · **Effort**: L · **Blocked by**: L3.2, L2.8 · **Blocks**: none

1. **Problem**: There is no agent-to-agent interoperability of any kind — no A2A, no agent card, no
   invocation endpoint, no capability advertisement. `register.FrameworkTools` is the only
   integration path and requires a foreign system to be written in Go and import the module. That is
   language-level embedding, not ecosystem participation. Interop is currently: "be a Go program."
2. **Architectural Fix**: Generate agent cards from the capability registry (L3.2) — identity,
   skills, input/output schemas, auth requirements — and expose loom agents over a networked
   invocation endpoint alongside the MCP HTTP transport (L2.8). Track the A2A protocol as the likely
   standard; the registry work is the prerequisite either way and is not wasted if the standard
   shifts.
3. **Target Files**: new `internal/interop/agentcard.go`,
   `shared/schemas/agent-frontmatter.schema.json`, `shared/mcp/cmd/mcp-server/main.go`
4. **Done when**: an external agent framework discovers and invokes a loom agent over the network.

---

# MILESTONE 4 — Cleanup (opportunistic, unblocked)

These have no dependencies and can be picked up by anyone at any time.

### C.1 — Stop committing 532 KB of generated prompt payload
**Workstream**: PLATFORM · **Effort**: S · *(audit H11)*

**SHIPPED** 2026-09-22 — `scripts/check-generated-drift.sh` regenerates every platform
config into a temp directory and fails on a byte difference (DRIFT), a generated file that is not
committed (MISSING), or a tracked file in a generator-owned directory the generator no longer emits
(ORPHAN). Took the done-when's second branch: the root copies are read by `install.sh`,
`test-install.sh`, `health-check.sh` and `check-parity.sh`, so untracking them is not an S. Proved red
four ways — a hand-edit to `.windsurfrules` (the H11 case), a `shared/rules/` change left
unregenerated, a missing file, an orphan — and green on `ubuntu:24.04`, byte-identical to macOS.
**The first version was vacuous on DRIFT**: it piped into the checking function, so the count ran in
a subshell and the script printed DRIFT then exited 0. Only the red proof caught it. Wired into the
`check-parity` job of `framework-ci.yml` and into `ci-check.sh` (gate #7 granted). Sizes have grown since the audit: 82 KB each for the identical pair, 262 KB
`.roomodes`.

1. **Problem**: `.cursorrules` and `.windsurfrules` are byte-identical (`c662f42f…`, 76 KB each).
   `AGENTS.md` and `.openai.md` have drifted apart. `.roomodes` is 233 KB. That is ~20k tokens of
   static rules prepended to every request on those platforms before any agent does any work — a
   fixed per-call tax multiplied across every stage.
2. **Architectural Fix**: Generate at install time from `shared/` rather than checking artifacts in.
   If they must be committed for editor discovery, add a CI drift check so identical files cannot
   silently diverge.
3. **Target Files**: `.cursorrules`, `.windsurfrules`, `AGENTS.md`, `.openai.md`, `.roomodes`,
   `scripts/generate-configs.sh`, `scripts/check-parity.sh`
4. **Done when**: repo root carries no generated prompt artifact, or CI fails when two that should
   match diverge.

### C.2 — Migrate the framework's own legacy workspace
**Workstream**: KERNEL · **Effort**: S

**PARTIAL** 2026-09-22 — the flat artifact is gone and its producers are fixed; the test half of
the done-when cannot be met as written.

- **Migrated.** The flat `analysis.md` (untracked; `.claude/feature-workspace/` is gitignored) moved
  by hand into `blog-posts-05-06/`. It predates Epic 63 (2026-07-25 against 2026-08-02) and its work
  had shipped. The skill's leftover `enabled).` fragment from L3.9 was removed.
- **Producers fixed.** Thirteen `shared/` files read a pipeline artifact, or named the delivery
  checkpoint, at the flat root, where nothing has written since Epic 63: `finops-engineer` and
  `chaos-engineer` (both now 1.1.0), the `adr`, `analyze-complexity`, `openapi`,
  `check-accessibility`, `validate-migrations` and `verify-dependencies` skills, the
  `checkpointStore` default in `pipeline-schema.md`, `interface.md` and `feature-delivery-workflow.md`,
  and `DOMAIN_DICTIONARY.md`'s *Context Manifest* and *Pipeline State* entries. Every one now names
  `<feature-name>/`, as `resume-pipeline` already did. The first count of this was "seven"; the
  grep behind it matched three filenames, and the real sweep found about fifty references.
- **Deliberately left flat.** Ad-hoc *outputs* of standalone skills and non-pipeline agents
  (`release-plan.md`, `proposed-ki.md`, `dependency-audit-report.md`, …) — Epic 63 records that these
  use the root as a scratch area (`docs/aos/parallel-delivery-isolation-design.md:229`). The two
  legacy-detection lines in `deliver-feature` are its recorded exceptions.
- **Not met: the test.** The legacy migration is prose in `deliver-feature/SKILL.md`, not code, so
  there is no branch to test.
- **Still open, found while verifying:** (1) detection keys on a flat `pipeline-state.json`, so a
  flat artifact without a state file — exactly what this workspace had — is invisible to the repair
  path; (2) `TDDWorkflow` still checkpoints to a flat `tdd-state.json` (`tdd-workflow.md`,
  `test-driven-developer.md`), which is the singleton problem Epic 63 fixed for delivery and never
  scoped for TDD.

1. **Problem**: `.claude/feature-workspace/analysis.md` sits flat at the root — the exact
   "legacy pre-Epic-63 singleton" state `deliver-feature` ships migration code to repair. The
   framework's own workspace has never been migrated by its own migration path.
2. **Architectural Fix**: Run the migration, or delete the stale artifact. Then use it as the
   regression fixture for the legacy-detection code path, which currently has no test.
3. **Target Files**: `.claude/feature-workspace/`, `shared/skills/deliver-feature/SKILL.md`
4. **Done when**: no flat artifact remains and a test covers the legacy-migration branch.

---

# Appendix A — Provenance map

Every `agy.md` item and where it landed. No item was dropped.

| `agy.md` item | Merged into | Notes |
|---|---|---|
| Decouple Tool Execution & Validation from Host IDEs | **M0.4**, L2.1, L4.7 | The "no native execution engine / host-IDE dependency" framing was the sharpest thing in `agy.md` and became the core of M0.4. |
| Migrate from Markdown Artifacts to Typed State Graphs | **L2.9** | LangGraph suggestion rejected in favor of a Go FSM — see L2.9 fix. |
| Implement Native OS-Level HITL Interrupts | **L2.13**, L2.14 | Split: interrupt mechanism vs. gate-reset enforcement. |
| Replace Hardcoded DAGs with Dynamic Routing | **L3.1**, L3.2 | Split: the router, and the capability registry it routes over. |
| Abstract Semantic and Episodic Memory | **L3.4**, L3.5 | Split: semantic (vector adapter) vs. episodic (run store). `sqlite-vss` → `sqlite-vec`. |
| Decouple Governance into Async CI/CD Gates | **L3.12**, L3.11 | Split: moving auditors async, and the eval gate that replaces them. |
| Implement Isolated "Reflexion" Error Recovery Cycles | **L4.1** | The context-isolation insight was absorbed; the `maxRetries: 3` claim was verified and made precise. |
| Automate Prompt & Tool Meta-Evolution | **L4.4** | Conflict resolved in favor of eval-gated promotion — see the callout in L4.4. |
| Build a Native MCP Client Runtime | **L4.7** | Unchanged in substance. |

# Appendix B — Corrections to source documents

Errors found while merging, corrected in this document:

| Source | Claim | Correction |
|---|---|---|
| `agy.md` | Target file `shared/mcp/internal/tools/search_ki.go` | No such file. It is `search_ki_tool.go`. |
| `agy.md` | Target file `shared/telemetry/events.jsonl` | No such file. The telemetry log is project-local at `.claude/telemetry/events.jsonl`, and **no instance exists anywhere in this repo**. |
| `agy.md` | "`maxRetries: 3` merely repeats the failed tool call in the same context window" | Verified partially accurate. `pipeline-schema.md:26,36` do set `maxRetries: 3`, and `audit-composition-pattern.md:68-72` re-invokes the producer with the latest findings appended — so not literally the same context, but with no isolation and no accumulation across attempts. Restated precisely in L4.1. |
| `agy.md` | `condition: "feature.hasUI == true"` cited as evidence of primitive evaluation | Verified accurate — `pipeline-schema.md:68,125`. Strengthened in L3.1 with the schema's own admission at line 128. |
| `maturity-todo` | 6 documented event types vs. "policy.* and workflow.completed" undocumented | Undercounted. **Nine** undocumented types exist; `audit.fail`, `audit.retry`, `audit.halt`, and `workspace.migrated` were missed. Corrected in L3.9. |

---

*Compiled 2026-08-29 against `main` @ `59efe14`. All cited paths verified to resolve on that date.*
