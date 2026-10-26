---
feature: "delivery-record-relative-paths"
bounded_context: ""
domain_terms: ["Delivery record", "Workspace", "Feature Archive", "Run State", "Run Event Timeline"]
files_touched: ["internal/memory/workspace.go", "internal/memory/ingest_test.go", "cmd/loom/cmd/run_memory.go"]
issue_refs: ["delivery-record-relative-paths"]
linked_adrs: []
linked_kis: []
---

# Implementation Notes: delivery-record-relative-paths

## Files Created

None

## Files Modified

- internal/memory/workspace.go
- internal/memory/ingest_test.go
- cmd/loom/cmd/run_memory.go

## Interface Design

- ArchiveRecords(workspaceDir, archiveDir, root string) error — root is the repository root; an empty root leaves archived content unchanged, matching today's behavior at every existing call site.
- relativizeRunPaths(content []byte, root string) []byte — unexported; strips every occurrence of root+separator from content, no-op when root is empty.
- copyRelativized(source, target, root string) error — unexported sibling to copyIfPresent; missing source is a no-op, writes relativizeRunPaths(content, root) instead of raw bytes.
- archiveTypedState(workspaceDir, archiveDir, root string) error and copyStageDocuments(entries, sourceDir, targetDir, root string) error — root threaded through to copyRelativized for .json stage documents.
- recordEpisode resolves root via filepath.Abs(".") (mirroring run.go's prepareRunWorkspace) and passes it to memory.ArchiveRecords; a resolution failure is reported to stderr and root falls back to "" so the strip no-ops rather than aborting the run.

## Named Refactoring Log

- **Extract Function** on `internal/memory/ingest_test.go — mustReadFile(t, path) []byte` — TestArchiveLeavesTheWorkspaceByteIdentical's four repeated read-then-check-err blocks pushed cyclomatic complexity to 9; extracting the read-or-fail pattern into one helper (as the analysis's own task list implied by keeping new functions low-complexity) brought the test back under 7, confirmed by golangci-lint's gocyclo rule.

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new/changed functions — golangci-lint run (gocyclo min-complexity 7) reports 0 issues after extracting mustReadFile from the one test that initially tripped it at 9.
- [x] No primitive obsession — root stays a plain string per architecture-notes.md's explicit decision against an options struct for a single parameter.
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — No new import added to internal/memory/workspace.go — relativizeRunPaths and copyRelativized use only bytes/os/filepath/strings, already imported.

## Simple Design Verification

- [x] Passes all tests — go test ./internal/memory/... ./cmd/loom/... -count=1 passes in full, including the real-binary end-to-end TestIngestRebuildsTheStoreFromTheArchive.
- [x] Reveals intention — copyRelativized and relativizeRunPaths named for exactly what they do; no renames needed beyond the plan.
- [x] No duplication — copyRelativized intentionally duplicates copyIfPresent's read-or-skip shape rather than adding a boolean flag to it — a deliberate trade-off recorded in architecture-notes.md, not an oversight.
- [x] Fewest elements — No options struct, interface, or extra parameter beyond the one root string the spec requires.

## Key Decisions

- Implemented exactly the architecture-notes.md plan: ArchiveRecords grows a third root parameter, a full-byte-content substring strip (not field-by-field JSON rewriting), and root is resolved independently in recordEpisode via filepath.Abs(".") rather than threaded from run.go.: architecture-notes.md already worked through the alternatives (options struct, boolean flag, threading ProjectRoot through run.go) and rejected each with a stated trade-off; re-deriving the same analysis during implementation would have added risk without adding information.
- Extracted mustReadFile in ingest_test.go rather than leaving TestArchiveLeavesTheWorkspaceByteIdentical as one block.: The straightforward before/after read-and-compare test tripped gocyclo at 9; the fix is a boy-scout-rule-sized extraction, not a design change to the test's assertions.

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- New tests already cover every AC directly: TestArchiveRelativizesPathsUnderRoot (AC1), TestArchivePreservesAPathOutsideRoot (AC2), TestArchiveStripsRootFromEventText (AC3), TestArchivedRecordsNeverContainTheRoot (AC4), TestArchiveLeavesTheWorkspaceByteIdentical (AC5). AC6 (ingest from relativized archive) and AC7 (go test + complexity + scope) are covered by the existing TestIngestRebuildsTheStoreFromTheArchive end-to-end test and golangci-lint, both verified green in this session.
- git diff --stat confirms only internal/memory/workspace.go, internal/memory/ingest_test.go and cmd/loom/cmd/run_memory.go changed outside this run's own delivery record — no file outside scope.paths was touched.
- grep -rn "ArchiveRecords(" --include="*.go" . confirms all 8 call sites (1 production in run_memory.go, 7 in ingest_test.go) were updated — no call site was missed.

## Notes for DevOps

None
