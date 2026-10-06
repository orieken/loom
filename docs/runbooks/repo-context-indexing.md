# Runbook: Indexing a repository and reviewing what the documenter wrote

Covers `loom index`, `loom document`, `loom docs`, and what to do when entries go stale. The decisions behind
them are in [ADR-010](../adrs/ADR-010-repo-context-indexing.md).

## When to use
- A repository has no usable map: `CODEMAP.md` is mostly placeholders, or agents keep re-deriving structure
  with grep.
- You want agents (`context-engineer`, `architect`, `refactor-engineer`, `unit-tester`) to pin files by real
  dependency, and the `mcp__loom__*` repo tools to answer instead of answering `not_found`.
- `loom health` reports stale or unindexed paths.

## Prerequisites
- `loom` on your `PATH`, and the project is a git repository (the index is built from `git ls-files`).
- For documentation: the `claude` CLI, and `loom install` run so the agents exist at `.claude/agents/`. Nothing
  else needs installing — indexing is local, offline and uses no model.

## Steps

### 1. Build the graph (deterministic, free)
```bash
loom index
```
Writes `.loom/graph.json`, `.loom/facts/` (one fact sheet per directory) and `.loom/index.json`. A rerun on an
unchanged repo writes nothing. Vendored or generated code that should not be mapped goes in `.loomignore` (one
gitignore-style pattern per line). `.loom/` is derived and project-specific; add it to `.gitignore` unless your team
wants it committed.

You now have: dependency edges (each `exact` or `approximate`), entrypoints, exported symbols, tests. Register the
MCP server (`claude mcp add loom -- loom mcp serve`) and agents can query them. Check with `loom health`.

### 2. Get a map without any model
```bash
loom docs plan --codemap-only
loom docs approve        # reads the summary, asks y/N
loom docs apply
```
This regenerates `CODEMAP.md` from the graph: entrypoints, highest fan-in, directories. Undocumented directories read
`_undocumented_` — never a placeholder to fill in.

### 3. (Optional) Have an agent write what each directory is for
Every directory is at least one model call, so look first:
```bash
loom document --dry-run              # what would be sent, children first; no calls
loom document --limit 10             # the deepest 10 changed directories
loom document --path internal/state  # one subtree
```
Each result is checked by a Go verifier (every name, tag and citation must exist in the fact sheet; every claim needs
evidence) and then by `repo-doc-auditor`, with up to `--max-rounds` revisions. A directory that still fails is
**unverified**: its last attempt is kept under `.loom/runs/<run>/unverified/` and nothing is accepted. `--no-audit` runs
the Go check alone, which is cheaper and weaker. Cost and tokens are recorded in the run's `traces.jsonl`
(`--no-telemetry` turns that off).

### 4. Review one changeset
```bash
loom docs plan                       # stages; writes nothing to your repo
loom docs plan --in-tree-readmes     # also real READMEs, inside generated-block markers
loom docs plan --doc-comments        # also doc.go, package-info.java, docstrings, JSDoc @module
```
Read the summary. In order of importance:
1. **CONFLICTS** — a generated block was edited by hand. It is not overwritten. Decide whether your edit or the
   generated text wins, fix the block (or delete its markers), and plan again.
2. **Skipped** — each says why (an existing package doc, a human `doc.go`, an `__init__.py` with a docstring). These are
   protections, not failures.
3. **Open questions** — what the documents decline to claim. A purpose left empty with a question is the agent being
   honest.
4. The staged files themselves, under `.loom/changes/<id>/files/`. You may edit one before approving; the approval
   then binds to your edit. **Editing one after approving resets the approval.**

### 5. Approve and apply
```bash
loom docs approve        # interactive y/N, or --yes to assert you reviewed it
loom docs apply
```
`apply` refuses if a staged file or any target changed since. Re-plan in that case.

## Handling stale entries
`loom health` lists paths whose documentation predates the directory's latest change.
- **Stale / unindexed / orphaned**: run `loom index`. A stale entry keeps its old purpose until re-documented.
- **Re-document**: `loom document --path <dir>`; unchanged directories are skipped, so this costs only what changed.
- **Many stale after a refactor**: re-run `loom index`, then `loom document --limit N` in batches, then plan and apply.
- **MCP answers say `fresh: false`**: the graph predates the working tree. Run `loom index`. The check re-reads the tree at
  most every 30 seconds, so a change in the last half minute may not show yet.
- **MCP answers say `not_found`**: the repository was never indexed.

## Verification
- `loom health` shows "context index fresh" and the maturity section lists "repo graph present".
- `git status` shows only the files in the changeset you applied.
- A tool call such as `mcp__loom__impact` returns `"fresh": true` with the commit you expect.

## Escalation
- Many directories end **unverified**: read a run record (`.loom/runs/<run>/run.json`) for the violation codes. A
  repeated `unknown-name` means the agent is quoting names the sheet does not have; a repeated auditor finding means it
  is overclaiming. Tighten the prompts in `shared/agents/repo-documenter.md` rather than lowering `--max-rounds`.
- A language is mapped badly (missing edges, odd entrypoints): extraction is pure Go and deliberately simple
  (ADR-010). Add a fixture under `tests/fixtures/repograph/` and an extractor case, or exclude the directory with
  `.loomignore`.
