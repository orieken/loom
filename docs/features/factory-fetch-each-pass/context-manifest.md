# Context Manifest: factory-fetch-each-pass

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/factory/supervisor.go` — Tick() is the one-pass loop (ADR-010). It currently calls ReadIntake(s.deps.Repository, s.config.IntakeBranch, ...) with no fetch step. Config and Dependencies need the new --remote/--no-fetch knobs and the fetch call belongs at the top of Tick, before ReadIntake.
- `internal/factory/intake.go` — Defines the consumer-side Repository interface (ResolveCommit, ListFiles, ShowFile, AddWorktree, RemoveWorktree) that worktree.Git satisfies, and ReadIntake(repo, branch, plans). A Fetch method belongs on this interface (architecture-guardrails #1/consumer-defines-interface), and ReadIntake's ref needs to become remote-tracking (<remote>/<branch>) rather than bare branch when fetching.
- `internal/worktree/manage.go` — Where ResolveCommit (already falls back to origin/<ref>), AddWorktree, and Push live. Push already shows the required pattern for a network git call: context timeout, GIT_TERMINAL_PROMPT=0, wrapped error with stderr. Fetch(remote, branch) must follow the same pattern and sits naturally next to Push.
- `internal/factory/start.go` — claimFor() defaults an order's base to s.config.IntakeBranch when the order sets no base-branch, and prepare() passes that base straight to AddWorktree. AC3 (worktree starts from <remote>/<branch>) means this default must become the same remote-tracking ref Tick resolved, not the bare branch name.
- `cmd/loom/cmd/factory.go` — Owns the --branch flag (default "main") and builds factory.Config in newFactorySupervisor. New --remote (default origin) and --no-fetch flags are added here and threaded into Config; this is also where the command's --help text (Long string) documents the new fetch behavior.
- `internal/factory/fixture_test.go` — newTestFactory/newSupervisor builds every factory test's repo and Config with no remote configured at all — this is the concrete case AC6 (no remote -> read local branch unchanged, existing tests keep passing) must hold against. Likely needs a helper added for a bare-repo remote fixture for the new fetch tests.
- `internal/worktree/manage_test.go` — Already has the exact bare-repo-as-remote pattern the spec's Test Approach asks for: an upstream bare repo, a clone, ResolveCommit falling back to origin/main (line ~94), and Push reaching origin (line ~157) plus a no-origin failure case (line ~255). The new Fetch tests extend this same pattern.
- `internal/factory/supervisor_test.go` — Existing Tick-level tests for the supervisor's one-pass behavior; new cases (fetch runs once per pass, fetch failure doesn't park/fail an order, no-fetch/no-remote preserves today's behavior) belong alongside these and must not break what's already asserted here.
- `templates/claude-feature-team/intake/README.md` — AC8 requires this file to say orders are read from the remote after a fetch, that a promotion must be pushed, and how --no-fetch restores the old behavior. The "Enforced at pickup, by loom factory (ADR-010)" section is the specific paragraph to amend.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — The ADR every touched file's comments cite ("ADR-010, 2B" for reading intake off a committed branch, "ADR-010, 1C" for claim snapshots, "ADR-010, 3A" for stop/resume). The analyst needs this to state the Owning Context consistently with the two prior deliveries in this bounded context and to check whether fetch-before-read needs an ADR amendment or is just an implementation detail under 2B.

## Token Budget

- **Status**: OK — ~21404 tokens, 17.8% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/factory/supervisor.go` | 5515 | 1378 |
| `internal/factory/intake.go` | 4218 | 1054 |
| `internal/worktree/manage.go` | 5901 | 1475 |
| `internal/factory/start.go` | 9786 | 2446 |
| `cmd/loom/cmd/factory.go` | 12732 | 3183 |
| `internal/factory/fixture_test.go` | 10689 | 2672 |
| `internal/worktree/manage_test.go` | 9292 | 2323 |
| `internal/factory/supervisor_test.go` | 12322 | 3080 |
| `templates/claude-feature-team/intake/README.md` | 7629 | 1907 |
| `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` | 7545 | 1886 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `internal/factory/layout.go — only path/constant definitions (StateDir, IntakeDir, BranchFor); no branch/remote logic, referenced by filename only where needed`
- `internal/worktree/git.go — Digest/ChangedPaths/DiffLinesSince, the undeclared-edit-detection half of the worktree package; unrelated to fetch or branch resolution`
- `internal/worktree/git_test.go — tests only the Digest/ChangedPaths behavior above; same exclusion reason`
- `internal/factory/control.go, ledger.go, outcome.go, finish.go, inbox.go, idle.go — other supervisor concerns (stop/resume/rerun requests, ledger replay, finish-line, inbox, idle reporting) that this change does not touch; pulling them in would blow past the 10-file cap for no benefit`
- `docs/roadmaps/factory-phases.md — background on how L3.65-L3.70 were phased; useful history but the spec itself already states the defect and the fix, so this is optional reading, not required`
- `DOMAIN_DICTIONARY.md — grepped for "factory"; no bounded-context entry exists for it today, and the spec itself says no new terms are introduced, so there is nothing here to pin`
- `docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md — the source of review items I-5/I-8/I-10/D-12 referenced in code comments; background only, not needed to implement this fetch-before-read change`
- `docs/features/factory-close-waiting-orders/** and docs/features/factory-close-shipped-by-hand/** — the two prior deliveries in this bounded context, both declaring Owning Context 'Factory supervisor — order lifecycle and ledger (ADR-010)' (same as this feature); read for context, not pinned as implementation files. Neither has a retrospective.md (checked directly in both directories and elsewhere under docs/features/) so there is no What-Went-Poorly/What-To-Improve content to carry forward — stating this explicitly rather than omitting it`
- `Repo-graph MCP tools (mcp__loom__impact, mcp__loom__dependents_of, mcp__loom__find_files) were not available in this session (ToolSearch found no matching tool); blastRadius is omitted rather than guessed. The dependency reasoning above (cmd/loom/cmd/factory.go -> internal/factory -> internal/worktree) was derived by Grep over imports and call sites instead, and is not graph-derived. Token budget is left for loom to measure from the pinned files above.`
