---
feature: "state-files-deleted"
bounded_context: "Agent Orchestration — Pipeline State Verification (typed stage claims and posture checking, roadmap L2.24/L3.30)"
domain_terms: []
files_touched: ["internal/state/claims.go", "internal/state/implementation_state.go", "internal/state/qa_state.go", "internal/state/security_state.go", "internal/state/render_implementation.go", "internal/state/render_qa.go", "internal/orchestrator/claims.go", "shared/schemas/pipeline/implementation.schema.json", "shared/schemas/pipeline/qa.schema.json", "internal/state/implementation_chain_test.go", "internal/state/qa_state_test.go", "internal/orchestrator/claims_test.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Feature Analysis: state-files-deleted

## Summary

loom's claim checker verifies that every path a stage names in filesCreated/filesModified/testFilesCreated/testFilesModified actually exists, but has no field for a path a stage removed or renamed away — exactly what happened when saturday-monorepo's golden-site-file-names order renamed seven files and listed them as "old -> new" in filesModified, so the claim check found no such file and parked a correct order as undone work. This feature adds an optional filesDeleted array to ImplementationState and testFilesDeleted to QAState, and adds the opposite-direction claim check to internal/orchestrator/claims.go: a deletion claim passes when the path is absent now and was present when the stage started (observed via the existing posture-check WorkTree's ChangedPaths, which already reports a deleted-tracked-file's old path against the run's base commit — no new WorkTree method is needed). It also rejects "old -> new" written into any single path-naming field across ImplementationState, QAState, and SecurityState, with a message describing the correct two-field encoding.

### Acceptance Criteria

- implementation.schema.json and qa.schema.json each gain an optional filesDeleted or testFilesDeleted array of paths, generated from the Go structs (never hand-edited), with a description stating that a rename is the new path in filesCreated and the old path in filesDeleted.
- A stage that lists a path in filesDeleted passes the claim when the path is absent now and existed when the stage started, observed as the path appearing in the posture check's changed-paths set (which reports status against the run's base commit).
- A path in filesDeleted that still exists on disk fails the stage with a ClaimError naming the field and the path: "the file still exists — the stage reported a deletion it did not do".
- A path in filesDeleted that never existed (absent now and absent from the posture check's changed-paths set) fails the stage with a ClaimError: "no such file was there to delete".
- An entry containing " -> " in any path-naming field (filesCreated, filesModified, filesDeleted, testFilesCreated, testFilesModified, testFilesDeleted, or security's filesModified) fails validation with a message instructing the caller to list the new path in filesCreated and the old path in filesDeleted.
  - Example: filesModified: ["old-name.go -> new-name.go"] is rejected before claim-checking even runs, at ImplementationState.Validate().
  - Example: The correct encoding — filesCreated: ["new-name.go"], filesDeleted: ["old-name.go"] — passes both validation and claim-checking.
- requireTouchedFiles counts an implementation that only deletes files as having touched something; it no longer requires filesCreated or filesModified to be non-empty when filesDeleted names at least one path.
- An existing state document with no filesDeleted or testFilesDeleted field (the omitempty default) validates exactly as it did before this change — no new required field, no changed behavior for filesCreated/filesModified/testFilesCreated/testFilesModified claims.
- Every new or changed function has cyclomatic complexity below 7 (enforced by golangci-lint's gocyclo, already configured at min-complexity 7 in .golangci.yml), every changed line is covered by a test proven able to fail, go test ./... and the lint pass, and no file outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**) changes.

### Non-Functional Requirements

None

## Proposed Fitness Functions

- **Generated JSON Schema for ImplementationState and QAState matches the Go structs (no hand-edited drift)**
  - **Verification**: go test ./internal/state/... — TestCommittedSchemasMatchTheStructs in internal/state/state_test.go already fails the build if shared/schemas/pipeline/*.json drifts from the structs; running go run ./cmd/gen-schemas after the struct change keeps it green
  - **Owner**: architect
- **Cyclomatic complexity < 7 on every new or changed function**
  - **Verification**: golangci-lint run (gocyclo, min-complexity: 7 — already enabled in .golangci.yml)
  - **Owner**: architect
- **A filesDeleted/testFilesDeleted claim is accepted only when the path is absent now and the posture check saw it as changed against the run's base commit; rejected otherwise with the exact wording the acceptance criteria specify**
  - **Verification**: internal/orchestrator/claims_test.go — table of cases: real deletion (passes), claimed-but-not-deleted (fails, "still exists"), never-existed path (fails, "no such file was there to delete"), rename reported correctly across filesCreated/filesDeleted (passes), "old -> new" single entry (fails validation)
  - **Owner**: developer

## Out of Scope

- Git rename detection — the stage states what it did (new path in filesCreated, old path in filesDeleted); loom only checks that claim against the filesystem and the posture check's changed-paths set, and never infers a rename from git history on its own.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Agent Orchestration — Pipeline State Verification (typed stage claims and posture checking, roadmap L2.24/L3.30)
- **Context Crossings**: internal/state (typed pipeline state structs and schemas) -> internal/orchestrator (claim verification against the filesystem and the WorkTree posture check)

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `internal/state/claims.go` — Add MustNotExist to PathClaim; add filesDeleted/testFilesDeleted entries to ImplementationState.PathClaims() and QAState.PathClaims(); add a requireNoRenameArrows(claims []PathClaim) error helper that rejects " -> " in any claimed path, reused by every Validate() below.
- `internal/state/implementation_state.go` — Add FilesDeleted []string `json:"filesDeleted,omitempty"` to ImplementationState; update requireTouchedFiles to count FilesDeleted too; call requireNoRenameArrows(i.PathClaims()) from Validate().
- `internal/state/qa_state.go` — Add TestFilesDeleted []string `json:"testFilesDeleted,omitempty"` to QAState; call requireNoRenameArrows(q.PathClaims()) from Validate().
- `internal/state/security_state.go` — Call requireNoRenameArrows(s.PathClaims()) from Validate() so the AC's "any files field" also covers security's filesModified, for one shared rule rather than two of three state types enforcing it.
- `internal/state/render_implementation.go` — Render a "## Files Deleted" section from FilesDeleted and include it in the touched-files list ImplementationState.frontmatter() builds, so a deletion-only implementation is visible in implementation-notes.md and not just in the typed JSON.
- `internal/state/render_qa.go` — Render a "## Test Files Deleted" section from TestFilesDeleted and include it in QAState.frontmatter()'s touched-files list, for the same reason as render_implementation.go.
- `internal/orchestrator/claims.go` — Split verifyOneClaim on claim.MustExist / claim.MustNotExist; keep today's existence check (renamed verifyExistenceClaim, unchanged behavior/wording) and add verifyDeletionClaim + existingPaths + unseenDeletions implementing the absent-now-and-previously-changed check against e.changedPaths(), with the exact ClaimError wording the acceptance criteria require.
- `shared/schemas/pipeline/implementation.schema.json` — Regenerated artifact (go run ./cmd/gen-schemas) — picks up the new optional filesDeleted property and its rename-encoding description. Never hand-edited.
- `shared/schemas/pipeline/qa.schema.json` — Regenerated artifact (go run ./cmd/gen-schemas) — picks up the new optional testFilesDeleted property. Never hand-edited.
- `internal/state/implementation_chain_test.go` — Extend validImplementation()-based tests: requireTouchedFiles accepts a deletion-only implementation; " -> " in filesCreated/filesModified/filesDeleted is rejected; TestImplementationMustNameAFile's existing substring check on "filesCreated" still passes once the ValidationError.Field text grows to "filesCreated/filesModified/filesDeleted".
- `internal/state/qa_state_test.go` — Add the equivalent rename-arrow rejection tests for QAState's path-naming fields, including testFilesDeleted.
- `internal/orchestrator/claims_test.go` — Add the five required cases from the spec's Test Approach, following the existing runQAWithPayload/writeAt idiom and the changingTree-style fake WorkTree (posture_test.go) needed to populate a non-nil ChangedPaths() result: a real deletion (passes), a deletion that did not happen (fails, "still exists"), a path that never existed (fails, "no such file was there to delete"), a rename reported correctly both ways (passes), and an "old -> new" single entry (fails validation, never reaches claim-checking).

### Data Model Changes

- Add optional filesDeleted array to ImplementationState and optional testFilesDeleted array to QAState. Both are new, omitempty, additive JSON Schema properties — no existing field is removed, renamed, or made required, so every previously-valid document keeps validating unchanged. (**expand** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. Add MustNotExist bool to state.PathClaim (internal/state/claims.go) with a comment explaining it marks a claim that a path is gone now but existed when the stage started.
2. Add FilesDeleted []string `json:"filesDeleted,omitempty"` to ImplementationState (internal/state/implementation_state.go) and a {Field: "filesDeleted", Paths: i.FilesDeleted, MustNotExist: true} entry to ImplementationState.PathClaims() (internal/state/claims.go).
3. Add TestFilesDeleted []string `json:"testFilesDeleted,omitempty"` to QAState (internal/state/qa_state.go) and a {Field: "testFilesDeleted", Paths: q.TestFilesDeleted, MustNotExist: true} entry to QAState.PathClaims() (internal/state/claims.go).
4. Update requireTouchedFiles (internal/state/implementation_state.go) to count len(FilesCreated)+len(FilesModified)+len(FilesDeleted), and rename the returned ValidationError.Field to "filesCreated/filesModified/filesDeleted" (the existing TestImplementationMustNameAFile test only asserts the field contains "filesCreated", so this stays compatible).
5. Add requireNoRenameArrows(claims []state.PathClaim) error to internal/state/claims.go: for every claim and every path containing " -> ", return a ValidationError naming claim.Field with a message instructing the new path go in filesCreated and the old path in filesDeleted. Call it from ImplementationState.Validate(), QAState.Validate(), and SecurityState.Validate() via each type's own PathClaims().
6. In internal/orchestrator/claims.go, rename the existing MustExist branch of verifyOneClaim to verifyExistenceClaim (unchanged behavior/wording) and dispatch on claim.MustExist vs claim.MustNotExist; add verifyDeletionClaim(stage, claim, root, changed) that fails with "the file still exists — the stage reported a deletion it did not do" if any path still exists, else — only when changed is non-nil (posture checking configured) — fails with "no such file was there to delete" for any claimed path absent from changed; add existingPaths and unseenDeletions as the small helper functions each check needs, keeping every function under cyclomatic complexity 7.
7. Add a "## Files Deleted" section (bullets(implementation.FilesDeleted)) to renderImplementation and include FilesDeleted in the touched slice ImplementationState.frontmatter() builds (internal/state/render_implementation.go).
8. Add a "## Test Files Deleted" section to renderQA and include TestFilesDeleted in the touched slice QAState.frontmatter() builds (internal/state/render_qa.go).
9. Run `go run ./cmd/gen-schemas` from the repo root to regenerate shared/schemas/pipeline/implementation.schema.json and qa.schema.json; commit only the files under shared/schemas/pipeline/.
10. Write unit tests beside the changed code (ADR-009): internal/state/implementation_chain_test.go (deletion-only implementation passes requireTouchedFiles; " -> " rejected on filesCreated/filesModified/filesDeleted), internal/state/qa_state_test.go (" -> " rejected on QAState's fields, including testFilesDeleted), and internal/orchestrator/claims_test.go (the five Test Approach cases: real deletion passes; false deletion claim fails with "still exists"; never-existed path fails with "no such file was there to delete"; a correctly-encoded rename across filesCreated/filesDeleted passes; an "old -> new" single entry fails validation before claim-checking runs) — reuse the changingTree-style fake WorkTree from posture_test.go to populate ChangedPaths() for the deletion-claim tests.

### QA Tasks

1. Run go test ./... and golangci-lint run (gocyclo min-complexity 7) across the touched packages and confirm both pass.
2. Confirm every changed line in internal/state/claims.go, implementation_state.go, qa_state.go, security_state.go, render_implementation.go, render_qa.go, and internal/orchestrator/claims.go is covered by a test that demonstrably fails when that line is reverted (ADR-008 / test-repair-contract.md's exemplar-test bar).
3. Confirm no file outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**) changed.

### Tech Writer Tasks

1. If internal/state/README.md enumerates the typed state documents' fields, add filesDeleted and testFilesDeleted to that description; leave shared/contracts/implementation-contract.md and qa-contract.md alone — they describe the markdown rendering contract, which context-engineer already scoped this feature away from.

### DevOps Tasks

None

## Edge Cases and Risks

- A stage lists the same path in both filesCreated and filesDeleted.: Each claim is checked independently — filesCreated requires the path to exist, filesDeleted requires it to be absent — so naming the same path in both always fails one of the two checks, which is the correct outcome for an incoherent claim.
- Posture checking is disabled for the run (no WorkTree configured, so e.changedPaths() returns nil).: verifyDeletionClaim cannot observe "existed at start" at all in that case, so it only checks that the path is now absent and does not fail the never-existed case. cmd/loom/cmd/run.go always wires a real worktree.Git for production runs, so this gap is limited to reduced test harnesses that deliberately omit WithWorkTree, not real runs.
- A rename's old path was created and deleted entirely within the same stage, never committed before this run.: git status reports nothing for a path that was never tracked, so it will not appear in ChangedPaths() and the claim fails with "no such file was there to delete" even though the stage's own history disagrees — acceptable per the spec's own "Out of Scope: Git rename detection" line; the stage states what it did and loom checks it against the filesystem and git status, nothing deeper.
- A path in filesDeleted escapes the project root (e.g. "../../etc/passwd").: resolveWithinRoot already rejects this; existingPaths treats a resolve error the same as "does not exist on disk", so the claim proceeds to the changed-set check, where an escaping path will essentially always fail as unseen — consistent with the existing TestAClaimEscapingTheProjectIsRejected test for the existence direction.

## Definition of Done

- shared/schemas/pipeline/implementation.schema.json and qa.schema.json each carry an optional filesDeleted/testFilesDeleted array, generated from the updated Go structs via go run ./cmd/gen-schemas, with TestCommittedSchemasMatchTheStructs green.
- internal/orchestrator/claims.go verifies a filesDeleted/testFilesDeleted claim both ways, with the exact ClaimError wording the acceptance criteria specify for "still exists" and "no such file was there to delete".
- Any path containing " -> " in any path-naming field on ImplementationState, QAState, or SecurityState fails validation with a message describing the correct two-field rename encoding.
- requireTouchedFiles accepts a deletion-only implementation.
- Every existing state document with no deleted-files field validates exactly as it did before this change.
- Every new or changed function has cyclomatic complexity below 7 (golangci-lint, gocyclo min-complexity 7).
- Every changed line is covered by a test proven able to fail.
- go test ./... and golangci-lint run both pass.
- No file outside scope.paths (internal/state/**, internal/orchestrator/**, shared/schemas/pipeline/**, docs/features/state-files-deleted/**) changed, excepting this run's own delivery record under docs/features/state-files-deleted/.
