---
intake: 1
id: factory-idle-message
type: feature
plan: deliver-feature
status: ready
priority: 3
created: 2026-10-25
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
  max-hours: 2
scope:
  paths: [internal/factory/**]
---

# Say when the factory supervisor is idle or waiting on a human

## Request

> After the second dogfood run shipped both orders, `loom factory run` printed nothing more. Twice the
> person running it reported it as "hanging" or "just sitting here": once while an order waited at
> `confirm-design`, once with every order shipped.

## Summary

The supervisor prints a line for each ledger event, and nothing when a tick changes nothing. A tick
that leaves it with no work of its own should say why, once, so a person can tell an idle supervisor
from a hung one. There are two such states:
- **Idle:** no order is running or claimable.
- **Waiting:** one or more orders are halted at a gate or in the inbox for a human.

## Acceptance Criteria

- [ ] Given a tick ends with no order running, none claimable, and none waiting on a human, when the
      supervisor was not already idle, then it prints exactly
      `idle — no ready orders; commit one to intake/ on <branch>`, with the intake branch it reads
      filled in.
- [ ] Given a tick ends with no order running and at least one order waiting on a human, then it
      prints `waiting on a human — <n> order(s) in the inbox: loom factory inbox`, where n counts
      those orders. A gate halt, a commit proposal, a PR proposal and a parked order each count.
- [ ] Given consecutive ticks in the same state, then the line is printed once, when the state is
      entered. It is never repeated on every tick.
- [ ] Given the supervisor leaves the state, for example because an order is claimed or an approval
      resumes a run, and later re-enters it, then the line is printed again. A change in the waiting
      count while waiting prints the line again with the new count.
- [ ] Given `loom factory run --once`, then the same rule applies to its single pass. It prints the
      line at most once.
- [ ] Given the change is complete, then `go test ./internal/factory/...` passes. Every new function
      has cyclomatic complexity below 7. Every changed line is covered. No file outside
      `internal/factory/` changes, the run's own delivery record under
      `docs/features/factory-idle-message/` excepted.

## Out of Scope

- Changing any ledger event, or recording idleness in the ledger. It is a console message only.
- A periodic heartbeat while work is running.

## Domain Language

- **Idle**: the supervisor has nothing to run and nothing waiting on a human.
- **Waiting on a human**: every order that is not shipped or idle is halted for an approval or an
  edit.

## Non-Functional Requirements

- Determining the state reads only the ledger replay and the intake the tick already loaded. It
  makes no extra git or filesystem pass per tick.

## Trust Boundaries

None.

## Test Approach

Go unit tests in `internal/factory` using the existing test factory and fake launcher. Drive ticks
through each transition and assert on the supervisor's output buffer.

## Open Questions

- Whether a `retrying` order, waiting on its backoff, counts as waiting on a human. Assumption if
  unresolved: no. It is waiting on time, so the supervisor is neither idle nor waiting on a human,
  and prints nothing.

## Infrastructure / Deploy Notes

None.
