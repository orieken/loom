# Context Manifest: factory-idle-message

## Scope

- **Tier**: developer
- **Built for**: developer

## Pinned Files

- `internal/factory/supervisor.go` — Tick/Run/Once/Drain and the say()/record()/detail() console-output helpers already in use; the idle/waiting transition detection and the once-per-transition print are wired in here.
- `internal/factory/replay.go` — OrderState, Phase constants, Replay(), and IsActive() — the idle/waiting/running classification is built from these, not a new parallel state.
- `internal/factory/start.go` — pendingWork/isClaimable/activeCount already compute 'running' and 'claimable'; the AC's 'none claimable' must reuse this exact test, not reimplement it.
- `internal/factory/intake.go` — Candidate.IsClaimable/IsReady and the Intake struct — what 'committed' (already loaded by the tick) holds going into the claimability check.
- `internal/factory/inbox.go` — ItemKind/Inbox()/itemsFor() define the four waiting kinds (gate halt, commit proposal, PR proposal, parked) the AC names; the new count must agree with `loom factory inbox` without copying its filesystem/git calls (NFR: no extra pass per tick).
- `internal/factory/ledger.go` — Event/Entry/Claim definitions (EventWaiting, EventProposed, EventParked, Claim.Finish) needed to classify phases from already-loaded state, with no new I/O.
- `internal/factory/finish.go` — finishCompleted's `Claim.Finish == "none"` rule — a completed order with finish:none ships immediately and is never a commit-proposal wait, so the waiting count must mirror this exactly or it will overcount.
- `internal/factory/supervisor_test.go` — Existing Tick-driven transition tests (e.g. TestAnOrphanedRunIsSettledFromItsState, TestTwoRunsStartAndTheThirdWaitsForASlot) are the style the new idle/waiting tests extend, asserting on the supervisor's output buffer.
- `internal/factory/fixture_test.go` — testFactory/orderSpec/fakeLauncher/tick()/inbox() fixtures the new tests drive; Dependencies.Output currently has no buffer wired in newSupervisor, so this is also where that gets added.
- `.claude/factory/claims/factory-idle-message.md` — The feature spec itself — exact wording of both printed lines, the four phases that count as waiting, and the retrying-order open question's resolved assumption (not waiting).

## Token Budget

- **Status**: OK — ~18637 tokens, 11.6% of the developer tier budget (160000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/factory/supervisor.go` | 4846 | 1211 |
| `internal/factory/replay.go` | 4927 | 1231 |
| `internal/factory/start.go` | 9066 | 2266 |
| `internal/factory/intake.go` | 4218 | 1054 |
| `internal/factory/inbox.go` | 9924 | 2481 |
| `internal/factory/ledger.go` | 8228 | 2057 |
| `internal/factory/finish.go` | 8511 | 2127 |
| `internal/factory/supervisor_test.go` | 11274 | 2818 |
| `internal/factory/fixture_test.go` | 10025 | 2506 |
| `.claude/factory/claims/factory-idle-message.md` | 3547 | 886 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `internal/factory/control.go`
- `internal/factory/settle.go`
- `internal/factory/layout.go`
- `internal/factory/summary.go`
- `internal/factory/summary_test.go`
- `internal/factory/launcher.go`
- `internal/factory/process_unix.go`
- `internal/factory/process_windows.go`
- `internal/factory/lock_unix.go`
- `internal/factory/lock_windows.go`
- `internal/factory/units_test.go`
- `internal/factory/lifecycle_test.go`
- `internal/factory/faults_test.go`
- `internal/memory/workspace.go (different bounded context — Documentation Knowledge Base, not touched by this change)`
- `cmd/loom/cmd/* (change surface is console text inside internal/factory only; no CLI wiring changes needed)`
