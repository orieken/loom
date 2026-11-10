---
feature: "intake-slice-check"
bounded_context: ""
domain_terms: ["Slice", "Level"]
files_touched: ["internal/intake/slice_test.go", "internal/intake/slice_check_test.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# QA Report: intake-slice-check

## Test Files Created

None

## Test Files Modified

- internal/intake/slice_test.go
- internal/intake/slice_check_test.go

## Coverage Summary

- Acceptance criteria covered: 10/10
- Total new tests: 3
- Statement coverage, lowest unit: **83.7%** (`cmd/loom/cmd`)
  - `internal/intake`: 100.0%
  - `cmd/loom/cmd`: 83.7%

## Test Results

- Passed: 406
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- Every new or changed function MUST have cyclomatic complexity below 7, every changed line MUST be covered by a test that can fail, and no file outside scope.paths MUST change except this run's own delivery record under docs/features/intake-slice-check/.: cmd/loom/cmd measures 83.7% package-wide, below the 85% floor, but the gap is entirely in featureSpecs' pre-existing notOrders filter branch (cmd/loom/cmd/intake.go:162), which this diff never touches (confirmed: that line does not appear in `git diff HEAD -- cmd/loom/cmd/intake.go`). Every line this feature's diff adds or changes in intake.go measures 100% at the function level (checkIntakeSlices, slicedSpecs, summarizeIntakeCheck, checkOrderFile, printViolations, orderFiles, withoutNonOrders, runnablePlans, init, runIntakeCheck, projectRoot all 100%).

## Notes for Tech Writer

- Verified templates/claude-feature-team/intake/TEMPLATE.md and README.md document exactly the Slice struct's five fields (of, spec, digest, manifest, covers) as declared in internal/intake/order.go:41-58 — no drift between docs and code.
- internal/intake/vocabulary_test.go is confirmed unmodified by this change (zero-line diff against HEAD) and passes, so the pre-existing template vocabulary regression guard (AC8) still holds.
