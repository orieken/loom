---
feature: "factory-fetch-each-pass"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: []
files_touched: ["internal/worktree/manage.go", "internal/factory/intake.go", "internal/factory/supervisor.go", "internal/factory/start.go", "cmd/loom/cmd/factory.go", "internal/worktree/manage_test.go", "internal/factory/supervisor_test.go", "internal/factory/fixture_test.go", "templates/claude-feature-team/intake/README.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Feature Analysis: factory-fetch-each-pass

## Summary

The factory supervisor's Tick() reads intake/ off the local base branch with no fetch, so a GitHub-side merge (a promoted order, a dependency shipping) is invisible until a person pulls — in the pilot this held up a merged order for a day. This change adds one `git fetch <remote> <branch>` at the start of every pass (ADR-010, 2B stays the decision; fetch-before-read is an implementation detail under it, not a new structural decision), then reads intake/ at the remote-tracking ref `<remote>/<branch>` instead of the bare local branch. `--remote` defaults to `origin`; `--no-fetch` or a repository with no remote by that name reads the local branch exactly as today, with zero behavior change for every existing test fixture (none of which configure a remote). A fetch that fails for a remote that does exist (network, credentials) logs one line and the pass goes on from the last-fetched ref; it never parks or fails an order, and it never pulls, checks out, or resets the person's own working tree, index, or branch.

### Acceptance Criteria

- Given a pass, when it starts, the supervisor MUST run `git fetch <remote> <branch>` exactly once and read intake/ at `<remote>/<branch>`. `--remote` defaults to `origin`.
  - Example: `loom factory run --once` with no `--remote` fetches `origin main` once before reading intake/.
- Given an order promoted to `ready` on the remote only, the next pass MUST claim it with no pull by a person, and the claim's `sourceCommit` MUST equal the fetched commit.
- Given a claim, its worktree's branch MUST start from `<remote>/<branch>` at that commit when the order sets no `base-branch` of its own.
- Given the person's checkout, a pass MUST NOT change its branch, index, or working tree, including uncommitted changes and local unpushed commits.
- Given a fetch that fails (no network, no remote by that name, bad credentials), the pass MUST go on from the last fetched `<remote>/<branch>` and print exactly one line saying the fetch failed and why. It MUST NOT park or fail any order.
- Given a repository with no remote by the configured name, or `--no-fetch`, the supervisor MUST read the local `<branch>` exactly as today, with no fetch attempted and no new output. Every existing test MUST keep passing with no change to what it asserts.
- Given a running or waiting order, a fetch that moves `<remote>/<branch>` MUST NOT change it. The order MUST keep its claimed `sourceCommit`; only `loom factory rerun` takes a newer version.
- Given `templates/claude-feature-team/intake/README.md`, it MUST say orders are read from the remote after a fetch, that a promotion must be pushed, and how `--no-fetch` restores the old behavior.
- Given the change is complete, every new or changed function MUST have cyclomatic complexity below 7, every changed line MUST be covered by a test that can fail, `go test ./...` and the lint MUST pass, and no file outside `scope.paths` MUST change, excepting this run's own delivery record under `docs/features/factory-fetch-each-pass/`.

### Non-Functional Requirements

- **performance**: A fetch adds at most one network call per pass. At the default 30 s interval that is two calls a minute. (network calls added per pass 1call)
- **security**: The fetch uses the repository's already-configured remote and the person's own git credentials (via the ambient git credential helper); it sends nothing new and introduces no secret or token handling.

## Proposed Fitness Functions

- **`git fetch` runs at most once per Tick, regardless of how many orders are claimed or continued in that pass**
  - **Verification**: go test ./internal/factory/... — a new test counts calls to a fake Repository.Fetch across one Tick with several claimable and continuing orders
  - **Owner**: developer
- **A pass never pulls, checks out, resets, or otherwise mutates the person's own checkout's branch, index, or working tree**
  - **Verification**: go test ./internal/factory/... and ./internal/worktree/... — new tests assert `git status --porcelain` and `git rev-parse HEAD` are unchanged across a Tick that fetches
  - **Owner**: developer
- **Cyclomatic complexity stays below 7 on every new or changed function (Supervisor.fetchIntakeRef, Git.Fetch, Git.Remotes, Supervisor.claimFor, etc.)**
  - **Verification**: golangci-lint (gocyclo/gocognit), the project's existing lint step, run in CI
  - **Owner**: devops-engineer
- **No file outside `scope.paths` changes for this delivery**
  - **Verification**: git diff --name-only against the claim's scope.paths at PR review; not a CI gate today
  - **Owner**: architect
- **Changed lines are >=85% covered by a test that can fail**
  - **Verification**: go test -cover ./internal/factory/... ./internal/worktree/..., plus a manual check that each new assertion fails against the pre-change code (ADR-008)
  - **Owner**: qa-engineer

## Out of Scope

- Pulling, merging, or rebasing the person's own checkout.
- Fetching more often than once per pass.
- `loom factory status` or `loom factory inbox` fetching — both keep reading the ledger and whatever `<remote>/<branch>` (or local branch) the last `run` pass left in place.
- `loom factory rerun`'s own intake read (`readCommittedIntake` in cmd/loom/cmd/factory.go) fetching — it keeps reading the bare configured branch exactly as today; only the supervisor's own Tick() fetches.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: internal/worktree (git adapter: Repository implementation gains Fetch and Remotes), cmd/loom/cmd (CLI flag surface: new --remote and --no-fetch flags on `loom factory run`), templates/claude-feature-team/intake/README.md (operator-facing documentation of the enforced-at-pickup contract)

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: Order
- **Read Models / Projections**: Intake

### Affected Components

- `internal/worktree/manage.go` — Add Fetch(remote, branch string) error, following Push's existing pattern (context timeout, GIT_TERMINAL_PROMPT=0, wrapped stderr); add Remotes() ([]string, error) via `git remote` (local-only, no network) so the supervisor can tell 'no remote by that name' apart from 'fetch failed reaching one that exists'.
- `internal/factory/intake.go` — Repository interface gains Remotes() ([]string, error) and Fetch(remote, branch string) error (consumer-defined, architecture-guardrails #1); Intake struct gains a Ref string field (the ref intake/ was actually read at) alongside the existing Commit field; ReadIntake sets Ref: branch.
- `internal/factory/supervisor.go` — Config gains Remote string and NoFetch bool; add DefaultRemote = "origin"; New() defaults an empty Remote to DefaultRemote; Tick() calls a new fetchIntakeRef() helper before ReadIntake instead of passing s.config.IntakeBranch directly.
- `internal/factory/start.go` — claimWork, claim, and claimFor change from taking a bare commit string to taking the whole Intake (Commit + Ref travel together — Fowler's Introduce Parameter Object), so claimFor can default an order's base-branch to committed.Ref (the fetched remote-tracking ref) instead of the bare s.config.IntakeBranch, satisfying AC3.
- `cmd/loom/cmd/factory.go` — factoryFlags gains remote string and noFetch bool; register --remote (default factory.DefaultRemote) and --no-fetch on factoryRunCmd; thread both into factory.Config in newFactorySupervisor; extend factoryRunCmd.Long to document the fetch-before-read behavior.
- `internal/worktree/manage_test.go` — New tests for Fetch (updates origin/<branch>; a failure carries git's stderr) and Remotes (empty on a bare repo; lists a name once `git remote add` is run), following the existing bare-upstream-as-remote pattern already in this file.
- `internal/factory/supervisor_test.go` — New Tick-level tests for: fetch runs once per pass and reads the remote-tracking ref; a remote-only promoted order is claimed with no local pull; a claim with no explicit base-branch starts from <remote>/<branch>; a failing fetch continues and prints one line; no-remote and --no-fetch read the local branch with zero behavior change; a running/waiting order's sourceCommit survives a later fetch. Also update the existing call at line ~180 (`claimFor(candidate, committed.Commit)`) to the new `claimFor(candidate, committed)` signature.
- `internal/factory/fixture_test.go` — newTestFactory/newSupervisor build a repo and Config with no remote configured — the exact fixture AC6 must hold against unchanged. May need a small helper for a bare-repo-as-upstream variant so the new fetch tests can push to an upstream the fixture's clone tracks.
- `templates/claude-feature-team/intake/README.md` — AC8: the 'Enforced at pickup, by `loom factory` (ADR-010)' section (lines 91-97) must say orders are read from the remote after a fetch, that a promotion must be pushed, and how --no-fetch restores the old local-branch-only behavior.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — No amendment needed — fetch-before-read is an implementation detail under Decision 2B ('read intake/ committed on base-branch, never the working tree'), not a new structural decision. Confirmed by reading the ADR; flagged here only so the developer doesn't open a new ADR unnecessarily.

### Data Model Changes

- Claim.BaseBranch's default value (set in claimFor when an order names no base-branch of its own) changes from the bare intake branch to the fetched remote-tracking ref (e.g. origin/main instead of main). The field's shape and persistence in the ledger are unchanged — only its default content differs, and only for orders that omit base-branch. (**none** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. internal/worktree/manage.go: add `const fetchTimeout = 5 * time.Minute` next to pushTimeout, and `func (g *Git) Fetch(remote, branch string) error` using context.WithTimeout(fetchTimeout), GIT_TERMINAL_PROMPT=0, and an error wrapped with the command's combined stderr output, mirroring Push exactly.
2. internal/worktree/manage.go: add `func (g *Git) Remotes() ([]string, error)` running `git remote` (no timeout — local read, like hasBranch), splitting non-empty lines into a []string.
3. internal/factory/intake.go: add `Remotes() ([]string, error)` and `Fetch(remote, branch string) error` to the Repository interface; add `Ref string` to the Intake struct; in ReadIntake, set `Ref: branch` on the returned Intake.
4. internal/factory/supervisor.go: add `const DefaultRemote = "origin"`; add `Remote string` and `NoFetch bool` to Config; in New(), default an empty config.Remote to DefaultRemote.
5. internal/factory/supervisor.go: in Tick(), replace the direct `ReadIntake(s.deps.Repository, s.config.IntakeBranch, s.config.Plans)` call with `ref, err := s.fetchIntakeRef(); ...; ReadIntake(s.deps.Repository, ref, s.config.Plans)`.
6. internal/factory/supervisor.go: add `func (s *Supervisor) fetchIntakeRef() (string, error)`: return `s.config.IntakeBranch` unchanged when `s.config.NoFetch`; otherwise call `s.deps.Repository.Remotes()`, return the bare branch unchanged when `s.config.Remote` is not among them (AC6); otherwise call `s.deps.Repository.Fetch(s.config.Remote, s.config.IntakeBranch)`, logging one `s.say(...)` line naming the remote, branch, and error on failure (AC5) without returning an error; then return `s.config.Remote + "/" + s.config.IntakeBranch` if that ref resolves via `s.deps.Repository.ResolveCommit`, else fall back to the bare branch (covers a first-ever fetch failing before any remote-tracking ref exists).
7. internal/factory/supervisor.go: add a small `hasRemote(remotes []string, name string) bool` helper for the membership check above.
8. internal/factory/start.go: change `claimWork(candidate Candidate, committed Intake) work` (was `committed.Commit string`); update its one call site in pendingWork to `s.claimWork(candidate, committed)`.
9. internal/factory/start.go: change `claim(candidate Candidate, committed Intake) error` and `claimFor(candidate Candidate, committed Intake) Claim` (both were `commit string`); in claimFor, default `base` to `committed.Ref` (was `s.config.IntakeBranch`) when `order.BaseBranch == ""`, and set `SourceCommit: committed.Commit` as before.
10. cmd/loom/cmd/factory.go: add `remote string` and `noFetch bool` to factoryFlags; register `--remote` (default `factory.DefaultRemote`, flag help: "remote the base branch is fetched from before each pass") and `--no-fetch` (default false, flag help: "read intake/ from the local branch only; never fetch") on factoryRunCmd.Flags(); thread `Remote: factoryArgs.remote, NoFetch: factoryArgs.noFetch` into the factory.Config built in newFactorySupervisor.
11. cmd/loom/cmd/factory.go: extend factoryRunCmd.Long to state that intake/ is read from `<remote>/<branch>` after one fetch per pass, and that `--no-fetch` restores reading the local branch only.

### QA Tasks

1. internal/worktree/manage_test.go: add TestFetchUpdatesTheRemoteTrackingRef (push a second commit to a bare upstream, Fetch, then ResolveCommit("origin/main") sees it) and TestFetchFailsWithGitsOwnStderr (fetch a remote with no reachable URL, assert the error carries git's message), alongside TestRemotesListsConfiguredNamesAndIsEmptyOnABareLocalRepo.
2. internal/factory/supervisor_test.go (or a new fetch_test.go in package factory): add a bare-upstream-plus-clone fixture variant; push an order to the upstream only, tick, assert it is claimed with Claim.SourceCommit equal to the upstream's commit and the clone's `git status --porcelain` and HEAD are unchanged (AC2, AC4).
3. Add a test where an order omits base-branch entirely: assert the created worktree's branch starts from `origin/main` at the fetched commit, not the bare local `main` (AC3) — existing fixtures hardcode `base-branch: main`, so this needs a new orderSpec variant without one.
4. Add a failing-fetch test (point --remote at an unreachable URL, or remove the upstream after one successful fetch): tick continues normally, claims orders as if from the last good fetch, and prints exactly one line naming the fetch failure (AC5) — assert via outputCount.
5. Add a no-remote test confirming the existing newTestFactory fixtures see zero Repository.Fetch calls and zero new output lines, and every existing assertion in idle_test.go / supervisor_test.go continues to pass unmodified (AC6).
6. Add a --no-fetch test: even with a remote configured and reachable, Fetch is never called and intake is read from the bare local branch.
7. Extend the TestRerunReclaimsTheEditedOrder-style flow: fetch a newer upstream commit while an order is running or waiting, tick, assert its Claim.SourceCommit is unchanged until a human runs `loom factory rerun` (AC7).
8. Update internal/factory/supervisor_test.go's existing call `factory.supervisor.claimFor(committed.Orders["alpha"], committed.Commit)` to the new `claimFor(candidate, committed)` signature.
9. Run `go test ./...` and the project's lint step; confirm no new findings and no coverage regression on changed lines (ADR-008).

### Tech Writer Tasks

1. templates/claude-feature-team/intake/README.md: amend the 'Enforced at pickup, by `loom factory` (ADR-010)' section (around lines 91-97) to say orders are read from the remote after one fetch per pass, that a promotion must be pushed (not just committed locally) to be seen, name `--remote`'s default of `origin`, and describe how `--no-fetch` restores the prior local-branch-only behavior.

### DevOps Tasks

1. No CI/CD pipeline change is required — `go test ./...` and the existing lint step already gate this change (AC9).
2. Infrastructure / Deploy Notes: after this ships, restart both running `loom factory run` supervisors with the new build so they pick up fetch-before-read.

## Edge Cases and Risks

- The person's checkout has uncommitted changes or local commits not yet pushed.: Fetch only updates the remote-tracking ref; it never touches HEAD, the index, or the working tree, so both stay exactly as they were (AC4).
- The repository has no remote named --remote (default origin) at all — e.g. every existing test fixture.: fetchIntakeRef checks Remotes() membership before attempting Fetch; absent, it returns the bare local branch with no fetch call and no new output, byte-for-byte today's behavior (AC6).
- --no-fetch is passed.: fetchIntakeRef short-circuits to the bare local branch before even checking Remotes().
- The named remote exists but the fetch fails (no network, bad credentials, remote renamed server-side).: Fetch's error is logged as one line via s.say(...); the pass continues and reads whatever <remote>/<branch> last resolved to (or the bare branch if that ref has never existed), never parking or failing an order (AC5).
- The remote exists but has never been fetched before (first-ever pass, and that first fetch fails).: ResolveCommit("<remote>/<branch>") also fails since the ref never existed; fetchIntakeRef falls back to the bare local branch rather than erroring the whole Tick.
- An order is running or waiting when a later fetch moves the remote branch forward.: The order's Claim (including SourceCommit and BaseBranch) was fixed at claim time and is never recomputed by continuation()/relaunch(); only RequestRerun re-reads intake and reclaims (AC7).
- An order sets its own base-branch explicitly (every current fixture does: base-branch: main).: claimFor only falls back to committed.Ref when order.BaseBranch is empty; an explicit base-branch is untouched by this change.
- `loom factory status`, `loom factory inbox`, and `loom factory rerun` are invoked between supervisor passes.: Per the spec's resolved Open Question, none of these fetch; they keep reading the ledger and whichever ref (remote-tracking or local) the last `loom factory run` pass left resolved, via the unchanged readCommittedIntake/factoryArgs.branch path.

## Definition of Done

- Every new or changed function (Supervisor.fetchIntakeRef, Supervisor.hasRemote, Git.Fetch, Git.Remotes, start.go's claimFor/claim/claimWork) has cyclomatic complexity below 7.
- Every changed line is covered by a test that fails against the pre-change code and passes after (ADR-008).
- `go test ./...` passes.
- The project's lint step passes with no new findings.
- No file outside `scope.paths` (internal/factory/**, internal/worktree/**, cmd/loom/cmd/factory*.go, templates/claude-feature-team/intake/README.md, docs/features/factory-fetch-each-pass/**) changes.
- `templates/claude-feature-team/intake/README.md` describes the fetch-before-read behavior, that a promotion must be pushed, and what `--no-fetch` restores.
- A reviewer can push a promoted order to the remote only, run `loom factory run --once` with no local pull, and see it claimed with the fetched commit as its sourceCommit.
