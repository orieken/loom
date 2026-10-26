---
feature: "delivery-record-relative-paths"
bounded_context: ""
domain_terms: ["ArchiveRecords", "run-state.json", "run-events.jsonl", "repository root"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: delivery-record-relative-paths

## Archived run-state.json stores paths relative to the repository root

Verifies: Given a workspace whose run-state.json holds absolute paths under the repository root, when ArchiveRecords runs, then the archived run-state.json holds the same paths relative to the repository root, slash-separated.

```gherkin
Scenario: Archived run-state.json stores paths relative to the repository root
  Given A workspace's run-state.json records file paths as absolute paths under the repository root
  When The run's delivery records are archived
  Then The archived run-state.json holds those same paths relative to the repository root
  And Each relative path uses forward slashes as separators
```

## A path outside the repository root is archived unchanged

Verifies: Given a path in run state that is not under the repository root, such as a spec outside the checkout, when the records are archived, that path is kept unchanged — never turned into a ../ path and never dropped.

```gherkin
Scenario: A path outside the repository root is archived unchanged
  Given A workspace's run state records a path that is not under the repository root, such as a spec file outside the checkout
  When The run's delivery records are archived
  Then The archived run state holds that path exactly as it was, byte for byte
  And The path is not rewritten as a relative '../' path
  And The path is not dropped from the archived record
```

## An absolute repository root quoted inside a run-events.jsonl line is stripped on archive

Verifies: Given a run-events.jsonl line whose text contains the absolute repository root, such as a stage error quoting a file path, when the records are archived, the archived line has the root prefix removed and is otherwise unchanged. Every archived line still parses as JSON.

```gherkin
Scenario: An absolute repository root quoted inside a run-events.jsonl line is stripped on archive
  Given A workspace's run-events.jsonl contains a line whose text quotes the absolute repository root, such as a stage error message quoting a file path
  When The run's delivery records are archived
  Then The archived run-events.jsonl line has the repository root prefix removed from that text
  And The archived line is otherwise unchanged
  And Every line in the archived run-events.jsonl still parses as valid JSON
```

## No archived delivery record contains the absolute repository root

Verifies: Given the change is complete, no archived run-state.json, run-events.jsonl or state/*.json contains the absolute repository root as a substring, and a test asserts this.

```gherkin
Scenario: No archived delivery record contains the absolute repository root
  Given A run's delivery records, including run-state.json, run-events.jsonl, and any state/*.json documents, have been archived
  When The archived files are inspected
  Then None of run-state.json, run-events.jsonl, or any state/*.json file contains the absolute repository root as a substring
```

## Archiving leaves the workspace's own records untouched

Verifies: Given the records are archived, the workspace's own run-state.json and run-events.jsonl are byte-identical to what they were before, and a test asserts this.

```gherkin
Scenario: Archiving leaves the workspace's own records untouched
  Given A workspace has a run-state.json and a run-events.jsonl before archiving
  When The run's delivery records are archived
  Then The workspace's run-state.json is byte-identical to what it was before archiving
  And The workspace's run-events.jsonl is byte-identical to what it was before archiving
```

## Memory ingest reads an archived run with relativized paths the same as before

Verifies: Given archived records with relative paths, when loom memory ingest reads docs/features/*/, it ingests the run exactly as it did before. The existing memory tests still pass.

```gherkin
Scenario: Memory ingest reads an archived run with relativized paths the same as before
  Given Delivery records for a run have been archived with relative paths under docs/features/<feature>/
  When loom memory ingest reads docs/features/*/
  Then The run is ingested exactly as it was before paths were relativized
  And The existing memory tests still pass
```

## The change meets its build, complexity, and scope guarantees

Verifies: Given the change is complete, go test ./internal/memory/... ./cmd/loom/... passes, every new function has cyclomatic complexity below 7, and no file outside scope.paths changes except this run's own delivery record.

```gherkin
Scenario: The change meets its build, complexity, and scope guarantees
  Given The relative-paths change is complete
  When go test ./internal/memory/... ./cmd/loom/... is run
  Then The test suite passes
  And Every new function has cyclomatic complexity below 7
  And No file outside scope.paths changes except this run's own delivery record
```
