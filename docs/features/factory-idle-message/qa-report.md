---
feature: "factory-idle-message"
bounded_context: ""
domain_terms: ["Supervisor", "Tick", "Phase", "Inbox", "Intake"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# QA Report: factory-idle-message

## Test Files Created

None

## Test Files Modified

None

## Coverage Summary

- Acceptance criteria covered: 6/6
- Total new tests: 9
- Statement coverage, lowest unit: **95.3%** (`internal/factory`)
  - `internal/factory`: 95.3%
  - `internal/factory/idle.go (new functions only)`: 96.0%
  - `internal/factory/supervisor.go (changed lines only)`: 100.0%

## Test Results

- Passed: 59
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- Every changed or added line in internal/factory/idle.go is covered by a test that can fail.: reportTickState's one-line `return err` forward from s.Orders() (idle.go:91) is uncovered — exercising it needs a ledger read failure mid-tick, which the fixture has no seam for. The identical idiom is already uncovered at multiple pre-existing sites in this same package (e.g. supervisor.go:88, 91, 112, 128, 131, 144, 168, 175), so this is established convention rather than a new gap introduced by this feature. Function-level coverage for reportTickState is 80%; all other new functions (waitingCount, hasUnshippedOrder, classify, announce) are 100%.

## Notes for Tech Writer

- User-visible behavior is exactly two new console lines from the supervisor: the idle line 'idle — no ready orders; commit one to intake/ on <branch>' and the waiting line 'waiting on a human — <n> order(s) in the inbox: loom factory inbox', each printed once per state entry.
- No new CLI flags, config fields, or ledger events were introduced — this is output-only, so no schema or data-model documentation changes are needed, only a behavior note if the supervisor's console output is documented anywhere user-facing.
