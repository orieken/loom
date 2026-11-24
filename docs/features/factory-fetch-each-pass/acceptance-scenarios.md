---
feature: "factory-fetch-each-pass"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Acceptance Scenarios: factory-fetch-each-pass

## A pass fetches the configured remote once before reading orders

Verifies: Given a pass, when it starts, the supervisor MUST run `git fetch <remote> <branch>` exactly once and read intake/ at `<remote>/<branch>`. `--remote` defaults to `origin`.

```gherkin
Scenario: A pass fetches the configured remote once before reading orders
  Given no --remote flag is given, so the remote defaults to origin
  And the branch under management is main
  When a supervisor pass starts
  Then the supervisor fetches origin main exactly once during the pass
  And intake is read from the remote-tracking ref origin/main, not the bare local branch
```

## An order promoted on the remote only is claimed without a local pull

Verifies: Given an order promoted to `ready` on the remote only, the next pass MUST claim it with no pull by a person, and the claim's `sourceCommit` MUST equal the fetched commit.

```gherkin
Scenario: An order promoted on the remote only is claimed without a local pull
  Given an order is promoted to ready and pushed to the remote
  And no person has pulled that promotion into the local checkout
  When the next supervisor pass runs
  Then the order is claimed
  And the claim's sourceCommit equals the commit the pass fetched
```

## A claimed order with no base-branch starts its worktree from the fetched remote branch

Verifies: Given a claim, its worktree's branch MUST start from `<remote>/<branch>` at that commit when the order sets no `base-branch` of its own.

```gherkin
Scenario: A claimed order with no base-branch starts its worktree from the fetched remote branch
  Given an order sets no base-branch of its own
  When the order is claimed
  Then the order's worktree branch starts from <remote>/<branch> at the commit the pass fetched
```

## A pass never disturbs the person's own checkout

Verifies: Given the person's checkout, a pass MUST NOT change its branch, index, or working tree, including uncommitted changes and local unpushed commits.

```gherkin
Scenario: A pass never disturbs the person's own checkout
  Given the person's checkout has uncommitted changes
  And the person's checkout has local commits not yet pushed
  When a supervisor pass runs, including fetching the remote
  Then the checkout's current branch, index, and working tree remain exactly as they were before the pass
  And the uncommitted changes and unpushed commits are untouched
```

## A failed fetch does not stop the pass or any order

Verifies: Given a fetch that fails (no network, no remote by that name, bad credentials), the pass MUST go on from the last fetched `<remote>/<branch>` and print exactly one line saying the fetch failed and why. It MUST NOT park or fail any order.

```gherkin
Scenario: A failed fetch does not stop the pass or any order
  Given the configured remote cannot be reached, such as no network, no remote by that name, or bad credentials
  When a supervisor pass runs
  Then the pass continues using the last successfully fetched <remote>/<branch>
  And exactly one line is printed stating the fetch failed and why
  And no order is parked or failed because the fetch failed
```

## No remote by the configured name reads the local branch unchanged

Verifies: Given a repository with no remote by the configured name, or `--no-fetch`, the supervisor MUST read the local `<branch>` exactly as today, with no fetch attempted and no new output. Every existing test MUST keep passing with no change to what it asserts.

```gherkin
Scenario: No remote by the configured name reads the local branch unchanged
  Given the repository has no remote by the configured name
  When a supervisor pass runs
  Then no fetch is attempted
  And intake is read from the local branch exactly as before this change
  And no new output is produced
```

## Passing --no-fetch reads the local branch unchanged even when a remote exists

Verifies: Given a repository with no remote by the configured name, or `--no-fetch`, the supervisor MUST read the local `<branch>` exactly as today, with no fetch attempted and no new output. Every existing test MUST keep passing with no change to what it asserts.

```gherkin
Scenario: Passing --no-fetch reads the local branch unchanged even when a remote exists
  Given a remote by the configured name exists and is reachable
  And the pass is run with --no-fetch
  When a supervisor pass runs
  Then no fetch is attempted
  And intake is read from the local branch exactly as before this change
  And no new output is produced
```

## A later fetch does not move a running or waiting order's claimed commit

Verifies: Given a running or waiting order, a fetch that moves `<remote>/<branch>` MUST NOT change it. The order MUST keep its claimed `sourceCommit`; only `loom factory rerun` takes a newer version.

```gherkin
Scenario: A later fetch does not move a running or waiting order's claimed commit
  Given an order is already running or waiting, claimed at a particular sourceCommit
  When a later supervisor pass fetches a remote branch that has moved forward
  Then the order's claimed sourceCommit remains unchanged
  And only an explicit loom factory rerun picks up the newer commit
```

## The intake README explains the fetch-before-read behavior

Verifies: Given `templates/claude-feature-team/intake/README.md`, it MUST say orders are read from the remote after a fetch, that a promotion must be pushed, and how `--no-fetch` restores the old behavior.

```gherkin
Scenario: The intake README explains the fetch-before-read behavior
  Given a person is reading templates/claude-feature-team/intake/README.md to understand how orders are picked up
  When they read the section describing how loom factory enforces pickup
  Then the README states orders are read from the remote after a fetch
  And the README states a promotion must be pushed, not just committed locally, to be seen
  And the README states how --no-fetch restores the previous local-branch-only behavior
```

## The delivered change meets the project's quality bar

Verifies: Given the change is complete, every new or changed function MUST have cyclomatic complexity below 7, every changed line MUST be covered by a test that can fail, `go test ./...` and the lint MUST pass, and no file outside `scope.paths` MUST change, excepting this run's own delivery record under `docs/features/factory-fetch-each-pass/`.

```gherkin
Scenario: The delivered change meets the project's quality bar
  Given the fetch-before-each-pass change is complete
  When the project's test suite and lint are run against the change
  Then every new or changed function has cyclomatic complexity below 7
  And every changed line is covered by a test that can fail
  And go test ./... passes and the lint step passes
  And no file outside scope.paths changes, except this run's own delivery record under docs/features/factory-fetch-each-pass/
```
