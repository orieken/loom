# Contract: repo-doc (typed state)

**Produced by**: repo-documenter
**Consumed by**: the Go verifier (`loom` Phase 5), `repo-doc-auditor` (a later phase), the renderer that writes
`.loom/docs/<dir>/README.md` and the context index (`.loom/index.json`)

## Typed State

This artifact is **typed state**, not a markdown document. The stage returns JSON conforming to
`shared/schemas/pipeline/repo-doc.schema.json` (generated from `internal/state/repodoc_state.go` — never hand-edit
it); the executor validates it, and any markdown view is *rendered* from that state. Nothing parses a rendered view
back, so editing one changes nothing a later stage reads.

There is no markdown heading structure to check, which is why this contract has no "Required Sections" list. The
schema is the structure.

## Input

The executor sends one directory per invocation, bottom-up, so a directory is documented after its children:

| Input block | Content |
|---|---|
| `fact-sheet` | the directory's `.loom/facts/<dir>.json`, verbatim |
| `child-summaries` | for each child directory, its generated summary (purpose, tags, layer) or `undocumented` |

Only a directory whose `input_hash` differs from the one it was last documented against is sent. The agent receives
no raw source. It may read specific files by line range when a fact-sheet field is ambiguous, and must record each
read under `filesRead`.

## Required Fields

| Field | Rule |
|---|---|
| `schemaVersion` | the build's state schema version |
| `path` | the directory documented, copied from the fact sheet; the executor rejects a mismatch |
| `inputHash` | the fact sheet's `input_hash`, copied exactly; the executor rejects a mismatch |
| `purpose` | one sentence; **empty only when `openQuestions` is non-empty** |
| `layer` | one of `domain`, `application`, `adapter`, `framework`, `shared`, `test`, `docs`, `config`, `tooling`, `unknown` |
| `evidence` | non-empty whenever there is a purpose, a responsibility or an invariant |

`responsibilities`, `keyEntrypoints`, `tags`, `invariants`, `openQuestions` and `filesRead` are lists and may be empty.

## Validation Rules

Checked by `Validate` in Go on receipt (structure only — it cannot see the fact sheet):

1. `path`, `inputHash` and `schemaVersion` are present and correct.
2. An empty `purpose` is accompanied by at least one open question: an unknown purpose says what is missing.
3. `layer` is in the closed set above.
4. Every `evidence` entry has a `claim` and at least one `fields` citation, and every citation begins with a
   fact-sheet field: `files`, `exports`, `imports`, `approximate_imports`, `dependents`, `approximate_dependents`,
   `entrypoints`, `tests`, `children` or `child_summaries`, optionally followed by `:value`.
5. Every invariant has a statement and at least one piece of evidence.
6. Every `filesRead` entry has a path, a reason, and a line range with `1 <= startLine <= endLine`.

## Checked against the fact sheet (Phase 5)

Not enforceable here, and not claimed to be: that every symbol, dependency and entrypoint named exists in the fact
sheet, that every tag is in the vocabulary (`shared/vocabulary/context-tags.yaml`, extended by `.loom/tags.yaml`), and
that each claim's text appears in `evidence`. Those need the sheet and run in the verifier. Until it lands, a
documenter result is a **candidate**, recorded under `.loom/runs/` and never written into the index.

## What this contract does not claim

That the purpose is *correct*. It guarantees the purpose cites facts, not that the interpretation is right — that is
what the human approval gate before anything is written into the target repository is for.
