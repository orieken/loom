# Extract Framework-Generic MCP Patterns Into `shared/mcp-patterns/`

> Historical extraction handoff. The copied Go templates introduced here were subsequently removed.
> Read [the current MCP patterns guidance](../../../shared/mcp-patterns/README.md) and
> [the MCP adoption prompt](../adopt-framework-mcp-tools.md) before planning a new integration.

Break the accidental coupling between framework consumers and `saturday-mcp` by extracting the framework-generic MCP tool patterns (retrievers, analyzers, walkutil, response structs, registration pattern) into `ai-assistant-dot-files/shared/mcp-patterns/`. After this ships, downstream projects that want the bridge pattern no longer need saturday-mcp cloned locally — the framework install brings the reference implementation with it.

## Why this matters

Today's `install-framework-with-mcp-bridge.md` and `update-installed-framework.md` both instruct the executing agent to read from `saturday-mcp/internal/tools/*.go`. That forces every downstream user of the framework to also have `saturday-monorepo` cloned, even if their project has nothing to do with Saturday. That's the definition of accidental coupling — the framework depends on a specific downstream project.

## Target repo

`/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`. Do NOT push.

## Prerequisites

- `saturday-monorepo` cloned locally (currently the source of the patterns being extracted; used one-time as the copy source for this op)
- Both working trees clean
- User confirmation on the split (see "What's generic vs. Saturday-specific" below) before Phase B starts

## What's generic vs. Saturday-specific

`saturday-mcp` currently has 22 tools. Not all belong in the framework. The split for this extraction:

**Framework-generic (extract) — the 6 mcp-expand M1 tools + their infrastructure:**
- `analyze_complexity` + `complexity_analyzer.go`
- `check_accessibility` + `accessibility_analyzer.go`
- `check_ubiquitous_language` + `ubiquitous_language_analyzer.go`
- `verify_dependencies` + `dependency_boundary_analyzer.go`
- `search_ki` (LLM-as-retriever)
- `search_docs` (BM25 via sqlite-fts5)
- Retrieval-adapter interface (`retriever.go` — `Retriever` + `Reference` + `KICorpusRetriever` + `BM25Retriever`)
- `walkutil.go` (shared file-walk helpers)
- Response struct patterns from `responses.go` — extract the 6 M1-relevant structs; leave Saturday-specific ones behind
- Testing patterns from `testfixtures_test.go` — extract `writeFile`, `buildRequest`, `extractText`, `silentLogger`; document as reusable
- Server registration pattern from `registration.go` + `tool_provider.go` — the "tools are a `[]domain.Tool` slice built by a provider function, iterated at registration" pattern

**Saturday-specific (leave in saturday-mcp) — the 16 remaining tools:**
- `generate_site`, `generate_page`, `generate_flow`, `generate_steps`, `generate_element`, `generate_service`
- `migrate_code`, `generate_documentation`, `analyze_framework`, `validate_patterns`, `suggest_improvements`
- `analyze_impact`, `analyze_performance`, `parse_test_failure`
- `run_tests` + `prioritize_tests` (workflows)

## Scope

### Phase A — Confirm the split with the user (one commit — design doc)

Draft `docs/aos/mcp-patterns-extraction-plan.md` with:
- The generic-vs-Saturday split confirmed above
- Target directory structure for `shared/mcp-patterns/`
- Which language(s) to ship reference code in (recommend Go primary — matches saturday-mcp's language; TS/Python porting guidance as prose)
- How saturday-mcp continues to work after the extraction (its copies stay in place; framework's patterns become the source-of-truth for NEW bridges, not a runtime dependency)

Recommended `shared/mcp-patterns/` structure:
```
shared/mcp-patterns/
├── README.md                    ← what this directory is, when to use
├── go/                          ← reference implementation
│   ├── tools/
│   │   ├── retriever.go         ← Retriever interface + KICorpusRetriever
│   │   ├── bm25_retriever.go
│   │   ├── analyze_complexity_tool.go
│   │   ├── check_accessibility_tool.go
│   │   ├── check_ubiquitous_language_tool.go
│   │   ├── verify_dependencies_tool.go
│   │   ├── search_ki_tool.go
│   │   ├── search_docs_tool.go
│   │   ├── responses.go         ← 6 M1 response structs
│   │   └── testfixtures.go      ← extracted test helpers (renamed from _test.go so it's importable)
│   ├── analyzers/
│   │   ├── complexity_analyzer.go
│   │   ├── accessibility_analyzer.go
│   │   ├── ubiquitous_language_analyzer.go
│   │   ├── dependency_boundary_analyzer.go
│   │   └── walkutil.go
│   ├── server/
│   │   ├── registration.go      ← the loop-based registration pattern
│   │   └── tool_provider.go     ← the provider-function pattern
│   └── README.md                ← Go-specific porting guidance
└── porting-guides/
    ├── typescript.md            ← how to port each pattern to TS
    ├── python.md
    └── java.md
```

Commit: `docs(aos): draft mcp-patterns extraction plan (Op A)`.

**Pause for user approval.**

### Phase B — Extract (multiple commits, one per file group)

For each file in the "extract" list:

1. Copy from `saturday-mcp/internal/tools/<file>` (or `internal/analyzers/`, `internal/server/`) into `shared/mcp-patterns/go/tools/` (or corresponding subdir)
2. Strip Saturday-specific imports and references. Common changes:
   - `github.com/orieken/saturday-mcp/internal/logging` → replace with a placeholder `<YOUR_MODULE>/internal/logging` note + brief spec of what the logger interface must provide
   - `github.com/orieken/saturday-mcp/internal/domain` → same treatment
   - Package name: `package tools` → keep, but add a header comment explaining "when you copy this into your project, adjust the module path"
3. Add a file header comment: what this is, where it originated (with the saturday-mcp commit SHA it was extracted from), what to adjust when copying downstream
4. Verify: `go build ./shared/mcp-patterns/go/...` should NOT be expected to build in-tree (the reference impl is intentionally not a real Go module — it's source to copy from). Add a `// +build ignore` build tag or similar so it doesn't confuse Go tooling

Commit per file group (retrievers together, analyzers together, tools batched by category):
- `feat(mcp-patterns): extract retriever + BM25 adapter (Op B1)`
- `feat(mcp-patterns): extract 4 analyzers + walkutil (Op B2)`
- `feat(mcp-patterns): extract 6 framework tools (Op B3)`
- `feat(mcp-patterns): extract responses + testfixtures (Op B4)`
- `feat(mcp-patterns): extract registration + tool_provider patterns (Op B5)`

### Phase C — Update the bridge prompts (one commit)

Modify:
- `docs/prompts/install-framework-with-mcp-bridge.md` — replace the "read saturday-mcp/internal/tools/" references with "read shared/mcp-patterns/go/tools/". Note that saturday-mcp is now optional (a real-world example of the pattern; not a required prerequisite)
- `docs/prompts/update-installed-framework.md` — same substitution. Also update Pattern B's "diff against saturday-mcp" instruction to "diff against shared/mcp-patterns/go/tools/"

Commit: `docs(prompts): reference shared/mcp-patterns instead of saturday-mcp (Op C)`.

### Phase D — Write porting guides (multiple commits, optional but recommended)

For each of TypeScript, Python, Java, create `shared/mcp-patterns/porting-guides/<lang>.md` covering:
- How the `Retriever` interface maps to language-idiomatic patterns
- Which stdlib equivalents to use (`filepath.Walk` → `fast-glob` / `pathlib.rglob` / `Files.walk`)
- Testing framework recommendations (per existing `<lang>-conventions.md`)
- Where sqlite-fts5 access lives (`better-sqlite3` / `sqlite3` / `sqlite-jdbc`)

Commit per language: `docs(mcp-patterns): add <lang> porting guide (Op D<n>)`.

Only ship the language guides you know something concrete about; halt on any language where the porting story is unclear and let the first real port drive the doc.

## Discipline (non-negotiable)

- One commit per op / file group.
- Conventional Commits.
- **NEVER `git add -A`.**
- Extraction is COPY, not MOVE — saturday-mcp keeps its files. This is deliberate: saturday-mcp is now one reference consumer of the patterns, not the source of truth.
- Do NOT push.

## Escalation criteria

Stop and report if:
- A file being extracted has non-trivial Saturday-specific coupling that can't be cleanly stripped — halt, describe the coupling, may need a redesign
- The `// +build ignore` (or equivalent) approach doesn't work cleanly with your Go tooling — halt, propose alternative
- Extraction reveals that the "6 M1 tools" aren't actually framework-generic (they secretly depend on Saturday concepts) — halt, describe. May need to demote some tools out of the extraction set.
- Phase C's prompt updates would need to change more than the reference paths (e.g., the extraction changed the tool shapes) — halt, describe.

## Report format (per phase, under 300 words)

### Phase A report
```
Plan commit: <sha>
Confirmed split:
  - Framework-generic extracted: <n> files
  - Saturday-specific retained in saturday-mcp: <n> files
Chosen structure: <matches recommended shape | describe deviations>
Language decision: <Go primary + TS/Python/Java porting guides | ...>
```

### Phase B report
```
Extraction commits:
  <sha> retrievers
  <sha> analyzers
  <sha> tools
  <sha> responses + testfixtures
  <sha> registration patterns

Files extracted: <n>
Files needing header comments (all should): <verified>
saturday-mcp working tree unchanged: verified (extraction is copy, not move)
```

### Phase C report
```
Prompt updates:
  install-framework-with-mcp-bridge.md: shared/mcp-patterns references landed
  update-installed-framework.md: same

Commit: <sha>
```

### Phase D report (per language guide)
```
Language: <TS | Python | Java>
Guide commit: <sha>
Sections landed: <list>
Skipped areas (halted or deferred): <list>
```

Go.
