---
feature: "intake-slice-check"
bounded_context: ""
domain_terms: ["Slice", "Level", "Fitness Function"]
files_touched: ["internal/intake/slice_check_test.go"]
issue_refs: ["intake-slice-check", "decision D-31"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Implementation Notes: intake-slice-check

## Files Created

None

## Files Modified

- internal/intake/slice_check_test.go

## Interface Design

None

## Named Refactoring Log

None

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new/changed functions — gocyclo -over 6 on slice.go, slice_check.go, intake.go reports zero hits; golangci-lint run over both packages reports 0 issues
- [x] No primitive obsession — unchanged from the salvaged implementation; only test-table additions this stage
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified — cmd/loom/cmd -> internal/intake via FileReader seam, unchanged

## Simple Design Verification

- [x] Passes all tests — go build ./... and go test ./... both pass (one unrelated flake in internal/provider/claude traced to an ambient TRACEPARENT env var in this shell, confirmed pre-existing and outside scope.paths by re-running with env -u TRACEPARENT)
- [x] Reveals intention — no rename needed; new test case names ('covers level outside api or ui', 'manifest-listed file deleted') follow the existing break-one-thing naming convention
- [x] No duplication — new cases reuse the existing validReagentMapFixture() and anyViolationContains() helpers, no new scaffolding
- [x] Fewest elements — only the two test cases the analysis identified as missing were added; no source code changes were needed since the salvaged implementation already handled both branches correctly

## Key Decisions

- Added the two missing test cases as new entries in the existing TestCheckSlicesReportsEachViolationKind table rather than new standalone test functions: Matches the established 'break the passing fixture in exactly one way' pattern (AC9) already used for every other violation kind in that table; keeps all violation-kind coverage in one place
- Made no changes to slice.go, slice_check.go, cmd/loom/cmd/intake.go, order.go, vocabulary.go, parse_test.go, slice.go test files, or the templates: Analysis confirmed by read-through and now confirmed by build/test/lint/coverage that the salvaged implementation (commit 432f912) already satisfies every acceptance criterion; the only developer tasks were verification and closing two untested branches, both of which were pure test additions
- Verified both new tests can fail by temporarily breaking the exact guard each one targets (removed the knownLevel check in applyCoversEntry; made checkListedFile swallow the read error) and confirming each test goes red with a message distinct from the happy path: ADR-008 / test-repair-contract requires every test prove a path to failure, not just assert on a string that might already be produced by a different, unrelated check

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- Both previously-untested branches named in analysis.md's edge cases are now closed: slice_check.go:351 ('is not a level (api or ui)') via the new 'covers level outside api or ui' case, and slice_check.go:238 ('which could not be read') via the new 'manifest-listed file deleted' case. Both were verified to go red when the guarded behavior is removed.
- go test ./internal/intake/... -coverprofile shows 98.9% statement coverage, well above the 85% floor; the handful of remaining uncovered lines (readManifest's malformed-line branch, two lines each in checkCoverage/applyCoversEntry) are pre-existing and were not named as gaps in analysis.md's edgeCases, so I left them out of scope for this stage.
- gocyclo -over 6 and golangci-lint run both report zero issues across internal/intake/slice.go, internal/intake/slice_check.go, and cmd/loom/cmd/intake.go — AC10's complexity requirement is confirmed by tool, not just read-through.
- internal/intake/vocabulary_test.go (the pre-existing template vocabulary test AC8 calls out) is unmodified by git diff and passes, confirming no regression to pre-existing order-level field checks.
- One unrelated test failure appears in `go test ./...` when TRACEPARENT is set in the ambient shell: internal/provider/claude's TestInvokeSetsNoTraceParentWhenUntraced. Confirmed environmental (passes with `env -u TRACEPARENT`), and internal/provider/claude is outside this order's scope.paths — not a regression from this change.
- git status confirms every changed/new file falls inside scope.paths (internal/intake/**, cmd/loom/cmd/intake*.go, templates/claude-feature-team/intake/**, docs/features/intake-slice-check/**).

## Notes for DevOps

None
