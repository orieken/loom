# Contract: repo-doc-audit (typed state)

**Produced by**: repo-doc-auditor
**Consumed by**: `loom document` (the revision loop's exit condition) and the human reading a run record

## Typed State

Typed state, not markdown: the stage returns JSON conforming to
`shared/schemas/pipeline/repo-doc-audit.schema.json` (generated from `internal/state/repodoc_audit_state.go` — never
hand-edit it). The loop reads the `verdict` field; nothing parses prose. There are no headings to check.

## Position in the loop

For each directory, `loom document` runs, in order:

1. the **documenter** (`repo-documenter`) — a candidate;
2. the **verifier** — deterministic Go checks of every name, tag and citation against the fact sheet;
3. only if the verifier finds nothing, **this auditor** — whether the cited facts support the claims;
4. on any problem, back to step 1 with the findings as typed feedback, up to a fixed round limit (default 3).

A directory that reaches the limit with problems outstanding is `unverified`: the last attempt is kept for a human and
nothing is accepted.

## Input

| Block | Content |
|---|---|
| `fact-sheet` | the directory's fact sheet, verbatim |
| `child-summaries` | verified summaries of child directories, or `undocumented` |
| `candidate` | the documenter's `repo-doc` result |

## Required Fields

| Field | Rule |
|---|---|
| `schemaVersion` | the build's state schema version |
| `path` | the directory audited; the executor rejects a mismatch |
| `inputHash` | the fact sheet's `input_hash`; the executor rejects a mismatch |
| `verdict` | `APPROVED` or `CHANGES_REQUESTED` |
| `findings` | may be empty only with `APPROVED`, and see below |

Each finding has a `claim` (as written in the document), a `problem`, an `instruction` and an optional `blocking`.

## Validation Rules

Checked in Go on receipt:

1. `path`, `inputHash` and `schemaVersion` are present and correct.
2. Every finding has a claim, a problem and an instruction.
3. `CHANGES_REQUESTED` has at least one **blocking** finding — a rejection with nothing to act on would spin the loop.
4. `APPROVED` has no blocking finding — a verdict that contradicts itself is rejected.

Only blocking findings cost a round; a non-blocking one is recorded and ignored by the loop.

## What this contract does not claim

That an `APPROVED` document is *true*. It means the cited facts support the claims, as judged by a model — the
deterministic verifier checks existence, this checks support, and the human approval gate before anything is written
into a target repository (ADR-010, Phase 6) is what checks the rest.
