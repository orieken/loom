---
feature: "delivery-record-relative-paths"
bounded_context: "Documentation Knowledge Base"
domain_terms: ["Delivery record", "Workspace"]
files_touched: ["internal/memory/workspace.go", "internal/memory/workspace.go", "internal/memory/workspace.go", "cmd/loom/cmd/run_memory.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Architecture Notes: delivery-record-relative-paths

## Structural Decisions

### Add `root string` as ArchiveRecords' third parameter rather than an options struct or a RootResolver interface.
- **Rationale**: This is a single string-rewrite concern local to one function family. An options struct or interface would be speculative generalization for one new parameter — Kent Beck's fewest-elements rule and CLAUDE.md's anti-pattern radar both argue against it. The compiler enforces every call site (production and all 7 test sites) is updated, so no feature flag or phased rollout is needed.
- **Trade-offs**:
  - A future 4th parameter would push past Sandi Metz's 4-parameter ceiling and force a parameter object then — not a cost worth paying now for a parameter that doesn't exist yet
  - Every existing test call site must pass an explicit "" rather than getting a default — intentional, since "" is itself meaningful (no-op strip)
- **Fitness Function**: Every call to memory.ArchiveRecords passes exactly 3 arguments — verified by go build ./... — a missing or extra argument at any call site is a compile error, not a runtime defect
### Resolve root independently inside recordEpisode via filepath.Abs(".") rather than threading run.go's StageInput.ProjectRoot through a new parameter chain.
- **Rationale**: scope.paths restricts this change to internal/memory/** and cmd/loom/cmd/run_memory.go. Threading ProjectRoot from run.go would touch run.go, outside scope. filepath.Abs(".") is the exact expression run.go's prepareRunWorkspace already uses to build ProjectRoot, and no os.Chdir exists anywhere in the codebase, so the two expressions are provably equal for the process's lifetime. This resolves the spec's own open question in the direction its stated assumption already pointed.
- **Trade-offs**:
  - Two independent call sites compute the same value instead of one shared value flowing through — duplication forced by scope.paths, not chosen for its own sake
  - If any future change introduces os.Chdir anywhere in the process, this duplication becomes a latent correctness bug the day the two call sites can disagree — nothing today enforces that they can't diverge except the absence of os.Chdir
- **Fitness Function**: No os.Chdir call exists anywhere in the Go source tree that could make run.go's and run_memory.go's filepath.Abs(".") disagree — verified by CI grep check: grep -rln 'os\.Chdir' --include='*.go' . must return no matches; add as a one-line fitness check alongside the existing golangci-lint run, or rely on code-reviewer catching a new os.Chdir call in review until that check is wired
### Implement relativization as a generic substring strip over each file's full byte content (bytes.ReplaceAll(content, []byte(root+separator), nil)), not a field-by-field rewrite of parsed JSON path fields.
- **Rationale**: Simple Design's priority order favors fewest elements once the tests and intent-revealing bar are met. A full-content strip automatically covers any future path-bearing field added to RunState or Event with no further change, and it is the only approach that also satisfies AC3 — rewriting a root-prefixed path quoted inside free-form error text, which no typed-field rewrite could reach.
- **Trade-offs**:
  - Cannot distinguish a genuine path occurrence from the same literal text appearing in unrelated prose — accepted deliberately; the spec's own AC3 example requires exactly this non-contextual stripping
  - Relies on root (a filesystem path) never containing a byte sequence that would corrupt JSON structure if removed — true for a real repository checkout path, but undocumented as an explicit precondition anywhere in the code
- **Fitness Function**: Archived run-state.json, run-events.jsonl and state/*.json never contain the absolute repository root as a substring, and every archived run-events.jsonl line still parses as JSON — verified by go test ./internal/memory/... — new test asserts !bytes.Contains(archivedContent, []byte(root)) across a fixture carrying root-prefixed paths in SpecPath, a StageRecord's ArtifactPath, an IterationArtifact's Path, and an Event's Error text, and re-parses each archived run-events.jsonl line with encoding/json
### Write copyRelativized as a new sibling helper to copyIfPresent, not a boolean `relativize bool` parameter added to copyIfPresent itself.
- **Rationale**: CLAUDE.md's flag list names a boolean parameter on a public method as something to split into two functions; copyIfPresent is already used unmodified by archiveArtifacts for .md artifacts, so adding a branch inside it would make one function serve two unrelated contracts. Two small, intention-revealing functions are simpler to read than one function with a mode switch.
- **Trade-offs**:
  - copyRelativized duplicates copyIfPresent's read-or-skip-then-write shape, differing only in the relativize call — accepted because merging them is exactly the boolean-parameter shape this decision avoids
- **Fitness Function**: No function in internal/memory/workspace.go takes a boolean parameter — verified by Manual review against CLAUDE.md's explicit flag item ("Boolean parameter on a public method") during code-reviewer's pass — no automated lint rule enforces this in the current .golangci.yml, so this is reviewer judgment until one is added
### A root-resolution failure inside recordEpisode is reported to stderr and treated as root = "" (relativization no-ops), rather than aborting ArchiveRecords.
- **Rationale**: The spec's NFR requires archiving to stay best-effort, matching recordEpisode's existing pattern where ArchiveRecords and ingestWorkspace failures are already reported via cmd.PrintErrf and never fail the run. Falling back to "" reuses relativizeRunPaths' own no-op branch rather than adding a second error path.
- **Trade-offs**:
  - A workspace whose root cannot be resolved silently archives with absolute paths intact instead of failing loudly — accepted because that is exactly today's behavior for every other archiving failure mode
- **Fitness Function**: recordEpisode never aborts or fails loom run when root resolution or archiving fails — verified by Manual review confirming the new root-resolution branch follows the same cmd.PrintErrf-and-continue shape as the existing ArchiveRecords and ingestWorkspace error branches in recordEpisode; filepath.Abs(".") failing in practice requires an unreadable cwd, which existing tests do not simulate

## Component Placement

- `relativizeRunPaths` → Adapters layer, `internal/memory/workspace.go` — Pure byte-rewrite helper colocated with the file-copy logic it supports; a single-purpose unexported function does not warrant its own file.
- `copyRelativized` → Adapters layer, `internal/memory/workspace.go` — Sibling to the existing copyIfPresent it specializes; keeps archive-writing logic in one file rather than splitting across files for a ~10-line helper.
- `ArchiveRecords (extended signature)` → Adapters layer, `internal/memory/workspace.go` — Already the sole entry point for archiving; the root parameter extends its existing contract rather than introducing a new component.
- `recordEpisode (modified call)` → Frameworks layer, `cmd/loom/cmd/run_memory.go` — CLI-adjacent glue resolving root and threading it through, consistent with run.go's prepareRunWorkspace resolving the same root expression at the same Frameworks/Drivers layer.

## Bounded Context

- **Owning Context**: Documentation Knowledge Base
- **Context Crossings**: Agent Orchestration — reads and rewrites Run State and Run Event Timeline content whose schema is owned there, though it does not change that schema

## Stability Design

- **Fail-Open (best-effort, non-blocking)** on ArchiveRecords' call site inside recordEpisode, including the new root-resolution step — Archiving is durability, not control flow, per the spec's NFR; a failure anywhere in this path — root resolution, relativization, or the copy itself — is reported to stderr and never aborts loom run, matching the existing ArchiveRecords/ingestWorkspace error handling already in recordEpisode.

## Observability Design

None

## Layer Boundary Checks

- [PASS] internal/memory must not add a new dependency edge outside internal/orchestrator, internal/state, and the standard library — No new import is added to internal/memory/workspace.go — relativizeRunPaths and copyRelativized use only bytes/os/filepath/strings, already imported. Verified by reading the file's import block directly; the loom MCP graph tools (mcp__loom__dependencies_of / dependents_of / describe) were not registered in this session, so the graph itself was not queried.
- [PASS] cmd/loom/cmd (Frameworks/Drivers) may depend on internal/memory (Adapters), never the reverse — recordEpisode already imports github.com/orieken/loom/internal/memory; this change only adds an argument to the existing ArchiveRecords call, not a new import or a reverse dependency.

## Anti-Pattern Check

- Shotgun Surgery: not found — The signature change touches one production call site (recordEpisode) and 7 mechanical test call sites, all adding the same third argument — the compiler enforcing completeness of a narrow change, not surgery spread across unrelated concerns.
- Leaky Abstraction: not found — root is a plain string; callers need only know 'repository root, or empty string to skip relativization' — not ArchiveRecords' internal substring-stripping mechanics.
- Premature Generalization: not found — No options struct, strategy interface, or path-rewriting framework introduced for what is a single string-strip operation on two known file kinds.
- Anemic Domain Model: not found — Not applicable — ArchiveRecords is a utility function operating on file bytes, not a domain entity losing behavior to a service.

## Fitness Functions

- **Every call to memory.ArchiveRecords passes exactly 3 arguments**
  - **Verification**: go build ./...
  - **Owner**: architect
- **No os.Chdir call exists anywhere in the Go source tree**
  - **Verification**: grep -rln 'os\.Chdir' --include='*.go' . must return no matches
  - **Owner**: DevOps
- **Archived run-state.json, run-events.jsonl and state/*.json never contain the absolute repository root as a substring; every archived run-events.jsonl line still parses as JSON**
  - **Verification**: go test ./internal/memory/... — asserts !bytes.Contains(archivedContent, []byte(root)) and re-parses each archived jsonl line
  - **Owner**: architect
- **The workspace's own run-state.json and run-events.jsonl are never mutated by archiving**
  - **Verification**: go test ./internal/memory/... — reads both workspace files before and after ArchiveRecords and asserts byte-for-byte equality
  - **Owner**: architect
- **A path outside the repository root survives archiving unchanged**
  - **Verification**: go test ./internal/memory/... — the existing fixtureRun's out-of-root SpecPath ("/specs/user-auth.md") is asserted byte-identical in the archived copy
  - **Owner**: architect
- **Every new or changed function has cyclomatic complexity below 7**
  - **Verification**: golangci-lint run — gocyclo min-complexity: 7, already configured in .golangci.yml
  - **Owner**: architect

## Refactoring Opportunities

None

## Developer Handoff Notes

- Change ArchiveRecords(workspaceDir, archiveDir string) error to ArchiveRecords(workspaceDir, archiveDir, root string) error in internal/memory/workspace.go.
- Add relativizeRunPaths(content []byte, root string) []byte: return content unchanged when root == ""; otherwise bytes.ReplaceAll(content, []byte(root+string(filepath.Separator)), nil). Keep it this small and unexported.
- Add copyRelativized(source, target, root string) error mirroring copyIfPresent's missing-source-is-a-no-op and error-wrapping behavior, but writing relativizeRunPaths(content, root) instead of the raw bytes read — do not add a boolean flag to copyIfPresent itself.
- In ArchiveRecords, use copyRelativized(..., root) for orchestrator.RunStateFileName and orchestrator.RunEventsFileName. Leave archiveArtifacts' .md copy on copyIfPresent, unchanged — markdown artifacts are out of this spec's scope.
- Thread root through archiveTypedState(workspaceDir, archiveDir, root string) error and copyStageDocuments(entries, sourceDir, targetDir, root string) error; use copyRelativized(..., root) for the .json stage documents.
- In cmd/loom/cmd/run_memory.go's recordEpisode, resolve root, err := filepath.Abs(".") — the same expression run.go's prepareRunWorkspace already uses. On error, report via cmd.PrintErrf exactly like the existing ArchiveRecords/ingestWorkspace branches, and pass root = "" to ArchiveRecords so the strip safely no-ops.
- Update every existing memory.ArchiveRecords(...) call in internal/memory/ingest_test.go (7 sites) to add a third argument, "" in every case, to preserve today's byte-for-byte behavior in tests that don't exercise relativization.
- Do not thread StageInput.ProjectRoot from run.go into recordEpisode — that would touch run.go, outside scope.paths. The independent filepath.Abs(".") in recordEpisode is a deliberate decision (see structuralDecisions), not an oversight to fix later.
- Keep relativizeRunPaths and copyRelativized single-branch and low-complexity — there is no reason either should approach the complexity-7 ceiling for this change.

## Open Architectural Questions

- Does scope.paths naming cmd/loom/cmd/run_memory.go implicitly cover cmd/loom/cmd/run_memory_test.go for an end-to-end assertion that the real call site never leaks the project directory? Recommend treating the named source file as covering its co-located _test.go, the same convention internal/memory/** already gives for free. This does not block development either way: equivalent AC coverage exists via the internal/memory unit tests regardless of how this is resolved.
