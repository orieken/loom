---
intake: 1
id: reclaim-onto-claimed-commit
type: bug
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-28
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 40
  max-hours: 3
scope:
  paths: [internal/factory/**, internal/worktree/**, internal/orchestrator/**, cmd/loom/cmd/**, docs/**]
---

# Run a re-claimed order on the commit it was claimed at

## Request

> `vocabulary-mismatch-message` was re-claimed at `f8dc4ba`, which added
> `.claude/delivery-policy.yaml`. Its pre-ship-checks stage still reported "no preShipChecks
> configured": the ledger said the claim was at `f8dc4ba`, but the run's code was the
> `factory/vocabulary-mismatch-message` branch cut from `2e8284c` three days earlier. A re-claim
> takes the new order snapshot and nothing else from the commit it claims, so a fix or config
> committed to `main` after the first claim never reaches the order.

## Summary

The supervisor prepares a run's worktree in `prepare` (`internal/factory/start.go`) through
`AddWorktree` (`internal/worktree/manage.go`). That call returns early when the worktree exists, and
checks out the existing `factory/<id>` branch as it is. A first claim branches from
`claim.BaseBranch`'s tip at launch, not from `claim.SourceCommit`. Make every claim run on the commit
it records:
- A first claim branches from `SourceCommit`.
- A re-claim merges `SourceCommit` into the order's branch before the run continues.
- The run measures its own changes against that merged base, so code that arrived from the merge is
  never counted as the run's work.

## Acceptance Criteria

- [ ] Given an order claimed for the first time, when its worktree is created, then `factory/<id>` is
      created at `claim.SourceCommit`, even if the base branch has moved since the claim was
      recorded.
- [ ] Given an order re-claimed at a commit its `factory/<id>` branch does not contain, when the
      supervisor prepares it, then it merges `claim.SourceCommit` into the branch in the worktree
      before launching, as a merge commit with the factory's committer fallback. Uncommitted changes
      the earlier run left in the worktree are kept.
- [ ] Given a re-claim whose branch already contains `claim.SourceCommit`, then no merge is
      attempted and no commit is made.
- [ ] Given a re-claim whose merge cannot complete, then the merge is aborted and the worktree's
      HEAD, index and working tree are left exactly as they were. "Cannot complete" means a conflict,
      or uncommitted changes the merge would overwrite. The order is parked with a reason that names
      the commit and every conflicting path. Nothing is launched.
- [ ] Given a re-claim that merged, when the run continues, then everything the run measures as its
      own diff is relative to the merged HEAD, not the run's original start commit. That covers
      `LOOM_BASE_COMMIT` for pre-ship checks and the diff lines recorded at each gate. A test shows
      that a Go file changed on `main` and merged in by the re-claim is not in the diff a pre-ship
      check measures.
- [ ] Given a re-claimed order that ships, then its pull request's "Changed files" section lists only
      the order's own changes, never the files the re-claim merged in from `main`.
- [ ] Given the change is complete, then `go test ./internal/factory/... ./internal/worktree/...
      ./internal/orchestrator/... ./cmd/loom/...` passes. Every new function has cyclomatic
      complexity below 7. Every changed line is covered as `cmd/diff-coverage` measures it. No file
      outside `scope.paths` changes, the run's own delivery record under
      `docs/features/reclaim-onto-claimed-commit/` excepted.

## Out of Scope

- Rebasing an order's branch, or rewriting its history in any other way.
- Resolving merge conflicts automatically. A conflict parks the order for a human.
- Moving a running or waiting order onto a newer commit. Only a claim moves an order's base, and only
  a parked order or one a human asked to re-run is ever re-claimed.

## Domain Language

- **Re-claim**: claiming an order already in the ledger, after its committed version changed. A
  parked order is re-claimed when edited; any other is re-claimed only after `loom factory rerun`.
- **Claim commit**: `claim.SourceCommit`, the commit the supervisor read the order at.

## Non-Functional Requirements

- Every git command the supervisor runs for this has an explicit timeout, like the existing push
  (architecture-guardrails.md #5).

## Trust Boundaries

None. The commits merged are the repository's own, at a commit the supervisor already read.

## Test Approach

Go tests against real git repositories, using the existing `internal/factory` test fixture (a real
repository, a scripted launcher) and `internal/worktree`'s test repositories. Cover:
- a first claim after the base branch has moved;
- a clean re-claim merge;
- a re-claim of a branch that already contains the claim commit;
- a conflicting merge;
- a merge blocked by uncommitted changes;
- the measured diff after a merge.

## Open Questions

- How the merged base reaches `loom run`. Assumption if unresolved: the supervisor passes it on the
  resume, and the run records it in place of its start commit, so the state keeps saying what the run
  measures against.

## Infrastructure / Deploy Notes

None.
