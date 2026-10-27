---
feature: "factory-idle-message"
bounded_context: "Agent Orchestration"
domain_terms: []
files_touched: ["internal/factory/idle.go", "internal/factory/idle.go", "internal/factory/supervisor.go", "internal/factory/fixture_test.go"]
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Architecture Notes: factory-idle-message

## Structural Decisions

### Add the whole idle/waiting determination (tickKind, tickState, classify, waitingCount, hasUnshippedOrder, announce, reportTickState) in a new file internal/factory/idle.go, not in supervisor.go or replay.go.
- **Rationale**: internal/factory already splits by concern — start.go (claim/launch), finish.go (finish line), settle.go (settling ended runs), replay.go (ledger→state fold), inbox.go (human-facing inbox). Idle/waiting determination is a new, distinct concern with its own reason to change (what to print, once, when nothing is running) and belongs in its own file by that same convention, not bolted onto Supervisor's own file or the generic replay fold.
- **Trade-offs**:
  - One more file to open when tracing Tick()'s full behavior end to end, but each file stays single-purpose and independently testable, matching every other phase of Tick().
- **Fitness Function**: internal/factory stays organized one concern per file — idle/waiting logic does not get mixed into supervisor.go, replay.go, or inbox.go — verified by judgment-only: no linter checks file-level cohesion in Go; enforced by code-reviewer against this package's existing precedent (start.go/finish.go/settle.go/replay.go/inbox.go split)
### classify(orders), waitingCount(orders), and hasUnshippedOrder(orders) take only map[string]*OrderState — the ledger replay Tick() already produces — never Dependencies, Repository, TreeOpener, PullRequests, or the committed Intake.
- **Rationale**: The NFR requires idle/waiting determination to read only the already-produced ledger replay and perform no extra git or filesystem pass, in particular never calling Inbox() or re-reading intake. Giving these three functions no parameter through which Repository, Trees, or Intake could reach them makes that an enforced property of the function signature, not a discipline the next change could quietly violate. It also matches analyst's architecturalFlags note: after a successful Tick(), activeCount(orders)==0 already implies nothing was claimable either (New() forces MaxConcurrent>=1, and a start failure aborts Tick() before classify runs), so re-deriving claimability from Intake inside classify would be dead code under that invariant.
- **Trade-offs**:
  - classify cannot independently re-verify claimability from Intake even if the activeCount invariant it depends on ever changes elsewhere in start.go; a change to that invariant must be cross-checked against classify by a human reviewer, not caught by a compiler.
- **Fitness Function**: idle-state determination performs zero I/O of its own — verified by compiler/go vet: classify, waitingCount, and hasUnshippedOrder have no parameter type through which Repository, TreeOpener, or PullRequests could reach them — any future call site that needs one of those to decide idle/waiting state requires widening the signature, which code-reviewer must flag as reopening the NFR
### Supervisor gets a lastReported tickState field (plain value, not a pointer); tickKind's zero value is tickBusy, so tickState{}'s zero value is the silent/busy state. (s *Supervisor) announce(state) compares state == s.lastReported before printing.
- **Rationale**: A fresh Supervisor's first busy tick must stay silent without a separate 'never reported yet' sentinel or bool. Making tickBusy the zero value and tickState a small comparable struct (int kind + int count) gets that for free from Go's zero-value semantics, the same way Phase already being a plain comparable string keeps OrderState simple in replay.go.
- **Trade-offs**:
  - tickState must stay comparable with == forever — adding any non-comparable field (slice, map, pointer-identity-sensitive type) to it later would break announce's comparison at compile time, which is the intended guardrail but does constrain future extension of tickState.
- **Fitness Function**: tickState remains directly comparable with == (no slice/map/func fields) — verified by compiler: the == comparison in announce fails to build the moment a non-comparable field is added to tickState, forcing the change through review instead of silently compiling to a different semantic
### reportTickState() is the last step of Tick(), invoked only after startWork(committed) returns nil, and re-derives orders via its own call to s.Orders() rather than Tick() threading the orders map startWork already computed through to it.
- **Rationale**: This matches existing precedent in the same function: settleEnded(), advanceFinishLine(), and startWork() already each call s.Orders() independently inside one Tick(), re-deriving state from the ledger rather than sharing one fold across phases. Ledger.Entries() is a single local os.ReadFile plus an in-memory fold — the NFR's 'no extra git or filesystem pass' explicitly targets Inbox() (per-order loadRunState plus git tree Digest()/ChangedPaths()) and re-reading intake (re-parsing every candidate file), not the ledger read itself, which is the NFR's own sanctioned source ('read only the ledger replay the tick already produces'). reportTickState calls neither Inbox() nor ReadIntake, so it stays inside the NFR regardless of being a 4th ledger read in the same tick.
- **Trade-offs**:
  - A 4th full ledger read-and-replay per tick, on top of the 3 the package already does — consistent with existing cost, but it does mean up to 4 separate in-memory folds of the same ledger contents inside one Tick() call rather than one shared result threaded through. Acceptable because the file is local, append-only, and typically small relative to a tick interval measured in seconds.
- **Fitness Function**: reportTickState never calls Inbox() or ReadIntake — its only I/O is through s.Orders() — verified by go test ./internal/factory/... plus code review: idle.go and idle_test.go must contain no reference to Inbox( or ReadIntake(; the NFR is otherwise structurally guaranteed by structural decision 2 above, since classify/waitingCount/hasUnshippedOrder cannot reach those functions regardless of call count
### waitingPhases is a package-level var map[Phase]bool{PhaseWaiting, PhaseCompleted, PhaseProposed, PhaseParked} — a lookup table — rather than a switch/if-chain inside waitingCount.
- **Rationale**: Follows this same package's existing idiom (phaseOf, entryUpdates in replay.go) of replacing per-event/per-phase conditionals with a lookup table, per design-principles.md's Complexity order (step 3: replace if/else chains with lookup tables before extracting more functions). It also keeps classify/waitingCount's cyclomatic complexity trivially low. The four phases are deliberately named identically to the four cases inbox.go's itemsFor already switches on (PhaseWaiting, PhaseCompleted, PhaseProposed, PhaseParked) — the same four kinds the AC enumerates (gate halt, commit proposal, PR proposal, parked).
- **Trade-offs**:
  - waitingPhases (idle.go) and itemsFor's switch (inbox.go) are two independent statements of 'which phases count as waiting on a human.' Nothing enforces they stay in lockstep; a future phase added to one and not the other drifts silently. Flagged to developer as a handoff note rather than gated here, since adding that cross-check is new test surface beyond this feature's scope.
- **Fitness Function**: waitingPhases (idle.go) and the phase switch in itemsFor (inbox.go) enumerate the same four phases — verified by judgment-only for this feature: no test currently asserts the two lists are identical. A follow-up fitness function would be a test asserting waitingCount(orders) agrees with len(Inbox(orders,...)) restricted to approvable+informational items — captured as a developer handoff note, not a gate for this change
### No feature flag, no new package, no new interface, and no RFC for this change — it is additive console output on an existing Tick() call path with no schema, ledger, API, or cross-package change.
- **Rationale**: None of the RFC triggers apply: no layer boundary is touched (internal/factory is a single flat package; idle.go imports nothing new), no new abstraction is introduced (tickKind/tickState are plain value types, not a pattern), and only one package (internal/factory) plus its test fixture is affected. Reversible by a normal git revert.
- **Fitness Function**: judgment-only — None of the RFC triggers apply: no layer boundary is touched (internal/factory is a single flat package; idle.go imports nothing new), no new abstraction is introduced (tickKind/tickState are plain value types, not a pattern), and only one package (internal/factory) plus its test fixture is affected. Reversible by a normal git revert.

## Component Placement

- `tickKind, tickState, classify, waitingCount, hasUnshippedOrder` → pure domain-replay helpers (operate only on map[string]*OrderState, no I/O) layer, `internal/factory/idle.go` — New, distinct concern (idle/waiting classification) kept pure and separate from Supervisor's orchestration, mirroring replay.go's existing pure-fold style.
- `(s *Supervisor) announce, (s *Supervisor) reportTickState` → supervisor orchestration (reads s.Orders(), writes s.deps.Output via s.say) layer, `internal/factory/idle.go` — Thin glue between the pure classification and the Supervisor's existing s.say output mechanism; kept in the same file as the types it reports on rather than supervisor.go, per the one-concern-per-file convention.
- `lastReported tickState field` → supervisor state layer, `internal/factory/supervisor.go` — Supervisor already owns per-run state (ledger, processes, draining); the print-once tracking field belongs alongside it, added after the existing fields.
- `output *bytes.Buffer field and outputLines()/outputCount() helpers` → test fixture layer, `internal/factory/fixture_test.go` — Dependencies.Output has no buffer wired into testFactory today (defaults to io.Discard); the fixture needs one to assert against, following the existing assertPhase/inbox helper style.

## Bounded Context

- **Owning Context**: Agent Orchestration
- **Context Crossings**: None

## Stability Design

None

## Observability Design

- console line (idle / waiting-on-a-human), via the Supervisor's existing s.say → deps.Output emitted from Supervisor.reportTickState, called from Tick() — the same mechanism every other ledger-event console line already uses (s.record → s.say) (cardinality: Two fixed message shapes (idle; waiting with an integer count), printed at most once per state entry — low cardinality, no PII, no raw order content.)

## Layer Boundary Checks

- [NOT_APPLICABLE] No new import crosses a layer boundary (domain → application → adapter → framework, per ARCHITECTURE_RULES.md) — internal/factory is a single flat Go package with no domain/usecase/adapter sub-layering; idle.go adds no new import at all (it depends only on types already defined in this same package: Phase, OrderState, Supervisor). The loom MCP graph tools (mcp__loom__dependencies_of/dependents_of/describe) were not available in this session (not registered), so this was verified by direct inspection of internal/factory/*.go imports rather than the repo graph; stated here per the architect's fallback-disclosure requirement.

## Anti-Pattern Check

- God Object: not found — Supervisor gains one small comparable field (lastReported) and one short delegating call (reportTickState); all the actual classification logic lives in idle.go's pure functions, not piled onto Supervisor.
- Shotgun Surgery: not found — Three files touched (new idle.go, a two-line change to supervisor.go's Tick(), and a fixture addition to fixture_test.go) — a single, cohesive change.
- Anemic Domain Model: not found — classify/waitingCount/hasUnshippedOrder add behavior derived from OrderState's existing Phase field, consistent with OrderState already carrying behavior (IsActive()).
- Leaky Abstraction: not found — No new public surface is exposed beyond the package; nothing forces a caller to understand idle.go's internals.
- Premature Generalization: not found — No new interface, strategy, or pattern introduced for what is three small pure functions and a print-once guard.

## Fitness Functions

- **Every new/changed function in internal/factory stays below cyclomatic complexity 7**
  - **Verification**: golangci-lint (gocyclo, min-complexity: 7) — already wired in .golangci.yml and CI; no new CI change needed for this feature
  - **Owner**: DevOps
- **Every changed line in this feature is covered by a test**
  - **Verification**: cmd/diff-coverage, run in .github/workflows/framework-ci.yml's 'Changed-line coverage (ADR-008, roadmap L3.58)' step against coverage.out — already wired; no new CI change needed
  - **Owner**: DevOps
- **go test ./internal/factory/... passes with the new idle/waiting behavior**
  - **Verification**: go test ./internal/factory/... — existing test command, no new tooling
  - **Owner**: DevOps
- **idle-state determination performs zero I/O of its own (reads only the already-produced ledger replay)**
  - **Verification**: compiler/go vet: classify, waitingCount, hasUnshippedOrder have no parameter through which Repository, TreeOpener, or PullRequests could reach them; code review blocks any signature widening that would reopen this
  - **Owner**: Architect

## Refactoring Opportunities

None

## Developer Handoff Notes

- Implement exactly the analyst's developerTasks in internal/factory/idle.go and the two-line change to supervisor.go's Tick() — this review found no gaps in that plan.
- waitingPhases in idle.go must enumerate the same four phases as inbox.go's itemsFor switch (PhaseWaiting, PhaseCompleted, PhaseProposed, PhaseParked). If a future change adds or removes a phase from one, add/remove it from the other in the same commit — nothing currently enforces they agree.
- reportTickState's call to s.Orders() is an intentional 4th ledger read within one Tick() (settleEnded, advanceFinishLine, and startWork already each call it independently) — this is existing precedent, not a regression to fix by threading one shared orders map through Tick().
- Keep classify/waitingCount/hasUnshippedOrder's signatures restricted to map[string]*OrderState only — do not add a Repository, TreeOpener, PullRequests, or Intake parameter to any of them, even if a future case seems to need one; that would silently reopen the 'no extra git/filesystem pass per tick' NFR this review verified is structurally closed.
- Dependencies.Output already defaults to io.Discard via New() when unset (supervisor.go:72-74) — the fixture change just needs to wire a *bytes.Buffer into Dependencies.Output explicitly in newSupervisor(), nothing in production code changes.

## Open Architectural Questions

None
