---
intake: 1
id: factory-close-shipped-by-hand
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-11-14
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
  paths: [internal/factory/**, cmd/loom/cmd/factory*.go, templates/claude-feature-team/intake/README.md, docs/features/factory-close-shipped-by-hand/**]
---

# Close an order that shipped by hand

## Request

> How do we clear the parked item? Add the small loom order for the fix.

## Summary

In the saturday-monorepo pilot, orders have parked and then been finished by hand in their own
pull requests. The ledger still holds each one as `parked`. Two problems follow:

- `loom factory inbox` lists a parked order on every call, so the finished order stays there for good.
- `depends-on` waits for the phase `shipped`, which only the supervisor records, after it opens its own
  PR. An order that depends on a hand-finished one never starts, so the dependency had to be removed
  from the dependent order by hand.

Add `loom factory close <order> --shipped-by <url>`. It records that a person shipped the order outside
the factory, in the pull request at `<url>`. From then on the order is `shipped`, as if the supervisor
had shipped it. Today the only other fix is to edit the ledger by hand, and the ledger is the audit
record.

## Acceptance Criteria

- [ ] Given a `parked` or `stopped` order, when a person runs `loom factory close <order> --shipped-by
      <url>`, then the ledger gets one `shipped` entry for the order, with the URL in `url` and the
      person in `approvedBy`. The approver comes from the same source as `loom factory approve`.
- [ ] Given that entry, then `loom factory status` shows the order as `shipped` with the URL in its
      detail column, and `loom factory inbox` no longer lists it.
- [ ] Given an order that lists the closed order in `depends-on`, then the supervisor claims it on its
      next pass, with no edit to either order.
- [ ] Given a closed order, then its run state is archived and its worktree is removed, the same way and
      in the same order as the supervisor's `ship`: record first, then archive, then remove. If the
      archive fails, the worktree is kept and the command says so.
- [ ] Given an order that is running, retrying, waiting at a gate, or waiting on a commit or PR approval,
      then `close` refuses with exit code 1, names the phase, and says to stop it first. The ledger does
      not change. The same applies to an order that has already shipped.
- [ ] Given an order the ledger does not hold, or a `--shipped-by` value that is missing or is not an
      `http` or `https` URL, then `close` refuses with exit code 1 and a message naming the problem. The
      ledger does not change.
- [ ] Given `close`, then it does not call GitHub or check that the PR merged. The person running it
      vouches for that, and `approvedBy` records who.
- [ ] Given the supervisor running while `close` runs, then the two never both act on the order. Use the
      same lock or ledger rule that `approve`, `stop` and `rerun` already use.
- [ ] Given `templates/claude-feature-team/intake/README.md`, then it says when to use `close`: after an
      order parks and a person finishes the work in their own PR, once that PR merges.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass,
      and no file outside `scope.paths` changes, the run's own delivery record under
      `docs/features/factory-close-shipped-by-hand/` excepted.

## Out of Scope

- Closing an order that did not ship, such as an abandoned one. That needs its own phase, because a
  dependent order must not start after it.
- Checking the PR on GitHub.
- Re-opening a closed order.

## Domain Language

- No new terms introduced. "Ship" and "park" keep their meanings in ADR-010.

## Non-Functional Requirements

- None beyond the existing ledger guarantees: one appended line, written atomically.

## Trust Boundaries

- The URL is stored and printed, never fetched or opened.

## Test Approach

Unit tests in `internal/factory` beside `control.go`'s existing tests for `RequestStop`, `RequestResume`
and `RequestRerun`, and a command test in `cmd/loom/cmd` like the other factory commands.

## Open Questions

- Should `close` also accept `--reason` text? Assumption if unresolved: no. The URL is the reason.

## Infrastructure / Deploy Notes

- Once this ships, in saturday-monorepo: `loom factory close reagent-map-visual-baselines --shipped-by
  https://github.com/orieken/saturday-monorepo/pull/23`.
