---
feature: "factory-fetch-each-pass"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["Intake", "Candidate", "Claim", "Repository", "Order", "base-branch", "remote-tracking ref"]
files_touched: ["internal/worktree", "internal/worktree", "internal/factory", "internal/factory", "internal/factory", "internal/factory", "cmd/loom/cmd/factory.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Architecture Notes: factory-fetch-each-pass

## Structural Decisions

### Fetch-before-read is an implementation detail under ADR-010 Decision 2B, not a new structural decision. No ADR amendment and no new RFC.
- **Rationale**: 2B already commits the supervisor to reading intake/ at a committed ref rather than the working tree, specifically so an order reaches the factory the way code reaches main: reviewed. Resolving that ref from <remote>/<branch> instead of the bare local <branch> changes which commit counts as 'committed,' not whether the read is working-tree or committed. Confirmed by reading ADR-010 directly — its Consequences and Fitness Function sections already describe 'a supervisor tick reads an order from the base-branch commit, not the working tree' with no remote/local distinction, so this closes a gap in that guarantee rather than adding a new one.
- **Trade-offs**:
  - No RFC means no recorded alternatives trail for this specific choice — acceptable because ADR-010 already recorded and accepted the trade-off (committed-ref reads are slower than working-tree reads, chosen anyway for provenance) that this change operates inside of.
- **Fitness Function**: judgment-only — 2B already commits the supervisor to reading intake/ at a committed ref rather than the working tree, specifically so an order reaches the factory the way code reaches main: reviewed. Resolving that ref from <remote>/<branch> instead of the bare local <branch> changes which commit counts as 'committed,' not whether the read is working-tree or committed. Confirmed by reading ADR-010 directly — its Consequences and Fitness Function sections already describe 'a supervisor tick reads an order from the base-branch commit, not the working tree' with no remote/local distinction, so this closes a gap in that guarantee rather than adding a new one.
### Repository interface (internal/factory/intake.go) gains Remotes() ([]string, error) and Fetch(remote, branch string) error. internal/worktree.Git implements both. The dependency direction stays internal/factory defining, internal/worktree implementing — internal/factory gains no new import.
- **Rationale**: Architecture-guardrails #1: inner layers never import outer layers, and the use-case layer (internal/factory, which holds order lifecycle policy) defines interfaces that the adapter layer (internal/worktree, which holds git mechanics) implements. This matches the existing five-method Repository interface exactly — ResolveCommit, ListFiles, ShowFile, AddWorktree, RemoveWorktree are all use-case-defined, adapter-implemented. Fetch and Remotes are the same shape: things the supervisor needs from git, described in the supervisor's own vocabulary.
- **Trade-offs**:
  - The Repository interface grows from 5 to 7 methods. Acceptable: both new methods are cohesive with the existing set (all git-reads-or-mutations the supervisor orchestrates) — this is not the data-clump smell that would call for splitting into two interfaces.
- **Fitness Function**: internal/factory has zero production import of internal/worktree (interface stays consumer-defined; implementation stays adapter-side) — verified by go list -deps ./internal/factory | grep internal/worktree returns nothing for non-test files; enforce via a dependency-cruiser-equivalent Go import check or a grep-based CI step — none exists yet for this specific pair, so this is a new, cheap fitness function to add
### Remotes() is a local-only call (git remote, no network, no timeout) — mirrors hasBranch's existing local-read pattern (internal/worktree/manage.go:66). Fetch() is a network call using the exact Push pattern: context.WithTimeout(fetchTimeout), GIT_TERMINAL_PROMPT=0, combined-stderr-wrapped error (internal/worktree/manage.go:151-161, const fetchTimeout = 5*time.Minute beside pushTimeout).
- **Rationale**: Two different failure semantics need two different calls, not one call with two error paths: AC6 (no remote configured → silent, unchanged local-branch fallback) and AC5 (remote configured but fetch fails → one logged line, pass continues) are distinguishable only if 'does this remote exist' is answered before any network attempt. Folding that check inside Fetch's own error handling would force fetchIntakeRef to string-match git's stderr to tell the two cases apart — a leaky abstraction this interface must not create.
- **Trade-offs**:
  - Two git invocations per pass (one local git remote, one network git fetch) instead of one. Negligible: git remote is a local metadata read with no measurable cost next to one network fetch.
- **Fitness Function**: Every network call (Fetch) has an explicit timeout; every local-only call (Remotes, matching hasBranch) has none and attempts no network I/O — verified by go test ./internal/worktree/... — new table test for Fetch asserts it wraps context.WithTimeout and that its error message carries git's stderr on failure; code review confirms Remotes contains no context.WithTimeout and runs git remote only (architecture-guardrails #5)
### fetchIntakeRef (new, internal/factory/supervisor.go) is the single call site deciding --no-fetch vs. no-remote vs. fetch-then-resolve, called exactly once per Tick before ReadIntake. Tick() itself gains no branching beyond calling it and using its returned ref string.
- **Rationale**: Kent Beck's simplest-design priority order, applied: Tick must still read as 'figure out what ref to use, then read intake at it' — one function per decision keeps Tick's own cyclomatic complexity unaffected by this feature. All three of AC1 (fetch once), AC5 (fetch failure logs and continues), and AC6 (no-remote / --no-fetch is silent and behavior-identical) are decided in one place, so a future reader finds the complete fetch-or-not policy in one function rather than scattered across Tick.
- **Trade-offs**:
  - fetchIntakeRef itself will carry most of this feature's new branching (no-fetch check, remote-membership check, fetch-error handling, ref-resolution fallback) — budgeted below 7 cyclomatic complexity per analysis.md's own task breakdown; if the fallback-to-bare-branch-on-first-ever-fetch path pushes it over, extract that one branch into a second named helper rather than inlining it.
- **Fitness Function**: fetchIntakeRef and every other new or changed function stays below cyclomatic complexity 7 — verified by golangci-lint (gocyclo/gocognit), already the project's CI lint step — no new tool needed
### claimWork, claim, and claimFor change signature from a bare commit string to the whole Intake struct (Commit + Ref). This is Fowler's Introduce Parameter Object, applied to data that already always travels together.
- **Rationale**: Commit and Ref are never independently meaningful to a claim — a claim needs 'which commit' (SourceCommit) and 'which ref to branch the worktree from' (BaseBranch default) from the same read, and today only Commit makes that trip. Passing the struct instead of peeling one field off it at the call site is Preserve Whole Object (design-principles.md #2.10), not a new abstraction — Intake already exists and already holds exactly this pair.
- **Trade-offs**:
  - claimFor's signature becomes slightly wider for callers outside this file (there are none today) — acceptable since Intake is already the public return type of ReadIntake, so no new exported surface is introduced.
- **Fitness Function**: claimFor defaults an order's base-branch to committed.Ref, not to the bare config.IntakeBranch, when the order sets no base-branch of its own — verified by go test ./internal/factory/... — new test asserts a Claim's BaseBranch equals '<remote>/<branch>' (not the bare branch) when fetch succeeds and the order omits base-branch

## Component Placement

- `Git.Fetch(remote, branch string) error` → Adapters layer, `internal/worktree` — Implements the consumer-defined Repository.Fetch; follows Push's existing network-call pattern in the same file (manage.go).
- `Git.Remotes() ([]string, error)` → Adapters layer, `internal/worktree` — Implements the consumer-defined Repository.Remotes; follows hasBranch's existing local-read pattern in the same file.
- `Repository.Fetch / Repository.Remotes (interface methods)` → Use Cases layer, `internal/factory` — Repository is defined in internal/factory/intake.go today; per architecture-guardrails #1, the use-case layer owns the interface, the adapter layer implements it. No new file — extends the existing interface.
- `Intake.Ref (new field)` → Use Cases layer, `internal/factory` — Intake is the use-case-layer read model (ADR-010's 'committed intake' concept); Ref travels alongside the existing Commit field on the same struct, set by ReadIntake.
- `Supervisor.fetchIntakeRef() (string, error)` → Use Cases layer, `internal/factory` — Encodes the fetch-vs-local-branch policy (AC1, AC5, AC6) that belongs to the supervisor's own orchestration logic, not to the git adapter.
- `Config.Remote, Config.NoFetch, DefaultRemote` → Use Cases layer, `internal/factory` — Supervisor configuration already lives in internal/factory/supervisor.go alongside IntakeBranch; these are new fields on the same Config struct, not a new concept.
- `--remote, --no-fetch CLI flags` → Frameworks & Drivers layer, `cmd/loom/cmd/factory.go` — CLI flag parsing is the thin outermost glue layer; it only threads values into factory.Config, exactly as every existing factoryFlags field already does.

## Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: internal/worktree (git adapter gains Fetch/Remotes, consumed through the existing Repository interface boundary), cmd/loom/cmd (CLI flag surface gains --remote/--no-fetch on an existing command), templates/claude-feature-team/intake/README.md (operator-facing documentation of the enforced-at-pickup contract)

## Stability Design

- **Fail Fast + graceful continuation (not Circuit Breaker)** on Supervisor.fetchIntakeRef's call to Repository.Fetch — A Circuit Breaker is for a call made repeatedly in a tight loop where tripping prevents cascading load; this fetch happens once per ~30s pass, so a breaker adds no protection a single try does not already give. On failure, fetchIntakeRef logs one line and falls back to the last-fetched ref — this is Nygard's 'fail fast, stay useful' applied at the granularity ADR-010 already chose (the pass goes on; no order is parked or failed).
- **Explicit timeout** on Git.Fetch's network call — Architecture-guardrails #5: every network call must have an explicit timeout. fetchTimeout = 5*time.Minute, matching pushTimeout's existing value and rationale exactly — no new timeout policy to invent.

## Observability Design

- One log line on fetch failure (via existing s.say(...)) emitted from internal/factory/supervisor.go — Supervisor.fetchIntakeRef, which is use-case-layer orchestration, not an adapter interceptor (cardinality: Low — one line per failed pass, naming remote, branch, and the wrapped git stderr; no new metric or span, consistent with this codebase's existing s.say-based logging rather than OTel spans in this layer)

## Layer Boundary Checks

- [PASS] internal/factory (use-case layer) must not import internal/worktree (adapter layer) — Confirmed by reading internal/factory/intake.go: Repository is an interface with no internal/worktree import. The repo graph (mcp__loom__dependencies_of) was not available in this session — verified by direct Read of intake.go and grep for import lines instead, and that fallback is noted here per the architect process's own instruction.
- [PASS] internal/worktree (adapter) implements internal/factory.Repository without internal/factory importing it back — internal/worktree/manage.go's Git type already implements all five existing Repository methods with no import of internal/factory; Fetch and Remotes follow the same file, same pattern, same direction.

## Anti-Pattern Check

- Leaky abstraction: not found — Checked specifically because this was the risk flagged in analysis.md: putting the no-remote-vs-fetch-failed distinction inside Fetch's own error handling would force the caller to parse git stderr. Avoided by giving Remotes its own local-only method instead — the interface stays honest about what each call can fail at.
- God object: not found — Supervisor already orchestrates claim/tick/ledger logic; adding one more orchestration method (fetchIntakeRef) at ~15-20 lines does not change its shape or its single reason to change (order-lifecycle policy).
- Shotgun surgery: not found — The change touches 7 files, but each touch is the same concern (threading a ref through the existing read-then-claim path) seen from its own layer — not independent changes rippling from an unrelated cause.

## Fitness Functions

- **git fetch runs at most once per Tick, regardless of how many orders are claimed or continued in that pass**
  - **Verification**: go test ./internal/factory/... — new test counts calls to a fake Repository.Fetch across one Tick with several claimable and continuing orders
  - **Owner**: developer
- **A pass never pulls, checks out, resets, or otherwise mutates the person's own checkout's branch, index, or working tree**
  - **Verification**: go test ./internal/factory/... and ./internal/worktree/... — new tests assert git status --porcelain and git rev-parse HEAD are unchanged across a Tick that fetches
  - **Owner**: developer
- **Cyclomatic complexity stays below 7 on every new or changed function (Supervisor.fetchIntakeRef, Git.Fetch, Git.Remotes, Supervisor.claimFor, etc.)**
  - **Verification**: golangci-lint (gocyclo/gocognit), the project's existing lint step, run in CI
  - **Owner**: devops-engineer
- **internal/factory has zero production import of internal/worktree**
  - **Verification**: go list -deps ./internal/factory (non-test files) excludes internal/worktree — new, cheap check worth adding to CI alongside the existing lint step since this boundary has no automated guard today
  - **Owner**: devops-engineer
- **No file outside scope.paths changes for this delivery**
  - **Verification**: git diff --name-only against the claim's scope.paths at PR review; not a CI gate today
  - **Owner**: architect
- **Changed lines are >=85% covered by a test that can fail**
  - **Verification**: go test -cover ./internal/factory/... ./internal/worktree/..., plus a manual check that each new assertion fails against the pre-change code (ADR-008)
  - **Owner**: qa-engineer

## Refactoring Opportunities

- Introduce Parameter Object on `claimWork(candidate, commit string) / claim(candidate, commit string) / claimFor(candidate, commit string) in internal/factory/start.go` — Commit and the newly-needed Ref always travel together from ReadIntake's output; passing the whole Intake replaces three call sites that would otherwise each need two parameters threaded through in lockstep (data clump, design-principles.md #2.7).

## Developer Handoff Notes

- No RFC accompanies this feature — confirmed against ADR-010 that fetch-before-read is implementation detail under the already-accepted Decision 2B. Do not open a new ADR.
- Keep Remotes() free of any context.WithTimeout or network I/O — it exists solely to distinguish 'no remote configured' (silent fallback, AC6) from 'remote exists but fetch failed' (logged line, AC5). Folding that distinction into Fetch's own error path was considered and rejected as a leaky abstraction.
- fetchIntakeRef is the one place all three of AC1/AC5/AC6's branching lives — keep Tick() itself limited to calling it and passing the result to ReadIntake, so Tick's own complexity doesn't grow.
- claimWork/claim/claimFor's signature change (commit string -> committed Intake) is mechanical — update the one call site in pendingWork (start.go ~line 64) and the one in supervisor.go/wherever claim is invoked for a continuing order; grep for `.Commit)` after the edit to confirm nothing still passes a bare string.
- fetchTimeout belongs beside pushTimeout in manage.go (both 5*time.Minute) — don't invent a different value without a stated reason; none is given in analysis.md or the spec.
- The CI import-boundary check (internal/factory must not import internal/worktree) does not exist yet. It's listed as a fitness function owned by devops-engineer — flag it in your implementation-notes.md so it isn't silently dropped between stages.

## Open Architectural Questions

- Carried from analysis.md, unresolved: should `loom factory status` and `loom factory inbox` also fetch? This architecture assumes no — both keep reading whatever <remote>/<branch> (or local branch) the last Tick resolved, consistent with `loom factory rerun`'s own unchanged intake read. Confirm before merge.
