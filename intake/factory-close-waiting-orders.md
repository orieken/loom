---
intake: 1
id: factory-close-waiting-orders
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-11-18
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: [factory-close-shipped-by-hand]
budget:
  max-attempts: 2
  max-usd: 15
  max-hours: 2
scope:
  paths: [internal/factory/**, cmd/loom/cmd/factory*.go, templates/claude-feature-team/intake/README.md, docs/features/factory-close-waiting-orders/**]
---

# Close a hand-shipped order that is waiting on a human

## Request

> Lets fix both: `close` should also work on an order waiting at a gate.

## Summary

`loom factory close <order> --shipped-by <url>` accepts only a `parked` or `stopped` order. In the
saturday-monorepo pilot, order `reagent-map-save` was finished by hand in PR #30 while the factory
re-ran its QA on its own copy. The factory's run was then waiting at `confirm-ship`. `close` refused
it, so the only way out was to approve the commit and the PR, then close the duplicate PR unmerged.

Let `close` also accept an order that is waiting on a human: at a run gate, at the commit, or at the
pull request. None of those has a process running.

## Acceptance Criteria

- [ ] Given an order waiting at a run gate (`waiting`), waiting for its commit to be approved
      (`completed`), or waiting for its pull request to be approved (`proposed`), when a person runs
      `loom factory close <order> --shipped-by <url>`, then the order is recorded as shipped exactly as
      for a `parked` order today: one `shipped` entry with the URL and `approvedBy`, then the run
      archived and the worktree removed.
- [ ] Given that order, then `loom factory inbox` no longer lists it, nothing is committed or pushed
      for it, and no pull request is opened for it, even if an approval for it was already in the
      ledger and not yet acted on.
- [ ] Given an order whose run process is alive (`running`), or one between a commit and its pull
      request (`committed`, `pushed`), or one scheduled to retry (`retrying`), then `close` still refuses
      with exit code 1 and names the phase. The ledger does not change. Already-shipped orders are
      refused as today.
- [ ] Given the supervisor ticking while `close` runs on a waiting order, then the supervisor never
      relaunches, commits or proposes the closed order afterwards. Prove it with a test that closes a
      waiting order and runs one more supervisor tick.
- [ ] Given `templates/claude-feature-team/intake/README.md`, then its `close` section names the
      phases it accepts.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass,
      and no file outside `scope.paths` changes, the run's own delivery record under
      `docs/features/factory-close-waiting-orders/` excepted.

## Out of Scope

- Closing a running order without stopping it first.
- Closing an order that did not ship.

## Domain Language

- No new terms introduced. Phases as in ADR-010 and `internal/factory/replay.go`.

## Non-Functional Requirements

- None beyond the existing ledger guarantees.

## Trust Boundaries

- None new. The URL is stored and printed, never fetched.

## Test Approach

Unit tests beside `closeable`'s existing ones in `internal/factory/control_test.go`, one per phase,
plus the supervisor-tick test above.

## Open Questions

- Should `retrying` be closeable? Assumption if unresolved: no. A retry can relaunch at any tick,
  so a person stops it first, as for `running`.

## Infrastructure / Deploy Notes

- None
