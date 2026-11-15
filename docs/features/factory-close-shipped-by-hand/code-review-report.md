---
feature: "factory-close-shipped-by-hand"
bounded_context: ""
domain_terms: ["order", "ledger", "phase", "shipped", "parked", "stopped", "worktree", "supervisor", "approver"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Code Review: factory-close-shipped-by-hand

## Overall Status

**APPROVED**

## Design Narrative

A new `Closer{Layout, Repository}` type in `internal/factory/control.go` lets a human record a hand-finished order as shipped through one ledger entry, then reuses `archiveAndRemove` — extracted from `Supervisor.ship` into `finish.go` so both callers share one archive-then-remove sequence and warning wording — to finish the order's workspace. `replay.go` gains an `EventShipped` handler that folds the recorded URL onto `OrderState` so existing projections (`status`, `inbox`) display it without any new read model. CLI wiring in `cmd/loom/cmd/factory.go` stays confined to that layer; `internal/factory` imports nothing from `cmd/` or GitHub, and `isHTTPURL` is syntax-only (`net/url.Parse`), so the 'close never calls GitHub' requirement is structural, not just a convention. Behaviour change: yes.

## Design Score

- **Clarity** 5/5
- **Cohesion** 5/5
- **Coupling** 5/5
- **Craft** 5/5

## Security Surface

- isHTTPURL is purely syntactic (net/url.Parse scheme/host check) — no network call is reachable from close, satisfying the NFR that --shipped-by is stored/printed only.
- approver comes from approverName() (OS user), the same source loom factory approve already uses — no second approval channel introduced.

## Performance Surface

None

## Test Design Review

- TestIsHTTPURL — YES: table test over valid/invalid URL forms, fails if scheme/host logic regresses.
- TestCloseableRefusesAnUnknownOrder — YES: asserts ErrNotApplicable for a missing order.
- TestCloseableRefusesAnInvalidURL — YES: asserts refusal for empty/malformed/non-http(s) URLs.
- TestCloseableAcceptsParkedAndStopped — YES: asserts acceptance for both allowed phases.
- TestCloseableRefusesEveryOtherPhase — YES: asserts refusal across every disallowed phase including PhaseShipped.
- TestCloseShipsAHandFinishedOrderAndUnblocksItsDependent — YES: asserts phase, URL fold, worktree removal, dependent unblocking, and ledger entry content against a real worktree.
- TestCloseRefusesAnOrderThatIsNotParkedOrStopped — YES: asserts error and that the ledger entry count is unchanged.
- TestCloseKeepsTheWorktreeWhenArchivingFails — YES: asserts non-empty warning, shipped phase, and worktree still present.
- TestCloseKeepsTheLedgerEntryWhenRemovingTheWorktreeFails — YES: asserts warning and that the ledger entry still records shipped.
- TestCloseReportsALedgerItCannotWrite — YES: asserts Close returns an error when the ledger write itself fails.
- TestApplyShippedIgnoresAnEmptyURL — YES: asserts no-op on empty URL, then asserts the fold on a non-empty one — both branches exercised.
- TestFactoryControlsReportTheirRefusalsInProcess (close branch added) — YES: asserts close is refused for an order waiting at a gate.
- TestFactoryCloseShipsAParkedOrderInProcess — YES: asserts CLI success message, status output, and inbox no longer listing the order.
- TestFactoryClosePrintsAWarningWhenFinishingFailsInProcess — YES: asserts the warning text and that status still shows shipped.
- TestFactoryCloseRefusesAWrongPhaseInProcess — YES: asserts a non-nil error for a claimed (not parked/stopped) order.
- TestFactoryCloseRefusesAnInvalidURLInProcess — YES: asserts refusal for a non-URL --shipped-by value.
- TestFactoryCloseRefusesAnUnknownOrderInProcess — YES: asserts refusal when the ledger holds no such order.
- TestFactoryCommandsReportACorruptLedgerInProcess (close branch added) — YES: asserts close refuses a corrupt ledger like every other control.

## Verification of Developer Self-Review

- Confirmed: no new function exceeds 30 LOC — Close, closeable, isHTTPURL, archiveAndRemove, runFactoryClose are all well under that.
- Confirmed: gocyclo -over 7 against every changed file reports zero violations, matching the developer's claim.
- Confirmed: dependency direction holds — grep shows no cmd/ import anywhere in internal/factory/*.go.
- Confirmed: gofmt -l reports no diffs on any changed file.
- Confirmed: golangci-lint's only reported issue (internal/factory/units_test.go:163 errcheck) is pre-existing and outside this diff's files, as claimed.
- Confirmed: go test ./internal/factory/... ./cmd/loom/cmd/... passes in full (this session, no ambient TRACEPARENT issue encountered).

## Feedback for the Developer

None
