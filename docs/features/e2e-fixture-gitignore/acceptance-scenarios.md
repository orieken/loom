---
feature: "e2e-fixture-gitignore"
bounded_context: ""
domain_terms: ["Fixture repository"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: e2e-fixture-gitignore

## The fixture repository ignores per-machine run-state the same way a real loom-adopting repository does

Verifies: The fixture at tests/e2e/testdata/repo/ MUST contain a .gitignore that ignores at least .claude/memory/ and .claude/feature-workspace/.

```gherkin
Scenario: The fixture repository ignores per-machine run-state the same way a real loom-adopting repository does
  Given the fixture repository at tests/e2e/testdata/repo/
  When the fixture's .gitignore is inspected
  Then it ignores .claude/memory/
  And it ignores .claude/feature-workspace/
```

## The harness carries the fixture's .gitignore into the project's first commit

Verifies: When the harness copies the fixture into its project, the .gitignore MUST be copied and committed in the project's first commit.

```gherkin
Scenario: The harness carries the fixture's .gitignore into the project's first commit
  Given the harness copies tests/e2e/testdata/repo/ into a fresh project worktree
  And the harness creates the project's first commit
  When the project's first commit is read back with git show HEAD:.gitignore
  Then the returned content is byte-for-byte identical to the fixture's own .gitignore
```

## The factory still delivers the fixture order cleanly with the mock provider

Verifies: When `go test ./tests/e2e/...` runs with the mock provider, TestTheFactoryDeliversTheFixtureOrder MUST pass and every check in its report MUST print PASS.

```gherkin
Scenario: The factory still delivers the fixture order cleanly with the mock provider
  Given the mock provider is configured for tests/e2e/...
  When go test ./tests/e2e/... runs TestTheFactoryDeliversTheFixtureOrder
  Then the test passes
  And every check in its report prints PASS
```

## The negative-control run still fails the same checks it always has

Verifies: TestTheHarnessFailsARunThatDeliversTheWrongThing MUST still pass, failing the same checks it fails today (the diff stays inside scope.paths, the hidden acceptance tests pass).

```gherkin
Scenario: The negative-control run still fails the same checks it always has
  Given TestTheHarnessFailsARunThatDeliversTheWrongThing runs a delivery that delivers the wrong thing
  When the harness evaluates that run's checks
  Then the check "the diff stays inside scope.paths" fails
  And the check "the hidden acceptance tests pass" fails
  And no other check changes its prior pass or fail outcome
```

## The change stays inside its scope and complexity budget

Verifies: Every new function MUST have cyclomatic complexity below 7, and no file outside tests/e2e/ MUST change except this run's own delivery record under docs/features/e2e-fixture-gitignore/.

```gherkin
Scenario: The change stays inside its scope and complexity budget
  Given the implementation for this feature is complete
  When the project's complexity lint runs over tests/e2e/... and the delivered diff is inspected
  Then every new function reports cyclomatic complexity below 7
  And no changed file lies outside tests/e2e/ except the delivery record under docs/features/e2e-fixture-gitignore/
```
