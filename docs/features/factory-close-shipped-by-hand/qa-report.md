---
feature: "factory-close-shipped-by-hand"
bounded_context: ""
domain_terms: ["order", "ledger", "parked", "stopped", "shipped", "depends-on", "inbox", "approvedBy", "worktree"]
files_touched: ["internal/factory/control_test.go", "cmd/loom/cmd/factory_inprocess_test.go", "cmd/loom/cmd/factory_test.go", "internal/factory/supervisor_test.go"]
issue_refs: []
linked_adrs: ["ADR-010"]
linked_kis: []
---

# QA Report: factory-close-shipped-by-hand

## Test Files Created

- internal/factory/control_test.go

## Test Files Modified

- cmd/loom/cmd/factory_inprocess_test.go
- cmd/loom/cmd/factory_test.go
- internal/factory/supervisor_test.go

## Coverage Summary

- Acceptance criteria covered: 10/10
- Total new tests: 20
- Statement coverage, lowest unit: **83.8%** (`cmd/loom/cmd`)
  - `internal/factory`: 95.5%
  - `cmd/loom/cmd`: 83.8%

## Test Results

- Passed: 484
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- close and the supervisor never both act on the same order: close reads the ledger fresh and validates the order's current phase before appending.: This is architectural, not a new mechanism: runFactoryClose calls openLedger() fresh then immediately validates and appends within one synchronous call, the same one-shot pattern RequestStop/RequestResume/RequestRerun already use, none of which carry a dedicated race-simulation test either. closeable()'s phase check (exercised by TestCloseableRefusesEveryOtherPhase and the ledger-unchanged assertion in TestCloseRefusesAnOrderThatIsNotParkedOrStopped) is the only testable surface; a true concurrent-process race isn't exercisable from a single-process unit test.
- close refuses when --shipped-by is missing, with exit code 1 before taking any action, naming --shipped-by as required.: Enforced by cobra's MarkFlagRequired, which only fires through Command.Execute() (verified via the new binary-level test TestFactoryCloseRequiresTheShippedByFlag in cmd/loom/cmd/factory_test.go). At the application layer, an empty --shipped-by also fails isHTTPURL('') (covered by TestIsHTTPURL and TestCloseableRefusesAnInvalidURL), giving the same refusal redundantly, consistent with how run --spec and state --stage/--gate's own required flags are (not separately) tested elsewhere in this codebase.

## Notes for Tech Writer

- templates/claude-feature-team/intake/README.md now documents `loom factory close <id> --shipped-by <url>` under a new "An order finished by hand" section — no further doc changes needed for this feature.
- The CLI help text on factoryCloseCmd (Long description) already states close never calls GitHub and never checks the PR merged; worth cross-referencing from any future `loom factory` command reference doc.
