# Context Manifest: pre-ship-checks

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/orchestrator/plan.go` — Defines Stage/Plan, the gates map (confirm-ship is on devops-engineer), the skippable map, and DefaultDeliverFeaturePlan's agent sequence — the analyst must place the new pre-ship-checks stage between tech-writer and devops-engineer here.
- `internal/orchestrator/router.go` — The only existing executor-internal stage. runInternalStage's switch-of-one is the dispatch pattern pre-ship-checks extends; shows how an internal stage reads StageInput and returns StageOutput without a provider call.
- `internal/orchestrator/executor.go` — The run loop that calls runInternalStage, enforces per-stage Timeout, and persists stage status/artifacts — needed to know where command execution, the LOOM_BASE_COMMIT env var, and failure propagation plug in.
- `internal/orchestrator/failure.go` — Terminal-failure semantics: the AC requires a failing check to halt the run (never reach confirm-ship) without a supervisor retry, which this file's failure model governs.
- `internal/policy/config.go` — delivery-policy.yaml's Config struct, keptKeys/removedKeys, and strict LoadConfig — preShipChecks must be added here as a new kept key with the same reject-unknown-key discipline.
- `internal/policy/decode.go` — The strict per-key YAML-node decode idiom (decodeCondition/addEntry) this package already uses — the pattern to reuse for validating each preShipChecks entry's name/command/timeout and rejecting unknown keys.
- `internal/policy/config_test.go` — Existing config decode/rejection test style to mirror when adding tests for preShipChecks entries, including the no-name/no-command/unknown-key rejection cases.
- `cmd/diff-coverage/run.go` — Defines the --base and --profile flags scripts/pre-ship-check.sh must invoke exactly as CI does, so the script reproduces CI's own coverage gate.
- `scripts/ci-diff-base.sh` — An existing shell script in this repo's own house style (small, bash -c friendly) to match when writing the new scripts/pre-ship-check.sh.
- `shared/skills/deliver-feature/SKILL.md` — The prose stage sequence (steps 11, 13, 25) that plan.go's Go code mirrors — the source of truth for confirming the new stage's placement relative to tech-writer and the confirm-ship gate.

## Token Budget

- **Status**: OK — ~27143 tokens, 22.6% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/orchestrator/plan.go` | 12800 | 3200 |
| `internal/orchestrator/router.go` | 6233 | 1558 |
| `internal/orchestrator/executor.go` | 26705 | 6676 |
| `internal/orchestrator/failure.go` | 2315 | 578 |
| `internal/policy/config.go` | 6089 | 1522 |
| `internal/policy/decode.go` | 6049 | 1512 |
| `internal/policy/config_test.go` | 5460 | 1365 |
| `cmd/diff-coverage/run.go` | 3808 | 952 |
| `scripts/ci-diff-base.sh` | 907 | 226 |
| `shared/skills/deliver-feature/SKILL.md` | 38217 | 9554 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `mcp__loom__impact / mcp__loom__dependents_of / mcp__loom__find_files were unavailable this session (loom MCP server not registered) — blastRadius is omitted rather than guessed; the pinned set above was chosen by direct Grep over the stage-dispatch, config-decode, and diff-coverage call sites instead of a graph query.`
- `Prior deliveries in this bounded context: docs/features/context-engineering-framework/analysis.md has Owning context 'Agent Orchestration', the same context this task touches, but that feature has no retrospective.md — no lesson to carry forward from it.`
- `internal/policy/gate.go, internal/orchestrator/gate.go, docs/patterns/gate-enforcement.md — the confirm-ship gate's enforcement mechanics are unchanged by this feature (the gate stays on devops-engineer); only the new stage's position relative to it matters, which plan.go already shows.`
- `intake/pre-ship-checks.md — a verbatim duplicate of the feature spec already supplied to this stage; no additional information.`
- `.claude/policies/evidence-clean-ship.policy.yaml, shared/plans/deliver-bugfix.yaml — a different automation path (the Go policy evaluator) and a plan explicitly out of scope per the spec's Out of Scope section.`
- `cmd/diff-coverage/main.go, cmd/diff-coverage/annotate.go, internal/gitdiff/gitdiff.go — diff-coverage internals beyond the CLI flags the script needs; run.go alone covers the invocation contract.`
