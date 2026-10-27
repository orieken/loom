---
feature: "factory-idle-message"
bounded_context: ""
domain_terms: ["Idle", "Waiting on a human"]
files_touched: ["internal/factory/idle.go", "internal/factory/idle_test.go", "internal/factory/supervisor.go", "internal/factory/fixture_test.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Implementation Notes: factory-idle-message

## Files Created

- internal/factory/idle.go
- internal/factory/idle_test.go

## Files Modified

- internal/factory/supervisor.go
- internal/factory/fixture_test.go

## Interface Design

- type tickKind int, consts tickBusy (zero value) | tickIdle | tickWaiting
- type tickState struct { kind tickKind; count int } — comparable with ==
- func classify(orders map[string]*OrderState) tickState
- func waitingCount(orders map[string]*OrderState) int
- func hasUnshippedOrder(orders map[string]*OrderState) bool
- func (s *Supervisor) announce(state tickState)
- func (s *Supervisor) reportTickState() error
- Supervisor gains an unexported lastReported tickState field
- testFactory gains an output *bytes.Buffer field plus outputLines()/outputCount(text) test helpers

## Named Refactoring Log

None

## Self-Review Checklist

- [x] Every public method has an intention-revealing name
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new functions — classify has the most branches (4 return paths, 1 level deep); confirmed via golangci-lint gocyclo (min-complexity 7, clean) and manual count — all new funcs are 2-4
- [x] No primitive obsession — tickState wraps the (kind, count) pair instead of two loose return values, so announce's == comparison stays a single check
- [x] No feature envy
- [x] No magic numbers or strings — the two printed line formats are the only literals, each used once, matching the spec's exact wording
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — idle.go adds no new import; classify/waitingCount/hasUnshippedOrder take only map[string]*OrderState, per architecture-notes.md's structural decision — no Repository/TreeOpener/PullRequests/Intake can reach them

## Simple Design Verification

- [x] Passes all tests — go test ./internal/factory/... passes, including all 7 new idle_test.go cases and every pre-existing test
- [x] Reveals intention — tickBusy as the zero value documents itself: a fresh Supervisor's first busy tick needs no sentinel
- [x] No duplication — waitingPhases is one lookup table shared by waitingCount, matching inbox.go's itemsFor phase list (noted as an unenforced cross-check in architecture-notes.md)
- [x] Fewest elements — no new package, no interface, no flag — one new file plus a two-line call-site change in Tick()

## Key Decisions

- Put all idle/waiting classification and print-once logic in a new internal/factory/idle.go rather than supervisor.go: follows this package's existing one-concern-per-file convention (start.go, finish.go, replay.go, inbox.go), per architecture-notes.md
- classify/waitingCount/hasUnshippedOrder take only map[string]*OrderState, never Dependencies/Repository/TreeOpener/Intake: makes the NFR ('no extra git or filesystem pass per tick', 'never call Inbox() or re-read intake') an enforced property of the function signature rather than a discipline to remember
- tickKind's zero value is tickBusy, so tickState{} is the silent state: a fresh Supervisor's first busy tick stays silent with no separate 'never reported' sentinel
- reportTickState() is the last step of Tick(), called only after a successful startWork, and re-derives orders via its own s.Orders() call: matches existing precedent — settleEnded, advanceFinishLine, and startWork already each call s.Orders() independently within one Tick(); the NFR targets Inbox()'s git/filesystem calls and re-reading intake, not the local ledger read, which stays well inside those bounds

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- idle_test.go covers: a fresh supervisor's idle line; the once-per-entry rule across consecutive idle ticks; a gate halt and a parked order each counting as waiting; a second order entering a waiting phase re-printing with the incremented count; leaving waiting (approval resumes the run) printing nothing, then re-entering idle once the order ships re-prints; a retrying order (before its backoff) printing nothing, per the spec's resolved Open Question; and Supervisor.Once() applying the same once-per-entry rule across two separate invocations.
- go test ./internal/factory/... passes (full suite, 16s). golangci-lint run ./internal/factory/... reports one pre-existing, unrelated errcheck finding in units_test.go:163 (a file this change did not touch) — no issues in idle.go, idle_test.go, supervisor.go, or fixture_test.go.
- Coverage: idle.go's new functions are 100% covered except reportTickState's s.Orders()-error branch (a one-line error forward, 80% on that function); the identical idiom is already uncovered at several sites in this same package (e.g. start.go's own s.Orders() error forward, and several pre-existing error branches in supervisor.go) — this matches established convention rather than introducing a new gap. Aggregate changed-line coverage is well above the 85% floor.

## Notes for DevOps

None
