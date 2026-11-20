---
feature: "factory-close-waiting-orders"
bounded_context: ""
domain_terms: ["waiting", "completed", "proposed", "parked", "stopped", "shipped", "running", "committed", "pushed", "retrying", "claimed", "closeable", "Closer", "ShippedByHand", "OrderState", "Ledger"]
files_touched: ["internal/factory/control.go", "internal/factory/control_test.go", "cmd/loom/cmd/factory.go", "cmd/loom/cmd/factory_inprocess_test.go", "cmd/loom/cmd/factory_test.go", "templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Implementation Notes: factory-close-waiting-orders

## Files Created

None

## Files Modified

- internal/factory/control.go
- internal/factory/control_test.go
- cmd/loom/cmd/factory.go
- cmd/loom/cmd/factory_inprocess_test.go
- cmd/loom/cmd/factory_test.go
- templates/claude-feature-team/intake/README.md

## Interface Design

- waitingOnHuman(phase Phase) bool — new private predicate in internal/factory/control.go naming the five phases closeable() accepts (parked, stopped, waiting, completed, proposed); closeable()'s own signature and return type are unchanged.

## Named Refactoring Log

- **Extract Function** on `internal/factory/control.go: closeable()` — Extracted the phase-allow-list check into waitingOnHuman(Phase) bool so closeable() reads as orderIn → isHTTPURL → waitingOnHuman, keeping closeable() itself short and the five-phase disjunction named and testable on its own; this is the plain boolean-expression form architecture-notes.md calls for, not a lookup table or Strategy.

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new functions — gocyclo -over 6 on control.go and factory.go reports nothing; waitingOnHuman is a single switch with 5 phases as one case, complexity 2.
- [x] No primitive obsession: domain concepts wrapped in value objects, not raw strings/ints — Phase was already a named type; no new primitive introduced.
- [x] No feature envy: methods use their own class's data more than another's
- [x] No magic numbers or strings: all literals are named constants — Phase constants (PhaseParked etc.) already existed; waitingOnHuman only references them.
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — control.go added no new import; cmd/loom/cmd still only calls into internal/factory.

## Simple Design Verification

- [x] Passes all tests — go test ./... passes; the one failure (internal/provider/claude TestInvokeSetsNoTraceParentWhenUntraced) is a pre-existing TRACEPARENT environment leak confirmed present on the unmodified baseline via a temporary stash, unrelated to this change.
- [x] Reveals intention — Renamed the implicit two-phase check into waitingOnHuman(phase), naming the concept the spec and architecture notes use: 'waiting on a human with no process of its own running'.
- [x] No duplication — closeable's refusal wording stays byte-identical and is produced from one call site, as before; no new duplication introduced.
- [x] Fewest elements — Chose a plain boolean switch over a map[Phase]bool or Strategy, per architecture-notes.md's explicit rejection of those as premature generalization for a closed 5-phase allow-list.

## Key Decisions

- Widen closeable()'s allow-list to PhaseParked, PhaseStopped, PhaseWaiting, PhaseCompleted, PhaseProposed via a new waitingOnHuman(Phase) bool, with no code change to replay.go, inbox.go, finish.go, or start.go.: Confirmed by reading all three phase-switches (itemsFor, finishStep, continuationKind) that none has a PhaseShipped case or an acting default branch, so a newly closed order already drops out of the inbox and is never relaunched/committed/pushed/proposed again — exactly as architecture-notes.md's Developer Handoff Notes required, with the supervisor-tick tests proving it rather than defensive code added on spec.
- Rewrote the two tests (control_test.go's TestCloseRefusesAnOrderThatIsNotParkedOrStopped, cmd/loom/cmd/factory_test.go and factory_inprocess_test.go's refusal cases) that asserted a waiting order is refused, since that assertion is now the opposite of the required behavior, rather than leaving them to fail.: These were existing tests encoding the old (now-wrong) contract per context-manifest.md's own note; test-repair-contract.md permits correcting an assertion when the intended behavior change is evidenced by the spec and the diff — here the spec (AC1/AC3) and architecture-notes.md are that evidence. Each rewritten test now proves the new, opposite behavior rather than being weakened or deleted.
- Added golden-path tests for each of the three newly accepted phases (waiting, completed, proposed) plus two supervisor-tick non-regression tests (AC4, and AC2's stale-approval case for both the commit gate and the open-pr gate) instead of one combined test.: Each phase is reached through a different code path (a run-gate halt, finishCompleted's commit wait, afterPush's propose step) and the stale-approval scenario specifically needs an Approval already recorded before Close runs — collapsing them would hide which path broke if one regressed, and QA Task 3/4 in analysis.md asked for both the plain no-relaunch case and the stale-approval case separately.

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- internal/factory/control_test.go now has TestCloseableAcceptsPhasesWaitingOnAHuman (unit-level, all 5 accepted phases), TestCloseableRefusesEveryOtherPhase/TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst (now excluding waiting/completed/proposed), TestCloseRefusesARunningOrder (replaces the old wrong-phase test), three new golden-path tests per newly accepted phase, and TestSupervisorTickAfterCloseDoesNotRelaunch / TestSupervisorTickAfterCloseIgnoresAStaleCommitApproval / TestSupervisorTickAfterCloseIgnoresAStaleOpenPRApproval for AC4/AC2.
- cmd/loom/cmd/factory_inprocess_test.go gained TestFactoryCloseAcceptsPhasesWaitingOnAHumanInProcess (table test, waiting/completed/proposed) and TestFactoryCloseRefusesPhasesWithAProcessInProcess (table test, running/committed/pushed/retrying); TestFactoryControlsReportTheirRefusalsInProcess no longer asserts close is refused for a waiting order.
- cmd/loom/cmd/factory_test.go's TestFactoryControlsRefuseTheWrongOrder no longer includes close in its refusal table (a waiting order is now accepted); added TestFactoryCloseAcceptsAnOrderWaitingAtAGate as the real-binary end-to-end counterpart.
- go test ./internal/factory/... ./cmd/loom/cmd/... -cover reports 95.5% and 83.8% statement coverage respectively; every changed line in control.go and factory.go is exercised by at least one new or updated test, each proven able to fail by the behavior it asserts being the opposite of the prior contract.
- Pre-existing, unrelated failure: internal/provider/claude's TestInvokeSetsNoTraceParentWhenUntraced fails in this shell because a TRACEPARENT environment variable is already set; verified present on the unmodified baseline via a temporary git stash, so it is not caused by this change.

## Notes for DevOps

None
