---
feature: "state-files-deleted"
bounded_context: "Agent Orchestration — Pipeline State Verification (typed stage claims and posture checking, roadmap L2.24/L3.30)"
domain_terms: ["PathClaim", "WorkTree", "ClaimError", "posture check"]
files_touched: ["internal/state/claims.go", "internal/state/implementation_state.go, internal/state/qa_state.go", "internal/state/claims.go", "internal/orchestrator/claims.go", "shared/schemas/pipeline/"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Architecture Notes: state-files-deleted

## Structural Decisions

### Add a second boolean, MustNotExist, to state.PathClaim (internal/state/claims.go) rather than replacing MustExist with a tri-state enum (Exists/Deleted/Unchecked).
- **Rationale**: The existing MustExist doc comment already anticipates this exact extension ("the flag exists so a future field can opt out explicitly"). Every PathClaims() implementation sets exactly one claim kind per field literal — there is no call site today, nor any planned one, that needs 'neither' or 'both' as a real state. A two-boolean struct is the smaller diff: orchestrator's verifyOneClaim dispatches on two conditions it already partially has, instead of introducing a new named type that every existing claim literal (filesCreated, filesModified, testFilesCreated, testFilesModified, security's filesModified) would need to be rewritten to use.
- **Trade-offs**:
  - The shape is nominally capable of representing an invalid state (MustExist: true, MustNotExist: true) that an enum could not express. This is accepted because nothing constructs a claim that way, and the fitness function below gives it a test-level guard instead of a type-level one.
  - A third claim kind in the future (e.g. 'must exist AND have changed, full stop, no posture leniency') would need a third boolean rather than a third enum value — acceptable now, revisit if a third kind actually arrives.
- **Fitness Function**: Every PathClaim literal returned by ImplementationState.PathClaims(), QAState.PathClaims(), and SecurityState.PathClaims() sets exactly one of MustExist or MustNotExist to true, never both and never neither. — verified by internal/orchestrator/claims_test.go table test covering each PathClaims() implementation's output; go vet / code review as a secondary check since this is a value-level invariant, not a type-level one
### Split verifyOneClaim (internal/orchestrator/claims.go) into a small dispatcher plus two bodies: verifyExistenceClaim (the current MustExist body, renamed, unchanged behavior and wording) and a new verifyDeletionClaim for MustNotExist.
- **Rationale**: verifyOneClaim's body already checks !claim.MustExist and returns nil; adding the deletion branch inline would push a single function across two independent failure modes (existence-required vs absence-required) and risk exceeding the complexity-7 budget design-principles.md and CLAUDE.md both set. Naming the split keeps each function doing one thing, which is also what the Fowler-operations habit this architecture process uses calls for: this is Extract Function, applied because the existing function's reason to change is about to become two reasons.
- **Trade-offs**:
  - One more function and one more name to keep in sync with the dispatcher, for a net gain in testability — each body gets its own table-test rows instead of one function's test covering two unrelated behaviors.
- **Fitness Function**: Cyclomatic complexity < 7 on verifyOneClaim's dispatcher, verifyExistenceClaim, and verifyDeletionClaim individually — verified by golangci-lint run (gocyclo, min-complexity: 7 — already enabled in .golangci.yml)
### verifyDeletionClaim's two failure modes have different data requirements: the "still exists" check is a pure filesystem stat and always runs; the "no such file was there to delete" check requires git history and MUST run only when changed != nil (i.e. the WorkTree answered successfully this run) — not merely when len(changed) == 0.
- **Rationale**: changedPaths() (internal/orchestrator/posture.go:110) returns nil for both 'posture checking disabled' (no WorkTree) and 'ChangedPaths() errored', and returns a non-nil map — possibly empty — when git ran and genuinely found nothing uncommitted. Those are different facts: a nil map means this executor has no opinion about history; a non-nil empty map means git looked and the claimed path was not among the changes, which is real evidence a fabricated deletion claim should be caught by. Checking len(changed)==0 (the pattern warnUntouched already uses, two functions up in the same file) would silently skip the 'no such file was there to delete' failure in the empty-but-observed case, which is exactly the fabrication case AC #4 exists to catch. Pin the check to the nil/non-nil distinction the executor's own doc comment on changedPaths() already promises, not emptiness.
- **Trade-offs**:
  - This only catches a never-existed path when posture checking is configured (a WorkTree is wired in). With no WorkTree, a stage can claim a deletion for a path that was never there and the claim passes — the same limitation every other posture-dependent check in this file already has, and consistent with 'nothing is auto-approved and this check is advisory where the tree can't be observed' rather than a new gap this feature introduces.
  - Depends on the pipeline's existing assumption (shared/rules/approval-gates.md gate #2) that a stage does not commit its own work mid-run — git-commit is a human-gated action, so HEAD stays fixed for the duration of a stage and git status --porcelain output is equivalent to 'changed since this stage's base commit'. If that assumption is ever relaxed, this check and the rest of posture checking break together, not just this feature.
- **Fitness Function**: A filesDeleted/testFilesDeleted claim for a path absent from disk and absent from a non-nil changed set fails with the exact 'no such file was there to delete' wording; the same claim with a nil changed set (no WorkTree) does not fail on that specific check — verified by internal/orchestrator/claims_test.go: table rows for (a) real deletion with WorkTree configured — passes, (b) claimed-but-still-exists — fails 'still exists' regardless of WorkTree, (c) never-existed path with WorkTree configured and empty ChangedPaths() — fails 'no such file was there to delete', (d) never-existed path with no WorkTree (nil changed) — does not fail on this check
### No new WorkTree method. internal/worktree/git.go's existing ChangedPaths() (git status --porcelain --untracked-files=all) already reports a deleted tracked file by its own path, and a git-detected rename as 'old -> new' split into both halves by the existing splitRename helper — confirmed by reading the implementation, not assumed.
- **Rationale**: Extending the WorkTree interface would require changing internal/worktree/git.go and every test fake implementing it (internal/orchestrator/post_review_test.go's editingTree, internal/orchestrator/posture_test.go's changingTree, internal/factory/faults_test.go's faultyTree) — all outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**). The data this feature needs is already surfaced by the method that exists.
- **Trade-offs**:
  - verifyDeletionClaim reads the same map shape (map[string]bool keyed by slash-separated repo-relative path) the existence-claim path already consumes, rather than a purpose-built 'what was deleted' signal — acceptable since porcelain output already resolves to exactly the set of changed paths this check needs, deletions included.
- **Fitness Function**: judgment-only — Extending the WorkTree interface would require changing internal/worktree/git.go and every test fake implementing it (internal/orchestrator/post_review_test.go's editingTree, internal/orchestrator/posture_test.go's changingTree, internal/factory/faults_test.go's faultyTree) — all outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**). The data this feature needs is already surfaced by the method that exists.
### requireNoRenameArrows(claims []state.PathClaim) error lives once in internal/state/claims.go and is called from ImplementationState.Validate(), QAState.Validate(), and SecurityState.Validate() via each type's own PathClaims() — not duplicated three times.
- **Rationale**: design-principles.md's Simple Design priority #3 (no duplication) and the Boy Scout Rule both apply directly: PathClaims() already exists as the one place each type declares its path-naming fields together, so the rename-arrow check reads that same list instead of re-enumerating fields per type.
- **Trade-offs**:
  - Couples the three Validate() methods to one shared helper's exact error wording — acceptable since the acceptance criteria require one consistent message across all three types.
- **Fitness Function**: An entry containing ' -> ' in any path-naming field across ImplementationState, QAState, and SecurityState fails Validate() before claim-checking runs, with a message naming the field and instructing the new path go in filesCreated/testFilesCreated and the old path in filesDeleted/testFilesDeleted — verified by internal/state/implementation_chain_test.go (filesCreated/filesModified/filesDeleted and SecurityState's filesModified — SecurityState is currently tested inside this file, there is no dedicated security_state_test.go), internal/state/qa_state_test.go (testFilesCreated/testFilesModified/testFilesDeleted)
### requireTouchedFiles (internal/state/implementation_state.go) counts FilesDeleted alongside FilesCreated and FilesModified, and its ValidationError.Field becomes the literal string "filesCreated/filesModified/filesDeleted".
- **Rationale**: Verified against the existing test: TestImplementationMustNameAFile only asserts the field contains the substring "filesCreated", so widening the literal stays backward compatible with that assertion while correctly describing the new third option.
- **Fitness Function**: An implementation naming only FilesDeleted (no FilesCreated, no FilesModified) passes requireTouchedFiles; one naming none of the three still fails with a field string containing 'filesCreated' — verified by internal/state/implementation_chain_test.go — add a deletion-only case alongside the existing TestImplementationMustNameAFile
### Render additions ("## Files Deleted" in render_implementation.go, "## Test Files Deleted" in render_qa.go) follow the exact bullets()/section() pattern every other list field in those two files already uses, and FilesDeleted/TestFilesDeleted are appended into the touched-files slice each frontmatter() builds.
- **Rationale**: No new rendering primitive is justified — the document/section/bullets helpers already generalize over any []string field. This is a mechanical extension of an existing seam, not a new pattern.
- **Fitness Function**: judgment-only — No new rendering primitive is justified — the document/section/bullets helpers already generalize over any []string field. This is a mechanical extension of an existing seam, not a new pattern.

## Component Placement

- `PathClaim.MustNotExist` → Entities layer, `internal/state/claims.go` — Pure data describing a claim's shape; internal/state imports nothing from internal/orchestrator (confirmed by grep — no mcp__loom graph tools were available in this session, so import inspection was used directly, stated per the architect process's fallback instruction) and must stay that way.
- `ImplementationState.FilesDeleted / QAState.TestFilesDeleted` → Entities layer, `internal/state/implementation_state.go, internal/state/qa_state.go` — Same typed-document layer as every other field on these structs; no persistence or filesystem logic belongs here, only shape and the Validate() rules the contract already states.
- `requireNoRenameArrows` → Entities layer, `internal/state/claims.go` — A pure validation rule over claimed strings, co-located with PathClaims() which it reads from — no I/O, consistent with the rest of claims.go.
- `verifyDeletionClaim, verifyExistenceClaim, existingPaths, unseenDeletions` → Use Cases / Adapters layer, `internal/orchestrator/claims.go` — This is where the typed claim meets the real filesystem and the WorkTree abstraction — the orchestrator package already owns all impure claim-checking, and WorkTree (the interface these functions consume) is defined here, in the consumer package, per CLAUDE.md's Interfaces rule and architecture-guardrails.md #1.
- `shared/schemas/pipeline/implementation.schema.json, qa.schema.json` → Frameworks layer, `shared/schemas/pipeline/` — Generated artifact, never hand-edited — go run ./cmd/gen-schemas regenerates it from the struct tags, and TestCommittedSchemasMatchTheStructs (internal/state/state_test.go) already fails the build on drift.

## Bounded Context

- **Owning Context**: Agent Orchestration — Pipeline State Verification (typed stage claims and posture checking, roadmap L2.24/L3.30)
- **Context Crossings**: internal/state (typed pipeline state structs, PathClaim/PathClaims, schema generation) -> internal/orchestrator (claim verification against the filesystem and the WorkTree posture check)

## Stability Design

None

## Observability Design

None

## Layer Boundary Checks

- [PASS] internal/state (Entities) must not import internal/orchestrator (Use Cases/Adapters) — mcp__loom__dependencies_of was not available in this session (no loom MCP server registered), so this was checked by grepping import statements directly, as the architect process's fallback instructs. No file under internal/state imports internal/orchestrator; only internal/orchestrator files import internal/state. This feature's changes preserve that direction — PathClaim.MustNotExist and the new struct fields stay pure data; all filesystem/git-touching logic (verifyDeletionClaim, existingPaths, unseenDeletions) goes into internal/orchestrator/claims.go, not internal/state.
- [PASS] WorkTree interface stays defined in its consumer package (internal/orchestrator), not in internal/state or internal/worktree — Confirmed by reading internal/orchestrator/posture.go:24 — WorkTree is already defined there; internal/worktree/git.go provides an implementation and is never imported by internal/state. This feature adds no new interface and does not disturb this.

## Anti-Pattern Check

- Shotgun Surgery: not found — The change touches ~10 files, which looks like shotgun surgery on its face, but every touch is a narrow, mechanical propagation of one additive field through an already-established seam (struct field -> PathClaims() entry -> claims.go verifier -> render section -> generated schema -> tests). The seam existed before this feature (filesCreated/filesModified already flow through the same files); this is the seam working as designed, not fragile coupling.
- Leaky Abstraction: not found — PathClaim stays a small struct (Field, Paths, MustExist, MustNotExist); callers of PathClaims() still don't need to know how verification works, only what they're claiming.
- God Object: not found — claims.go (both the state and orchestrator files) stay focused on one responsibility each — shape-of-claim and truth-of-claim respectively.
- Premature Generalization: not found — Rejected the tri-state enum in favor of the smaller two-boolean extension specifically to avoid generalizing for a third claim kind nothing currently needs — see the PathClaim structural decision above.
- Distributed Monolith: not found — Not applicable — single-process, single-module change with no new network or service boundary.

## Fitness Functions

- **Generated JSON Schema for ImplementationState and QAState matches the Go structs — no hand-edited drift**
  - **Verification**: go test ./internal/state/... (TestCommittedSchemasMatchTheStructs already fails the build on drift); go run ./cmd/gen-schemas regenerates the committed files after the struct change
  - **Owner**: architect
- **Cyclomatic complexity < 7 on every new or changed function in internal/state and internal/orchestrator**
  - **Verification**: golangci-lint run (gocyclo, min-complexity: 7 — already enabled in .golangci.yml)
  - **Owner**: architect
- **Every PathClaim sets exactly one of MustExist/MustNotExist**
  - **Verification**: internal/orchestrator/claims_test.go table coverage over each state type's PathClaims() output
  - **Owner**: developer
- **verifyDeletionClaim's two failure modes behave per the nil-vs-non-nil changed-map distinction documented above**
  - **Verification**: internal/orchestrator/claims_test.go: real deletion passes; false deletion claim fails 'still exists'; never-existed path with posture configured fails 'no such file was there to delete'; never-existed path with no WorkTree does not fail on that check; a correctly-encoded rename across filesCreated/filesDeleted passes
  - **Owner**: developer
- **' -> ' in any path-naming field (filesCreated, filesModified, filesDeleted, testFilesCreated, testFilesModified, testFilesDeleted, security's filesModified) fails Validate() before claim-checking runs**
  - **Verification**: internal/state/implementation_chain_test.go and internal/state/qa_state_test.go table cases, one per field
  - **Owner**: developer
- **An implementation naming only FilesDeleted satisfies requireTouchedFiles**
  - **Verification**: internal/state/implementation_chain_test.go deletion-only case
  - **Owner**: developer

## Refactoring Opportunities

- Harmonize Predicate on `internal/orchestrator/claims.go warnUntouched (existing, pre-dates this feature) vs the new verifyDeletionClaim` — warnUntouched treats 'posture checking configured' as len(changed) == 0 (empty means not-observed), while verifyDeletionClaim (this feature) treats it as changed != nil (nil means not-observed, a non-nil empty map is real evidence). Both are individually correct for their own risk profile — warnUntouched is a soft warning where under-firing is safe, verifyDeletionClaim is a hard gate where it isn't — but the inconsistency is worth a dedicated harmonization pass later. Do not fix now: test-repair-contract.md and this codebase's own convention keep refactoring and behavior changes in separate commits, and warnUntouched has no test coverage gap this feature is responsible for closing.

## Developer Handoff Notes

- Dispatch on claim.MustExist vs claim.MustNotExist in a small top-level function; keep the renamed verifyExistenceClaim's behavior and wording byte-for-byte identical to today's verifyOneClaim MustExist branch.
- verifyDeletionClaim's 'still exists' failure is unconditional (pure filesystem stat); its 'no such file was there to delete' failure must check changed != nil, not len(changed) == 0 — see the structural decision above for why the distinction matters here specifically.
- SecurityState currently has no dedicated test file — its tests live in internal/state/implementation_chain_test.go. Add the rename-arrow rejection case for SecurityState.FilesModified there, not in a new file, unless the test file is reorganized as a separate task.
- Do not touch internal/worktree/git.go or any WorkTree test fake (editingTree, changingTree, faultyTree) — ChangedPaths() already surfaces both a deleted tracked file (its own path) and a detected rename (both halves, via splitRename) with no interface change needed.
- requireNoRenameArrows must be called from all three Validate() methods (ImplementationState, QAState, SecurityState) via each type's own PathClaims() — one function, three call sites, not three copies of the substring check.
- No new telemetry or signal is introduced — the existing ClaimError struct carries the new 'still exists' / 'no such file was there to delete' Problem strings exactly as it already carries today's wording; nothing in architecture-guardrails.md #8/#9 is newly implicated.

## Open Architectural Questions

None
