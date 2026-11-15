# Context Manifest: factory-close-shipped-by-hand

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/factory/control.go` — Closest existing pattern for a human-issued control command (RequestStop/RequestResume/RequestRerun): same ErrNotApplicable error style, same orderIn() lookup, same per-phase-refusal shape close's validation must follow. The spec also says close's new unit tests live beside these.
- `internal/factory/finish.go` — Supervisor.ship() is the exact record-then-archive-then-remove sequence (and the 'kept, and says so' failure messages) the close command must reproduce outside the supervisor; archiveRun() here is reused directly.
- `internal/factory/ledger.go` — Defines Event/Entry, including EventShipped and the already-present URL/ApprovedBy fields close's new ledger line needs, plus Ledger.Append's atomic single-write guarantee the feature's NFR relies on.
- `internal/factory/replay.go` — Phase constants (PhaseParked, PhaseStopped, PhaseShipped), OrderState, IsActive(), and the entryUpdates map that folds each Entry onto an order — close's EventShipped line needs a URL update added here so `status` can display it.
- `internal/factory/layout.go` — Layout.RunArchivePath/WorktreePath/LedgerPath are the paths close must resolve for the same archive-then-remove step ship() uses.
- `internal/factory/intake.go` — Defines the Repository interface (RemoveWorktree) close depends on to remove a closed order's worktree.
- `cmd/loom/cmd/factory.go` — Where the new `close` subcommand, its --shipped-by flag, and its RunE wire in; also owns approverName() (the AC's 'same source as approve') and statusDetail() (the detail column close's URL must show).
- `internal/factory/fixture_test.go` — testFactory/orderSpec fixture (real git repo + scripted launcher) the new internal/factory unit tests for close reuse, per the spec's Test Approach section.
- `cmd/loom/cmd/factory_inprocess_test.go` — writeLedger/claimedEntry/runFactoryCommand helpers already used to put an order into a given phase (parked, stopped, etc.) without running anything — the exact pattern the new close command tests reuse.
- `templates/claude-feature-team/intake/README.md` — An acceptance criterion requires this file say when to use `close`; its existing Lifecycle/Status table is what the new guidance must fit alongside.

## Token Budget

- **Status**: OK — ~17674 tokens, 14.7% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/factory/control.go` | 2207 | 551 |
| `internal/factory/finish.go` | 8511 | 2127 |
| `internal/factory/ledger.go` | 8595 | 2148 |
| `internal/factory/replay.go` | 5908 | 1477 |
| `internal/factory/layout.go` | 4123 | 1030 |
| `internal/factory/intake.go` | 4218 | 1054 |
| `cmd/loom/cmd/factory.go` | 11102 | 2775 |
| `internal/factory/fixture_test.go` | 10689 | 2672 |
| `cmd/loom/cmd/factory_inprocess_test.go` | 8574 | 2143 |
| `templates/claude-feature-team/intake/README.md` | 6790 | 1697 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `internal/factory/inbox.go — Approve()/approver pattern already visible via cmd/loom/cmd/factory.go's approverName(); not otherwise needed to implement close`
- `internal/factory/outcome.go — run-outcome judging (completed/failed/budget) is unrelated to closing an already-parked-or-stopped order`
- `internal/factory/supervisor.go — Supervisor internals; the one relevant method (ship) is already pinned via finish.go`
- `internal/factory/start.go — claim/launch logic for active runs; out of scope, since close only ever acts on a parked or stopped order`
- `internal/factory/faults_test.go, lifecycle_test.go, units_test.go, supervisor_test.go — further test-pattern files; fixture_test.go and factory_inprocess_test.go already cover the two fixtures close's tests need, and pinning all of them would exceed the 10-file cap`
- `cmd/loom/cmd/factory_test.go — heavier end-to-end test that builds the real binary; factory_inprocess_test.go's lighter in-process pattern is pinned instead`
- `DOMAIN_DICTIONARY.md — the framework's own meta-dictionary (Agent, Skill, Pipeline…); it does not define factory/ledger/order vocabulary, which ADR-010 and the spec itself already cover`
- `shared/knowledge/*.md — searched by filename and content; none mention the factory supervisor, its ledger, or the finish line`
