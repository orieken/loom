# Context Manifest: state-files-deleted

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/state/claims.go` — Defines PathClaims/PathClaim — the interface and struct this feature extends. MustExist already has a comment flagging it as the seam for a deleted-file field ("the flag exists so a future field can opt out explicitly"). ImplementationState.PathClaims() and QAState.PathClaims() here are what filesDeleted/testFilesDeleted must be added to.
- `internal/orchestrator/claims.go` — verifyOneClaim only ever checks MustExist==true (missingPaths). The new deletion check — path absent now, existed at stage start — is new logic that belongs beside it; this is the file whose shape the acceptance criteria (ClaimError wording, field-naming) must match.
- `internal/state/implementation_state.go` — Owns the ImplementationState struct, its PathClaims() entries, and requireTouchedFiles — AC asks that a deletion-only implementation still counts as having touched something, which is this function.
- `internal/state/qa_state.go` — Owns QAState, which needs the parallel testFilesDeleted field per the acceptance criteria.
- `internal/orchestrator/posture.go` — Defines the WorkTree interface (Digest, ChangedPaths, HeadRef, DiffLinesSince) and changedPaths(). The spec requires checking a deleted path 'existed when the stage started... tracked at the run's base commit, or in the tree the posture check saw' — this is the only existing mechanism that observes the tree's prior state, and the analyst must decide whether it's sufficient or needs a new WorkTree method.
- `internal/orchestrator/typed.go` — Holds resolveWithinRoot, fileExists, and projectRootFor — the primitives missingPaths (claims.go) is built from, and whatever existence check filesDeleted needs will reuse them.
- `internal/state/schema.go` — Schema generation is struct-driven, never hand-edited (StageSchemas, Generate, Decode, documentFactories). Adding filesDeleted/testFilesDeleted means adding fields to the Go structs and running the generator — the analyst needs to know schemas are derived, not authored, so the task breakdown includes 'regenerate shared/schemas/pipeline/*.json' rather than editing the JSON by hand.
- `internal/orchestrator/claims_test.go` — Shows the existing test idiom for claim verification (runQAWithPayload, writeAt, a fabricated-then-true-then-escaping-path progression) that the Test Approach section asks the new tests to follow: a real deletion, a deletion that didn't happen, a never-existed path, a rename reported both ways, and an old -> new entry.
- `shared/schemas/pipeline/implementation.schema.json` — Generated artifact the AC directly names ("each has an optional filesDeleted or testFilesDeleted array"). Pinned to show the current generated shape developer/analyst must diff against after adding the Go field — not to be hand-edited.
- `shared/schemas/pipeline/qa.schema.json` — Same as above, for testFilesDeleted on the QA schema.

## Token Budget

- **Status**: OK — ~12722 tokens, 10.6% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/state/claims.go` | 2319 | 579 |
| `internal/orchestrator/claims.go` | 4064 | 1016 |
| `internal/state/implementation_state.go` | 4781 | 1195 |
| `internal/state/qa_state.go` | 6010 | 1502 |
| `internal/orchestrator/posture.go` | 4550 | 1137 |
| `internal/orchestrator/typed.go` | 9459 | 2364 |
| `internal/state/schema.go` | 7040 | 1760 |
| `internal/orchestrator/claims_test.go` | 3674 | 918 |
| `shared/schemas/pipeline/implementation.schema.json` | 4558 | 1139 |
| `shared/schemas/pipeline/qa.schema.json` | 4449 | 1112 |

## Knowledge Items

None

## ADRs

None

## Pruned

- `internal/state/conditional.go — declares conditional-requirement rules (e.g. structuralDecision.fitness) for a different kind (architecture); not the mechanism this feature needs, and pinning it would suggest filesDeleted's rename-syntax rule belongs in that table when it more plausibly belongs in Validate() on each affected state struct`
- `internal/state/state.go — holds generic ValidationError/firstError/requireText helpers; useful but inferable from implementation_state.go and qa_state.go's existing calls to them, and the 10-file cap is tighter than including it`
- `cmd/gen-schemas — the schema generator's entrypoint; schema.go already shows the generation API the analyst needs, and the entrypoint itself is mechanical (go run cmd/gen-schemas)`
- `shared/contracts/implementation-contract.md and shared/contracts/qa-contract.md — prose contracts for the markdown rendering of these states; the feature is about the typed JSON claim-checking path, not the markdown contract, and spec explicitly scopes to internal/state, internal/orchestrator, shared/schemas/pipeline`
- `docs/features/pre-ship-checks/* and docs/features/intake-slice-check/* — same Agent Orchestration bounded context as prior deliveries, checked for retrospectives; neither has a retrospective.md (only docs/features/2026-07-aos-foundations-and-frontmatter/ has one, and it belongs to a different bounded context), so nothing to pull forward`
- `internal/orchestrator/executor.go — orchestrates stage execution generally but is not where claim verification or the field additions live; verifyPathClaims is called from here but the call site itself needs no change for this feature`
