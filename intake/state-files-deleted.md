---
intake: 1
id: state-files-deleted
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-11-23
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 20
  max-hours: 3
scope:
  paths: [internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**]
---

# Let a stage report the files it deleted or renamed

## Request

> Yes: a small loom change so an order that renames or deletes files doesn't park on its own correct
> work.

## Summary

A developer state lists `filesCreated` and `filesModified`, and loom checks that every listed path
exists (`internal/orchestrator/claims.go`). There is no field for a file the stage removed. In
saturday-monorepo, order `golden-site-file-names` renamed seven files and listed them as
`old -> new` in `filesModified`. The claim check found no file by that name and parked the order as
"work it did not do", although the work was done.

Add `filesDeleted` to the implementation state, and `testFilesDeleted` to the QA state. The claim
check verifies the opposite way for these: each path must not exist now, and must have existed when
the stage started. `PathClaim.MustExist` already leaves room for this.

## Acceptance Criteria

- [ ] Given `shared/schemas/pipeline/implementation.schema.json` and the QA schema, then each has an
      optional `filesDeleted` or `testFilesDeleted` array of paths. The description says a rename is
      the new path in `filesCreated` and the old path in `filesDeleted`.
- [ ] Given a stage that lists a path in `filesDeleted`, when the path is absent now and existed when
      the stage started (tracked at the run's base commit, or in the tree the posture check saw), then
      the claim passes.
- [ ] Given a path in `filesDeleted` that still exists, then the stage fails with a `ClaimError` naming
      the field and the path: "the file still exists — the stage reported a deletion it did not do".
- [ ] Given a path in `filesDeleted` that never existed, then the stage fails the same way: "no such
      file was there to delete".
- [ ] Given an entry in any files field that contains ` -> `, then validation fails with a message
      saying to list a rename as the new path in `filesCreated` and the old one in `filesDeleted`.
- [ ] Given `requireTouchedFiles`, then an implementation that only deletes files counts as having
      touched something.
- [ ] Given existing state documents with no deleted-files field, then they validate exactly as today.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass,
      and no file outside `scope.paths` changes, the run's own delivery record under
      `docs/features/state-files-deleted/` excepted.

## Out of Scope

- Git rename detection. The stage says what it did, and loom checks it.

## Domain Language

- No new terms introduced.

## Non-Functional Requirements

- None.

## Trust Boundaries

- None. Paths are checked inside the run's own worktree only.

## Test Approach

Unit tests beside `internal/orchestrator/claims.go`'s and `internal/state`'s existing ones: a real
deletion, a deletion that did not happen, a path that never existed, a rename reported both ways,
and an `old -> new` entry.

## Open Questions

- None.

## Infrastructure / Deploy Notes

- None.
