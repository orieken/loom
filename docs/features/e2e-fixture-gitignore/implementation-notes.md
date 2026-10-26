---
feature: "e2e-fixture-gitignore"
bounded_context: ""
domain_terms: ["Fixture repository"]
files_touched: ["tests/e2e/testdata/repo/.gitignore", "tests/e2e/harness/units_test.go"]
issue_refs: ["#12"]
linked_adrs: []
linked_kis: []
---

# Implementation Notes: e2e-fixture-gitignore

## Files Created

- tests/e2e/testdata/repo/.gitignore

## Files Modified

- tests/e2e/harness/units_test.go

## Interface Design

- No new public surface — tests/e2e/testdata/repo/.gitignore is a static fixture file, and TestSetUpProjectCommitsTheFixturesGitignore is a package-internal test calling the existing unexported setUpProject and run helpers.

## Named Refactoring Log

None

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC — TestSetUpProjectCommitsTheFixturesGitignore is 15 lines.
- [x] Cyclomatic complexity < 7 on all new functions — golangci-lint run ./tests/e2e/... reported 0 issues; the new test has three sequential error checks, no branching logic.
- [x] No primitive obsession: domain concepts wrapped in value objects, not raw strings/ints — Not applicable — a static .gitignore fixture and a test calling existing typed helpers.
- [x] No feature envy: methods use their own class's data more than another's
- [x] No magic numbers or strings: all literals are named constants — The .gitignore paths and order id 'x' are inherent to the fixture/test, not business-logic literals.
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — No internal/ or cmd/ files touched; test only calls existing tests/e2e/harness package functions.

## Simple Design Verification

- [x] Passes the tests — go test ./tests/e2e/... — all tests pass, including TestTheFactoryDeliversTheFixtureOrder (every check PASS) and TestTheHarnessFailsARunThatDeliversTheWrongThing (still fails only its two expected checks).
- [x] Reveals intention — TestSetUpProjectCommitsTheFixturesGitignore names exactly what it proves; the fixture .gitignore mirrors the root .gitignore's own comments for its two entries.
- [x] No duplication — Test reuses the existing fixture(), run(), and setUpProject() helpers rather than reimplementing git plumbing.
- [x] Fewest elements — One new file, one new test function, no changes to deliver.go or exec.go — confirmed by inspection that copyTree already copies dotfiles unfiltered.

## Key Decisions

- Scoped the fixture's .gitignore to only the two per-machine entries (.claude/feature-workspace/, .claude/memory/) rather than mirroring the full root .gitignore.: The spec and analysis both call for only these two per-machine run-state entries; the fixture has no .env, .loom/, or other paths of its own that the rest of the root .gitignore excludes, so copying more would be dead weight.
- Wrote the new test against setUpProject directly (not the full Deliver/Run path) and read the commit back via `git show HEAD:.gitignore` using the package's existing run() helper.: setUpProject is exactly the step that copies the fixture tree and makes the project's first commit; calling it directly keeps the test fast and isolates the behaviour the acceptance criterion is about, without needing a factory pass.
- Made no changes to deliver.go or exec.go.: copyTree (exec.go) walks the fixture with filepath.WalkDir and no name filtering, and setUpProject's existing `git add -A` already stages dotfiles — verified by the new test passing without any change to either file, resolving the spec's Open Question.

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- go test ./tests/e2e/... passes end to end with the mock provider: TestTheFactoryDeliversTheFixtureOrder prints PASS for every check (including the pre-existing ones), and TestTheHarnessFailsARunThatDeliversTheWrongThing still fails only 'the diff stays inside scope.paths' and 'the hidden acceptance tests pass'.
- golangci-lint run ./tests/e2e/... reports 0 issues.
- The new TestSetUpProjectCommitsTheFixturesGitignore in tests/e2e/harness/units_test.go asserts byte-for-byte equality between tests/e2e/testdata/repo/.gitignore and the file read back via `git show HEAD:.gitignore` in the project worktree setUpProject creates.

## Notes for DevOps

None
