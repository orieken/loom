# Install Framework + Bridge Its Tools Into an Existing Project MCP Server

> Historical implementation handoff. Do not execute this file for a new adoption.
> Its `shared/mcp-patterns/go/` copy paths no longer exist, and its retrieval and installation assumptions are outdated.
> Use [Adopt framework MCP tools](../adopt-framework-mcp-tools.md) instead.
> Commit messages and phase descriptions below are historical plans, not authorization to create commits.

Set up the ai-assistant-dot-files framework in a target project **AND** expose selected framework tools through that project's own MCP server (following Option B — copy tool source files, keep the MCP sovereign, adjust for the target project's layout).

## When to use this prompt

- Target project has its own MCP server (`./project/mcp/` or similar) — not using `saturday-mcp` directly
- Project structure resembles Saturday's shape (packages monorepo + tests/features + steps) but the project **isn't** Saturday and may use different tooling
- Team uses mixed clients (Claude Code + Cursor MCP + Claude Desktop + others) — needs framework capabilities reachable via MCP, not just Claude Code
- Want the framework AND control over what the project's MCP exposes

**Don't use this prompt if**:
- Project is greenfield and doesn't have an MCP server yet → use `docs/prompts/bootstrap-project.md` (via `/bootstrap-project`) instead
- Project uses `saturday-mcp` directly → adopt saturday-mcp updates as they ship; no bridge needed
- Only Claude Code users consume framework capabilities → skip the MCP bridge; just install the framework normally

## Reference material

**Primary source:**
- `shared/mcp-patterns/go/tools/`, `shared/mcp-patterns/go/analyzers/`, `shared/mcp-patterns/go/server/` — reference tool source shipped with the framework itself (retrievers, analyzers, walkutil, response structs, registration pattern). Copy from here for any new bridge.
- `shared/mcp-patterns/porting-guides/<language>.md` — if the target project's MCP is TypeScript, Python, or Java, read the matching porting guide first.

**Optional real-world example:**
- `saturday-monorepo/saturday-mcp/` — a real-world downstream consumer of these patterns. Useful as a secondary reference, but optional (saturday-monorepo does NOT need to be cloned locally to bridge tools).

**Always relevant:**
- `saturday-monorepo/saturday-mcp/mcp-expand-plan.md` — documents the tool patterns, retrieval-adapter interface, path resolution strategy. Useful even after extraction as a real-world worked example.
- `docs/patterns/deliver-feature-workflow.md` — shows how framework agents relate to framework MCP tools (agents author, MCP tools serve any client).

## Prerequisites

Before firing this prompt in a fresh chat, gather:

1. **Absolute path to the target project** (e.g., `/Users/x/Projects/myapp`)
2. **Language of the existing MCP server** (Go / TypeScript / Python — determines which saturday-mcp files copy cleanly vs. need porting)
3. **Path to the framework clone** (typically `/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files` — needed for `install.sh`)
4. **Confirmation the project's MCP server currently builds + tests green** — bridging into a broken MCP is a bad first move

## Scope

Three phases. Get user approval between each.

### Phase A — Investigate (one commit — discovery notes)

Discover the target project's structure and existing MCP server state:

1. Confirm the assumed directory shape: `./project/mcp/`, `./project/docs/`, `./project/tests/features/`, `./project/steps/`, `./project/packages/`. Note any deltas.
2. Read the existing MCP server:
   - What language? What framework (e.g., `mark3labs/mcp-go`, `@modelcontextprotocol/sdk`, Python `mcp` package)?
   - How many tools already registered? (Look for `AddTool`, `add_tool`, or equivalent)
   - Is there an existing pattern for tool wiring (something like saturday-mcp's `tool_provider.go`)?
3. Enumerate the corpus paths the framework tools will scan:
   - Docs: `./project/docs/`?
   - Features: `./project/tests/features/`?
   - Packages: `./project/packages/`?
   - KIs: `./project/.claude/knowledge/` (will exist after framework install)
4. Read `shared/mcp-patterns/go/tools/analyze_complexity_tool.go` + `shared/mcp-patterns/go/tools/retriever.go` + `shared/mcp-patterns/go/server/tool_provider.go` to internalize the pattern being ported.

Write findings to `./project/docs/framework-mcp-bridge-investigation.md`.

Commit (in target repo): `docs(mcp): investigate framework bridge feasibility`.

**Pause. Get user approval on the discovery before Phase B.**

### Phase B — Propose the bridge (one commit — plan document)

Draft `./project/docs/framework-mcp-bridge-plan.md` with:

- **Tools to bridge (recommended M1 set)**:
  - `analyze_complexity` — cyclomatic complexity + function-length enforcement
  - `check_accessibility` — semantic HTML violations (if UI code exists)
  - `check_ubiquitous_language` — DOMAIN_DICTIONARY.md compliance
  - `verify_dependencies` — Clean Architecture layer boundaries
  - `search_ki` — LLM-as-retriever over framework + project KIs
  - `search_docs` — BM25 over `./project/docs/`
- **Path configuration** — env vars matching saturday-mcp's convention:
  - `AI_ASSISTANT_DOTFILES_PATH` — framework install location
  - `<PROJECT>_MCP_DOCS_FTS_PATH` — docs BM25 index override (rename prefix per project)
  - Optional: `<PROJECT>_MCP_PACKAGES_PATH` — the packages monorepo root
- **Monorepo adaptation** — the target has `./project/packages/` (multi-package). Analytical tools should optionally accept `packagePath` input to scope to a single package (e.g., `@mypackage/core`). Document how this input flows through the existing analyzers (may require small changes to the copied analyzer code).
- **Language mapping**:
  - Target MCP in Go → files copy near-verbatim from `shared/mcp-patterns/go/`
  - Target MCP in TypeScript → files need porting (design each analyzer + tool in TS following the same shape); adjust go-specific stdlib calls (`filepath.Walk` → `fast-glob` / `fs.readdir`, etc.)
  - Target MCP in Python → similar porting story
- **Op-by-op breakdown** — enumerated below.
- **Estimated commit count** — realistic based on language and tool count.
- **Open questions requiring user approval** — e.g., tool naming conflicts if the target MCP already has a `search_*` tool.

Commit (in target repo): `docs(mcp): propose framework bridge plan`.

**Pause. Get user approval on the plan before Phase C.**

### Phase C — Execute (one commit per op)

**Op 1 — Framework install**:
```bash
cd /path/to/ai-assistant-dot-files && ./install.sh --project /path/to/project
```
Verifies: `./project/.claude/agents/`, `.claude/skills/`, `.claude/rules/`, `.claude/knowledge/` all exist and symlink correctly.

Commit (in target repo, if install created any tracked files): `chore(framework): install ai-assistant-dot-files context framework`.

**Op 2 — Bridge foundation**:
- If target MCP is Go: copy `shared/mcp-patterns/go/tools/retriever.go` and `bm25_retriever.go` into the target project's tools directory, adjust the package name.
- If target MCP is TS/Python: port the `Retriever` interface + `KICorpusRetriever` (LLM-as-retriever) + `BM25Retriever` (sqlite-fts5) to the target language.

Commit: `feat(mcp): add framework retrieval-adapter foundation (bridge Op 2)`.

**Ops 3-8 — Per-tool bridges** (one commit per tool from the recommended M1 set):
- Copy or port the tool + its underlying analyzer
- Adapt corpus paths to use `./project/`-shaped env vars
- Add `packagePath` optional input where relevant
- Wire into the existing MCP server's tool provider / registration
- Add unit tests following `shared/mcp-patterns/go/tools/testfixtures.go` pattern

Each commit: `feat(mcp): bridge <tool_name> (bridge Op N)`.

**Op 9 — Path + env documentation**:
- Update project's MCP README with the new env vars + how to configure them
- Update project's root README if it lists the MCP's tool inventory

Commit: `docs(mcp): document framework bridge configuration (bridge Op 9)`.

**Op 10 — Verify + close**:
- `go build ./... && go test ./...` (or language-equivalent) — must be green
- Run the MCP server standalone, verify tool discovery lists the new tools
- Update the framework-mcp-bridge-plan.md to close-out with commit SHAs

Commit: `chore(mcp): close framework bridge Milestone 1 (bridge Op 10)`.

## Discipline (non-negotiable)

- Match the discipline of `saturday-mcp`'s mcp-expand execution:
- One commit per op.
- Conventional Commits (`feat(mcp): ...`).
- **NEVER `git add -A`** — target project may have untracked WIP; stage explicit paths only.
- `git status --short` after staging, before every commit.
- Build + test green per commit.
- Coverage ≥ 85% on new files.
- Do NOT push in the target repo — human step.

## Escalation criteria

Stop and report if:
- Target MCP is in a language other than Go/TypeScript/Python — halt, describe. May need a language-specific port that's beyond this prompt's scope.
- Target MCP already has 20+ tools registered — halt, propose whether to bridge all M1 tools or a subset to keep the inventory manageable.
- Corpus paths differ significantly from the assumed shape (e.g., docs in `./project/documentation/` not `./project/docs/`) — halt, get user confirmation on env-var defaults.
- Framework install creates conflicts with existing `./project/.claude/` content — halt, describe. Never overwrite user files.
- An analyzer's file-walk logic needs project-specific ignore patterns (e.g., a `./project/generated/` directory that shouldn't be scanned) — halt, propose adding to `walkutil.SkippedDirNames`.
- Any tool name collides with an existing tool in the target MCP — halt, propose namespacing.

## Report format (per-phase, under 300 words each)

### Phase A report
```
Target project: <absolute path>
MCP server language: <Go | TypeScript | Python | other>
MCP framework: <mark3labs/mcp-go v<X> | @modelcontextprotocol/sdk v<X> | ...>
Existing tool count: <n>
Corpus paths verified:
  - docs: <path>
  - features: <path>
  - packages: <path>
  - KIs: <will exist after Op 1>
Deltas from assumed structure: <list, or "None">

Investigation commit: <sha>
Recommended next: proceed to Phase B (draft plan)
```

### Phase B report
```
Plan commit: <sha>
Tools proposed for bridging: <list>
Language mapping strategy: <copy | port>
Env var naming: <list>
packagePath monorepo support: <yes | no + rationale>
Open questions surfaced: <list>
Estimated Phase C commits: <n>

Recommended next: user approves plan → Phase C
```

### Phase C report
```
Commits landed (Op 1 → Op N):
  <sha> <message>
  ...

Final state:
  - Framework installed: yes
  - Tools bridged: <n>/<planned>
  - Coverage per new file: <pcts>
  - MCP tool discovery lists new tools: verified

Post-Phase-C follow-ups worth queuing:
  - <e.g., "add search_features vector search per M2">
  - <e.g., "add project-specific test advisor per Sunday pattern">
```

Go.
