---
feature: "pre-ship-checks"
bounded_context: ""
domain_terms: []
files_touched: ["internal/orchestrator/executor.go", "internal/policy/config.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Implementation Notes: pre-ship-checks

## Files Created

None

## Files Modified

- internal/orchestrator/executor.go
- internal/policy/config.go

## Interface Design

- Executor.persistCompletedStage(state *RunState, stage Stage, plan Plan, input StageInput, output StageOutput) error — new private method, same signature shape as persistCompletion, holding everything after the skip-dispatch
- policy.decodeAndValidate(raw []byte, path string) (Config, error) — new private helper holding the checkKeys -> decodeConfig -> checkValues -> decodePreShipChecksKey sequence

## Named Refactoring Log

- **Extract Function** on `internal/orchestrator/executor.go: Executor.persistCompletion -> Executor.persistCompletedStage` — code-reviewer finding: the skip-dispatch branch added this feature pushed persistCompletion to cyclomatic complexity 8, over budget and worse than the pre-existing complexity-7 debt. Moved everything from artifactFor through the final return nil into persistCompletedStage, leaving persistCompletion as a 2-line dispatcher (measured complexity 2). persistCompletedStage now measures at complexity 7 — the same pre-existing debt level as before this feature touched the file, not worsened.
- **Extract Function** on `internal/policy/config.go: LoadConfig -> decodeAndValidate` — code-reviewer finding: adding decodePreShipChecksKey and its error check pushed LoadConfig from complexity 5 to 7, a new violation this diff introduced. Extracted the checkKeys -> decodeConfig -> checkValues -> decodePreShipChecksKey sequence into decodeAndValidate(raw, path), leaving LoadConfig as just the os.ReadFile + os.IsNotExist handling plus one call. Measured: LoadConfig now complexity 3, decodeAndValidate complexity 5 — both under the <7 budget.

## Self-Review Checklist

- [x] Every public method has an intention-revealing name
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new/touched functions — Verified with gocyclo: persistCompletion=2, persistCompletedStage=7 (pre-existing debt, unchanged by this fix per reviewer's own instruction), LoadConfig=3, decodeAndValidate=5
- [x] No primitive obsession — No new primitives introduced; both changes are pure structural extraction
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified — No import changes; purely intra-file method/function extraction

## Simple Design Verification

- [x] Passes all tests — go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/... all pass
- [x] Reveals intention — persistCompletedStage and decodeAndValidate names state exactly what each does, matching the reviewer's prescribed names
- [x] No duplication — No duplication introduced or removed; this was a pure split
- [x] Fewest elements — Exactly two new functions, matching the two blocking findings — no extra abstraction added

## Key Decisions

- Applied the code-reviewer's prescribed extraction verbatim (method names, split points, and boundaries) rather than devising an alternative decomposition: The review findings specified exact target names (persistCompletedStage, decodeAndValidate) and exact split points, and verifying against gocyclo confirmed the prescribed split hits the complexity targets stated in the findings (dispatcher at ~2, extracted function at ~5-7). No reason to deviate from a review that was already precise and correct.

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- No behavior changed — this was a pure Extract Function refactor addressing two blocking code-review findings (complexity violations in persistCompletion and LoadConfig). Existing test suites for internal/orchestrator and internal/policy already cover both call paths and all pass unchanged.
- persistCompletedStage still carries complexity 7, which is pre-existing debt the reviewer explicitly said to leave alone (not introduced by this feature) — flagging for visibility, not asking for QA action.

## Notes for DevOps

None
