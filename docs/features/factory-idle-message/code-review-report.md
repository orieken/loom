---
feature: "factory-idle-message"
bounded_context: ""
domain_terms: ["Idle", "Waiting on a human"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Code Review: factory-idle-message

## Overall Status

**APPROVED**

## Design Narrative

The supervisor's existing Tick() gains one new last step: classify the current ledger replay into busy/idle/waiting (a pure fold over map[string]*OrderState, no I/O), then announce it only if it differs from the last state reported, via the Supervisor's existing s.say output mechanism. All new logic lives in a single new file (idle.go) matching the package's existing one-concern-per-file convention, with a zero-value tickBusy sentinel on a new lastReported field giving print-once-per-entry semantics for free. Behaviour change: yes — the supervisor now prints two new console lines it previously never printed.

## Design Score

- **Clarity** 5/5
- **Cohesion** 5/5
- **Coupling** 5/5
- **Craft** 5/5

## Security Surface

None

## Performance Surface

- reportTickState adds a 4th independent s.Orders() ledger fold per Tick(), consistent with existing precedent (settleEnded/advanceFinishLine/startWork each already do their own); local, append-only file read, not flagged as a regression.

## Test Design Review

- TestAFreshSupervisorWithNoOrdersReportsIdle — YES: asserts exact idle string once; fails if announce doesn't fire or branch name is wrong.
- TestIdlePrintsOnceAcrossConsecutiveTicks — YES: fails if the once-per-entry guard regresses to printing every tick.
- TestAGateHaltCountsAsWaiting — YES: fails if waitingPhases drops PhaseWaiting from the count.
- TestAParkedOrderCountsAsWaiting — YES: fails if PhaseParked stops counting as waiting.
- TestASecondWaitingOrderReprintsWithTheIncrementedCount — YES: fails if count changes don't force a reprint.
- TestLeavingWaitingThenReturningToIdleReprints — YES: fails if leaving a state wrongly reprints, or re-entering idle stays silent.
- TestARetryingOrderPrintsNothing — YES, negative test: fails if a retrying order is misclassified as idle or waiting.
- TestOnceAppliesTheOncePerEntryRuleToItsSinglePass — YES: fails if Once() reprints the idle line on an unchanged second pass.
- None of the diff's tests are vacuous, self-fulfilling, or unreached; all assert on the actual printed output buffer rather than a mock's own configured return.

## Verification of Developer Self-Review

- Complexity claim verified independently: golangci-lint (gocyclo min-complexity 7) reports zero issues in idle.go/supervisor.go — matches developer's claim.
- Coverage claim verified independently: go test -coverprofile shows idle.go's new functions at 100% except reportTickState's one-line error-forward branch at 80%, matching the stated exception; aggregate package coverage 95.2%.
- No-new-import / dependency-direction claim verified: idle.go has zero import statements, confirming classify/waitingCount/hasUnshippedOrder cannot reach Repository/TreeOpener/PullRequests/Intake.
- Scope claim verified: git status shows only internal/factory/* and the feature's own docs/features/factory-idle-message/ changed, matching the AC's file-scope constraint.

## Feedback for the Developer

None
