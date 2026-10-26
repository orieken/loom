# Context Manifest: delivery-record-relative-paths

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/memory/workspace.go` — Defines ArchiveRecords, archiveTypedState, copyStageDocuments, and copyIfPresent — the exact functions the bug asks to change so the archived copy relativizes paths while the workspace copy stays byte-identical.
- `cmd/loom/cmd/run_memory.go` — The only production call site of memory.ArchiveRecords (inside recordEpisode, called from run.go's defer). Whatever root/projectRoot value the fix needs must be threaded through this function's signature.
- `cmd/loom/cmd/run.go` — prepareRunWorkspace (builds orchestrator.StageInput.ProjectRoot as the absolute cwd) and the deferred recordEpisode(...) call at line 194 are both here — this is where the repository root the open question asks about already exists and must be passed down.
- `internal/orchestrator/provider.go` — Declares StageInput.ProjectRoot with the comment establishing it as 'the repository the run targets' — the authoritative definition of the root value this fix needs to reach ArchiveRecords.
- `internal/orchestrator/state.go` — RunState.SpecPath and StageRecord.ArtifactPath/ViewPath/IterationArtifacts are the run-state.json fields that store absolute paths today and must end up relative in the archived copy. Read-only reference — this file is not edited by scope.paths.
- `internal/orchestrator/timeline.go` — Event.DiffPath and Event.Error are the run-events.jsonl fields that can embed the absolute repository root (e.g. a stage-failure error quoting a rejected-payload path). Read-only reference, not edited by scope.paths.
- `internal/memory/ingest_test.go` — Holds every existing memory.ArchiveRecords(...) call and the fixtureRun() fixture (absolute-looking but non-root SpecPath/DiffPath values). The new root parameter changes this call's signature everywhere it's used, and these tests characterize the behavior that must not regress.
- `internal/memory/helpers_test.go` — writeWorkspace(), the shared helper every archive test in ingest_test.go uses to lay down run-state.json/run-events.jsonl exactly as the executor does.
- `cmd/loom/cmd/run_memory_test.go` — End-to-end test that builds the real loom binary and runs `loom run --provider mock` against a temp project, then asserts on the archived docs/features/user-auth/run-state.json — the natural place for a new test proving the archived file has no absolute-root substring, using buildLoomBinary/writeSpec/runLoom helpers already defined here.

## Token Budget

- **Status**: OK — ~17249 tokens, 14.4% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/memory/workspace.go` | 7801 | 1950 |
| `cmd/loom/cmd/run_memory.go` | 1673 | 418 |
| `cmd/loom/cmd/run.go` | 14573 | 3643 |
| `internal/orchestrator/provider.go` | 2661 | 665 |
| `internal/orchestrator/state.go` | 15253 | 3813 |
| `internal/orchestrator/timeline.go` | 6614 | 1653 |
| `internal/memory/ingest_test.go` | 13813 | 3453 |
| `internal/memory/helpers_test.go` | 1938 | 484 |
| `cmd/loom/cmd/run_memory_test.go` | 4682 | 1170 |

## Knowledge Items

None

## ADRs

None

## Pruned

- `docs/adrs/ADR-004-bugfix-artifacts-use-features-namespace.md — about where bugfix artifacts live (docs/features/ vs a separate namespace), not about path rewriting; this run uses plan: deliver-feature, not deliver-bugfix, so it doesn't even apply here.`
- `docs/adrs/ADR-002-corpus-aware-retrieval-strategy.md, ADR-008, ADR-001, ADR-010 — found by a broad grep for 'archive/memory' but all concern KI retrieval, test evidence, or repo indexing, not run-record path handling.`
- `cmd/loom/cmd/state.go — also calls prepareRunWorkspace, but it's the `loom state` inspection command and is outside scope.paths; not part of the archiving path.`
- `docs/features/i-4-vocabulary-test/** and docs/features/context-engineering-framework/** — the only two prior deliveries with an Owning Context line (intake; Agent Orchestration). Neither matches this feature's context (Documentation Knowledge Base, per DOMAIN_DICTIONARY.md's Episodic Store/Feature Archive entries) — none found for this bounded context.`
- `internal/state/*.go (typed stage-document schemas) — the AC calls out state/*.json in its blanket no-absolute-root assertion, but every typed document's own schema already declares repo-relative paths (e.g. context-engineer's pinnedFiles), so there is no known field there holding an absolute path to pin against.`
