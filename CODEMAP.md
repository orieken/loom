<!-- loom:generated:start hash=sha256:8e5f604dfa5ed1c9a931954f767260cc06ff9aaea51316e0ea5b13382f2b2c26 -->
# CODEMAP

Entry-point map for this repository, generated from its import graph and the context index. Read it first to orient, then search for specifics.

## Entrypoints

- `cmd/diff-coverage/main.go`
- `cmd/diff-mutation/main.go`
- `cmd/gen-schemas/main.go`
- `cmd/loom/main.go`
- `examples/embedding/main.go`
- `scripts/generate-roomodes.py`
- `scripts/resolve-model-tier.py`
- `scripts/validate-frontmatter.py`
- `shared/mcp/cmd/mcp-server/main.go`

## Highest fan-in

Directories imported by the most others (exact edges only).

- `internal/repograph` — 10 dependents — _undocumented_
- `internal/orchestrator` — 8 dependents — Executes a run's stages and records its state and timeline, with loop-round primitives also defined in the package.
- `internal/state` — 8 dependents — This directory defines the typed state, schema, routing and rendering model for each pipeline stage, exporting per-stage types such as `AnalysisState`, `ArchitectureState`, `ContextState`, `ImplementationState`, `QAState`, `RepoDocState`, `RepoDocAuditState`, `ReviewState`, `ScenariosState` and `SecurityState`.
- `internal/factsheet` — 7 dependents — _undocumented_
- `internal/contextindex` — 6 dependents — Builds, persists and checks the freshness of a documentation index built from `internal/factsheet` and `internal/repograph`.
- `tools` — 5 dependents — Defines the core `Tool` abstraction and related types, including `ToolRequest`, `ToolResult`, `Registry`, and `ToolError`, depended on by `examples/embedding`, `examples/embedding/echotool`, `shared/mcp/internal/domain`, `shared/mcp/internal/server`, and `shared/mcp/register`.
- `internal/telemetry` — 4 dependents — This directory defines telemetry types and functions for starting and configuring telemetry, tracing tool calls and retrieval operations, propagating trace context from environment variables, resolving trace output file names and paths, and redacting secret attribute values.
- `shared/mcp/internal/logging` — 4 dependents — Defines a `Logger` type and a `NewLogger` constructor that four other packages in the MCP server depend on for logging.
- `cmd/loom/internal/ownership` — 3 dependents — Defines ownership-decision types `Decision`, `State`, `Reason`, `Digest` and `Ledger`, with `Update` and `Write` functions and skip predicates `IsSkip`, `Skip` and `SkipModified`, used by `cmd/loom/cmd`, `cmd/loom/internal/fs` and `cmd/loom/internal/manifest`.
- `internal/diffcover` — 3 dependents — Parses diffs and Go coverage profiles and measures diff coverage while applying file exclusion rules.

## Layers

- **adapter**: `cmd/loom/internal/fs`, `cmd/loom/internal/platform`, `internal/gitdiff`, `internal/provider/claude`, `internal/provider/mock`, `shared/mcp/internal/server`, `shared/mcp/internal/tools`, `shared/mcp/register`
- **application**: `cmd/diff-coverage`, `internal/contextindex`, `internal/orchestrator`, `internal/repoindex`, `internal/repoquery`
- **config**: `.claude`, `.claude/policies`, `.cursor`, `.github`, `.vscode`, `cmd/loom/internal/levels`, `shared/configs`, `shared/hooks`, `shared/hooks/examples`, `shared/vocabulary`, `templates/claude-feature-team/.claude`, `templates/claude-feature-team/.claude/agents`
- **docs**: `.aiassistant`, `.aiassistant/rules`, `.clinerules`, `.cursor/rules`, `.github/instructions`, `.junie`, `.roo/rules`, `docs`, `docs/adrs`, `docs/agent-metrics`, `docs/agent-metrics/evals`, `docs/aos`, `docs/aos/AOS_Governance_Design_Pack`, `docs/aos/prompts`, `docs/audits`, `docs/audits/loom-e2e-run-2026-09-07`, `docs/audits/loom-e2e-run-4`, `docs/audits/loom-e2e-run-4-specs`, `docs/audits/loom-e2e-run-7`, `docs/audits/loom-e2e-run-8`, `docs/audits/loom-e2e-run-9`, `docs/blog-posts`, `docs/blueprints`, `docs/features`, `docs/features/2026-07-aos-foundations-and-frontmatter`, `docs/features/context-engineering-framework`, `docs/features/loom-codemap`, `docs/features/token-optimization-strategies`, `docs/incidents`, `docs/lessons-learned`, `docs/patterns`, `docs/pipeline-retrospectives`, `docs/prompts`, `docs/prompts/done`, `docs/roadmaps`, `docs/runbooks`, `internal/references/testdata/fixture/shared/skills`, `internal/references/testdata/fixture/shared/skills/real`, `shared/agents`, `shared/blueprints`, `shared/contracts`, `shared/evaluation`, `shared/evaluation/agent-evals`, `shared/knowledge`, `shared/mcp-patterns`, `shared/mcp-patterns/porting-guides`, `shared/orchestration`, `shared/policies`, `shared/policies/examples`, `shared/rag`, `shared/rag/adapters`, `shared/rules`, `shared/skills`, `shared/skills/adr`, `shared/skills/agent-eval`, `shared/skills/agent-scorecard`, `shared/skills/backfill-unit-tests`, `shared/skills/bootstrap-project`, `shared/skills/chaos-experiment`, `shared/skills/complexity-check`, `shared/skills/create-ki`, `shared/skills/deliver-bugfix`, `shared/skills/deliver-feature`, `shared/skills/event-storm`, `shared/skills/five-whys`, `shared/skills/generate-fuzz-tests`, `shared/skills/learning-engine`, `shared/skills/mcp-add`, `shared/skills/memory-compression`, `shared/skills/memory-engineer`, `shared/skills/memory-expansion`, `shared/skills/new-feature`, `shared/skills/numpath-alignment`, `shared/skills/numpath-strategy`, `shared/skills/on-call`, `shared/skills/openapi`, `shared/skills/performance-profile`, `shared/skills/pipeline-retrospective`, `shared/skills/pipeline-trace`, `shared/skills/quality-optimizer`, `shared/skills/query-memory`, `shared/skills/resume-pipeline`, `shared/skills/retrospective`, `shared/skills/search-ki-semantic`, `shared/skills/sunday-test-advisor`, `shared/skills/validate-artifact`, `shared/templates`, `shared/workflows`, `templates/claude-feature-team`, `templates/claude-feature-team/.claude/rules`, `templates/claude-feature-team/.claude/skills`, `templates/claude-feature-team/.claude/skills/adr`, `templates/claude-feature-team/.claude/skills/badr`, `templates/claude-feature-team/.claude/skills/chaos-experiment`, `templates/claude-feature-team/.claude/skills/complexity-check`, `templates/claude-feature-team/.claude/skills/db-migration`, `templates/claude-feature-team/.claude/skills/design-review`, `templates/claude-feature-team/.claude/skills/five-whys`, `templates/claude-feature-team/.claude/skills/openapi`, `templates/claude-feature-team/.claude/skills/performance-profile`, `templates/claude-feature-team/.claude/skills/refactor-to-pattern`, `tests/platform-verification`
- **domain**: `cmd/loom/internal/manifest`, `cmd/loom/internal/ownership`, `internal/genblock`, `internal/mutation`, `internal/planfile`, `internal/policy`, `internal/state`, `shared/mcp/internal/analyzers`, `shared/mcp/internal/domain`, `tools`
- **framework**: `cmd/loom`, `shared/mcp/cmd/mcp-server`
- **shared**: `internal/diffcover`, `internal/injectionscan`, `internal/telemetry`, `shared`, `shared/mcp`, `shared/mcp/internal/logging`, `shared/schemas`, `shared/schemas/pipeline`, `shared/schemas/repograph`, `shared/schemas/telemetry`
- **test**: `internal/provider/claude/testdata`, `internal/references/testdata`, `internal/references/testdata/fixture`, `internal/references/testdata/fixture/docs`, `internal/references/testdata/fixture/docs/history`, `internal/references/testdata/fixture/shared/agents`, `internal/references/testdata/fixture/shared/workflows`, `internal/repoquery/fixture`, `internal/state/testdata`, `internal/testlint/testdata`, `internal/testlint/testdata/fixture`, `internal/testlint/testdata/fixture/cases`, `internal/testlint/testdata/fixture/helpers`, `internal/testlint/testdata/unparseable`, `tests/agents`, `tests/agents/accessibility-engineer`, `tests/agents/agent-evaluator`, `tests/agents/analyst`, `tests/agents/architect`, `tests/agents/code-reviewer`, `tests/agents/context-auditor`, `tests/agents/context-engineer`, `tests/agents/data-engineer`, `tests/agents/developer`, `tests/agents/devops-engineer`, `tests/agents/documentation-auditor`, `tests/agents/documentation-manager`, `tests/agents/exemplar-auditor`, `tests/agents/knowledge-auditor`, `tests/agents/memory-auditor`, `tests/agents/model-tier-auditor`, `tests/agents/pattern-reviewer`, `tests/agents/performance-engineer`, `tests/agents/privacy-auditor`, `tests/agents/product-owner`, `tests/agents/prompt-evaluator`, `tests/agents/qa-engineer`, `tests/agents/refactor-engineer`, `tests/agents/release-manager`, `tests/agents/repo-doc-auditor`, `tests/agents/repo-documenter`, `tests/agents/retrieval-evaluator`, `tests/agents/rule-auditor`, `tests/agents/security-reviewer`, `tests/agents/spec-writer`, `tests/agents/sre-engineer`, `tests/agents/tech-writer`, `tests/agents/tool-validator`, `tests/agents/unit-tester`, `tests/agents/visual-qa-engineer`, `tests/platform-verification/results`
- **tooling**: `.github/workflows`, `cmd/diff-mutation`, `cmd/gen-schemas`, `internal/references`, `scripts`, `scripts/api-generator`, `scripts/api-generator/emitters`, `shared/skills/check-accessibility`, `shared/skills/check-ubiquitous-language`, `shared/skills/search-ki`, `shared/skills/verify-dependencies`, `templates/claude-feature-team/.claude/skills/validate-migrations`
- **unknown**: `.`, `.roo`, `.windsurf`, `cmd`, `cmd/loom/internal`, `cmd/loom/internal/mcpprobe`, `examples`, `examples/embedding`, `examples/embedding/echotool`, `internal`, `internal/orchestrator/testdata`, `internal/provider`, `internal/references/testdata/fixture/exact`, `internal/references/testdata/fixture/shared`, `internal/testlint`, `internal/worktree`, `scripts/hooks`, `shared/mcp/cmd`, `shared/mcp/internal`, `shared/plans`, `shared/skills/analyze-complexity`, `shared/skills/api-contract-verify`, `shared/skills/api-ingest`, `shared/skills/badr`, `shared/skills/context-audit`, `shared/skills/context-engineer`, `shared/skills/cost-optimizer`, `shared/skills/db-migration`, `shared/skills/debug-environment`, `shared/skills/debug-tests`, `shared/skills/deliver-atdd`, `shared/skills/dependency-update`, `shared/skills/design-review`, `shared/skills/extract-lessons`, `shared/skills/forgetting-engine`, `shared/skills/health-check`, `shared/skills/list-agents`, `shared/skills/onboard`, `shared/skills/orchestrate`, `shared/skills/promote-memory`, `shared/skills/refactor-to-pattern`, `shared/skills/review-pr`, `shared/skills/run-tests`, `shared/skills/saturday-test-advisor`, `shared/skills/scaffold-docs`, `shared/skills/scheduler`, `shared/skills/ship-feature`, `shared/skills/spec-writer`, `shared/skills/summarize-artifact`, `shared/skills/team-topology-check`, `shared/skills/threat-model`, `shared/skills/validate-migrations`, `shared/telemetry`, `templates`, `templates/claude-feature-team/.claude/skills/analyze-complexity`, `templates/claude-feature-team/.claude/skills/api-contract-verify`, `templates/claude-feature-team/.claude/skills/check-accessibility`, `templates/claude-feature-team/.claude/skills/check-ubiquitous-language`, `templates/claude-feature-team/.claude/skills/deliver-feature`, `templates/claude-feature-team/.claude/skills/dependency-update`, `templates/claude-feature-team/.claude/skills/event-storm`, `templates/claude-feature-team/.claude/skills/generate-fuzz-tests`, `templates/claude-feature-team/.claude/skills/on-call`, `templates/claude-feature-team/.claude/skills/run-tests`, `templates/claude-feature-team/.claude/skills/threat-model`, `templates/claude-feature-team/.claude/skills/verify-dependencies`, `templates/claude-feature-team/features`, `tests`

## Directories

| Directory | Purpose |
|---|---|
| `.aiassistant/` | Serves as a container directory that groups the project's rule documentation under its `.aiassistant/rules` child directory. |
| `.aiassistant/rules/` | Holds the project's numbered markdown rule and convention documents. |
| `.claude/` | _undocumented_ |
| `.claude/policies/` | This directory holds the `evidence-clean-ship.policy.yaml` policy file. |
| `.clinerules/` | This directory holds a set of markdown rule and convention documents covering approval gates, design and architecture guardrails, an agent roster, testing conventions, and per-language coding conventions. |
| `.cursor/` | _undocumented_ |
| `.cursor/rules/` | Holds rule documents covering agent roster, approval gates, architecture, coding conventions for C#, Java, Python and TypeScript, Go-backend and Vue-frontend guidance, design principles, a test-repair contract and testing guidance. |
| `.github/` | This directory holds the repository's GitHub-specific configuration, including a `copilot-instructions.md` file, `agents` and `skills` entries, and the `.github/instructions` and `.github/workflows` subdirectories documented separately. |
| `.github/instructions/` | This directory holds instruction documents covering conventions for several programming languages and frameworks plus testing. |
| `.github/workflows/` | This directory holds the GitHub Actions workflow files that define the framework's CI workflow (`framework-ci.yml`) and Loom's release workflow (`loom-release.yml`). |
| `.junie/` | This directory holds a single file, `guidelines.md`, and nothing else is recorded for it. |
| `.roo/` | _undocumented_ |
| `.roo/rules/` | Contains markdown rule documents covering language-specific coding conventions and cross-cutting engineering guardrails. |
| `.vscode/` | Holds an example VS Code editor settings file. |
| `.windsurf/` | _undocumented_ |
| `cmd/` | This directory groups the repository's command-line entrypoints, given as the subdirectories `cmd/diff-coverage`, `cmd/diff-mutation`, `cmd/gen-schemas` and `cmd/loom`. |
| `cmd/diff-coverage/` | This directory is the command-line entrypoint for computing diff coverage, built on `internal/diffcover` and `internal/gitdiff`. |
| `cmd/diff-mutation/` | This directory provides a command-line entrypoint and exports no symbols for other packages to import. |
| `cmd/gen-schemas/` | A command-line program, entered via `main.go`, that writes the JSON Schema for every typed pipeline stage from the Go structs in `internal/state` into the repository's shared schema directory. |
| `cmd/loom/` | This directory is the executable entrypoint for the program, containing `main.go`. |
| `docs/` | This directory is the repository's root documentation collection, holding top-level documentation files alongside fourteen documentation subdirectories. |
| `docs/adrs/` | Collects the repository's architecture decision records as individual markdown files (`ADR-001-adopt-rag-friendly-docs-structure.md` through `ADR-010-repo-context-indexing.md`) alongside a `README.md`. |
| `docs/agent-metrics/` | This directory holds documentation about agent metrics, consisting of a `README.md` and the `docs/agent-metrics/evals` subdirectory. |
| `docs/aos/` | A documentation directory holding markdown design, planning, migration and integration documents about AOS, together with two child directories of further AOS documentation. |
| `docs/audits/` | This directory holds standalone audit and analysis reports as markdown files, together with six child directories. |
| `docs/blog-posts/` | This directory holds a numbered series of blog post files, each topic represented by two files, plus a `README.md`. |
| `docs/blueprints/` | _undocumented_ |
| `docs/features/` | This directory collects feature documentation, with each feature's material held in its own subdirectory. |
| `docs/incidents/` | _undocumented_ |
| `docs/lessons-learned/` | Holds project documentation of lessons learned, in a `README.md` and a dated lessons file. |
| `docs/patterns/` | docs/patterns is a documentation directory holding markdown reference files on software design, architecture, process and authoring patterns. |
| `docs/pipeline-retrospectives/` | Holds `README.md`, which documents the pipeline-retrospective skill's dated cross-delivery timing and iteration-count trend reports and the naming convention for them. |
| `docs/prompts/` | This directory holds markdown files for epics and planning documents, alongside a `docs/prompts/done` child directory holding markdown specification files for individual framework prompts, epics and documentation-cleanup tasks. |
| `docs/roadmaps/` | Holds markdown documents tracking the project's build roadmap, an architectural audit and a maturity to-do list. |
| `docs/runbooks/` | Holds a set of markdown documentation files for the repository. |
| `examples/` | A directory that contains the example program in `examples/embedding`. |
| `examples/embedding/` | An example program with entrypoint `main.go` that imports `examples/embedding/echotool`, `shared/mcp/register` and `tools`. |
| `internal/` | This directory has no files, exports or imports of its own and groups the repository's 22 internal child packages. |
| `internal/contextindex/` | Builds, persists and checks the freshness of a documentation index built from `internal/factsheet` and `internal/repograph`. |
| `internal/diffcover/` | Parses diffs and Go coverage profiles and measures diff coverage while applying file exclusion rules. |
| `internal/docchange/` | _undocumented_ |
| `internal/factsheet/` | _undocumented_ |
| `internal/genblock/` | Defines the status and syntax vocabulary for generated blocks and provides `Extract` and `Hash` to read and verify them, used by `internal/docchange`. |
| `internal/gitdiff/` | `internal/gitdiff` exports a `Diff` function used by `cmd/diff-coverage` and `cmd/diff-mutation`. |
| `internal/injectionscan/` | Exposes `Scan`, `Rules`, and `Finding`. |
| `internal/memory/` | _undocumented_ |
| `internal/mutation/` | Models mutation-testing plans, targets and mutants, parses their results, and summarizes outcomes by status. |
| `internal/orchestrator/` | Executes a run's stages and records its state and timeline, with loop-round primitives also defined in the package. |
| `internal/planfile/` | internal/planfile parses, renders, and discovers plan file definitions along a `SearchPath`. |
| `internal/policy/` | Defines and evaluates policy configuration that decides which action (auto-approve, auto-reject, escalate, or require human) applies at a named gate, based on conditions checked against known fields. |
| `internal/provider/` | This directory has no files, exports or imports of its own and groups the two provider implementations in `internal/provider/claude` and `internal/provider/mock`. |
| `internal/references/` | `Scan` finds `Finding` values in a set of files, checked against `Rules`. |
| `internal/repodoc/` | _undocumented_ |
| `internal/repograph/` | _undocumented_ |
| `internal/repoindex/` | Exports `Run` and `Result`, drawing on `internal/contextindex`, `internal/factsheet` and `internal/repograph`. |
| `internal/repoquery/` | `internal/repoquery` defines a `Store`, constructed by `NewStore`; it imports `internal/contextindex`, `internal/factsheet` and `internal/repograph`, and is imported by `cmd/loom/cmd`, `shared/mcp/internal/server` and `shared/mcp/internal/tools`. |
| `internal/state/` | This directory defines the typed state, schema, routing and rendering model for each pipeline stage, exporting per-stage types such as `AnalysisState`, `ArchitectureState`, `ContextState`, `ImplementationState`, `QAState`, `RepoDocState`, `RepoDocAuditState`, `ReviewState`, `ScenariosState` and `SecurityState`. |
| `internal/telemetry/` | This directory defines telemetry types and functions for starting and configuring telemetry, tracing tool calls and retrieval operations, propagating trace context from environment variables, resolving trace output file names and paths, and redacting secret attribute values. |
| `internal/testlint/` | _undocumented_ |
| `internal/worktree/` | Exports a `Git` type and a `New` function, consumed by `cmd/loom/cmd`. |
| `scripts/` | Holds shell and Python scripts, including three Python entrypoints: `generate-roomodes.py`, `resolve-model-tier.py` and `validate-frontmatter.py`. |
| `scripts/api-generator/` | Exports `parseOpenApi`, `SundayApiSpec` and `SundayEndpoint`. |
| `scripts/hooks/` | _undocumented_ |
| `shared/` | This directory's children are predominantly documentation, configuration or schema subdirectories, alongside top-level governance and registry files such as `ARCHITECTURE_RULES.md`, `DOMAIN_DICTIONARY.md` and `TEAM_TOPOLOGY.md`, with one child, `shared/mcp`, exporting code rather than documentation. |
| `shared/agents/` | This directory holds a markdown file for each of the current set of named roles (such as `architect.md`, `code-reviewer.md` and `security-reviewer.md`), plus a `CHANGELOG.md` that records changes to that set. |
| `shared/blueprints/` | This directory holds markdown blueprint files for distinct project archetypes — clean-architecture CLI, library, and service variants, a frontend archetype, an MCP server archetype, and an archetype documented in `scribe-cli.md` — plus two files, `saturday.md` and `sunday.md`, whose names do not indicate an archetype. |
| `shared/configs/` | This directory collects linter and style-checker configuration files for multiple language toolchains. |
| `shared/contracts/` | Holds one markdown contract document per domain-specific output format. |
| `shared/evaluation/` | This directory holds documentation about evaluation work — an agent evaluation harness design, a pipeline retrospective, and a retrieval regression writeup — plus the `agent-evals` subdirectory. |
| `shared/hooks/` | This directory documents a hook schema in `hooks-schema.md`, provides a `README.md`, and holds two hook configuration files, `on-retrospective-written.yaml` and `scheduled-monthly.yaml`. |
| `shared/knowledge/` | This directory holds markdown documentation and defines no exported code, imports, entrypoints or tests. |
| `shared/mcp/` | `shared/mcp` is the root of the MCP reference source tree: it exports `SourceFS`, an embedded filesystem bundling `README.md` and the `cmd`, `internal` and `register` subtrees, and is depended on by `.`. |
| `shared/mcp-patterns/` | This directory gives guidance for building or extending a Model Context Protocol server that speaks the framework's tools in a language other than Go, directing Go consumers to import the framework's library directly and directing non-Go ports to the `shared/mcp-patterns/porting-guides` child directory. |
| `shared/orchestration/` | This directory holds documentation describing an orchestration subsystem, including its `interface.md`, `pipeline-schema.md`, `policy-evaluator.md` and `audit-composition-pattern.md`. |
| `shared/plans/` | _undocumented_ |
| `shared/policies/` | Documents the shape of policy files through `policy-schema.md` and an overview in `README.md`. |
| `shared/rag/` | Documents the RAG retriever interface and delegates adapter-specific documentation to `shared/rag/adapters`. |
| `shared/rules/` | Holds markdown documents that state engineering rules and per-language coding conventions. |
| `shared/schemas/` | Holds the JSON Schema definitions for agent, KI and skill frontmatter, and contains child directories of schemas for the pipeline, repograph and telemetry artifacts. |
| `shared/skills/` | This directory holds a `SKILL_TEMPLATE.md` file and contains the repository's collection of skill-definition subdirectories. |
| `shared/telemetry/` | _undocumented_ |
| `shared/templates/` | This directory holds a collection of markdown and YAML template files, each named for a specific report, notes, or document kind, along with one file, `my-first-feature.md`, that does not follow that naming pattern. |
| `shared/vocabulary/` | Holds `context-tags.yaml`, the controlled vocabulary of tags that may be assigned to a directory. |
| `shared/workflows/` | This directory holds `feature-delivery-workflow.md`, a document describing a feature delivery workflow. |
| `templates/` | _undocumented_ |
| `templates/claude-feature-team/` | _undocumented_ |
| `tests/` | This directory groups two child directories, `tests/agents` and `tests/platform-verification`, which hold agent-evaluation fixtures and platform-verification documentation respectively. |
| `tests/agents/` | This directory is a top-level fixtures collection for agent-evaluation tests, holding `README.md` and `fixture-manifest.txt` alongside subdirectories holding test fixtures for individual agents, documented with testing and fixtures tags at a test layer. |
| `tests/platform-verification/` | This directory holds a `README.md` plus three platform-named markdown files (`antigravity.md`, `copilot.md`, `cursor.md`), with a `results` child directory holding per-platform files for the same three platforms. |
| `tools/` | Defines the core `Tool` abstraction and related types, including `ToolRequest`, `ToolResult`, `Registry`, and `ToolError`, depended on by `examples/embedding`, `examples/embedding/echotool`, `shared/mcp/internal/domain`, `shared/mcp/internal/server`, and `shared/mcp/register`. |
<!-- loom:generated:end -->
