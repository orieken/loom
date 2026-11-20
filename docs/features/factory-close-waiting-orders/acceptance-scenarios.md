---
feature: "factory-close-waiting-orders"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: factory-close-waiting-orders

## Closing an order waiting at a run gate records it shipped and archives its run

Verifies: Given an order waiting at a run gate (waiting), waiting for its commit to be approved (completed), or waiting for its pull request to be approved (proposed), closing it with `loom factory close <order> --shipped-by <url>` records it shipped exactly as a parked order today: one shipped ledger entry with the URL and approvedBy, then the run archived and the worktree removed.

```gherkin
Scenario: Closing an order waiting at a run gate records it shipped and archives its run
  Given An order is waiting at a run gate
  When An operator closes the order with loom factory close <order> --shipped-by <url>
  Then Exactly one shipped ledger entry is recorded carrying the URL and the approver
  And The order's run is archived
  And The order's worktree is removed
```

## Closing an order waiting for commit approval records it shipped and archives its run

Verifies: Given an order waiting at a run gate (waiting), waiting for its commit to be approved (completed), or waiting for its pull request to be approved (proposed), closing it with `loom factory close <order> --shipped-by <url>` records it shipped exactly as a parked order today: one shipped ledger entry with the URL and approvedBy, then the run archived and the worktree removed.

```gherkin
Scenario: Closing an order waiting for commit approval records it shipped and archives its run
  Given An order is waiting for its commit to be approved
  When An operator closes the order with loom factory close <order> --shipped-by <url>
  Then Exactly one shipped ledger entry is recorded carrying the URL and the approver
  And The order's run is archived
  And The order's worktree is removed
```

## Closing an order waiting for pull request approval records it shipped and archives its run

Verifies: Given an order waiting at a run gate (waiting), waiting for its commit to be approved (completed), or waiting for its pull request to be approved (proposed), closing it with `loom factory close <order> --shipped-by <url>` records it shipped exactly as a parked order today: one shipped ledger entry with the URL and approvedBy, then the run archived and the worktree removed.

```gherkin
Scenario: Closing an order waiting for pull request approval records it shipped and archives its run
  Given An order is waiting for its pull request to be approved
  When An operator closes the order with loom factory close <order> --shipped-by <url>
  Then Exactly one shipped ledger entry is recorded carrying the URL and the approver
  And The order's run is archived
  And The order's worktree is removed
```

## Closing a waiting, completed, or proposed order removes it from the inbox and takes no further action, even with a stale unacted-on approval

Verifies: After closing a waiting, completed, or proposed order, `loom factory inbox` no longer lists it, nothing is committed or pushed for it, and no pull request is opened for it — even if an approval for it was already in the ledger and not yet acted on.

```gherkin
Scenario: Closing a waiting, completed, or proposed order removes it from the inbox and takes no further action, even with a stale unacted-on approval
  Given An order is waiting, waiting for commit approval, or waiting for pull request approval
  And The order may already carry an approval in the ledger that has not yet been acted on
  When The order is closed with loom factory close <order> --shipped-by <url>
  Then loom factory inbox no longer lists the order
  And Nothing is committed or pushed for the order
  And No pull request is opened for the order
```

## Closing an order whose run is still active, mid-flight, or scheduled to retry is refused

Verifies: An order whose run process is alive (running), one between a commit and its pull request (committed, pushed), or one scheduled to retry (retrying) still refuses close with exit code 1 and the phase named; the ledger does not change. An already-shipped order is refused as today.

```gherkin
Scenario: Closing an order whose run is still active, mid-flight, or scheduled to retry is refused
  Given An order's run process is alive, or the order is between a commit and its pull request, or the order is scheduled to retry
  When An operator attempts to close the order with loom factory close <order> --shipped-by <url>
  Then The close command exits with code 1
  And The refusal message names the order's current phase
  And The ledger does not change
```

## Closing an already-shipped order is refused

Verifies: An order whose run process is alive (running), one between a commit and its pull request (committed, pushed), or one scheduled to retry (retrying) still refuses close with exit code 1 and the phase named; the ledger does not change. An already-shipped order is refused as today.

```gherkin
Scenario: Closing an already-shipped order is refused
  Given An order has already shipped
  When An operator attempts to close the order with loom factory close <order> --shipped-by <url>
  Then The close command exits with code 1
  And The refusal message states the order has already shipped
  And The ledger does not change
```

## A supervisor tick after closing a waiting order takes no further action on it

Verifies: With the supervisor ticking, closing a waiting order and then running one more supervisor tick never relaunches, commits, or proposes that order again.

```gherkin
Scenario: A supervisor tick after closing a waiting order takes no further action on it
  Given An order was waiting and has been closed with --shipped-by
  And The supervisor is running its regular tick loop
  When The supervisor runs one more tick
  Then The order is not relaunched
  And No commit is made for the order
  And No pull request is opened for the order
  And No new ledger entry is appended for the order
```

## The intake README's close section lists every phase close accepts

Verifies: templates/claude-feature-team/intake/README.md's close section names every phase close now accepts (parked, stopped, waiting, completed, proposed).

```gherkin
Scenario: The intake README's close section lists every phase close accepts
  Given The intake README at templates/claude-feature-team/intake/README.md documents loom factory close
  When A reader reviews the close section's list of acceptable phases
  Then The list names parked, stopped, waiting, completed, and proposed
```

## The implementation meets the project's complexity, coverage, and lint requirements

Verifies: Every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, go test ./... and the lint pass, and no file outside scope.paths changes except this run's own delivery record under docs/features/factory-close-waiting-orders/.

```gherkin
Scenario: The implementation meets the project's complexity, coverage, and lint requirements
  Given The feature's implementation changes are complete
  When go test ./... and the project's lint and complexity checks run
  Then Every new or changed function has cyclomatic complexity below 7
  And Every changed line is covered by a test that fails when the behavior it checks is broken
  And go test ./... passes
  And The lint check passes
  And No file outside scope.paths changes except this run's own delivery record under docs/features/factory-close-waiting-orders/
```
