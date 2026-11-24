---
feature: "state-files-deleted"
bounded_context: ""
domain_terms: []
files_touched: ["internal/orchestrator/posture.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Implementation Notes: state-files-deleted

## Files Created

None

## Files Modified

- internal/orchestrator/posture.go

## Interface Design

None

## Named Refactoring Log

- **Reconcile Contradictory Comment** on `internal/orchestrator/posture.go:changedPaths() doc comment` — code-reviewer CHANGES_REQUESTED: the function's own contract comment said an empty set means 'not observed', but verifyDeletionClaim in internal/orchestrator/claims.go (and warnUntouched) now rely on nil vs. non-nil-but-empty carrying different evidentiary weight. Rewrote the comment to state the real invariant at its source (nil = not observed; non-nil, even length zero, = observed and is real evidence) per architecture-guardrails.md #7 (PRECONDITION named where the condition lives).

## Self-Review Checklist

- [x] Every public method has an intention-revealing name
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new/changed functions — Comment-only change; no logic touched
- [x] No primitive obsession
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified

## Simple Design Verification

- [x] Passes all tests — go build ./... and go test ./internal/orchestrator/... ./internal/state/... both green
- [x] Reveals intention — changedPaths()'s doc comment now states the nil-vs-empty invariant that verifyDeletionClaim and warnUntouched both depend on, instead of contradicting one of them
- [x] No duplication
- [x] Fewest elements — No code change, only the doc comment the reviewer flagged

## Key Decisions

- Updated only the doc comment on changedPaths(), made no behavioral change: code-reviewer confirmed the existing runtime behavior (nil vs. non-nil-empty) is already correct for the Git-backed WorkTree implementation and is what verifyDeletionClaim/warnUntouched already rely on; the defect was purely that the canonical comment at the source of truth said the opposite. Fixing the comment in place, rather than adding an abstraction, satisfies architecture-guardrails.md #7 without expanding scope.

## Deviations from Analysis

- Non-blocking finding: requireNoRenameArrows error message references filesCreated/filesDeleted, which SecurityState (internal/state/security_state.go) does not have: code-reviewer marked this explicitly non-blocking — the shared wording is spec-mandated (analysis.md AC #5 requires identical wording across all three claim types) — and recommended only a follow-up note next time this area is touched. Left unchanged to avoid deviating from the analysis's explicit requirement without a separate approved spec change.

## Dependencies Added

None

## Notes for QA

- Only internal/orchestrator/posture.go changed in this pass (one doc-comment edit) to resolve the code-reviewer's blocking finding; no behavior changed, so no new test scenarios are needed beyond what already covers claims.go/verifyDeletionClaim.

## Notes for DevOps

None
