# Context Manifest: factory-close-waiting-orders

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/factory/control.go` — Primary change surface: closeable() (line 106) currently accepts only PhaseParked/PhaseStopped; the spec requires it also accept PhaseWaiting, PhaseCompleted, and PhaseProposed. Close() and isHTTPURL() beside it are unaffected but must be read for the full refusal/ship contract.
- `internal/factory/control_test.go` — Existing unit tests encode the current (now-wrong) contract: TestCloseableAcceptsParkedAndStopped, TestCloseableRefusesEveryOtherPhase, TestCloseableRefusalNamesThePhaseAndInstructsStoppingFirst, and TestCloseRefusesAnOrderThatIsNotParkedOrStopped all assert that waiting/completed/proposed are refused. Per the spec's own Test Approach, new per-phase tests belong beside these.
- `internal/factory/replay.go` — Defines every Phase constant, the phaseOf/entryUpdates maps EventShipped folds through, and IsActive(). Confirms PhaseShipped is terminal and that appending EventShipped from waiting/completed/proposed needs no change here — applyShipped and the phase fold already handle an arbitrary prior phase.
- `internal/factory/inbox.go` — itemsFor() (line 79) switches on order.Phase (PhaseWaiting/PhaseCompleted/PhaseProposed/PhaseParked) to build inbox items. Confirms AC2 (closed order leaves the inbox) is satisfied automatically once Phase becomes PhaseShipped — no separate inbox change needed, but the analyst must verify this reasoning against the real switch.
- `internal/factory/finish.go` — Supervisor.finishStep() (line 72) switches on Phase to decide whether to commit/push/open a PR; PhaseShipped matches none of its cases. This is the evidence for AC3/AC4 that the supervisor never acts on a closed order again — needed to justify, not just assert, the no-relaunch acceptance criterion.
- `internal/factory/start.go` — Supervisor.continuationKind() (line 132) is the other half of the no-relaunch guarantee: it switches on Phase to decide whether to resume/retry an order, and PhaseShipped matches none of its cases. AC4 explicitly asks for a test that ticks the supervisor once after close() — this function is what that test must exercise.
- `cmd/loom/cmd/factory.go` — Wires `loom factory close`: factoryCloseCmd's Long text (line 110) currently documents only 'parked or stopped'; runFactoryClose (line 347) calls factory.Closer unchanged. In scope.paths (cmd/loom/cmd/factory*.go) — the Long description likely needs updating even though runFactoryClose itself does not.
- `cmd/loom/cmd/factory_inprocess_test.go` — TestFactoryCloseRefusesAWrongPhaseInProcess (line 237) currently proves a waiting order is refused end-to-end through the CLI — this assertion is now wrong per the spec and must be replaced with phase-specific in-process coverage (accepted: waiting/completed/proposed; still refused: running/committed/pushed/retrying).
- `templates/claude-feature-team/intake/README.md` — AC explicitly requires the close section (lines 32-36) to name the phases it accepts; it currently says only 'a parked or stopped order'.
- `docs/features/factory-close-shipped-by-hand/analysis.md` — Only prior delivery in this bounded context (Factory supervisor — order lifecycle and ledger, ADR-010). No retrospective.md exists for it. Its analysis.md is the precedent for closeable()'s exact refusal wording ('stop it first, or it has already shipped'), the Close/archiveAndRemove sequencing, and the Developer/QA task phrasing this delivery should extend rather than reinvent.

## Token Budget

- **Status**: OK — ~24822 tokens, 20.7% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/factory/control.go` | 4795 | 1198 |
| `internal/factory/control_test.go` | 12056 | 3014 |
| `internal/factory/replay.go` | 6301 | 1575 |
| `internal/factory/inbox.go` | 11403 | 2850 |
| `internal/factory/finish.go` | 9203 | 2300 |
| `internal/factory/start.go` | 9786 | 2446 |
| `cmd/loom/cmd/factory.go` | 12551 | 3137 |
| `cmd/loom/cmd/factory_inprocess_test.go` | 12658 | 3164 |
| `templates/claude-feature-team/intake/README.md` | 7301 | 1825 |
| `docs/features/factory-close-shipped-by-hand/analysis.md` | 13254 | 3313 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `internal/factory/fixture_test.go — test-harness helpers (newTestFactory, assertPhase, waitingState/completedState state builders, exit-code constants) the developer will need when writing new tests, but not required for the analyst's phase-level reasoning; left out to stay inside the 10-file pin cap in favor of the direct change-surface files.`
- `internal/factory/lifecycle_test.go, internal/factory/supervisor_test.go — broader supervisor test suites; not required to analyze this narrow phase-acceptance change.`
- `internal/factory/ledger.go — Entry/Event struct definitions; the event vocabulary needed (EventShipped, EventApproved, etc.) is already visible in replay.go's phaseOf/entryUpdates maps.`
- `shared/knowledge/*.md — searched (search-ki); none of the 13 existing KIs (aider-repo-map, ast-grep, context-engineer-pipeline-wiring, ctx-session-history, docs-rag-structure, flake-triage, git-churn, incident-evidence, ollama-embeddings, repomix, subagent-isolation, tokei-budget, unified-candidate-record) cover factory order phases, the ledger, or close — no match to surface.`
- `docs/adrs/ADR-010-repo-context-indexing.md — name collision with the real ADR-010 (factory-order-state-and-policy-configuration); unrelated to this feature, confirmed by content.`
