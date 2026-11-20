---
feature: "factory-close-waiting-orders"
bounded_context: ""
domain_terms: ["parked", "stopped", "waiting", "completed", "proposed", "claimed", "running", "retrying", "committed", "pushed", "shipped", "closeable"]
files_touched: ["internal/factory/control_test.go", "cmd/loom/cmd/factory_inprocess_test.go", "cmd/loom/cmd/factory_test.go"]
issue_refs: []
linked_adrs: ["ADR-010"]
linked_kis: []
---

# QA Report: factory-close-waiting-orders

## Test Files Created

None

## Test Files Modified

- internal/factory/control_test.go
- cmd/loom/cmd/factory_inprocess_test.go
- cmd/loom/cmd/factory_test.go

## Coverage Summary

- Acceptance criteria covered: 6/6
- Total new tests: 11
- Statement coverage, lowest unit: **83.8%** (`cmd/loom/cmd`)
  - `internal/factory`: 95.5%
  - `cmd/loom/cmd`: 83.8%

## Test Results

- Passed: 493
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- Every changed line is covered by a test that can fail (AC6, changed-line coverage): cmd/loom/cmd's whole-package statement coverage is 83.8%, below the 85% floor, but that figure spans the entire pre-existing package, most of which this feature never touched. Verified with go tool cover -func that every changed function in cmd/loom/cmd/factory.go (runFactoryClose and its Long/Short doc strings, the only lines this feature's diff touches there) is at 100%; the package-wide shortfall sits in pre-existing, unrelated code (e.g. approverName at 66.7%, newFactorySupervisor at 80.0%) that this feature did not modify. internal/factory, which carries the real logic change, is at 95.5%, and every changed line in control.go (closeable, waitingOnHuman) is at 100%.

## Notes for Tech Writer

- templates/claude-feature-team/intake/README.md's 'An order finished by hand' section already lists every phase close now accepts (parked, stopped, waiting at a run gate, completed and awaiting its commit approval, proposed and awaiting its pull-request approval) and the refusal phases (claimed, running, retrying, committed, pushed) — AC5 is satisfied, no further doc change needed for this feature.
- cmd/loom/cmd/factory.go's close command Short/Long help text was updated by the developer to describe the newly accepted phases; verified it matches the implemented behavior.
- Pre-existing, unrelated test failure: internal/provider/claude's TestInvokeSetsNoTraceParentWhenUntraced fails in this shell because a TRACEPARENT environment variable is already set in the ambient shell environment (confirmed via `env | grep -i traceparent`); this package is untouched by this feature and the failure is an environment artifact, not a regression.
