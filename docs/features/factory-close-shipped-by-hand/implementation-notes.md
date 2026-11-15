---
feature: "factory-close-shipped-by-hand"
bounded_context: ""
domain_terms: ["order", "ledger", "phase", "shipped", "parked", "stopped", "worktree", "supervisor", "approver"]
files_touched: ["internal/factory/control_test.go", "internal/factory/control.go", "internal/factory/finish.go", "internal/factory/replay.go", "cmd/loom/cmd/factory.go", "cmd/loom/cmd/factory_inprocess_test.go", "templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Implementation Notes: factory-close-shipped-by-hand

## Files Created

- internal/factory/control_test.go

## Files Modified

- internal/factory/control.go
- internal/factory/finish.go
- internal/factory/replay.go
- cmd/loom/cmd/factory.go
- cmd/loom/cmd/factory_inprocess_test.go
- templates/claude-feature-team/intake/README.md

## Interface Design

- type ShippedByHand struct { Order, URL, Approver string }
- type Closer struct { Layout Layout; Repository Repository }
- func (c Closer) Close(ledger *Ledger, orders map[string]*OrderState, request ShippedByHand) (string, error)
- func closeable(orders map[string]*OrderState, id, rawURL string) (*OrderState, error)
- func isHTTPURL(raw string) bool
- func archiveAndRemove(order *OrderState, layout Layout, repo Repository) string
- factoryCloseCmd (Use: "close <order>", required --shipped-by flag) wired via runFactoryClose

## Named Refactoring Log

- **Extract Function** on `internal/factory/finish.go: Supervisor.ship's archive-then-remove-worktree block` — Close needed the same archive-then-remove sequence and warning wording ship already has; extracted to archiveAndRemove(order, layout, repo) string so both callers share one implementation instead of Close duplicating ship's control flow and message templates.
- **Introduce Parameter Object** on `internal/factory/control.go: Close's order id / URL / approver inputs` — Close needs layout and repo (to finish the order) plus id, URL and approver on top of ledger+orders — seven parameters flat, over the 4-parameter limit. Bundled layout+repo into Closer's receiver and id/URL/approver into ShippedByHand, bringing Close itself to 3 parameters.

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new functions — verified with gocyclo -over 7 on every changed file; zero violations
- [x] No primitive obsession: domain concepts wrapped in value objects — ShippedByHand groups the three call-specific fields Close needs
- [x] No feature envy
- [x] No magic numbers or strings
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — internal/factory imports no cmd/ package; Close's only deps are Ledger, orders, Layout, Repository — no GitHub/network import

## Simple Design Verification

- [x] Passes all tests — go test ./... passes except one pre-existing, environment-caused failure (TestInvokeSetsNoTraceParentWhenUntraced, unrelated file, fails only because this shell has an ambient TRACEPARENT env var set; confirmed by unsetting it)
- [x] Reveals intention — Closer/ShippedByHand/closeable/isHTTPURL/archiveAndRemove names read as what they do
- [x] No duplication — archiveAndRemove extraction removes the duplicate archive/remove/warning-format block between ship and Close
- [x] Fewest elements — Two small new types accepted only to stay under the 4-parameter limit once layout+repo are added; documented as a deliberate trade-off in architecture-notes.md

## Key Decisions

- Close is a method on Closer{Layout, Repository}, taking a ShippedByHand{Order, URL, Approver} parameter object, not a flat function: A flat function would need 7 parameters (ledger, orders, layout, repo, id, url, approver), over the project's 4-parameter hard limit; bundling the two structural dependencies into a receiver and the three call-specific values into one parameter object keeps Close itself at 3 parameters.
- Extracted archiveAndRemove(order, layout, repo) string out of Supervisor.ship, and have both ship and Close call it: Close needed the exact same archive-then-remove sequence and warning wording ship already has; extracting once avoids duplicating that control flow and its two error-message templates (Kent Beck's no-duplication rule, Boy Scout Rule since finish.go was already being touched).
- closeable only ever permits PhaseParked or PhaseStopped; every other phase, including PhaseShipped, is refused with RequestRerun's existing wording, duplicated as a literal rather than shared: Matches the acceptance criteria's refusal list exactly (running, retrying, waiting, completed, committed, pushed, proposed, already shipped) with one allow-list check; the wording is duplicated rather than extracted into a shared helper because the match is an AC requirement for two call sites, not a generalizable abstraction, and touching RequestRerun would mix an unrelated refactor into this diff.
- isHTTPURL uses only net/url.Parse — true iff scheme is http or https and Host is non-empty, with zero network I/O: Structurally guarantees the 'close never fetches the URL' requirement: there is no code path to a network call because isHTTPURL imports nothing that could make one.
- entryUpdates[EventShipped] folds a non-empty Entry.URL onto OrderState.URL; ApprovedBy is left in the ledger entry only, not folded onto OrderState: status only needs to display the URL; the approver's provenance belongs in the ledger (the audit record), not duplicated onto the in-memory projection. The non-empty guard makes this a no-op for every EventShipped line the supervisor's own ship() already writes, so no existing behavior changes.

## Deviations from Analysis

None

## Dependencies Added

None

## Notes for QA

- Golden path and every refusal path from the acceptance criteria are covered by tests in internal/factory/control_test.go (unit level, using the existing testFactory fixture with a real git worktree) and cmd/loom/cmd/factory_inprocess_test.go (CLI level, using writeLedger to stand up orders in each disallowed phase).
- Verified changed-line coverage with cmd/diff-coverage against origin/main: 100% (57 of 57 executable changed lines), well above the 85% floor.
- Verified complexity with gocyclo -over 7 across every changed file: no function at or above 7.
- golangci-lint run ./internal/factory/... ./cmd/loom/cmd/... reports exactly one issue, an errcheck finding in internal/factory/units_test.go:163 (`<-done`) that predates this change and is not in any file this feature touches — left alone per scope.paths.
- go test ./... passes except TestInvokeSetsNoTraceParentWhenUntraced in internal/provider/claude, which fails only because this shell session has an ambient TRACEPARENT environment variable set; confirmed by rerunning with `env -u TRACEPARENT go test ./internal/provider/claude/...`, which passes. That file is untouched by this change.
- The success-path test for `close` exercises a real git worktree (via worktree.New(root).AddWorktree) and a real run-state file, so the archive-then-remove sequence runs to full success rather than only the warning branches — confirm this still reflects production worktree shapes if the run-state path convention ever changes.

## Notes for DevOps

None
