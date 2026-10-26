---
feature: "delivery-record-relative-paths"
bounded_context: ""
domain_terms: ["run state", "run-events.jsonl", "typed stage document", "archive", "workspace"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# QA Report: delivery-record-relative-paths

## Test Files Created

None

## Test Files Modified

None

## Coverage Summary

- Acceptance criteria covered: 7/7
- Total new tests: 5
- Statement coverage, lowest unit: **50.0%** (`cmd/loom/cmd/run_memory.go:recordEpisode`)
  - `internal/memory`: 81.3%
  - `internal/memory/workspace.go:relativizeRunPaths`: 100.0%
  - `internal/memory/workspace.go:copyRelativized`: 75.0%
  - `internal/memory/workspace.go:ArchiveRecords`: 62.5%
  - `cmd/loom/cmd`: 83.5%
  - `cmd/loom/cmd/run_memory.go:recordEpisode`: 50.0%

## Test Results

- Passed: 612
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- recordEpisode's filepath.Abs(".") failure branch (cmd/loom/cmd/run_memory.go:35-39): Not covered by any test — filepath.Abs(".") cannot be made to fail without OS-level mocking (e.g. deleting the cwd mid-process), so the fallback-to-empty-root path is exercised only by code review, not a unit test. Already flagged by security-reviewer as LOW; mitigation is the distinctive stderr message, not a test.
- copyRelativized's os.ReadFile/os.WriteFile error branches (workspace.go:196-197, 199-200): Pre-existing error-wrapping pattern mirrored from copyIfPresent, which was never unit-tested for permission-denied/disk-full conditions either. Not new behavior introduced by this feature; consistent with the rest of the file's convention of not testing OS-level I/O failures.

## Notes for Tech Writer

- This feature has no user-facing or API surface — it only changes what bytes get written into archived docs/features/<name>/run-state.json, run-events.jsonl, and state/*.json. No README or usage docs need updating; the workspace.go doc comment on ArchiveRecords already explains the root-relativization behavior for future maintainers.
- All 7 acceptance criteria are directly covered by name-matched tests in internal/memory/ingest_test.go: TestArchiveRelativizesPathsUnderRoot (AC1), TestArchivePreservesAPathOutsideRoot (AC2), TestArchiveStripsRootFromEventText (AC3), TestArchivedRecordsNeverContainTheRoot (AC4), TestArchiveLeavesTheWorkspaceByteIdentical (AC5), TestIngestRebuildsTheStoreFromTheArchive (AC6, re-verified green), and the full suite + golangci-lint run clean (AC7).
- golangci-lint run internal/memory/... cmd/loom/cmd/... reports 0 issues, confirming both new functions (relativizeRunPaths, copyRelativized) stay under the configured gocyclo threshold.
- git diff --stat confirms only internal/memory/workspace.go, internal/memory/ingest_test.go, cmd/loom/cmd/run_memory.go, and this run's own docs/features/delivery-record-relative-paths/ changed — scope is clean.
