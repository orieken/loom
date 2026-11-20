---
feature: "factory-close-waiting-orders"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["Phase", "PhaseWaiting", "PhaseCompleted", "PhaseProposed", "PhaseParked", "PhaseStopped", "closeable", "Closer", "ShippedByHand", "OrderState", "Ledger", "EventShipped", "inbox"]
files_touched: ["internal/factory (control.go)"]
issue_refs: []
linked_adrs: ["ADR-010"]
linked_kis: []
---

# Architecture Notes: factory-close-waiting-orders

## Structural Decisions

### Widen closeable()'s existing boolean phase check in internal/factory/control.go (line ~114) to also accept PhaseWaiting, PhaseCompleted, and PhaseProposed, written as a direct disjunction over the five allow-listed phases — not a lookup table, not a Strategy pattern, not a new type.
- **Rationale**: Five phases in an exhaustive, closed allow-list is a textbook case for a plain boolean expression. Simple Design (Kent Beck) ranks 'fewest elements' last but still ranks it: a map[Phase]bool or a Strategy per phase adds a name, a type, and an indirection for a condition that reads clearly as `order.Phase != PhaseParked && ... `. Introducing a dispatch abstraction here would be Premature Generalization — there is no second behavior varying by phase, only a single allow/refuse decision.
- **Trade-offs**:
  - A sixth or seventh phase added later re-touches this same line instead of adding a table row — acceptable, because PhaseRetrying, PhaseRunning, PhaseCommitted, and PhasePushed are refused on purpose (an order with its own process still running), not by omission, so the allow-list is expected to stay closed, not grow.
- **Fitness Function**: Cyclomatic complexity of closeable() stays below 7 after widening from 2 to 5 allowed phases — verified by golangci-lint (gocyclo) in CI, per shared/rules/design-principles.md's Sandi Metz limits
### No new package, interface, or cross-layer boundary is introduced. closeable(), Closer, and ShippedByHand all stay in internal/factory, the same use-case layer that already owns order-lifecycle decisions (RequestStop, RequestResume, RequestRerun live beside closeable() in control.go today).
- **Rationale**: ADR-010's order-lifecycle logic already lives in internal/factory as the use-case layer, with cmd/loom/cmd as its only adapter/CLI caller. Widening one function's condition does not change who owns the decision or where the decision is made, so no RFC and no new fitness function beyond the existing dependency-direction check are warranted.
- **Fitness Function**: judgment-only — ADR-010's order-lifecycle logic already lives in internal/factory as the use-case layer, with cmd/loom/cmd as its only adapter/CLI caller. Widening one function's condition does not change who owns the decision or where the decision is made, so no RFC and no new fitness function beyond the existing dependency-direction check are warranted.
### Rely on the absence of a PhaseShipped case in inbox.go's itemsFor, finish.go's finishStep, and start.go's continuationKind, rather than adding an explicit early-return or guard for the shipped phase in any of the three. Direct inspection (this session) confirms all three switches already fall through to their default/no-op return for any phase they don't name, including PhaseShipped.
- **Rationale**: Adding a defensive 'case PhaseShipped: return nil' to three files that already behave correctly by omission would be exactly the kind of speculative, unrequested change shared/rules/design-principles.md's Boy Scout Rule and the top-level 'don't add validation for scenarios that can't happen' instruction warn against. The switches are the fitness function here: a Phase that falls through does nothing, by construction.
- **Trade-offs**:
  - Correctness depends on these three switches staying exhaustive-by-omission (no default branch that acts). A later reviewer adding a catch-all 'default: do X' to any of the three would silently reopen AC2/AC4 — flagged as a developer handoff note and a regression test target, not mitigated by a code change now.
- **Fitness Function**: A supervisor tick immediately after closing a waiting, completed, or proposed order appends no launch/commit/push/propose ledger entry for that order, even with a stale unacted-on approval present — verified by unit test in internal/factory (e.g. TestSupervisorTickAfterCloseDoesNotRelaunch) that closes an order in each of the three newly-accepted phases, calls the supervisor's tick harness once, and asserts the ledger length is unchanged except for the shipped entry already written by Close — this is AC4's own required test (ADR-008: a test that fails if PhaseShipped were ever added to one of the three switches)
### Keep closeable()'s refusal error wording ('stop it first, or it has already shipped') byte-for-byte unchanged for every phase that remains refused (claimed, running, committed, pushed, retrying, shipped) — the widening only moves which phases satisfy the condition, never the message produced when they don't.
- **Rationale**: RequestRerun (control.go line 53) already refuses with this exact wording for the same reason. Keeping both call sites textually identical avoids two near-duplicate error strings drifting apart — a 'no duplication' concern (Kent Beck, Simple Design) more than a correctness one, since both paths mean the same thing: an order with its own process in flight cannot be closed or rerun by hand.
- **Fitness Function**: Every still-refused phase (claimed, running, committed, pushed, retrying, shipped) produces the unchanged refusal string and exit code 1 — verified by extend TestCloseableRefusesEveryOtherPhase and TestFactoryCloseRefusesAWrongPhaseInProcess to assert the literal message substring for each remaining phase, per AC3

## Component Placement

- `closeable` → Use Cases layer, `internal/factory (control.go)` — Unchanged placement — closeable() already lives in the use-case layer alongside RequestStop/RequestResume/RequestRerun; this feature only widens its condition, it does not relocate it.

## Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: cmd/loom/cmd/factory.go — CLI adapter layer surfacing the close control's accepted-phases help text, templates/claude-feature-team/intake/README.md — operator-facing lifecycle documentation, not code

## Stability Design

None

## Observability Design

None

## Layer Boundary Checks

- [PASS] internal/factory (Use Cases) must not import cmd/loom/cmd (Frameworks/CLI) or any adapter package — control.go's widened closeable() adds no new import; it only extends an existing in-memory Phase comparison against the already-defined Phase constants in the same package. Checked by reading control.go directly — mcp__loom__dependencies_of was not available in this session, so this is the import-reading fallback, stated per the architect process's fallback instruction.
- [PASS] cmd/loom/cmd (Frameworks/CLI) may depend on internal/factory (Use Cases) — dependency points inward, correct direction — factoryCloseCmd already calls factory.Closer.Close; only its Short/Long help strings change, not the dependency edge.
- [PASS] internal/factory/inbox.go, finish.go, start.go (Use Cases) require no new case or import to stay correct for PhaseShipped — Confirmed this session by reading itemsFor (inbox.go:79-92), finishStep (finish.go:72-84), and continuationKind (start.go:132-146): none has a PhaseShipped case, and none has a default branch that acts, so each already no-ops for a shipped order.

## Anti-Pattern Check

- Premature generalization: not found — Considered and rejected introducing a map[Phase]bool or Strategy dispatch for the allow-list — a plain boolean condition over 5 named constants is the simpler, already-idiomatic form used elsewhere in this same file (RequestStop, RequestRerun).
- Shotgun surgery: not found — The behavior change touches exactly one function (closeable); the other four edits (control_test.go, factory.go help text, factory_inprocess_test.go, README.md) are tests and documentation naming the same five phases, not independent logic changes rippling outward.
- Leaky abstraction: not found — closeable()'s signature and return type are unchanged; callers (Close) see no new concept.
- Anemic domain model: not found — Not applicable to this change — OrderState and Phase already carry behavior (IsActive, IsReady) elsewhere in the package; this feature doesn't touch that.
- God object: not found — control.go stays a small set of focused lifecycle-request functions; no new responsibility is added to Closer or OrderState.
- Distributed monolith: not found — Single-process, single-repo change; no service boundary involved.

## Fitness Functions

- **Cyclomatic complexity of closeable and every other new or changed function stays below 7**
  - **Verification**: golangci-lint (gocyclo) in CI
  - **Owner**: architect
- **Every changed line in this delivery is covered by a test that fails when the behavior it checks is broken**
  - **Verification**: go test ./... -coverprofile, plus a manual check that flipping closeable's new phase branches fails its test (ADR-008)
  - **Owner**: architect
- **No file outside scope.paths changes, except this feature's own delivery record**
  - **Verification**: the run executor's existing scope.paths check against the committed diff
  - **Owner**: devops-engineer
- **The supervisor never acts again on an order closed through close, even with a stale unacted-on approval present**
  - **Verification**: a unit test that closes a waiting (and separately, a completed-with-pending-approval) order, ticks the supervisor once more, and asserts no new launch/commit/push/propose ledger entry is appended (AC2, AC4)
  - **Owner**: architect
- **Every still-refused phase produces closeable's unchanged refusal wording and exit code 1**
  - **Verification**: table-driven assertion over claimed/running/committed/pushed/retrying/shipped in control_test.go and factory_inprocess_test.go (AC3)
  - **Owner**: architect

## Refactoring Opportunities

None

## Developer Handoff Notes

- Only widen the boolean condition at control.go:114 — do not refactor closeable() into a table or switch; the five-phase disjunction is the chosen, simplest form.
- Do not touch replay.go, inbox.go, finish.go, or start.go — confirmed by this review that none needs a code change. Add tests against them instead (AC2, AC4).
- Keep the refusal message text byte-identical to today's for every phase that remains refused; only the condition that reaches that branch changes.
- The supervisor-tick regression test (AC4) is the load-bearing test for this feature — it is the only test that would fail if a future change added a PhaseShipped case with an acting default branch to inbox.go/finish.go/start.go. Treat it as the exemplar test for this delivery.
- cmd/loom/cmd/factory.go and templates/claude-feature-team/intake/README.md changes are text-only (naming five phases instead of two) — no logic change, no new fitness function needed for them beyond the writing-style conventions already in force for docs.

## Open Architectural Questions

- Should PhaseRetrying be closeable? Resolved by the spec itself: no, for the same reason PhaseRunning is refused — an order with its own process still scheduled to run is not 'waiting on a human with no process of its own running.' Flagged here only so a developer does not reopen it without a human decision.
