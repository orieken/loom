---
feature: "state-files-deleted"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: state-files-deleted

## Generated schemas document the new deletion fields and the rename convention

Verifies: implementation.schema.json and qa.schema.json each gain an optional filesDeleted or testFilesDeleted array of paths, generated from the Go structs (never hand-edited), with a description stating that a rename is the new path in filesCreated and the old path in filesDeleted.

```gherkin
Scenario: Generated schemas document the new deletion fields and the rename convention
  Given the ImplementationState and QAState Go structs declare filesDeleted and testFilesDeleted fields
  When the schema generator runs
  Then implementation.schema.json contains an optional filesDeleted array of paths
  And qa.schema.json contains an optional testFilesDeleted array of paths
  And each field's description states that a rename is encoded as the new path in filesCreated and the old path in filesDeleted
```

## A deletion claim passes when the path is gone now and was present at stage start

Verifies: A stage that lists a path in filesDeleted passes the claim when the path is absent now and existed when the stage started, observed as the path appearing in the posture check's changed-paths set (which reports status against the run's base commit).

```gherkin
Scenario: A deletion claim passes when the path is gone now and was present at stage start
  Given a stage lists a path in filesDeleted
  And the path existed when the stage started and is absent from disk now
  And the posture check's changed-paths set, measured against the run's base commit, includes that path
  When the filesDeleted claim is checked for that stage
  Then the claim passes
```

## A deletion claim fails when the file still exists on disk

Verifies: A path in filesDeleted that still exists on disk fails the stage with a ClaimError naming the field and the path: "the file still exists — the stage reported a deletion it did not do".

```gherkin
Scenario: A deletion claim fails when the file still exists on disk
  Given a stage lists a path in filesDeleted
  And the path still exists on disk
  When the filesDeleted claim is checked for that stage
  Then the stage fails with a ClaimError naming the field and the path
  And the error message reads "the file still exists — the stage reported a deletion it did not do"
```

## A deletion claim fails when the path never existed

Verifies: A path in filesDeleted that never existed (absent now and absent from the posture check's changed-paths set) fails the stage with a ClaimError: "no such file was there to delete".

```gherkin
Scenario: A deletion claim fails when the path never existed
  Given a stage lists a path in filesDeleted
  And the path is absent from disk now
  And the path is absent from the posture check's changed-paths set
  When the filesDeleted claim is checked for that stage
  Then the stage fails with a ClaimError reading "no such file was there to delete"
```

## A rename encoded as an arrow in a path field is rejected before claim-checking

Verifies: An entry containing " -> " in any path-naming field (filesCreated, filesModified, filesDeleted, testFilesCreated, testFilesModified, testFilesDeleted, or security's filesModified) fails validation with a message instructing the caller to list the new path in filesCreated and the old path in filesDeleted.

```gherkin
Scenario: A rename encoded as an arrow in a path field is rejected before claim-checking
  Given a stage's state document lists an entry containing " -> " in a path-naming field
  When the state document is validated
  Then validation fails before any claim is checked
  And the error message instructs the caller to list the new path in filesCreated and the old path in filesDeleted
```

## A deletion-only implementation satisfies the touched-files requirement

Verifies: requireTouchedFiles counts an implementation that only deletes files as having touched something; it no longer requires filesCreated or filesModified to be non-empty when filesDeleted names at least one path.

```gherkin
Scenario: A deletion-only implementation satisfies the touched-files requirement
  Given an implementation state document with empty filesCreated and empty filesModified
  And the same document names at least one path in filesDeleted
  When requireTouchedFiles evaluates the document
  Then the document is accepted as having touched something
  And requireTouchedFiles does not demand a non-empty filesCreated or filesModified
```

## A state document without a deleted-files field validates unchanged

Verifies: An existing state document with no filesDeleted or testFilesDeleted field (the omitempty default) validates exactly as it did before this change — no new required field, no changed behavior for filesCreated/filesModified/testFilesCreated/testFilesModified claims.

```gherkin
Scenario: A state document without a deleted-files field validates unchanged
  Given a state document with no filesDeleted field and no testFilesDeleted field
  When the document is validated and its claims are checked
  Then validation succeeds with no new required field
  And the existing filesCreated, filesModified, testFilesCreated, and testFilesModified claims behave exactly as before this change
```

## The implementation meets the project's complexity, coverage, and scope bars

Verifies: Every new or changed function has cyclomatic complexity below 7 (enforced by golangci-lint's gocyclo, already configured at min-complexity 7 in .golangci.yml), every changed line is covered by a test proven able to fail, go test ./... and the lint pass, and no file outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**) changes.

```gherkin
Scenario: The implementation meets the project's complexity, coverage, and scope bars
  Given the filesDeleted feature has been implemented
  When golangci-lint runs with gocyclo min-complexity 7
  And go test ./... runs
  And the set of changed files is compared against scope.paths
  Then every new or changed function has cyclomatic complexity below 7
  And every changed line is covered by a test that fails when that line is reverted
  And go test ./... and the lint pass both succeed
  And no changed file falls outside internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, or docs/features/state-files-deleted/**
```
