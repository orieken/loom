---
feature: "delivery-record-relative-paths"
bounded_context: "Documentation Knowledge Base"
domain_terms: ["Feature Archive", "Feature Workspace", "Run State", "Run Event Timeline"]
files_touched: ["internal/memory/workspace.go", "internal/memory/ingest_test.go", "cmd/loom/cmd/run_memory.go"]
issue_refs: ["delivery-record-relative-paths"]
linked_adrs: []
linked_kis: []
---

# Feature Analysis: delivery-record-relative-paths

## Summary

At the end of `loom run`, `memory.ArchiveRecords` (internal/memory/workspace.go) copies a run's Run State (run-state.json), Run Event Timeline (run-events.jsonl), and typed stage documents (state/*.json) from the Feature Workspace into the Feature Archive (docs/features/<feature>/), which is committed to git. The Feature Workspace copy legitimately stores absolute paths under the repository root (SpecPath, each stage's ArtifactPath/ViewPath/IterationArtifacts, and free-form Error/DiffPath text) because the executor, resume, digest verification, and the factory supervisor all read them from there. The committed Feature Archive copy must not leak the machine's checkout path — PR #13 committed 48 lines naming `/Users/oscarrieken/Projects/...`. The fix adds a `root` parameter to `ArchiveRecords` and strips `root + separator` as a literal substring from the copied `run-state.json`, `run-events.jsonl`, and each `state/*.json` file before writing them into the archive, leaving the workspace's own files byte-for-byte untouched and leaving any path that is not under `root` unchanged. `root` is resolved independently inside `recordEpisode` (cmd/loom/cmd/run_memory.go) via `filepath.Abs(".")` — the same expression `cmd/loom/cmd/run.go`'s `prepareRunWorkspace` already uses to build `StageInput.ProjectRoot` — rather than threaded through a new `run.go` parameter, which keeps the entire diff inside the spec's declared `scope.paths`.

### Acceptance Criteria

- Given a workspace whose run-state.json holds absolute paths under the repository root, when ArchiveRecords runs, then the archived run-state.json holds the same paths relative to the repository root, slash-separated.
  - Example: <root>/.claude/feature-workspace/f/state/analyst.json becomes .claude/feature-workspace/f/state/analyst.json
- Given a path in run state that is not under the repository root, such as a spec outside the checkout, when the records are archived, that path is kept unchanged — never turned into a ../ path and never dropped.
- Given a run-events.jsonl line whose text contains the absolute repository root, such as a stage error quoting a file path, when the records are archived, the archived line has the root prefix removed and is otherwise unchanged. Every archived line still parses as JSON.
- Given the change is complete, no archived run-state.json, run-events.jsonl or state/*.json contains the absolute repository root as a substring, and a test asserts this.
- Given the records are archived, the workspace's own run-state.json and run-events.jsonl are byte-identical to what they were before, and a test asserts this.
- Given archived records with relative paths, when loom memory ingest reads docs/features/*/, it ingests the run exactly as it did before. The existing memory tests still pass.
- Given the change is complete, go test ./internal/memory/... ./cmd/loom/... passes, every new function has cyclomatic complexity below 7, and no file outside scope.paths changes except this run's own delivery record.

### Non-Functional Requirements

- **other**: Archiving MUST stay best-effort, as it is today: a record that cannot be rewritten or relativized is reported to stderr and never fails the run.

## Proposed Fitness Functions

- **Every changed or new function has cyclomatic complexity below 7**
  - **Verification**: golangci-lint run — gocyclo min-complexity: 7 is already configured in .golangci.yml
  - **Owner**: architect
- **Archived run-state.json, run-events.jsonl and state/*.json never contain the absolute repository root as a substring**
  - **Verification**: go test ./internal/memory/... — a new test asserts !bytes.Contains(archivedContent, []byte(root)) after ArchiveRecords, across a workspace fixture carrying root-prefixed paths in SpecPath, a StageRecord's ArtifactPath, an IterationArtifact's Path, and an Event's Error text
  - **Owner**: architect
- **The workspace's own run-state.json and run-events.jsonl are never mutated by archiving**
  - **Verification**: go test ./internal/memory/... — a new test reads both workspace files before and after ArchiveRecords and asserts byte-for-byte equality
  - **Owner**: architect
- **A path outside the repository root survives archiving unchanged**
  - **Verification**: go test ./internal/memory/... — the existing fixtureRun's out-of-root SpecPath ("/specs/user-auth.md") is asserted byte-identical in the archived copy
  - **Owner**: architect

## Out of Scope

- Changing how the executor writes run state in the Feature Workspace — absolute paths there are load-bearing for resume, digest verification, and the factory supervisor.
- The factory's archive under .claude/factory/runs/<id>, which is never committed.
- Rewriting delivery records already committed, such as docs/features/i-4-vocabulary-test/.
- Relativizing the .md artifacts archiveArtifacts copies (analysis.md, qa-report.md, etc.) — the AC's blanket no-root-substring assertion names only run-state.json, run-events.jsonl and state/*.json.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Documentation Knowledge Base
- **Context Crossings**: Agent Orchestration — the fix reads and rewrites Run State and Run Event Timeline content whose schema is owned there, though it does not change that schema

### Domain Events (Event Storming Lite)

- **Commands**: ArchiveRecords(workspaceDir, archiveDir, root)
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: Feature Archive (docs/features/<feature>/run-state.json, run-events.jsonl, state/*.json)

### Affected Components

- `internal/memory/workspace.go` — ArchiveRecords gains a root parameter; a new relativizing copy helper replaces copyIfPresent for run-state.json, run-events.jsonl, and the typed state/*.json documents. copyIfPresent itself, and its use for .md artifacts in archiveArtifacts, is unchanged.
- `internal/memory/ingest_test.go` — Every existing memory.ArchiveRecords(...) call (7 sites) gets a third argument; new tests assert each AC using root-prefixed fixture paths.
- `cmd/loom/cmd/run_memory.go` — recordEpisode resolves root via filepath.Abs(".") — mirroring run.go's own prepareRunWorkspace derivation — and passes it to memory.ArchiveRecords, without widening the change into run.go.

### Data Model Changes

- No field or schema changes to RunState, Event, or any typed stage document. Only the string values of existing path-bearing fields differ between the Feature Workspace copy (absolute, unchanged) and the Feature Archive copy (root-stripped) of the same files. (**none** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. In internal/memory/workspace.go, change ArchiveRecords(workspaceDir, archiveDir string) error to ArchiveRecords(workspaceDir, archiveDir, root string) error.
2. Add an unexported helper relativizeRunPaths(content []byte, root string) []byte in internal/memory/workspace.go: return content unchanged when root == ""; otherwise return bytes.ReplaceAll(content, []byte(root+string(filepath.Separator)), nil).
3. Add an unexported helper copyRelativized(source, target, root string) error mirroring copyIfPresent (a missing source is a no-op, matching error wrapping) but writing relativizeRunPaths(content, root) instead of the raw bytes read from source.
4. In ArchiveRecords, replace copyIfPresent with copyRelativized(..., root) for both orchestrator.RunStateFileName and orchestrator.RunEventsFileName. Leave the .md copy in archiveArtifacts on copyIfPresent, unchanged.
5. Thread root through archiveTypedState(workspaceDir, archiveDir, root string) error and copyStageDocuments(entries, sourceDir, targetDir, root string) error; use copyRelativized(..., root) for the .json stage documents instead of copyIfPresent.
6. In cmd/loom/cmd/run_memory.go, inside recordEpisode, resolve root, err := filepath.Abs(".") (same expression run.go's prepareRunWorkspace already uses). On error, report it the same best-effort way recordEpisode already reports other failures and pass root = "" to ArchiveRecords so the strip no-ops rather than failing the run. Pass root to memory.ArchiveRecords(workspaceDir, featureArchiveDir(feature), root).
7. Update every existing memory.ArchiveRecords(...) call in internal/memory/ingest_test.go (7 sites, lines 213, 235, 265, 305, 316, 336, 356) to add a third argument — "" for every existing test, since none of them exercise relativization and "" preserves today's byte-for-byte behavior unchanged.

### QA Tasks

1. In internal/memory/ingest_test.go, add a test that builds root := t.TempDir(), a workspace under filepath.Join(root, ".claude", "feature-workspace", "f"), sets state.SpecPath and a StageRecord.ArtifactPath to absolute paths under that workspace, archives with that root, reloads the archived run-state.json via orchestrator.NewStateStore(...).Load(), and asserts both fields are now relative (slash-separated, no leading separator) — covering AC1.
2. Add a test asserting fixtureRun's out-of-root SpecPath ("/specs/user-auth.md") is byte-identical in the archived run-state.json after archiving with a real root — covering AC2.
3. Add a test that appends an Event whose Error field contains the absolute root (e.g. quoting a file path under root), archives, reads the archived run-events.jsonl, asserts the root substring is gone from that line, and asserts every line still unmarshals via orchestrator.NewTimeline(...).Read() (or json.Unmarshal per line) — covering AC3.
4. Add a test that writes typed state documents (writeTypedState helper) alongside root-prefixed run-state.json/run-events.jsonl content, archives, and asserts !bytes.Contains against root across the archived run-state.json, run-events.jsonl, and every file under state/ — covering AC4.
5. Add a test that reads the workspace's run-state.json and run-events.jsonl bytes before calling ArchiveRecords and again after, and asserts byte-for-byte equality — covering AC5.
6. Re-run the existing TestIngestRebuildsTheStoreFromTheArchive (cmd/loom/cmd/run_memory_test.go) unmodified in behavior and confirm it still passes against relativized archived files — covering AC6.
7. Run go test ./internal/memory/... ./cmd/loom/... and confirm it passes; grep the repository for ArchiveRecords( and recordEpisode( to confirm no call site outside the ones listed in developer tasks was missed.
8. Run golangci-lint run (or confirm CI's existing gocyclo check) against the two new/changed functions (relativizeRunPaths, copyRelativized) and confirm complexity is well under 7.
9. Confirm with git diff --stat that no file outside internal/memory/**, cmd/loom/cmd/run_memory.go, and docs/features/delivery-record-relative-paths/ changed.

### Tech Writer Tasks

None

### DevOps Tasks

None

## Edge Cases and Risks

- root is the empty string — a caller that cannot resolve filepath.Abs(".") (unlikely, but os.Getwd/Abs can fail if the cwd was removed mid-run), or a test call site passing "" deliberately.: relativizeRunPaths must treat an empty root as a no-op and return content unchanged, never panic or strip an empty-string prefix from every position in the file.
- A run-events.jsonl line's text contains the root string as ordinary prose rather than as a path prefix (e.g. a message that happens to mention the directory name without a following separator).: The substring strip has no context awareness and will still remove it — accepted deliberately, since the spec's own AC3 example (an error message quoting a path) requires exactly this non-contextual stripping; flagged here rather than silently assumed.
- A path in run state equals root exactly, with no trailing separator (e.g. the workspace directory itself recorded as a bare value, with nothing appended).: Not stripped by the root+separator prefix match, since there is no separator to match. Not covered by any AC example and not asserted by a test; documented as a known, accepted gap rather than fixed speculatively.
- state/*.json typed stage documents hold no absolute-path field in any schema that exists today.: The relativizing copy is applied to them anyway, so AC4's assertion holds by construction rather than by the field's current absence, and a future typed field carrying an absolute path is covered without further code changes.
- Windows-style backslash path separators.: Out of scope. filepath.Separator and this project's own CI/dev platforms (Darwin/Linux) are forward-slash throughout; no test or fitness function asserts Windows behavior.
- A crash or interruption partway through ArchiveRecords' copy loop.: Unaffected by this change — copy semantics (best-effort, one file at a time, reported and never fatal) are exactly as they are today; relativization happens in memory before each write, not across files.

## Definition of Done

- go test ./internal/memory/... ./cmd/loom/... passes.
- A test exists and passes for each of the spec's seven acceptance criteria, each asserting the behavior directly rather than inferring it from an unrelated pass.
- relativizeRunPaths and copyRelativized (and any other new or changed function) each have cyclomatic complexity below 7, verified by golangci-lint's configured gocyclo rule.
- git diff touches only internal/memory/**, cmd/loom/cmd/run_memory.go, and docs/features/delivery-record-relative-paths/ (this run's own delivery record) — no other file changes.
- cmd/loom/cmd/run.go is unchanged: root is resolved independently inside recordEpisode, not threaded through StageInput.
- code-reviewer approval recorded; no security-sensitive surface is touched (this run rewrites files loom itself wrote, with no new trust boundary).
