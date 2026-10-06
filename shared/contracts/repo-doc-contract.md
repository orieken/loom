# Contract: repo-doc (typed state)

**Produced by**: repo-documenter
**Consumed by**: the Go verifier, `repo-doc-auditor`, the renderer that writes
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
| `child-summaries` | for each child directory, its **verified** summary (purpose, tags, layer) or `undocumented` |
| `allowed-tags` | the vocabulary the agent may draw `tags` from |
| `revision-feedback` | only on a later round: the previous attempt and the typed findings to fix |

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

## Checked against the fact sheet (the verifier)

`Validate` cannot see the sheet, so a second, deterministic check does — `Verify` in `internal/repodoc`, run by
`loom document` on every result before anything else:

- every `tags` entry is in the vocabulary (`shared/vocabulary/context-tags.yaml`, extended by `.loom/tags.yaml`);
- every `keyEntrypoints` entry is an export or an entrypoint the sheet lists;
- every purpose, responsibility and invariant statement appears, **verbatim**, as the `claim` of an `evidence` entry;
- every citation names a value its field actually holds, and a citation of an **empty** field is rejected — it
  supports nothing. To evidence an *absence* the author cites `field:none` (`imports:none`), accepted only when the
  sheet really lists none;
- an invariant may cite a source line range only if the same range is covered by a `filesRead` entry;
- every `filesRead` path is a file of the directory;
- every name **written in backticks** in a claim exists in the sheet. The agent is told backticks are reserved for such
  names; a name written bare is outside what a program can see and is left to the auditor;
- a name known only through an `approximate_*` field is not stated as fact — it must read as possible or unconfirmed.

Each finding is a typed `Violation` (source, code, where, message) and is returned to the documenter as
`revision-feedback` on the next round.

Then `repo-doc-auditor` judges whether the cited facts *support* the claims (`repo-doc-audit-contract.md`). The
whole loop is bounded (default 3 rounds); a directory that does not verify is `unverified`, kept for a human under
`.loom/runs/<run>/unverified/`, and never accepted.

Until the approval gate (ADR-010, Phase 6) a verified document is still only a **candidate**, recorded under
`.loom/runs/` and not written into the index.

## What this contract does not claim

That the purpose is *correct*. It guarantees the purpose cites facts, not that the interpretation is right — that is
what the human approval gate before anything is written into the target repository is for.
