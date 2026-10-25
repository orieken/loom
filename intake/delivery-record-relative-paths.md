---
intake: 1
id: delivery-record-relative-paths
type: bug
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-24
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 25
  max-hours: 2
scope:
  paths: [internal/memory/**, cmd/loom/cmd/run_memory.go]
---

# Archive a run's records without the machine's absolute paths

## Request

> The first real factory PR (#13) committed `docs/features/i-4-vocabulary-test/run-state.json` and
> `run-events.jsonl` with 48 lines naming `/Users/oscarrieken/Projects/...`. The delivery record is
> committed and pushed, so every run publishes the path of the checkout it ran in.

## Summary

At the end of every `loom run`, `memory.ArchiveRecords` (`internal/memory/workspace.go`) copies the
run's records from the workspace into `docs/features/<feature>/`, the committed delivery record. It
copies `run-state.json`, `run-events.jsonl` and the typed state documents byte for byte. Run state
stores absolute paths (`specPath`, and each stage's `artifactPath` and `viewPath`), and event error
text can embed them too. The archived copy MUST name paths relative to the repository root instead.
The workspace copy MUST stay exactly as it is: resume, digest verification, the factory supervisor
and its PR summary all read absolute paths from it.

## Acceptance Criteria

- [ ] Given a workspace whose `run-state.json` holds absolute paths under the repository root, when
      `ArchiveRecords` runs, then the archived `run-state.json` holds the same paths relative to the
      repository root, slash-separated. Example: `<root>/.claude/feature-workspace/f/state/analyst.json`
      becomes `.claude/feature-workspace/f/state/analyst.json`.
- [ ] Given a path in run state that is not under the repository root, such as a spec outside the
      checkout, when the records are archived, then that path is kept unchanged. It is never turned
      into a `../` path and never dropped.
- [ ] Given a `run-events.jsonl` line whose text contains the absolute repository root, such as a
      stage error quoting a file path, when the records are archived, then the archived line has the
      root prefix removed and is otherwise unchanged. Every archived line still parses as JSON.
- [ ] Given the change is complete, then no archived `run-state.json`, `run-events.jsonl` or
      `state/*.json` contains the absolute repository root as a substring, and a test asserts this.
- [ ] Given the records are archived, then the workspace's own `run-state.json` and
      `run-events.jsonl` are byte-identical to what they were before, and a test asserts this.
- [ ] Given archived records with relative paths, when `loom memory ingest` reads
      `docs/features/*/`, then it ingests the run exactly as it did before. The existing memory tests
      still pass.
- [ ] Given the change is complete, then `go test ./internal/memory/... ./cmd/loom/...` passes,
      every new function has cyclomatic complexity below 7, and no file outside `scope.paths`
      changes. The run's own delivery record under `docs/features/delivery-record-relative-paths/`
      is the one exception.

## Out of Scope

- Changing how the executor writes run state in the workspace. Absolute paths there are load-bearing.
- The factory's archive under `.claude/factory/runs/<id>`, which is never committed.
- Rewriting delivery records already committed, such as `docs/features/i-4-vocabulary-test/`.

## Domain Language

- **Delivery record**: the committed copy of a run's records and artifacts under
  `docs/features/<feature>/`.
- **Workspace**: the run's working directory under `.claude/feature-workspace/<feature>/`. It is
  never committed.

## Non-Functional Requirements

- Archiving MUST stay best-effort, as it is today. A record that cannot be rewritten is reported and
  never fails the run.

## Trust Boundaries

None. The change rewrites files loom itself wrote.

## Test Approach

Go unit tests in `internal/memory` with the standard `testing` package. Build a temporary repository
root, write a workspace whose run state and events name absolute paths under that root, archive it,
and assert on both copies.

## Open Questions

- How the repository root reaches `ArchiveRecords`. Assumption if unresolved: the caller in
  `cmd/loom/cmd/run_memory.go` passes the absolute working directory, which is the directory
  `loom run` resolves `docs/features/` against.

## Infrastructure / Deploy Notes

None.
