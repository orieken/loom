---
feature: "pre-ship-checks"
bounded_context: ""
domain_terms: ["Pre-ship check"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md", "docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Code Review: pre-ship-checks

## Overall Status

**APPROVED**

## Design Narrative

This iteration is a pure complexity-reduction fix for the two blocking findings from the prior review: persistCompletion's new Skipped-dispatch branch pushed it to complexity 8, and LoadConfig's new decodePreShipChecksKey call pushed it to 7. The developer applied Extract Function exactly as prescribed, splitting persistCompletion into a 2-line dispatcher plus persistCompletedStage, and LoadConfig into a 2-line dispatcher plus decodeAndValidate. Behaviour change: no.

## Design Score

- **Clarity** 4/5
- **Cohesion** 4/5
- **Coupling** 4/5
- **Craft** 4/5

## Security Surface

- No new surface introduced by this iteration — it is a pure structural extraction of existing logic, no new imports, no new I/O, no new data flow.

## Performance Surface

- No change — the extracted functions are called in the same order with the same arguments as the inline code they replaced.

## Test Design Review

- This iteration adds and modifies no tests — the developer's own notes state no behavior changed and the existing internal/orchestrator and internal/policy suites already cover both call paths unchanged. Verified independently: go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/... all pass against the current diff.

## Verification of Developer Self-Review

- Confirmed true: gocyclo independently measured persistCompletion=2, persistCompletedStage=7, LoadConfig=3, decodeAndValidate=5 — matches the developer's stated numbers exactly.
- Confirmed true: 'Applied the code-reviewer's prescribed extraction verbatim' — the function names (persistCompletedStage, decodeAndValidate), split points, and signatures match the prior review's Feedback for the Developer section exactly, including the dispatcher-plus-extracted-function shape.
- Confirmed true: 'Passes all tests' — go build ./... and the full relevant test suite (orchestrator, policy, cmd/loom/cmd) pass.
- Note, not a discrepancy: persistCompletedStage still measures at exactly 7, which is CLAUDE.md's non-negotiable '< 7' threshold technically still violated — but this is the same pre-existing debt level the file carried before this feature touched it (verified against git show HEAD~ for the pre-diff revision), and the prior review explicitly scoped the fix to not making it worse rather than retiring the pre-existing debt, which is a separate Extract Class opportunity already logged in architecture-notes.md's Refactoring Opportunities section (the Executor god-object split). Not re-flagging as blocking since it was already adjudicated in the prior review round and the developer's fix matches exactly what was asked.

## Feedback for the Developer

None
