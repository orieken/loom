---
feature: "factory-idle-message"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: factory-idle-message

## Supervisor reports idle when intake has never had a committed order

Verifies: When a tick ends with no order running, no order claimable, and no order waiting on a human, and the supervisor was not already idle, it MUST print exactly 'idle — no ready orders; commit one to intake/ on <branch>', with the configured intake branch filled in.

```gherkin
Scenario: Supervisor reports idle when intake has never had a committed order
  Given the intake branch is configured as "main"
  And intake/ has never had a committed order
  And the supervisor was not already idle
  When a tick ends with no order running, no order claimable, and no order waiting on a human
  Then the supervisor prints exactly "idle — no ready orders; commit one to intake/ on main"
```

## Supervisor reports idle when every claimed order has shipped

Verifies: When a tick ends with no order running, no order claimable, and no order waiting on a human, and the supervisor was not already idle, it MUST print exactly 'idle — no ready orders; commit one to intake/ on <branch>', with the configured intake branch filled in.

```gherkin
Scenario: Supervisor reports idle when every claimed order has shipped
  Given every claimed order has reached the shipped phase
  And intake has no new ready candidate
  And the supervisor was not already idle
  When a tick ends with no order running, no order claimable, and no order waiting on a human
  Then the supervisor prints exactly "idle — no ready orders; commit one to intake/ on main"
```

## Supervisor reports one order waiting when a single order is halted at a gate

Verifies: When a tick ends with no order running and at least one order waiting on a human, it MUST print 'waiting on a human — <n> order(s) in the inbox: loom factory inbox', where n counts a gate halt, a commit proposal, a PR proposal, and a parked order.

```gherkin
Scenario: Supervisor reports one order waiting when a single order is halted at a gate
  Given one order is halted at a gate awaiting human confirmation
  And no other order is running
  When a tick ends with no order running and at least one order waiting on a human
  Then the supervisor prints exactly "waiting on a human — 1 order(s) in the inbox: loom factory inbox"
```

## Supervisor counts a gate halt and a parked order together

Verifies: When a tick ends with no order running and at least one order waiting on a human, it MUST print 'waiting on a human — <n> order(s) in the inbox: loom factory inbox', where n counts a gate halt, a commit proposal, a PR proposal, and a parked order.

```gherkin
Scenario: Supervisor counts a gate halt and a parked order together
  Given one order is halted at a gate awaiting human confirmation
  And one additional order is parked
  And no other order is running
  When a tick ends with no order running and at least one order waiting on a human
  Then the supervisor prints exactly "waiting on a human — 2 order(s) in the inbox: loom factory inbox"
```

## Idle line prints only on the tick that enters the idle state

Verifies: Given consecutive ticks remain in the same state, the line MUST print exactly once, on the tick that enters the state, and never again while the state persists unchanged.

```gherkin
Scenario: Idle line prints only on the tick that enters the idle state
  Given the supervisor's previous tick entered the idle state and printed the idle line
  When a second consecutive tick ends in the same unchanged idle state
  Then the idle line does not print again on the second tick
```

## Waiting line prints only on the tick that enters the waiting state

Verifies: Given consecutive ticks remain in the same state, the line MUST print exactly once, on the tick that enters the state, and never again while the state persists unchanged.

```gherkin
Scenario: Waiting line prints only on the tick that enters the waiting state
  Given the supervisor's previous tick entered the waiting state with one order in the inbox and printed the waiting line
  When a second consecutive tick ends in the same unchanged waiting state with the same count
  Then the waiting line does not print again on the second tick
```

## Idle line prints again after the supervisor leaves and re-enters idle

Verifies: Given the supervisor leaves idle or waiting and later re-enters it, the line MUST print again; while waiting, a change in the waiting count MUST print the line again with the new count.

```gherkin
Scenario: Idle line prints again after the supervisor leaves and re-enters idle
  Given the supervisor printed the idle line once
  And the supervisor then left idle by claiming and shipping an order
  And the supervisor later has no ready orders again
  When a tick ends in the idle state for a second time
  Then the idle line prints again
```

## Waiting line reprints with the new count when the waiting count changes

Verifies: Given the supervisor leaves idle or waiting and later re-enters it, the line MUST print again; while waiting, a change in the waiting count MUST print the line again with the new count.

```gherkin
Scenario: Waiting line reprints with the new count when the waiting count changes
  Given the supervisor is in the waiting state with one order in the inbox
  And the waiting line already printed once for count 1
  When a second order enters a waiting phase within the same waiting state
  Then the supervisor prints the waiting line again with count 2
```

## run --once prints the idle or waiting line at most once

Verifies: Given loom factory run --once, the same once-per-entry rule MUST apply to its single tick, so the line prints at most once for that invocation.

```gherkin
Scenario: run --once prints the idle or waiting line at most once
  Given the supervisor is invoked via "loom factory run --once"
  When the single tick ends in the idle or waiting state
  Then the idle or waiting line prints at most once for that invocation
```

## The idle-message change meets the project's quality and boundary constraints

Verifies: The change MUST pass go test ./internal/factory/..., every new function MUST have cyclomatic complexity below 7, every changed line MUST be covered, and no file outside internal/factory/ MUST change except the run's own delivery record under docs/features/factory-idle-message/.

```gherkin
Scenario: The idle-message change meets the project's quality and boundary constraints
  Given the idle-message feature is implemented in internal/factory/
  When go test ./internal/factory/... is run
  Then all tests pass
  And every new function has cyclomatic complexity below 7
  And every changed or added line is covered by a test that can fail
  And no file outside internal/factory/ changed, other than the run's own delivery record under docs/features/factory-idle-message/
```
