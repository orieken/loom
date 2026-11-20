---
feature: "factory-close-waiting-orders"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["waiting", "completed", "proposed", "parked", "stopped", "shipped", "running", "committed", "pushed", "retrying", "claimed"]
files_touched: ["internal/factory/control.go", "internal/factory/control_test.go", "cmd/loom/cmd/factory.go", "cmd/loom/cmd/factory_inprocess_test.go", "templates/claude-feature-team/intake/README.md", "internal/factory/replay.go", "internal/factory/inbox.go", "internal/factory/finish.go", "internal/factory/start.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Feature Analysis: factory-close-waiting-orders

## Summary

`loom factory close <order> --shipped-by <url>` currently refuses any order outside PhaseParked or PhaseStopped. The saturday-monorepo pilot hit this when order `reagent-map-save` was finished by hand while the factory's own run was still waiting at a confirm-ship gate: the only way out was to approve the commit and PR anyway, then close the duplicate PR unmerged. This feature widens closeable()'s allow-list in internal/factory/control.go to also accept PhaseWaiting (waiting at a run gate), PhaseCompleted (waiting for its commit to be approved), and PhaseProposed (waiting for its pull request to be approved) — the three phases where an order is waiting on a human with no process of its own running. Every other phase (claimed, running, retrying, committed, pushed, shipped) stays refused exactly as today. Because inbox.go's itemsFor, finish.go's finishStep, and start.go's continuationKind all switch on Phase with no PhaseShipped case, closing one of these orders automatically drops it from the inbox and stops the supervisor from ever committing, pushing, proposing, or relaunching it again — no code change is needed in those three files, only tests proving it. The only other changes are documentation: the close command's help text and the intake README's 'order finished by hand' section must name the phases close now accepts.

### Acceptance Criteria

- Given an order waiting at a run gate (waiting), waiting for its commit to be approved (completed), or waiting for its pull request to be approved (proposed), closing it with `loom factory close <order> --shipped-by <url>` records it shipped exactly as a parked order today: one shipped ledger entry with the URL and approvedBy, then the run archived and the worktree removed.
  - Example: loom factory close reagent-map-save --shipped-by https://github.com/orieken/saturday-monorepo/pull/30
- After closing a waiting, completed, or proposed order, `loom factory inbox` no longer lists it, nothing is committed or pushed for it, and no pull request is opened for it — even if an approval for it was already in the ledger and not yet acted on.
- An order whose run process is alive (running), one between a commit and its pull request (committed, pushed), or one scheduled to retry (retrying) still refuses close with exit code 1 and the phase named; the ledger does not change. An already-shipped order is refused as today.
- With the supervisor ticking, closing a waiting order and then running one more supervisor tick never relaunches, commits, or proposes that order again.
  - Example: A unit test closes a waiting order, calls the test harness's tick() once more, and asserts the ledger gains no launch/commit/push/propose entry.
- templates/claude-feature-team/intake/README.md's close section names every phase close now accepts (parked, stopped, waiting, completed, proposed).
- Every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, go test ./... and the lint pass, and no file outside scope.paths changes except this run's own delivery record under docs/features/factory-close-waiting-orders/.

### Non-Functional Requirements

None

## Proposed Fitness Functions

- **Cyclomatic complexity of closeable and every other new or changed function stays below 7**
  - **Verification**: the project's existing complexity lint rule (golangci-lint / gocyclo) in CI
  - **Owner**: architect
- **Every changed line in this delivery is covered by a test that fails when the behavior it checks is broken**
  - **Verification**: go test ./... -coverprofile plus a manual check that flipping closeable's new phase branches fails its test (ADR-008)
  - **Owner**: architect
- **No file outside scope.paths changes, except this feature's own delivery record**
  - **Verification**: the run executor's existing scope.paths check against the committed diff
  - **Owner**: devops-engineer
- **The supervisor never acts again on an order closed through close, even with a stale unacted-on approval present**
  - **Verification**: a unit test that closes a waiting (and separately, a completed-with-pending-approval) order, ticks the supervisor once more, and asserts no new launch/commit/push/propose ledger entry is appended (AC2, AC4)
  - **Owner**: architect

## Out of Scope

- Closing a running order without stopping it first.
- Closing an order that did not ship.
- Checking whether the named pull request actually merged, or calling GitHub at all.
- Re-opening a closed (shipped) order.
- Making PhaseRetrying closeable.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: cmd/loom/cmd — CLI command wiring for the close control, templates/claude-feature-team/intake/README.md — operator-facing lifecycle documentation

### Domain Events (Event Storming Lite)

- **Commands**: close
- **Events Produced**: shipped (existing event, now also written for an order waiting at a gate, awaiting commit approval, or awaiting pull-request approval, not only a parked or stopped one)
- **Owning Aggregates**: OrderState
- **Read Models / Projections**: OrderState (folded by Replay), loom factory status detail column, loom factory inbox

### Affected Components

- `internal/factory/control.go` — closeable()'s phase check (line 114, `order.Phase != PhaseParked && order.Phase != PhaseStopped`) must also accept PhaseWaiting, PhaseCompleted, and PhaseProposed, keeping the existing orderIn lookup, isHTTPURL check, and refusal wording unchanged for every other phase.
- `internal/factory/control_test.go` — TestCloseableAcceptsParkedAndStopped, TestCloseableRefusesEveryOtherPhase, and TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst assert the old allow-list; each must move PhaseWaiting/PhaseCompleted/PhaseProposed from the refused group into the accepted one, and new golden-path tests must cover each newly accepted phase the way TestCloseShipsAHandFinishedOrderAndUnblocksItsDependent already does for parked.
- `cmd/loom/cmd/factory.go` — factoryCloseCmd's Short (line 109) and Long (lines 110-116) text says 'a parked or stopped order'; must name all five accepted phases per AC5. runFactoryClose itself (line 348) needs no change — it already calls factory.Closer.Close unchanged regardless of which phase closeable() accepts.
- `cmd/loom/cmd/factory_inprocess_test.go` — Add in-process tests proving waiting/completed/proposed orders are accepted end-to-end (status shows shipped with the URL, inbox drops the order) alongside the existing TestFactoryCloseShipsAParkedOrderInProcess; TestFactoryCloseRefusesAWrongPhaseInProcess (line 237) uses claimedEntry alone (PhaseClaimed), which stays refused and needs no edit, but add explicit refusal coverage for running/committed/pushed/retrying per AC3.
- `templates/claude-feature-team/intake/README.md` — '### An order finished by hand' (lines 30-36) says 'A parked or stopped order is sometimes finished by a person instead'; must name the same five phases close now accepts, per AC5.
- `internal/factory/replay.go` — No code change: phaseOf/entryUpdates already fold EventShipped from any prior phase, and applyShipped only ever reads Entry.URL, never the order's prior phase — confirmed by inspection, nothing here depends on which phase closeable allowed.
- `internal/factory/inbox.go` — No code change: itemsFor()'s switch (lines 80-91) has no PhaseShipped case, so a newly closed order already drops out of the inbox the moment Replay folds its shipped entry — satisfies AC2's inbox clause automatically.
- `internal/factory/finish.go` — No code change: finishStep()'s switch (lines 73-83) has no PhaseShipped case, so the supervisor never commits, pushes, or opens a pull request for a closed order even if a stale commit/open-pr Approval is still set on OrderState — satisfies AC2's 'even if an approval was already in the ledger' clause.
- `internal/factory/start.go` — No code change: continuationKind()'s switch (lines 133-145) has no PhaseShipped case, so pendingWork/continuation never relaunches a closed order on the supervisor's next tick — this is exactly what AC4's required supervisor-tick test exercises and proves.

### Data Model Changes

- No ledger schema change: EventShipped and its URL/ApprovedBy fields already exist. The only logic change is closeable's in-memory phase allow-list in control.go, not any stored or on-disk structure. (**none** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. In internal/factory/control.go, widen closeable()'s phase check (line ~114) to also accept PhaseWaiting, PhaseCompleted, and PhaseProposed alongside PhaseParked and PhaseStopped, keeping cyclomatic complexity below 7 and the existing refusal wording ('stop it first, or it has already shipped') unchanged for every still-refused phase.
2. In cmd/loom/cmd/factory.go, update factoryCloseCmd's Short and Long text (lines 108-117) to name all five accepted phases — parked, stopped, waiting at a gate, completed awaiting commit approval, and proposed awaiting pull-request approval — instead of only 'parked or stopped'.
3. In templates/claude-feature-team/intake/README.md, update the '### An order finished by hand' section (lines 30-36) to name the same five phases close now accepts.
4. Make no change to internal/factory/replay.go, inbox.go, finish.go, or start.go — this analysis confirms none of their phase switches need a PhaseShipped case or any other edit for this feature; only new tests against them are needed (see QA tasks).

### QA Tasks

1. In internal/factory/control_test.go, move PhaseWaiting, PhaseCompleted, and PhaseProposed out of TestCloseableRefusesEveryOtherPhase and TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst and into the accepted-phase test, confirming PhaseClaimed, PhaseRunning, PhaseRetrying, PhaseCommitted, and PhasePushed stay refused with the phase named, and PhaseShipped stays refused as 'already shipped'.
2. Add one golden-path test per newly accepted phase (waiting, completed, proposed), following TestCloseShipsAHandFinishedOrderAndUnblocksItsDependent's shape: close the order, assert exactly one new shipped entry carrying the URL/approver, the worktree archived and removed, and a dependent order's depends-on satisfied.
3. Add the supervisor-no-relaunch test AC4 requires: close a waiting order, run one more supervisor tick, and assert the order stays PhaseShipped with no new launch/commit/push/propose ledger entry appended.
4. Add the AC2 stale-approval case: close a completed or proposed order that already carries an unacted-on commit-gate or open-pr Approval in the ledger, run a supervisor tick, and assert the supervisor still takes no action on it.
5. In cmd/loom/cmd/factory_inprocess_test.go, add in-process tests proving waiting/completed/proposed orders are accepted end-to-end (status shows shipped with the URL, inbox drops the order), and add explicit refusal coverage for running, committed, pushed, and retrying phases (AC3).
6. Run go test ./... and the project's lint/complexity tool; confirm both pass, every new or changed function stays below cyclomatic complexity 7, changed-line coverage is at least 85%, and every new test fails when the behavior it checks is broken.
7. Confirm no file outside scope.paths changed except this run's own delivery record under docs/features/factory-close-waiting-orders/.

### Tech Writer Tasks

1. Confirm templates/claude-feature-team/intake/README.md's updated 'An order finished by hand' section reads consistently with the existing Lifecycle table and names the same phase terms control.go/replay.go use (waiting, completed, proposed, parked, stopped).

### DevOps Tasks

None

## Edge Cases and Risks

- An order is waiting at a run gate (PhaseWaiting) when close runs.: closeable accepts it; Close appends one shipped entry with the URL/approver, archives the run state, and removes the worktree, identically to today's parked/stopped path.
- An order is completed and awaiting a commit approval (PhaseCompleted), with an unacted-on commit-gate Approval already in the ledger.: closeable accepts it regardless of the pending Approval; after the shipped entry is recorded, finishStep's switch has no PhaseShipped case, so the supervisor never commits the tree on a later tick, even though applyShipped never explicitly clears OrderState.Approval.
- An order is a proposed pull request awaiting the open-pr approval (PhaseProposed), with that approval already given.: closeable accepts it; openApprovedPR's switch excludes PhaseShipped, so the supervisor never opens the pull request on a later tick — the stale approval is simply never acted on again.
- The order's run process is still alive (PhaseRunning), it is between a commit and its pull request (PhaseCommitted, PhasePushed), or it is scheduled to retry (PhaseRetrying).: closeable still refuses with ErrNotApplicable, naming the phase and instructing the operator to stop it first; the ledger is unchanged, matching today's behavior for every phase outside the allow-list.
- The order has already shipped (PhaseShipped).: closeable still refuses, naming the phase and noting it has already shipped, exactly as today — PhaseShipped is never added to the allow-list.
- The supervisor ticks once right after close runs on a waiting/completed/proposed order.: continuationKind and finishStep both switch on Phase with no PhaseShipped case, so pendingWork finds no continuation and finishPass finds no step; the order is not relaunched, committed, pushed, or proposed again.
- Whether PhaseRetrying should be closeable, raised as an open question in the spec.: Resolved by the spec's own stated assumption: no — a retry can relaunch at any tick, so a person stops it first, the same as a running order.

## Definition of Done

- go test ./... passes, including new and updated tests in internal/factory and cmd/loom/cmd.
- The project's lint/complexity tool reports no function at or above cyclomatic complexity 7 in the changed files.
- Changed lines are covered at 85% or better by tests that fail when the behavior they check is broken.
- closeable accepts PhaseWaiting, PhaseCompleted, and PhaseProposed in addition to PhaseParked and PhaseStopped, and still refuses every other phase with the phase named.
- A test proves the supervisor does not relaunch, commit, push, or propose a closed order on the tick immediately after close, including when a stale, unacted-on approval is present.
- templates/claude-feature-team/intake/README.md's close section names every phase it accepts.
- No file outside scope.paths changed, excepting docs/features/factory-close-waiting-orders/.
