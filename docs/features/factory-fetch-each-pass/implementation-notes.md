---
feature: "factory-fetch-each-pass"
bounded_context: ""
domain_terms: []
files_touched: ["internal/factory/supervisor.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Implementation Notes: factory-fetch-each-pass

## Files Created

None

## Files Modified

- internal/factory/supervisor.go

## Interface Design

- func (s *Supervisor) readIntake() (Intake, error) — new private helper combining fetchIntakeRef() + ReadIntake(), called once from Tick

## Named Refactoring Log

- **Extract Function** on `internal/factory/supervisor.go — Supervisor.Tick` — Tick measured at cyclomatic complexity 7 (the project's hard ceiling, not under it) after the fetch-before-read change added fetchIntakeRef()+ReadIntake() with two error branches. Extracted both calls into a new Supervisor.readIntake() helper so Tick drops one branch and returns to 6, matching the naming style already used for fetchIntakeRef.

## Self-Review Checklist

- [x] Every public method has an intention-revealing name
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new/changed functions — Verified with gocyclo: Tick=6 (was 7, fixed this pass by extracting readIntake), fetchIntakeRef=6, hasRemote=3, withDefaults=6, readIntake=2. All now confirmed under 7, including Tick which the prior self-review omitted.
- [x] No primitive obsession
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified

## Simple Design Verification

- [x] Passes all tests — go test ./internal/factory/... passes; go build ./... clean
- [x] Reveals intention — readIntake() names the fetch-then-read sequence as a single step, matching fetchIntakeRef's naming style
- [x] No duplication
- [x] Fewest elements — One small helper added, no other elements introduced

## Key Decisions

None

## Deviations from Analysis

- Updating implementation-notes.md's Self-Review Checklist to name Tick explicitly: This session's output contract disallows writing any file under .claude/ (implementation-notes.md lives there). The corrected, complete complexity claim is recorded instead in this stage's selfReview field above for the pipeline/human record.

## Dependencies Added

None

## Notes for QA

- No behavior change from the prior implementation — this pass only extracts Supervisor.readIntake() to bring Tick's cyclomatic complexity back under 7. Existing fetch-before-read test coverage (fetch_test.go, supervisor_test.go) still applies unchanged.

## Notes for DevOps

None
