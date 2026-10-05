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
