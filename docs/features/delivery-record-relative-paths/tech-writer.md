# Documentation Report: delivery-record-relative-paths

## Files Updated

None. This is an internal correctness fix with no user-facing or API surface: it only changes which bytes `ArchiveRecords` writes into `docs/features/<feature>/run-state.json`, `run-events.jsonl`, and `state/*.json` — the path strings inside those files go from absolute to repository-relative. No command, flag, config key, or output format changes for a user of `loom`.

## Files Unchanged (and why)

- `internal/memory/workspace.go` — Already carries the documentation this change needs. The developer's doc comments on `ArchiveRecords`, `copyRelativized`, and `relativizeRunPaths` (lines 50–63, 185–215) state the `root` parameter's contract, why an empty root is a no-op, and why the workspace's own copy is read, never written. Duplicating that into a separate doc would drift from the code it describes; the convention this codebase already follows (see the file's other functions) is doc comments at the call site, not a separate prose document.
- `README.md`, `cmd/loom/README.md` — Neither documents the internal shape of archived path strings, only that `run-state.json` / `run-events.jsonl` get archived into `docs/features/<name>/` at all. That fact is unchanged by this feature, so neither needed an edit.
- `docs/ARCHITECTURE.md` — Same reasoning as above: its "Measured" artifact inventory (lines 164–226) describes what gets archived and why, not the absolute-vs-relative shape of path values inside those files. No claim there becomes false.
- `docs/MIGRATION.md` — Its one hit on "absolute path" (`source_repo` in the install receipt) is an unrelated field; not in scope.
- No root `CHANGELOG.md` exists in this repository (only subdirectory changelogs under `shared/agents/`, `shared/schemas/`, and a test fixture), so there is no changelog entry to add.
- No ADR was written. `architecture-notes.md`'s structural decisions (the `root` parameter shape, independent resolution in `recordEpisode`, full-byte-content strip, the `copyRelativized` sibling function) are already captured there and in the code's own doc comments; none of them rises to a project-wide decision an ADR exists to record, and none of the existing ADRs (checked `docs/adrs/`) claims anything this change contradicts.
- `docs/features/delivery-record-relative-paths/` — This run's own delivery record is populated by the pipeline's standard persistence step, not by this report.

## Notes for DevOps

None.