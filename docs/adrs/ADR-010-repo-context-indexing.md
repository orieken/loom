# ADR-010: Repo Context Indexing — extracted structure, generated intent

## Status
Accepted

## Date
2026-10-05

## Context
`CODEMAP.md` takes each directory's purpose from its README. Target repos rarely have READMEs, so the map is
mostly `TODO` lines exactly where an agent needs orientation, and agents fall back to grep to rebuild structure
on every task.

The feature scans a target repository and produces four things: a repo graph (import edges), fact sheets
(per-directory verifiable facts), a context index (`.loom/index.json`), and generated docs. It is built in eight
phases, one PR each (`docs/new-features/loom-context-indexing/`). This ADR records the decisions that hold
across phases and the two places Phase 1 departs from the implementation prompt.

## Decision

### 1. Facts come from Go; the model writes only intent
Graph edges, fact sheets, hashes and freshness are computed deterministically. A model writes purpose, tags and
intent on top of a fact sheet, cites the fields it relied on, and may answer "unknown". Generated claims with no
backing fact are rejected by the auditor (Phase 5), not softened.

### 2. Edges carry a confidence label
Every edge is `exact` or `approximate`. `exact` means the import statement was read from the source and the
target was resolved to a tracked file or package in the repo. `approximate` means resolution depended on a
heuristic (for example a Python absolute import that more than one source root could satisfy). Imports that
cannot be resolved at all — dynamic specifiers, targets not found — are recorded under `unresolved`, never
guessed into an edge. Tools surface the label.

### 3. Nodes are directories
A node is a directory holding source files of one supported language. Its id is the directory path. A directory
holding more than one language is split into `dir[lang]` nodes so ids stay unambiguous. This matches the
per-directory fact sheets of Phase 2 and the example schema in the prompt.

### 4. Extraction is pure Go: this departs from the prompt (CGO)
The prompt specifies `go/packages` or `go list` for Go and tree-sitter for TypeScript, Python and Java. Two
facts conflict with that:

- `.goreleaser.yaml` builds with `CGO_ENABLED=0` for five OS/arch targets, and `go install` and the Homebrew
  formula build from source. Tree-sitter's Go bindings require cgo, so adopting them means either breaking the
  release matrix or shipping a separate cgo build of `loom`.
- `go list` and `go/packages` invoke the target repo's toolchain, which needs the repo to compile, its modules
  to be resolvable and, for a cold module cache, network access. That contradicts "offline and local" and fails
  on exactly the half-broken repos an indexer is most useful for.

Decision: each language implements one `Extractor` interface in pure Go.

| Language | Technique | Edge confidence |
|---|---|---|
| Go | stdlib `go/parser` in imports-only mode plus `go.mod` module paths | `exact` for in-module imports |
| TypeScript / JavaScript | comment- and string-aware import scanner; relative paths and `tsconfig.json` `paths` / `baseUrl` aliases | `exact` when resolved to a tracked file |
| Python | line scanner for `import` / `from … import`; relative and absolute resolution | relative `exact`; absolute `exact` only from the repo root or `src/`, else `approximate` |
| Java | line scanner for `import` / `package`; resolved against a package → file index built from the repo itself | `exact` for a unique match, `approximate` for a wildcard spanning several directories |

No language is dropped. Because every language sits behind the `Extractor` interface, a tree-sitter or
compiler-backed extractor can replace any of these later without touching the graph, the fact sheets or the
tools. Known losses, accepted for v1:

- Go build tags and `cgo` files are not evaluated, so an import behind a tag not active on the current platform
  still produces an edge. `_test.go` files are excluded from edges so the graph describes the shipped
  dependency structure; Phase 2 lists them as tests.
- The TypeScript and Python scanners read static import syntax only. A scanner can be fooled by exotic syntax a
  real parser would handle; such a miss shows up as a missing edge, never a false `exact` one.

Follow-up if the scanners prove too weak: a `go list`-backed Go extractor (build-tag aware) behind an opt-in
flag, and a pure-Go tree-sitter runtime if one is vetted.

### 5. Sidecar by default; `.loom/` layout
Output goes to `.loom/`. Nothing in the target repo's own source is touched unless an opt-in flag asks
(Phase 6), and in-tree writes sit inside `loom:generated` markers.

```
.loom/graph.json   facts/   index.json   docs/   runs/
```

### 6. Deterministic and incremental
Output is sorted with stable ids. `graph.json` carries `graph_hash`, a SHA-256 of the canonical nodes, edges and
unresolved list. `generated_at` and `git_sha` are metadata outside that hash. `loom index` rewrites
`graph.json` only when `graph_hash` changes, so a rerun on an unchanged repo writes zero files — including after
a new commit that did not alter the graph. The prompt asks for both "no timestamps inside hashed content" and a
zero-diff rerun; excluding metadata from the hash is what reconciles them. `graph_hash` is also what Phase 7
tools compare to report `fresh`.

### 7. Where it lands in `shared/levels.yaml`
Deterministic indexing (Phases 1–3) and the MCP tools (Phase 7) are Level 2: they need no model and no extra
trust. Generated docs (Phases 4–6) are an opt-in bundle at Level 2, because they spend model calls and require
an approval gate. Placement is finalised in Phase 8.

### 8. File listing uses git
Files come from `git ls-files`, so untracked and ignored files never enter the graph. `.loomignore` (gitignore-
style, root-anchored when it contains a slash) removes tracked vendored and generated code. A tracked file that
has been deleted from the working tree is skipped.

## Consequences
- The release matrix stays `CGO_ENABLED=0`; `loom index` works offline on a repo that does not build.
- Extraction quality for TypeScript, Python and Java is bounded by what a scanner can see. The `exact` /
  `approximate` / `unresolved` split makes that bound visible instead of hiding it.
- Adding a language is one `Extractor` implementation plus a fixture repo.
- Every phase after this one reads the graph through `graph_hash`, so phases 2–7 do not need to know how an
  edge was found.
- Fitness functions: golden-graph tests per fixture repo, a byte-identical determinism test, a no-op rerun test
  (no writes), and an ignore-handling test. They live in `internal/repograph`.

## Amendment 2026-10-05: Phase 2 (fact sheets)
- **Layout.** `.loom/facts/<dir>.json` mirrors the directory path; the repo root is `_root.json`. A directory
  holding only subdirectories still gets a sheet, so ancestors can carry a hash. Sheets of deleted directories are
  removed, and a sheet is rewritten only when its bytes change.
- **Approximate edges stay out of `imports` and `dependents`.** The prompt's example has one list each. Edge
  confidence (decision 2) would be lost in it, so `approximate_imports` and `approximate_dependents` sit beside them.
- **`input_hash`** covers the digests of the directory's direct files, every fact on the sheet and the children's
  hashes. It includes the sheet's `dependents`, as the prompt specifies, so a change that alters a directory's imports
  also re-stales the directories that depend on it.
- **What counts as a fact.** Exports are top-level exported symbols of non-test source: Go identifiers, TypeScript
  `export` declarations and lists, Python names (or `__all__`), and the Java public type named after its file. Entrypoints
  are a Go `func main` in `package main`, a Python `__main__.py` or `__main__` guard, a Java `main` method, and the tracked
  `main` / `bin` files of a `package.json`. A Go cobra command is not detected as an entrypoint; that needs a call graph
  (out of scope for v1). A file is a test by its language's naming convention.

## Amendment 2026-10-05: Phase 3 (context index and freshness)
- **Shape.** `.loom/index.json` is `{"schema_version": 1, "entries": {<path>: {...}}}`. The prompt shows a bare map;
  the wrapper gives the file a version like every other artifact. Entries are keyed by directory path.
- **`input_hash` is the hash the entry was documented against**, and is empty until the documenter runs (Phase 4).
  Freshness compares it with the working tree's current fact-sheet hash. The stored `status` is a snapshot refreshed
  by `loom index`; `loom health` recomputes from the tree and never trusts it.
- **Indexing never overwrites recorded data.** A rerun keeps purpose, tags, layer, hashes and open questions and
  recomputes only `status`. New directories start `undocumented`; entries for deleted directories are dropped.
- **No timestamps yet.** `generated_at` is per entry and empty until documented, so an unchanged tree rewrites zero
  bytes. There is no top-level timestamp, for the same reason.
- **`loom health`.** Silent when a project has no index (opt-in). Stale, unindexed and orphaned paths each warn; a
  warning, never a failure, since a stale index is out of date and not broken. Undocumented paths warn only once some
  entry has been documented — otherwise every repo that ran `loom index` and never adopted generated docs would carry
  a permanent warning. It runs even when there is no install manifest, since a project may only have run `loom index`.

## Amendment 2026-10-05: Phase 4 (the repo-documenter)
- **Typed output, registered like any other stage.** The documenter returns a `repo-doc` state kind
  (`internal/state/repodoc_state.go`), so the existing provider inlines its generated schema and validates the reply
  the way it does for the delivery stages. `shared/contracts/repo-doc-contract.md` describes it; there are no markdown
  headings to check. The closed `layer` set is an enum derived from the same constants `Validate` checks.
- **The executor drives, in Go.** `loom document` (`internal/repodoc`) plans the work: directories whose recorded
  input hash differs from the current one, deepest first, so a parent is documented after its children and `--limit`
  keeps leaves rather than a parent without them. One invocation per directory; it receives the fact sheet, its
  children's summaries and the allowed tags, never source. A child that fails is recorded and its parent is told it
  is undocumented rather than blocked.
- **The executor, not the agent, checks the echo.** A reply that documents another directory, or a stale hash, is
  rejected. Structural validation lives in `Validate`; whether a cited symbol or tag *exists* needs the sheet and is
  Phase 5.
- **Results are candidates.** They are written under `.loom/runs/<run>/` with a run record (outcomes, reported usage,
  source reads) and **never to the index**. Until verification (Phase 5) and the approval gate (Phase 6) exist,
  nothing becomes `fresh`, so rerunning `loom document` re-sends every non-fresh directory. That is deliberate: the
  alternative is an index entry that says "documented" about unverified text. Reruns are bounded by `--limit` and
  `--path`.
- **Tag vocabulary.** `shared/vocabulary/context-tags.yaml` (embedded in the binary), extended but never reduced by
  `.loom/tags.yaml`. The prompt did not specify its contents; the initial 30 tags are a proposal.
- **No golden baseline yet.** `tests/agents/repo-documenter/` has the input (with planted traps), patterns and a rubric
  that penalizes every unsupported claim. `actual-output.md` needs a live run and is not committed, so the agent suite
  reports a SKIP rather than a pass for it.

## Amendment 2026-10-05: Phase 5 (verification)
- **Two checks, cheap first.** `Verify` (Go) runs on every reply: tags in the vocabulary, entrypoints and citations
  that exist, every claim backed by an `evidence` entry with its exact text, citations of *empty* fields rejected,
  source-line citations only where a `filesRead` covers them, and every backticked name present in the sheet. Only a
  reply with no violations reaches `repo-doc-auditor`, a new read-only agent that judges whether the cited facts
  *support* the claims. The prompt's alternative of extending `documentation-auditor` was rejected: its scope is
  README and inventory staleness, and its fixture and rubric would have to serve two unrelated jobs.
- **Backticks are the contract that makes names checkable.** A program cannot find a name inside prose, so the
  documenter is told to write every symbol, file or directory name in backticks and nothing else. A bare name escapes
  the verifier and is left to the auditor. This is a limit, stated rather than hidden.
- **A typed loop.** Problems return as `revision-feedback`: the previous attempt plus typed violations (source, code,
  where, message), whether from the verifier or the auditor. The executor counts rounds (default 3, `--max-rounds`) and
  reads the auditor's `verdict` field. A reply that fails the schema, or documents the wrong directory, is *fed back*
  rather than fatal, since the agent can fix that; a provider error cannot be revised away and fails the directory.
- **Exhaustion is a status, not a crash.** A directory still failing at the limit is `unverified`: its last attempt is
  kept under `unverified/`, every round is retained under `rounds/`, and nothing is accepted. A parent is shown an
  unverified child as `undocumented`, never its unverified text. `loom document` exits non-zero if any directory is
  failed or unverified.
- **Cost.** A directory is now one to two calls per round. `--no-audit` runs the verifier alone (cheaper, weaker);
  `--limit`, `--path` and `--max-rounds` bound the rest.
- **The auditor's model tier is `default`, not `light`.** Read-only counter agents are usually `light`, but this
  one's value is a judgement a lax model would waste. That is a cost decision worth revisiting with real data.

## Amendment 2026-10-05: Phase 6 (rendering and the approval gate)
- **Three steps, one gate.** `loom docs plan` stages a **Documentation Changeset** under `.loom/changes/<id>/` — the final content of every
  file it would write, a `change.json` listing them, and nothing else. `loom docs approve` records a human decision;
  `loom docs apply` writes. Approval is asked for on a terminal, or asserted with `--yes`; with neither it refuses, since
  silence is not consent. Nothing an agent returns creates an approval.
- **The approval binds to digests.** It records the SHA-256 of `change.json` and of every staged file. Editing any of
  them — or deleting one — resets it: the record is kept, naming what changed, and the changeset must be approved again,
  as a `loom run` gate is (roadmap L2.14). A human may edit a staged file *before* approving; the approval then binds to
  the edited content.
- **Apply is all-or-nothing up front.** Before writing a byte it checks the approval, every digest, and that each target
  is still what it was when planned (`BaseDigest`); a target edited since fails the whole apply with a message to
  re-plan. A target already holding the staged content (an interrupted earlier apply) is skipped, so apply is
  repeatable. A changeset applies once.
- **Generated blocks (`internal/genblock`).** Everything written in-tree sits between `loom:generated:start hash=…` and
  `loom:generated:end` in the file's own comment syntax. A file with no block keeps all its text and gains one at the end.
  The recorded hash is of the literal text between the markers; a mismatch means the block was edited by hand and is a
  **conflict**: not overwritten, shown at the gate, withheld on apply. A start marker with no end is also a conflict, not
  a guess.
- **What the model wrote and what was computed are visibly separate.** A rendered document is the typed result
  followed by a "Facts (computed from source)" section listing dependencies, dependents, entrypoints and tests, with
  approximate edges labelled as such.
- **Results are re-verified at plan time**, against the tree as it is then: a candidate whose directory changed since,
  or that no longer passes `Verify` (say the vocabulary shrank), is left out and listed. Accepted results are recorded in
  the index with `input_hash`, `doc_hash` (the digest of the written sidecar) and a timestamp, so `loom health` reports
  them fresh — and stale the moment the directory changes.
- **Doc comments never overwrite human documentation.** Go gets a `doc.go`, Java a `package-info.java`; both only if
  absent or already generated, and Go only when no other file documents the package. Python gets a docstring at the top
  of an *existing* `__init__.py` with none, TypeScript/JavaScript a JSDoc `@module` block on an *existing* `index.*` with
  no leading doc comment. No file is created that could change semantics (a namespace package), and a skip is recorded
  at the gate with its reason. The markers appear in the rendered doc text; that is the price of the same marker
  semantics in every language.
- **`CODEMAP.md` is a changeset artifact** (entrypoints, highest fan-in, layers, directories with one-line purposes;
  a directory without a purpose reads `_undocumented_`, never a placeholder). A file written by the retired script is
  wholly generated and replaced; any other file keeps its human text and gets a block. `scripts/generate-codemap.sh`
  now runs `loom index` and plans the change; `--codemap-only` needs no documenter run. It is derived from the fact
  sheets (the graph per directory) and the index.
- **A `.loomignore` was added to this repository** for `tests/fixtures/`, which are inputs to the indexer's own tests and
  otherwise show up as entrypoints.

## Amendment 2026-10-05: Phase 7 (MCP tools)
- **Six read-only tools in `loom mcp serve`:** `dependencies_of`, `dependents_of`, `impact`, `entrypoints`, `find_files`,
  `describe`. The logic lives in `internal/repoquery` over the stored graph, fact sheets and index; the tools are thin
  wrappers sharing one `repoTool` base. The server's tool count goes from 8 to 14, and the fitness tests that pin the
  list, the safe-argument allowlist and argument validation were extended deliberately.
- **A path is a key, not a file.** Arguments are repo-relative paths looked up in the graph (a source file resolves to its
  directory's node). They are never opened, so the workspace root has nothing to confine; `/`, `../../etc` and the like
  are refused as unknown keys, with up to five near-miss suggestions. The six tools are therefore listed as "pathless" in
  the root-confinement test, and a separate test proves the escapes are refused through the real registry.
- **`fresh` means the stored graph matches the working tree now.** It compares the stored `graph_hash` with a freshly
  extracted one. Extraction reads every tracked file, so it runs at most once per 30 seconds and at once when a new graph
  is written; a change in the last half minute may not show. This is a deliberate trade against re-reading the tree on
  every call. A check that cannot run (not a git repo) reports `fresh: false` rather than failing the question. Each
  result also carries `graphSha` (the commit; the prompt's "graph SHA") and `graphHash` (the content identity).
- **Stale answers are still answers.** A graph that no longer matches is served with `fresh: false` and a hint naming
  `loom index`; refusing would leave an agent with nothing, and the flag already says not to trust it.
- **`impact` labels confidence along the whole chain.** Breadth-first over importers; a directory keeps its shortest
  distance and is `exact` only if some shortest path uses exact edges throughout, so one approximate edge makes it and
  everything beyond it approximate. An unknown path is an error that names every unknown one, never a quietly smaller
  radius.
- **Small outputs.** Lists are capped (`limit`, default 50; 20 for `find_files`; hard ceilings 200 and 50), report the true
  `total`, and say when `truncated`. `describe` caps its summary at 3000 bytes and says so. Typical answers are a few
  hundred bytes.
- **Model-written text is reference, not instruction.** A `purpose`, open question or summary that trips the injection
  scan is withheld and listed under `injectionFlags`, as `search_ki` treats a Knowledge Item — a repository's own text can
  reach a documenter through names in its facts. `query` is never in the telemetry allowlist (guardrail #9).
- **`find_files` is lexical** (paths, tags and purposes, weighted 3/2/1, ties by path), deterministic, with no embeddings:
  the prompt's v1 constraint. A graph with no context index is a `not_found` error for it and for `describe`.

## Amendment 2026-10-05: Phase 8 (integration)
- **Blast radius is computed by the executor, not transcribed by the agent.** `context-engineer` calls
  `mcp__loom__impact` to choose what to pin, but the context manifest records only the *question* — `blastRadius.seeds`
  and `depth`. The executor resolves it from the repo graph and writes the result over whatever the agent put there,
  exactly as it does the token budget (roadmap L3.25), through an injected `BlastRadiusResolver` so the state package
  stays free of the graph (guardrail #1). The prompt asked only that the radius be recorded; a model copying a tool's
  output can mis-copy it, and a wrong "nothing depends on this" is the error the section exists to prevent. A graph that
  is missing or fails is recorded as *unresolved*, with the reason, never as an empty radius. The list is capped at 50
  and reports its true total. The section is optional, so existing manifests and the contract's required headings are
  unchanged.
- **Layer checks are one graph query.** `dependencies_of` and `dependents_of` edges now carry the `layer` the index
  records for the other end (omitted when undocumented). With the documenter's closed layer set — `domain` →
  `application` → `adapter` → `framework`, the rest neutral — `architect` and `verify-dependencies` check a boundary by
  asking for a directory's dependencies and reading the layers. An `exact` crossing is a violation; an `approximate` one is
  a *possible* violation; an edge into an undocumented directory is unclassified, not passed. `verify-dependencies` keeps
  its standalone mode and its `check.sh`, which still covers third-party libraries imported into the domain.
- **Agents may now declare MCP tools in frontmatter** (`mcp__<server>__<tool>`). The schema and the frontmatter contract
  allowed only built-in tool names; without this the headless allowlist `loom run` builds from that line could never
  include a graph tool. Each of the four agents declares exactly the tools its prompt uses, and says what to do — fall
  back and say so — when the server is not registered or the tool answers `not_found`. A test pins both directions:
  every `mcp__loom__*` name an agent or skill mentions is a registered tool, and each agent's declared set equals what its
  prompt uses. The Roo mode generators (Go and Python) file any `mcp__` tool under Roo's `mcp` group.
- **Telemetry reuses the executor's span vocabulary.** A documentation run emits a root span (plan `repo-context-index`),
  a stage span per attempt — `Iteration` is the revision round, as in the review loop — and inside it a provider span
  carrying the tokens and cost the provider reported, exported to `.loom/runs/<run>/traces.jsonl` and, when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set, over OTLP (`--no-telemetry` stops the file). An indexing run is a root span with
  one internal stage whose reason carries the counts; it records no tokens because there are none. The prompt said "loom
  run": indexing and documentation are not stages of the delivery plan, so they are traced as their own runs rather than
  by inserting them into it.
- **Where it lands in `shared/levels.yaml`.** Level 2 gains `repo-index` — required evidence is that
  `.loom/graph.json` exists, since without it the MCP repo tools answer "run loom index"; nothing is installed, because the
  binary is the install, and the installer says what to run. It also gains `repo-docs`, `optIn: true`: never installed as
  part of a level and never evidence, so declining it cannot lower a level and adopting it cannot raise one. The
  deterministic tools were already in Level 2 through the MCP registration. Consequence worth knowing: a project at
  Level 2 that never ran `loom index` now reports a gap naming the command. The two documenter agents ship in Level 1's
  `agents` bundle because bundles are directory-granular; they are inert until `loom document` is run on purpose.
  Roadmap item L3.63 is recorded as landed.
- **Registered as a memory source** (`repo-context-index`, lexical, optional path `.loom/`), so the registry and
  `query-memory` know it exists. It is regenerated rather than curated, so `memory-engineer` does not expire it.
- **No live golden baseline for context-engineer.** The prompt asked for a golden-file test where `context-engineer` uses
  `impact()`. A baseline is a recorded model run, which is not something to hand-write, and adding a pattern to
  `tests/agents/context-engineer/expected-patterns.txt` without re-recording would fail the existing suite. Instead the
  rendered manifest with a blast radius is a golden file (`internal/state/testdata/`), and prompt/frontmatter tests pin that
  the agent is told to use `impact()`, to record only the question, and to fall back. Re-recording the agent baseline
  is a live run still owed.
