---
feature: "factory-close-shipped-by-hand"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: []
files_touched: ["internal/factory/control.go", "internal/factory/replay.go", "internal/factory/control_test.go", "cmd/loom/cmd/factory.go", "cmd/loom/cmd/factory_inprocess_test.go", "templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Feature Analysis: factory-close-shipped-by-hand

## Summary

Adds `loom factory close <order> --shipped-by <url>` so a human can mark a parked or stopped order shipped after finishing it by hand in its own merged pull request. It appends one `shipped` ledger entry carrying the URL and the approver (same source as `loom factory approve`), reusing the ledger's existing `url`/`approvedBy` fields; folds that URL onto `OrderState` so `status` can show it; refuses on any other phase or a missing/invalid order or URL; and reuses `Supervisor.ship`'s record-then-archive-then-remove sequence so a closed order's run state is archived and its worktree removed the same way, without ever calling GitHub. Because `dependenciesShipped` already checks `Phase == PhaseShipped`, a dependent order unblocks automatically once `close` runs, with no edit to either order.

### Acceptance Criteria

- Closing a parked or stopped order with `loom factory close <order> --shipped-by <url>` appends exactly one `shipped` ledger entry carrying the URL in `url` and the approver — from the same source as `loom factory approve` — in `approvedBy`.
  - Example: loom factory close reagent-map-visual-baselines --shipped-by https://github.com/orieken/saturday-monorepo/pull/23
- After that entry, `loom factory status` shows the order as `shipped` with the URL in its detail column, and `loom factory inbox` no longer lists it.
- An order whose `depends-on` names the closed order is claimed by the supervisor on its next pass, with no edit to either order.
- Closing an order archives its run state and removes its worktree, in the same order as the supervisor's own ship: record the ledger line first, archive, then remove; if archiving fails, the worktree is kept and the command reports why.
- `close` refuses an order that is running, retrying, waiting at a gate, waiting on a commit or PR approval, or already shipped, with exit code 1, the order's phase named, and an instruction to stop it first; the ledger does not change.
- `close` refuses an order the ledger does not hold, or a `--shipped-by` value that is missing or not an `http`/`https` URL, with exit code 1 and a message naming the problem; the ledger does not change.
- `close` never calls GitHub and never checks that the named pull request merged; the human running it vouches for that, and `approvedBy` records who.
- `close` and the supervisor never both act on the same order: `close` reads the ledger fresh and validates the order's current phase before appending, the same TOCTOU-tolerant pattern `approve`, `stop` and `rerun` already use.
- `templates/claude-feature-team/intake/README.md` documents when to use `close`: after an order parks and a person finishes the work in their own pull request, once that PR merges.
- Every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass, and no file outside `scope.paths` changes except this run's own delivery record under `docs/features/factory-close-shipped-by-hand/`.

### Non-Functional Requirements

- **security**: close must store and print the --shipped-by URL only; it must never fetch, open, or otherwise make a network call to it.
- **other**: Each close call appends exactly one ledger line, written atomically, the same single-write guarantee Ledger.Append already provides for every other event.
- **other**: close must not introduce a second way to approve or ship an order outside the ledger; approvedBy must come from the same source approve already uses (the operating system user).

## Proposed Fitness Functions

- **Cyclomatic complexity of every new or changed function (Close, closeable, isHTTPURL, runFactoryClose) stays below 7**
  - **Verification**: the project's existing complexity lint rule (golangci-lint / gocyclo) in CI
  - **Owner**: architect
- **Every changed line in this delivery is covered by a test that fails when the behavior it checks is broken**
  - **Verification**: go test ./... -coverprofile plus a manual check that flipping each new branch fails its test (ADR-008)
  - **Owner**: architect
- **No file outside scope.paths changes, except this feature's own delivery record**
  - **Verification**: the run executor's existing scope.paths check against the committed diff
  - **Owner**: devops-engineer

## Out of Scope

- Closing an order that did not ship, such as an abandoned one.
- Checking the PR on GitHub or whether it merged.
- Re-opening a closed order.
- A --reason flag — the URL is the reason.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: cmd/loom/cmd — CLI command wiring for the new control, templates/claude-feature-team/intake/README.md — operator-facing lifecycle documentation

### Domain Events (Event Storming Lite)

- **Commands**: close
- **Events Produced**: shipped (existing event, now also written by a human control instead of only the supervisor)
- **Owning Aggregates**: OrderState
- **Read Models / Projections**: OrderState (folded by Replay), loom factory status detail column, loom factory inbox

### Affected Components

- `internal/factory/control.go` — Add Close, closeable and isHTTPURL beside the existing RequestStop/RequestResume/RequestRerun human-issued controls, following the same orderIn lookup and ErrNotApplicable refusal style.
- `internal/factory/replay.go` — Add an entryUpdates[EventShipped] handler that carries a non-empty Entry.URL onto OrderState.URL, so a closed order's URL renders in `status` the same way a normal ship's PR URL already does.
- `internal/factory/control_test.go` — New unit tests for Close/closeable/isHTTPURL, beside control.go per the spec's Test Approach, covering every acceptance criterion and refusal path.
- `cmd/loom/cmd/factory.go` — Add the `close` subcommand, its required --shipped-by flag, and runFactoryClose, wiring factory.Close with openLedger(), worktree.New(layout.Root) as the Repository, and approverName().
- `cmd/loom/cmd/factory_inprocess_test.go` — Add in-process command tests for `close`, reusing the existing writeLedger/claimedEntry/runFactoryCommand helpers that already stand up an order in a given phase without running anything.
- `templates/claude-feature-team/intake/README.md` — An acceptance criterion requires this file say when to use close: after an order parks and a person finishes the work in their own merged pull request.

### Data Model Changes

- No on-disk schema change: Entry.URL and Entry.ApprovedBy already exist in ledger.go. Only the in-memory fold in replay.go's entryUpdates map gains a handler for EventShipped that copies a non-empty Entry.URL onto OrderState.URL. (**none** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. In internal/factory/replay.go, add entryUpdates[EventShipped] = func(o *OrderState, e Entry) that sets o.URL = e.URL only when e.URL is non-empty, leaving the supervisor's own ship() (which appends no URL) unaffected.
2. In internal/factory/control.go, add isHTTPURL(raw string) bool using net/url.Parse, true only for scheme http or https with a non-empty host.
3. In internal/factory/control.go, add closeable(orders map[string]*OrderState, id, rawURL string) (*OrderState, error): look up the order via the existing orderIn, validate rawURL with isHTTPURL, and refuse unless order.Phase is PhaseParked or PhaseStopped, reusing RequestRerun's existing refusal text ('%s is %s; stop it first, or it has already shipped').
4. In internal/factory/control.go, add Close(ledger *Ledger, orders map[string]*OrderState, layout Layout, repo Repository, id, rawURL, approver string) (string, error): call closeable, append one Entry{Event: EventShipped, Order: id, URL: rawURL, ApprovedBy: approver}, then call archiveRun(order, layout.RunArchivePath(id)) and repo.RemoveWorktree(order.Claim.Worktree), returning a warning string (nil error) when either step fails, matching Supervisor.ship's own wording ('%s shipped, but its run state was not archived, so its worktree is kept: %v' / '%s shipped, but its worktree was not removed: %v').
5. In cmd/loom/cmd/factory.go, add factoryCloseCmd (Use: "close <order>", Args: cobra.ExactArgs(1)) with a required --shipped-by string flag, register it in init(), and add runFactoryClose that calls openLedger(), then factory.Close(ledger, orders, layout, worktree.New(layout.Root), args[0], the flag value, approverName()), printing the returned warning or a success line naming the order and URL.
6. In templates/claude-feature-team/intake/README.md, add close to the Lifecycle/Status material: use it once a parked or stopped order's work was finished by hand in its own pull request, after that PR merges.

### QA Tasks

1. Run go test ./... and the project's lint/complexity tool; confirm both pass with the new code.
2. Verify changed-line coverage is at least 85% for the control.go/replay.go/factory.go additions, and that every new test fails when the behavior it checks is broken (flip the assertion or comment out the branch).
3. Exercise the golden path against the mock provider: park an order (and separately, stop one), run loom factory close <id> --shipped-by <url>, and confirm loom factory status shows shipped with the URL in its detail column, loom factory inbox no longer lists it, and a dependent order is claimed on the supervisor's next pass with no edits to either order.
4. Exercise every refusal path: unknown order, missing --shipped-by, a non-http(s) --shipped-by, and each disallowed phase (running, retrying, waiting, completed, committed, pushed, proposed, shipped) — confirm exit code 1 and an unchanged ledger for each.

### Tech Writer Tasks

1. Confirm templates/claude-feature-team/intake/README.md's new close guidance reads consistently with the existing Lifecycle table and Who-writes-what section, and cross-reference it from the status/inbox rows if helpful.

### DevOps Tasks

None

## Edge Cases and Risks

- The named order is not in the ledger at all.: closeable's orderIn refuses with 'ErrNotApplicable: no order "<id>" in the ledger'; exit code 1; ledger unchanged.
- --shipped-by is missing.: cobra's required-flag check refuses before RunE runs, so the ledger is never touched; Close's own isHTTPURL check gives the same refusal if ever called directly with an empty value.
- --shipped-by is present but not an http or https URL (e.g. a bare path, or ftp://).: isHTTPURL returns false; closeable refuses naming --shipped-by; exit code 1; ledger unchanged.
- The order is running, retrying, waiting at a gate, completed, committed, pushed, or proposed.: closeable's phase allow-list (Parked or Stopped only) refuses with the order's phase named and 'stop it first, or it has already shipped'; exit code 1; ledger unchanged.
- The order has already shipped.: Same refusal path and message as above — PhaseShipped is not in the allow-list; exit code 1; ledger unchanged.
- Archiving the run's workspace fails after the ledger line is recorded.: Close still returns success (nil error) with a non-empty warning string; the worktree is kept; runFactoryClose prints the warning instead of the success line, matching Supervisor.ship's own behavior.
- Removing the worktree fails after a successful archive.: Close returns success with a warning naming that the worktree was not removed; the ledger entry is already recorded as shipped either way.
- A dependent order lists the closed order in depends-on.: No change needed in start.go: dependenciesShipped already checks state.Phase == PhaseShipped, which Close's ledger entry now sets; the supervisor claims the dependent on its next pass.
- close runs while the supervisor is mid-tick on the same order.: close reads the ledger fresh and checks the order's current phase before appending — the same pattern RequestStop/RequestResume/RequestRerun already use; a genuine race either sees the now-changed phase and refuses, or the supervisor's own next read sees the new shipped phase and stops acting on the order.

## Definition of Done

- go test ./... passes, including new tests in internal/factory and cmd/loom/cmd.
- The project's lint/complexity tool reports no function at or above cyclomatic complexity 7 in the changed files.
- Changed lines are covered at 85% or better by tests that fail when the behavior they check is broken.
- loom factory close is documented in templates/claude-feature-team/intake/README.md.
- No file outside scope.paths changed, excepting docs/features/factory-close-shipped-by-hand/.
- QA has confirmed the golden path (parked and stopped orders) and every refusal path from the acceptance criteria.
