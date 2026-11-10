---
feature: "intake-slice-check"
bounded_context: "Agent Orchestration"
domain_terms: ["Slice", "Level", "Fitness Function"]
files_touched: ["internal/intake/slice.go", "internal/intake/slice_check.go", "internal/intake/slice_check_test.go", "internal/intake/slice_test.go", "internal/intake/order.go", "internal/intake/vocabulary.go", "internal/intake/parse_test.go", "cmd/loom/cmd/intake.go", "cmd/loom/cmd/intake_test.go", "templates/claude-feature-team/intake/TEMPLATE.md", "templates/claude-feature-team/intake/README.md"]
issue_refs: ["intake-slice-check", "decision D-31"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Feature Analysis: intake-slice-check

## Summary

Adds a cross-order check, loom intake check, that proves a hand-split feature's slices together cover the spec they split: an order's frontmatter gains an optional slice block (of, spec, digest, manifest, covers), and when several orders in a checked directory share one slice.of, CheckSlices verifies every criterion level the spec declares is covered by exactly one slice, that all slices agree on a manifest digest matching the manifest's own file hashes, and that depends-on edges among them form no cycle and name no missing order. The implementation (salvaged from commit 432f912, already applied and staged in this worktree) is essentially complete and matches every acceptance criterion by direct read-through; this analysis verifies that claim file-by-file and identifies two untested branches plus a scope-lock conflict for the developer/QA stages to close before this ships.

### Acceptance Criteria

- An order's frontmatter MUST accept an optional slice block (of, spec, digest, manifest, covers); an order with no slice block MUST validate exactly as it did before the field existed.
  - Example: internal/intake/order.go: Order.Slice *Slice, yaml tag "slice"; nil when absent
  - Example: TestParseLeavesSliceNilWhenAbsent and TestParseReadsAnOptionalSliceBlock in internal/intake/parse_test.go prove both halves
- A spec criterion with observableVia api or ui MUST have one level to cover, and both MUST give it two (api and ui); AC1:ui covers only that level, a bare AC2 covers every level AC2 has.
  - Example: internal/intake/slice.go: levelsOf() and parseCoversEntry()
  - Example: TestLevelsOf and TestParseCoversEntry in internal/intake/slice_test.go
- Across every order sharing one slice.of, loom intake check MUST report a violation naming the criterion and level for an uncovered level, for a level two slices both cover (naming both orders), and for a covers entry naming a criterion or level the spec does not have.
  - Example: internal/intake/slice_check.go: checkCoverage, checkLevelCoverage, applyCoversEntry
  - Example: TestCheckSlicesReportsEachViolationKind cases: "uncovered level", "doubly covered level", "unknown covers criterion", "unknown covers level"
- Slices of one spec MUST report a violation when their slice.digest values differ, when a digest does not match the SHA-256 of its manifest file, or when a manifest-listed file no longer matches its recorded hash, naming the file.
  - Example: internal/intake/slice_check.go: checkDigestAgreement, checkManifestDigests, checkManifestFiles/checkListedFile
  - Example: TestCheckSlicesReportsEachViolationKind cases: "slice.digest disagreement", "slice.digest does not match the manifest", "manifest-listed file hash mismatch"
- Slices of one spec MUST report a violation for a depends-on cycle among them, naming every order in the cycle, and for a depends-on entry naming no order in the directory.
  - Example: internal/intake/slice_check.go: checkDependsOnCycle (DFS via findCycle), checkDanglingDependsOn
  - Example: TestCheckSlicesReportsEachViolationKind cases: "depends-on cycle", "dangling depends-on"
- A spec or manifest file that cannot be read MUST be a violation on the order naming it, and a path that leaves the project root MUST be a violation; an unverified split must never be reported valid.
  - Example: internal/intake/slice.go: isWithinProjectRoot, readRooted (lexical check, never calls FileReader on an escaping path)
  - Example: TestCheckSlicesReportsEachViolationKind cases: "unreadable spec file", "unreadable manifest file", "slice.spec path leaves the project root", "slice.manifest path leaves the project root"
- A passing split MUST print one line per spec with its slice and level counts, and the exit code contract MUST stay unchanged: 0 only when every order is valid.
  - Example: cmd/loom/cmd/intake.go: checkIntakeSlices prints "%s: %d slice(s), %d level(s) covered"; summarizeIntakeCheck folds slicedSpecs()-len(summaries) into the existing invalid counter
  - Example: TestIntakeCheckVerifiesASliceSplitEndToEnd, TestIntakeCheckFailsOnASliceCoverageGap in cmd/loom/cmd/intake_test.go
- templates/claude-feature-team/intake/TEMPLATE.md and README.md MUST document the slice block, and the existing template vocabulary test MUST still pass.
  - Example: TEMPLATE.md gains a commented slice: block plus a "SLICE" explanation section; README.md gains five bullet points describing the cross-order check
  - Example: internal/intake/vocabulary_test.go is unmodified by the patch — checks only the pre-existing order-level fields, so it stays a regression guard, not new-behavior evidence
- A fixture modeled on the reagent-map split MUST pass (AC1 and AC3 covered once at each level by different slices, AC14 covered bare by one slice), and each violation above MUST have a test built by breaking that fixture in one way, which fails when the check is removed.
  - Example: internal/intake/slice_test.go: validReagentMapFixture() + TestCheckSlicesAcceptsTheReagentMapSplit
  - Example: internal/intake/slice_check_test.go: TestCheckSlicesReportsEachViolationKind (13 break-one-thing cases) and TestCheckSlicesReportsEveryBrokenSpecIndependently
- Every new or changed function MUST have cyclomatic complexity below 7, every changed line MUST be covered by a test that can fail, and no file outside scope.paths MUST change except this run's own delivery record under docs/features/intake-slice-check/.
  - Example: git diff --stat confirms only internal/intake/**, cmd/loom/cmd/intake*.go, and templates/claude-feature-team/intake/** changed — all inside scope.paths
  - Example: Two changed lines appear untested today: slice_check.go:351 (covers entry with a level outside {api, ui}, e.g. AC1:mobile) and slice_check.go:238 (a manifest-listed file that has been deleted rather than tampered) — see edgeCases

### Non-Functional Requirements

- **performance**: A directory with no slice blocks MUST read no extra files beyond what single-order Check already reads.
- **security**: slice.spec and slice.manifest arrive in an order's frontmatter, which may come from an external order; a path outside the project root MUST be refused without ever being passed to the file reader.

## Proposed Fitness Functions

- **CheckSlices performs zero FileReader calls when no order in the batch sets a slice block**
  - **Verification**: go test ./internal/intake -run TestCheckSlicesReadsNothingWhenNoOrderSetsSlice
  - **Owner**: architect
- **slice.spec, slice.manifest, and every manifest-listed file path are refused, not read, when they would resolve outside the project root**
  - **Verification**: go test ./internal/intake -run 'TestIsWithinProjectRoot|TestReadRooted'
  - **Owner**: architect
- **Every function in internal/intake/slice.go, slice_check.go, and cmd/loom/cmd/intake.go has cyclomatic complexity below 7**
  - **Verification**: golangci-lint run with a complexity linter (gocyclo or cyclop) over the changed files
  - **Owner**: architect
- **No file outside an intake order's own scope.paths globs changes during its delivery, delivery-record directory excepted**
  - **Verification**: git diff --name-only against the order's scope.paths globs
  - **Owner**: devops

## Out of Scope

- The slice-order skill that drafts a split, the size warning, and deliver-spec emitting slices
- intake: 2 and inputs; the slice block stays optional under schema version 1, and loom intake pin can replace digest later
- Enforcing anything at claim time — this is loom intake check only
- Adding slice blocks to saturday-monorepo's existing orders (a follow-up in that repository)

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Agent Orchestration
- **Context Crossings**: cmd/loom/cmd (CLI/adapter layer) calling into internal/intake (domain/use-case layer) via the FileReader seam, matching Go Clean Architecture layering

### Domain Events (Event Storming Lite)

- **Commands**: loom intake check [file|directory]...
- **Events Produced**: None
- **Owning Aggregates**: intake.Document (one parsed order), a slice.of group (the set of Documents sharing one Slice.Of, the unit CheckSlices evaluates)
- **Read Models / Projections**: SliceSummary (one printed line per spec whose slices covered it with no violation)

### Affected Components

- `internal/intake/slice.go` — New file: FileReader seam, project-root path guard (isWithinProjectRoot/readRooted), spec package and manifest parsing, SHA-256 digest helpers.
- `internal/intake/slice_check.go` — New file: CheckSlices and every rule it composes — coverage, digest agreement, manifest verification, depends-on cycle/dangling checks.
- `internal/intake/slice_check_test.go` — New file: the reagent-map-modeled fixture, the happy-path test, and one break-the-fixture test per violation kind (AC9).
- `internal/intake/slice_test.go` — New file: unit tests for slice.go's parsing, path guard, and digest helpers, independent of the integration-level fixture.
- `internal/intake/order.go` — Order gains Slice *Slice; the Slice struct declares of/spec/digest/manifest/covers (AC1's schema).
- `internal/intake/vocabulary.go` — Adds the closed vocabularies knownObservableVia (api/ui/both) and knownLevel (api/ui) that criterion and covers-entry validation draw from.
- `internal/intake/parse_test.go` — Adds TestParseLeavesSliceNilWhenAbsent and TestParseReadsAnOptionalSliceBlock, proving AC1's two halves at the parse layer.
- `cmd/loom/cmd/intake.go` — Collects parsed Documents, calls intake.CheckSlices per batch, prints violations and the one-line-per-spec summary, and folds slice violations into the existing invalid/exit-code counter (AC7).
- `cmd/loom/cmd/intake_test.go` — End-to-end command tests over a fixture directory: a clean two-slice split exits 0 with a summary line, and a coverage gap fails the run (AC7).
- `templates/claude-feature-team/intake/TEMPLATE.md` — Documents the optional slice: block and adds a "SLICE" explanation section (AC8).
- `templates/claude-feature-team/intake/README.md` — Documents the cross-order slice check's behavior for a human reading the intake contract (AC8).

### Data Model Changes

- Order's YAML frontmatter gains an optional slice block (of, spec, digest, manifest, covers). An order with no block has Slice == nil and parses/validates exactly as before — additive and backward compatible under intake schema version 1. (**expand** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. Run go build ./... and go test ./internal/intake/... ./cmd/loom/cmd/... locally to confirm the salvaged, already-staged patch actually builds and passes (this analysis could not execute the Go toolchain in its sandbox).
2. Add a slice_check_test.go case where a covers entry names a level outside {api, ui} (e.g. AC1:mobile) and assert the 'is not a level (api or ui)' violation — closes the untested branch at internal/intake/slice_check.go:351.
3. Add a slice_check_test.go case where a manifest lists a file that has been deleted (not merely tampered) and assert the '... which could not be read' violation from checkListedFile — closes the untested branch at internal/intake/slice_check.go:238, distinct from the existing hash-mismatch case.
4. Run a complexity linter (gocyclo, cyclop, or golangci-lint if configured for this repo) over internal/intake/slice.go, slice_check.go, and cmd/loom/cmd/intake.go to confirm every function measures below cyclomatic complexity 7; by read-through none appear close, but AC10 requires a measured confirmation, not an inspection.

### QA Tasks

1. Run go test -coverprofile over internal/intake and cmd/loom/cmd and confirm every changed line in this diff is exercised by a test that can fail (ADR-008), with particular attention to the two branches named under edgeCases.
2. Confirm templates/claude-feature-team/intake/TEMPLATE.md and README.md's documented slice: shape matches internal/intake/order.go's Slice struct fields exactly (of, spec, digest, manifest, covers).
3. Confirm internal/intake/vocabulary_test.go — the pre-existing template vocabulary test AC8 calls out — still passes unmodified, proving the new slice vocabulary did not regress the old field checks.
4. Confirm git diff --name-only against this order's scope.paths (internal/intake/**, cmd/loom/cmd/intake*.go, templates/claude-feature-team/intake/**, docs/features/intake-slice-check/**) shows no file outside that set, per AC10.

### Tech Writer Tasks

None

### DevOps Tasks

None

## Edge Cases and Risks

- An order declares slice.of but shares it with no other order in the batch (a group of exactly one).: CheckSlices still evaluates it as a group of one; every level its own covers list omits is reported uncovered, so a solo slice can never silently pass as a complete split. Proven by TestCheckSlicesEvaluatesAGroupOfOneSlice.
- A covers entry's level suffix is syntactically present but not api or ui (e.g. AC1:mobile).: applyCoversEntry rejects it before even looking up the criterion, with 'is not a level (api or ui)' (slice_check.go:351). Untested today — see developer tasks.
- A manifest lists a file that no longer exists at all, rather than one whose content changed.: checkListedFile reports '%s lists %q, which could not be read: %v' (slice_check.go:238), distinct from the hash-mismatch message. Untested today — see developer tasks.
- Two slice.of groups appear in one batch, and only one of them is broken.: CheckSlices evaluates each group independently and keeps collecting violations across groups rather than stopping at the first broken one. Proven by TestCheckSlicesReportsEveryBrokenSpecIndependently.
- A spec package's criterion has no observableVia, or one outside {api, ui, both}.: levelsOf returns ok=false; checkCoverage reports a violation naming the criterion against the spec file itself — the resolved Open Question's stated assumption, implemented exactly as written.
- A depends-on cycle spans more than two orders within one slice.of group.: findCycle's local DFS detects a cycle of any length and names every order on the cycle path, not just the two endpoints.

## Definition of Done

- go build ./... succeeds and go test ./internal/intake/... ./cmd/loom/cmd/... passes, including the two new test cases closing the coverage gaps named under edgeCases.
- A complexity tool confirms every new or changed function in internal/intake and cmd/loom/cmd/intake.go is below cyclomatic complexity 7.
- go test -coverprofile confirms every line this diff adds or changes is covered by a test that can fail.
- templates/claude-feature-team/intake/TEMPLATE.md and README.md document the slice block, and internal/intake/vocabulary_test.go still passes unmodified.
- git diff --name-only against this order's scope.paths shows no file outside internal/intake/**, cmd/loom/cmd/intake*.go, templates/claude-feature-team/intake/**, and docs/features/intake-slice-check/**.
- loom intake check over the reagent-map-modeled fixture exits 0 and prints one summary line per spec with correct slice and level counts; the existing exit-code contract (0 only when every order is valid) is unchanged for both single-file and cross-order violations.
