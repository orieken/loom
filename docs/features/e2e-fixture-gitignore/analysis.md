---
feature: "e2e-fixture-gitignore"
bounded_context: "factory end-to-end harness (tests/e2e/harness)"
domain_terms: ["Fixture repository"]
files_touched: ["tests/e2e/testdata/repo/.gitignore", "tests/e2e/harness/units_test.go", "tests/e2e/harness/deliver.go", "tests/e2e/harness/exec.go", "docs/features/e2e-fixture-gitignore/"]
issue_refs: ["#12"]
linked_adrs: []
linked_kis: []
---

# Feature Analysis: e2e-fixture-gitignore

## Summary

The factory's end-to-end harness copies tests/e2e/testdata/repo/ into a throwaway project directory, then runs `git add -A` and commits it as that project's first commit. The fixture currently has no `.gitignore`, so the harness never exercises git behaviour against an ignored path the way a real loom-adopting repository does — the exact gap that let the `:(exclude)`-on-an-ignored-path failure from #12 go unnoticed. This chore adds a `.gitignore` to the fixture mirroring the two per-machine entries (`.claude/memory/`, `.claude/feature-workspace/`) loom's own root `.gitignore` holds, and adds one harness-level assertion proving that file is copied and lands in the project's first commit. No production code changes: `copyTree` (tests/e2e/harness/exec.go) already walks dotfiles via `filepath.WalkDir` with no name filtering, so the existing `git add -A` step in `setUpProject` (tests/e2e/harness/deliver.go) picks the new file up unchanged.

### Acceptance Criteria

- The fixture at tests/e2e/testdata/repo/ MUST contain a .gitignore that ignores at least .claude/memory/ and .claude/feature-workspace/.
  - Example: These are the same two per-machine run-state entries the root .gitignore holds (lines 14 and 22 of /.gitignore).
- When the harness copies the fixture into its project, the .gitignore MUST be copied and committed in the project's first commit.
  - Example: A harness test calls setUpProject (or Deliver) directly, then reads the file back via `git show HEAD:.gitignore` in the project worktree and asserts it equals the fixture's own .gitignore byte-for-byte.
- When `go test ./tests/e2e/...` runs with the mock provider, TestTheFactoryDeliversTheFixtureOrder MUST pass and every check in its report MUST print PASS.
- TestTheHarnessFailsARunThatDeliversTheWrongThing MUST still pass, failing the same checks it fails today (the diff stays inside scope.paths, the hidden acceptance tests pass).
- Every new function MUST have cyclomatic complexity below 7, and no file outside tests/e2e/ MUST change except this run's own delivery record under docs/features/e2e-fixture-gitignore/.

### Non-Functional Requirements

- **other**: The harness MUST still need no network beyond what it uses today — adding a static fixture file introduces no new network call.
- **performance**: The mock-provider run MUST take no longer than it does today, within normal variance — the change adds one small file to copy and commit, not a new pass or network round trip.

## Proposed Fitness Functions

- **The fixture's .gitignore reaches the project's first commit with unchanged content**
  - **Verification**: go test ./tests/e2e/harness/... — new test asserting `git show HEAD:.gitignore` in the project worktree equals tests/e2e/testdata/repo/.gitignore
  - **Owner**: developer
- **New functions keep cyclomatic complexity below 7**
  - **Verification**: golangci-lint (per .golangci.yml) run over tests/e2e/... in CI
  - **Owner**: devops

## Out of Scope

- Any change to internal/ or cmd/ — the :(exclude)-on-an-ignored-path commit fix itself shipped in #12.
- Adding the fixture's .gitignore entries to the harness's own scope or diff checks beyond the acceptance criteria listed.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: factory end-to-end harness (tests/e2e/harness)
- **Context Crossings**: None

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `tests/e2e/testdata/repo/.gitignore` — New file (does not exist today). Must ignore at least .claude/memory/ and .claude/feature-workspace/, the two per-machine run-state entries the root .gitignore carries, so the fixture behaves like a real loom-adopting repository.
- `tests/e2e/harness/units_test.go` — Home for the new package-internal assertion that setUpProject commits the fixture's .gitignore and it reads back unchanged from the project's first commit — consistent with the existing fixtureWithoutRepo / TestDeliverRefusesABrokenFixture style in this file.
- `tests/e2e/harness/deliver.go` — Read only, not modified: setUpProject's existing `copyTree` then `git add -A` + `git commit` step is what carries the new .gitignore into the first commit — confirms no change is needed here.
- `tests/e2e/harness/exec.go` — Read only, not modified: copyTree walks the fixture tree with filepath.WalkDir and no name filtering, which resolves the spec's Open Question — dotfiles are copied like any other file, so the new .gitignore needs no special-casing.
- `docs/features/e2e-fixture-gitignore/` — This run's own delivery record — the one path the acceptance criteria explicitly allow outside tests/e2e/.

### Data Model Changes

None

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. Create tests/e2e/testdata/repo/.gitignore containing at minimum the two lines `.claude/feature-workspace/` and `.claude/memory/`, mirroring the equivalent entries in the root .gitignore (lines 14 and 22). Scope it to only these per-machine run-state paths — do not copy the full root .gitignore, since the fixture has no .env, .loom/, or similar paths of its own to exclude.
2. Add a test to tests/e2e/harness/units_test.go (e.g. TestSetUpProjectCommitsTheFixturesGitignore) that calls setUpProject directly with a Config pointing at the real fixture, then reads the committed .gitignore back from the project's first commit via `git show HEAD:.gitignore` (using the package's existing `run` helper in exec.go), and asserts it equals the bytes of tests/e2e/testdata/repo/.gitignore exactly.
3. Run `go test ./tests/e2e/...` with the mock provider locally and confirm TestTheFactoryDeliversTheFixtureOrder passes with every check printing PASS, and TestTheHarnessFailsARunThatDeliversTheWrongThing still fails only its two expected checks.
4. Verify no change is needed in tests/e2e/harness/exec.go or deliver.go — copyTree's filepath.WalkDir already copies dotfiles, and setUpProject's `git add -A` already stages them into the first commit.

### QA Tasks

1. Re-run go test ./tests/e2e/... with the mock provider and confirm the full report for TestTheFactoryDeliversTheFixtureOrder prints PASS for every check, including the new first-commit .gitignore assertion.
2. Confirm TestTheHarnessFailsARunThatDeliversTheWrongThing is unchanged in behaviour — it must still fail exactly 'the diff stays inside scope.paths' and 'the hidden acceptance tests pass'.
3. Run the project's standard lint command (golangci-lint per .golangci.yml) over tests/e2e/... and confirm no new function exceeds cyclomatic complexity 6.

### Tech Writer Tasks

None

### DevOps Tasks

None

## Edge Cases and Risks

- The fixture's .gitignore could be written to ignore more than the two required paths, accidentally dropping something the harness or hidden acceptance tests expect to find tracked under tests/e2e/testdata/repo/.: Keep the new .gitignore scoped to exactly `.claude/memory/` and `.claude/feature-workspace/`. The existing fixture has no files under those paths today, so this is additive and inert until a future fixture scenario adds one.
- The new .gitignore is added in the wrong location (e.g. at tests/e2e/testdata/ instead of inside repo/) and never gets copied into the project at all.: The new harness test reads the file back from the project's first commit via `git show HEAD:.gitignore` — it fails loudly with a missing-path error rather than silently passing if the file isn't where copyTree expects it.
- Spec's Open Question: whether the harness's copy step (copyTree) skips dotfiles when walking the fixture tree.: Resolved by inspection, not assumption: copyTree (tests/e2e/harness/exec.go) uses filepath.WalkDir with no name filtering, so it copies .gitignore exactly as any other file. The new test additionally proves this mechanically for .gitignore specifically.

## Definition of Done

- tests/e2e/testdata/repo/.gitignore exists and ignores at least .claude/memory/ and .claude/feature-workspace/.
- A new harness test proves the fixture's .gitignore reaches the project's first commit and its content round-trips exactly via `git show HEAD:.gitignore`.
- go test ./tests/e2e/... passes with the mock provider; TestTheFactoryDeliversTheFixtureOrder reports every check as PASS.
- TestTheHarnessFailsARunThatDeliversTheWrongThing still fails only 'the diff stays inside scope.paths' and 'the hidden acceptance tests pass'.
- Every new function has cyclomatic complexity below 7.
- No file outside tests/e2e/ changes, except this run's own delivery record under docs/features/e2e-fixture-gitignore/.
