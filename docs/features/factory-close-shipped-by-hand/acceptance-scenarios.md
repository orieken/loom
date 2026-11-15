---
feature: "factory-close-shipped-by-hand"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: factory-close-shipped-by-hand

## Close a parked order records a shipped ledger entry

Verifies: Closing a parked or stopped order with `loom factory close <order> --shipped-by <url>` appends exactly one `shipped` ledger entry carrying the URL in `url` and the approver — from the same source as `loom factory approve` — in `approvedBy`.

```gherkin
Scenario: Close a parked order records a shipped ledger entry
  Given an order "reagent-map-visual-baselines" is parked in the ledger
  When the operator runs "loom factory close reagent-map-visual-baselines --shipped-by https://github.com/orieken/saturday-monorepo/pull/23"
  Then the ledger gains exactly one new entry with phase "shipped"
  And that entry's "url" field holds "https://github.com/orieken/saturday-monorepo/pull/23"
  And that entry's "approvedBy" field holds the same approver identity that "loom factory approve" records
```

## Close a stopped order records a shipped ledger entry

Verifies: Closing a parked or stopped order with `loom factory close <order> --shipped-by <url>` appends exactly one `shipped` ledger entry carrying the URL in `url` and the approver — from the same source as `loom factory approve` — in `approvedBy`.

```gherkin
Scenario: Close a stopped order records a shipped ledger entry
  Given an order "order-x" is stopped in the ledger
  When the operator runs "loom factory close order-x --shipped-by https://github.com/example/repo/pull/1"
  Then the ledger gains exactly one new entry with phase "shipped"
  And that entry's "url" field holds the given URL
  And that entry's "approvedBy" field holds the approver identity
```

## Shipped order appears in status with its URL and drops out of the inbox

Verifies: After that entry, `loom factory status` shows the order as `shipped` with the URL in its detail column, and `loom factory inbox` no longer lists it.

```gherkin
Scenario: Shipped order appears in status with its URL and drops out of the inbox
  Given order "order-x" was just closed with "--shipped-by https://example.com/pr/1"
  When the operator runs "loom factory status"
  And the operator runs "loom factory inbox"
  Then "loom factory status" lists order "order-x" with phase "shipped" and its detail column showing "https://example.com/pr/1"
  And "loom factory inbox" no longer lists order "order-x"
```

## A dependent order is claimed once its dependency ships

Verifies: An order whose `depends-on` names the closed order is claimed by the supervisor on its next pass, with no edit to either order.

```gherkin
Scenario: A dependent order is claimed once its dependency ships
  Given order "order-y" has "depends-on: order-x"
  And order "order-x" is parked
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  And the supervisor runs its next pass
  Then the supervisor claims order "order-y"
  And neither order-x's nor order-y's definitions were edited
```

## Closing archives the run state and removes the worktree in order

Verifies: Closing an order archives its run state and removes its worktree, in the same order as the supervisor's own ship: record the ledger line first, archive, then remove; if archiving fails, the worktree is kept and the command reports why.

```gherkin
Scenario: Closing archives the run state and removes the worktree in order
  Given order "order-x" is parked with an active worktree
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the shipped ledger entry is recorded before the run state is archived
  And the run state is archived before the worktree is removed
  And the worktree no longer exists afterward
```

## A failed archive keeps the worktree and explains why

Verifies: Closing an order archives its run state and removes its worktree, in the same order as the supervisor's own ship: record the ledger line first, archive, then remove; if archiving fails, the worktree is kept and the command reports why.

```gherkin
Scenario: A failed archive keeps the worktree and explains why
  Given order "order-x" is parked
  And archiving the run state will fail
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the shipped ledger entry is still recorded
  And the worktree is kept
  And the command reports the reason archiving failed
```

## close refuses a running order

Verifies: `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.

```gherkin
Scenario: close refuses a running order
  Given order "order-x" is running
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message names the phase "running"
  And the message instructs the operator to stop the order first
  And the ledger is unchanged
```

## close refuses a retrying order

Verifies: `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.

```gherkin
Scenario: close refuses a retrying order
  Given order "order-x" is retrying
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message names the phase "retrying"
  And the message instructs the operator to stop the order first
  And the ledger is unchanged
```

## close refuses an order waiting at a gate

Verifies: `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.

```gherkin
Scenario: close refuses an order waiting at a gate
  Given order "order-x" is waiting at a gate
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message names the order's waiting-at-a-gate phase
  And the message instructs the operator to stop the order first
  And the ledger is unchanged
```

## close refuses an order waiting on commit or PR approval

Verifies: `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.

```gherkin
Scenario: close refuses an order waiting on commit or PR approval
  Given order "order-x" is waiting on a commit or PR approval, covering committed, pushed, and proposed phases
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message names the order's current phase
  And the message instructs the operator to stop the order first
  And the ledger is unchanged
```

## close refuses an order that has already shipped

Verifies: `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.

```gherkin
Scenario: close refuses an order that has already shipped
  Given order "order-x" has already shipped
  When the operator runs "loom factory close order-x --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message names the phase "shipped" and notes it has already shipped
  And the ledger is unchanged
```

## close refuses an order the ledger does not hold

Verifies: `close` refuses an order the ledger does not hold, or a `--shipped-by` value that is missing or not an `http`/`https` URL, with exit code 1 and a message naming the problem; the ledger does not change.

```gherkin
Scenario: close refuses an order the ledger does not hold
  Given no order "ghost-order" exists in the ledger
  When the operator runs "loom factory close ghost-order --shipped-by https://example.com/pr/1"
  Then the command exits with code 1
  And the message states no order "ghost-order" exists in the ledger
  And the ledger is unchanged
```

## close refuses when --shipped-by is missing

Verifies: `close` refuses an order the ledger does not hold, or a `--shipped-by` value that is missing or not an `http`/`https` URL, with exit code 1 and a message naming the problem; the ledger does not change.

```gherkin
Scenario: close refuses when --shipped-by is missing
  Given order "order-x" is parked
  When the operator runs "loom factory close order-x" without "--shipped-by"
  Then the command exits with code 1 before taking any action
  And the message names "--shipped-by" as required
  And the ledger is unchanged
```

## close refuses a --shipped-by value that is not an http or https URL

Verifies: `close` refuses an order the ledger does not hold, or a `--shipped-by` value that is missing or not an `http`/`https` URL, with exit code 1 and a message naming the problem; the ledger does not change.

```gherkin
Scenario: close refuses a --shipped-by value that is not an http or https URL
  Given order "order-x" is parked
  When the operator runs "loom factory close order-x --shipped-by ftp://example.com/file"
  Then the command exits with code 1
  And the message names "--shipped-by" as the problem
  And the ledger is unchanged
```

## Closing trusts the human-provided URL without verifying the pull request

Verifies: `close` never calls GitHub and never checks that the named pull request merged; the human running it vouches for that, and `approvedBy` records who.

```gherkin
Scenario: Closing trusts the human-provided URL without verifying the pull request
  Given order "order-x" is parked
  And the pull request at the given URL has not actually merged
  When the operator runs "loom factory close order-x --shipped-by https://github.com/example/repo/pull/99"
  Then the command completes without making any call to GitHub
  And the shipped ledger entry's "approvedBy" field records the operator who vouched for the merge
```

## close re-reads the ledger and refuses if the phase changed before it could append

Verifies: `close` and the supervisor never both act on the same order: `close` reads the ledger fresh and validates the order's current phase before appending, the same TOCTOU-tolerant pattern `approve`, `stop` and `rerun` already use.

```gherkin
Scenario: close re-reads the ledger and refuses if the phase changed before it could append
  Given order "order-x" was parked when the close command started
  And the supervisor transitions order "order-x" to "running" before close appends its entry
  When close re-reads the ledger and validates the order's current phase immediately before appending
  Then close refuses the close because the order is no longer parked or stopped
  And the ledger is unchanged by the close attempt
```

## The intake README documents when to use close

Verifies: `templates/claude-feature-team/intake/README.md` documents when to use `close`: after an order parks and a person finishes the work in their own pull request, once that PR merges.

```gherkin
Scenario: The intake README documents when to use close
  Given "templates/claude-feature-team/intake/README.md" exists
  When a person reads the README looking for guidance on closing an order
  Then it explains that "close" is used after an order parks and a person finishes the work in their own pull request, once that PR merges
```

## The change set meets the project's quality and scope gates

Verifies: Every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass, and no file outside `scope.paths` changes except this run's own delivery record under `docs/features/factory-close-shipped-by-hand/`.

```gherkin
Scenario: The change set meets the project's quality and scope gates
  Given the completed implementation for this feature
  When "go test ./..." runs
  And the project's lint/complexity tool runs
  Then every new or changed function has cyclomatic complexity below 7
  And every changed line is covered by a test that fails when the behavior it checks is broken
  And both commands pass
  And no file outside "scope.paths" changed, except this run's delivery record under "docs/features/factory-close-shipped-by-hand/"
```
