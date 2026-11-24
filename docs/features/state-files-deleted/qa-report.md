---
feature: "state-files-deleted"
bounded_context: ""
domain_terms: ["ClaimError", "PathClaim", "posture check"]
files_touched: ["internal/orchestrator/claims_test.go"]
issue_refs: []
linked_adrs: ["ADR-008", "ADR-009"]
linked_kis: []
---

# QA Report: state-files-deleted

## Test Files Created

None

## Test Files Modified

- internal/orchestrator/claims_test.go

## Coverage Summary

- Acceptance criteria covered: 8/8
- Total new tests: 19
- Statement coverage, lowest unit: **89.2%** (`internal/orchestrator`)
  - `internal/state`: 91.4%
  - `internal/orchestrator`: 89.2%

## Test Results

- Passed: 482
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

None

## Notes for Tech Writer

- shared/schemas/pipeline/implementation.schema.json and qa.schema.json each gained an optional filesDeleted/testFilesDeleted array, generated from the Go structs and verified green by TestCommittedSchemasMatchTheStructs; worth a line in any pipeline-contract docs covering the implementation-notes.md / qa-report.md field lists.
- A rename is always two fields now: the new path in filesCreated/testFilesCreated, the old path in filesDeleted/testFilesDeleted. An 'old -> new' entry in any single path-naming field (including security's filesModified) fails validation before any claim is checked.
