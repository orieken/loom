---
feature: "pre-ship-checks"
bounded_context: ""
domain_terms: ["Pre-ship check"]
files_touched: ["internal/orchestrator/preship_test.go", "cmd/loom/cmd/run_policies_test.go"]
issue_refs: ["PR #18"]
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md", "docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# QA Report: pre-ship-checks

## Test Files Created

None

## Test Files Modified

- internal/orchestrator/preship_test.go
- cmd/loom/cmd/run_policies_test.go

## Coverage Summary

- Acceptance criteria covered: 8/8
- Total new tests: 2
- Statement coverage, lowest unit: **83.5%** (`cmd/loom/cmd`)
  - `internal/orchestrator`: 88.4%
  - `internal/policy`: 92.6%
  - `cmd/loom/cmd`: 83.5%

## Test Results

- Passed: 609
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

- Every changed line is covered as cmd/diff-coverage measures it (>=85% per ADR-008): Overall changed-line coverage is 91.1% (204/224), well above the 85% floor. 20 lines stay uncovered: internal/policy/preship.go:38-39 and 55-56 are defensive branches (malformed-node / non-mapping top-level) that decodeAndValidate's own prior steps (topLevelKeys, decodeConfig) already guarantee can't occur by the time decodePreShipChecksKey runs — unreachable from the only public entry point, LoadConfig. internal/orchestrator/preship.go:75-76, 87-88, 139-140 are os.WriteFile failure-wrapping branches, reachable only via filesystem fault injection (e.g. a read-only workspace dir), which none of this codebase's existing tests do for any other artifact writer either. internal/orchestrator/executor.go:430-431, 447-448, 670-671, 697-698, 763-764 are gate/approval error-propagation lines that only appear as 'changed' because the code-reviewer-mandated Extract Function pass (persistCompletedStage, checkApprovedGate) shifted pre-existing, already-adjudicated-as-unchanged logic into new line ranges — code-review-report.md verified this iteration changed no behavior and added/removed no tests.
- Every new function MUST have cyclomatic complexity below 7: gocyclo reports two functions at exactly 7: cmd/loom/cmd/run.go's runRun (pre-existing at 7 on the base commit, 2e8284c, before this feature touched the file — confirmed by checking out that revision and re-running gocyclo; this feature only renamed a return value and added one field to a struct literal inside it, no new branches) and internal/orchestrator/executor.go's persistCompletedStage (a newly-named function, but it is the code-reviewer-prescribed Extract Function split of persistCompletion's pre-existing branches, not new logic — code-review-report.md's Verification of Developer Self-Review section independently re-measured the same number and explicitly adjudicated it as pre-existing debt the extraction surfaced rather than introduced, scoped to 'do not make worse,' with the actual Extract Class fix already logged in architecture-notes.md's Refactoring Opportunities). No function introduced by this feature's own new logic (decodePreShipChecks, runOnePreShipCheck, runPreShipChecks, all of internal/policy/preship.go, internal/orchestrator/preship.go) exceeds complexity 6.

## Notes for Tech Writer

- scripts/pre-ship-check.sh, the preShipChecks config schema (name/command/timeout, 10-minute default), and the pre-ship-checks stage's position (after tech-writer, before devops-engineer/confirm-ship) are all new project-facing surface worth a mention in docs/MIGRATION.md or a delivery-policy.yaml example if one doesn't already show preShipChecks — docs/MIGRATION.md was touched by this feature already, worth checking it documents the new key.
- The stage writes pre-ship-checks.md to the run's workspace with each check's name/command/duration/status and output (last 50 lines on pass, full output on failure/timeout) — nothing from it reaches telemetry or run state's Error field (verified by test), consistent with architecture-guardrails.md #9.
