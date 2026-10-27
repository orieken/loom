---
feature: "pre-ship-checks"
bounded_context: "Agent Orchestration"
domain_terms: ["Pre-ship check"]
files_touched: ["internal/policy/config.go", "internal/policy/preship.go", "internal/policy/preship_test.go", "internal/policy/switch.go", "internal/orchestrator/plan.go", "internal/orchestrator/router.go", "internal/orchestrator/preship.go", "internal/orchestrator/preship_test.go", "internal/orchestrator/provider.go", "internal/orchestrator/failure.go", "internal/orchestrator/executor.go", "internal/orchestrator/plan_test.go", "internal/orchestrator/testdata/shape-deliver-feature.json", "cmd/loom/cmd/run_policies.go", "cmd/loom/cmd/run.go", "cmd/loom/cmd/run_test.go", "scripts/pre-ship-check.sh", "docs/MIGRATION.md", "docs/ARCHITECTURE.md", "docs/features/pre-ship-checks/"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Feature Analysis: pre-ship-checks

## Summary

A project declares the commands that must pass before a human is asked to ship, as a `preShipChecks` list (name, command, optional timeout) in `.claude/delivery-policy.yaml`. `loom run` adds a second executor-internal stage, `pre-ship-checks`, between `tech-writer` and `devops-engineer` in the built-in `deliver-feature` plan — after the last reviewing/documenting stage and before the stage carrying the `confirm-ship` gate. The stage runs each configured command in order through `bash -c` at the project root with `LOOM_BASE_COMMIT` set to the run's `StartCommit`, under an explicit per-check timeout (default 10 minutes). A failing or timed-out check is a terminal stage failure — named, with its full output kept in the stage's artifact, no supervisor retry, and the run never reaches `confirm-ship`; checks after it never run. With no `preShipChecks` configured, the stage records itself SKIPPED with the reason "no preShipChecks configured" rather than COMPLETED. `loom`'s own repository gets `scripts/pre-ship-check.sh`, a shell script that reproduces CI's changed-line coverage gate (`go test ./... -coverprofile=...` then `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile ...`) and refuses to run with an empty `LOOM_BASE_COMMIT`. This closes the gap PR #18 exposed: every agent's tests passed and the PR was approved, but CI's own coverage gate still rejected it after `confirm-ship`, because nothing in the pipeline had run the check CI runs.

### Acceptance Criteria

- Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list where each entry has a `name`, a `command`, and an optional `timeout`, when the policy config loads, then `policy.LoadConfig` decodes every entry. It MUST reject an entry with no `name`, no `command`, or an unknown key with `ErrInvalidConfig` naming the entry and the offending key.
  - Example: `preShipChecks: [{name: coverage, command: scripts/pre-ship-check.sh}]` decodes to one PreShipCheck with the default 10-minute timeout.
  - Example: `preShipChecks: [{command: scripts/x.sh}]` (no name) is rejected naming the entry by its command since it has no name to use.
  - Example: `preShipChecks: [{name: coverage, command: scripts/x.sh, retries: 3}]` is rejected naming entry "coverage" and the unknown key `retries`.
- Given the built-in `deliver-feature` plan, then it MUST contain an executor-internal stage with ID `pre-ship-checks` placed after `tech-writer` and before `devops-engineer` (the stage carrying the `confirm-ship` gate), so the stage runs before that gate can halt the run.
- Given one or more configured `preShipChecks`, when the stage runs, then it MUST run each command in order through `bash -c`, with the working directory set to the project root, the environment variable `LOOM_BASE_COMMIT` set to the run's `StartCommit`, and an explicit timeout per check (the entry's own, or 10 minutes when the entry sets none).
- Given every configured check exits 0, then the stage MUST complete, and its artifact MUST record each check's name, exit status, duration, and the last 50 lines of its output.
- Given a check exits non-zero or times out, then the stage MUST fail, name the check, and keep that check's full output in the stage's artifact. The failure MUST be terminal (no supervisor retry), the run MUST NOT reach `confirm-ship`, and checks listed after the failing one MUST NOT run.
- Given no `preShipChecks` are configured, then the stage MUST be recorded SKIPPED with the reason "no preShipChecks configured", and MUST NOT report COMPLETED for checks that never ran.
- Given the change is complete, then `scripts/pre-ship-check.sh` MUST exist, run `go test ./... -coverprofile=<a temporary file>` followed by `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile <that file>`, exit non-zero when either step fails, and refuse to run when `LOOM_BASE_COMMIT` is empty.
- Given the change is complete, then `go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/...` MUST pass, every new function MUST have cyclomatic complexity below 7, every changed line MUST be covered as `cmd/diff-coverage` measures it, and no file outside `scope.paths` MUST change (the run's own delivery record under `docs/features/pre-ship-checks/` excepted).

### Non-Functional Requirements

- **other**: Every pre-ship check command runs under an explicit timeout (architecture-guardrails.md #5, no custom retry/sleep loop); an entry that sets none falls back to a documented default rather than running unbounded. (default per-check timeout when an entry sets none 10minutes)
- **security**: A check's command output may contain secrets from the project's own environment. The stage's on-disk artifact keeps that output so a human can diagnose a failure, but anything the executor emits to telemetry (spans, traces) MUST record only the check's name, exit status and duration, never the output text (architecture-guardrails.md #9).
- **security**: Pre-ship check commands come only from the project's committed `.claude/delivery-policy.yaml`, the same trust level as the project's own CI configuration, and are never read from a work order or any other source that could be external (shared/rules/memory-trust-boundary.md's provenance discipline, applied here even though delivery-policy.yaml is not a KI).

## Proposed Fitness Functions

- **Every `preShipChecks` entry decodes with a non-empty `name` and `command`, and an unknown key or a missing field is rejected naming the entry.**
  - **Verification**: go test ./internal/policy/... -run TestLoadConfig (new preShipChecks decode/rejection cases)
  - **Owner**: DevOps
- **The built-in plan's `pre-ship-checks` stage sits after `tech-writer` and before `devops-engineer`, is Internal, carries no Gate of its own, and is absent from the router's skippable-stage map.**
  - **Verification**: go test ./internal/orchestrator/... -run TestDefaultDeliverFeaturePlan (new stage-order assertion) plus the regenerated testdata/shape-deliver-feature.json baseline (TestShapeOfTheBuiltInPlanIsStable)
  - **Owner**: Architect
- **A failing or timed-out check halts the run before `confirm-ship`, is classified terminal (no supervisor retry), and checks after it never run.**
  - **Verification**: go test ./internal/orchestrator/... -run TestPreShipChecks (stub commands: true, false, a sleep past a short per-check timeout)
  - **Owner**: DevOps
- **`scripts/pre-ship-check.sh` reproduces CI's own changed-line coverage gate and refuses to run with an empty `LOOM_BASE_COMMIT`.**
  - **Verification**: manual: `LOOM_BASE_COMMIT= scripts/pre-ship-check.sh` exits non-zero; `LOOM_BASE_COMMIT=<a real commit> scripts/pre-ship-check.sh` run against this repo matches what CI's diff-coverage step reports
  - **Owner**: DevOps

## Out of Scope

- Writing loom's own `.claude/delivery-policy.yaml` entry — agents cannot write under `.claude/`; a human adds `preShipChecks: [{name: changed-line coverage, command: scripts/pre-ship-check.sh}]` after this merges.
- Feeding a failed check back to the developer for another attempt — that is roadmap L2.18's bounded contract retry.
- Adding `preShipChecks` to `deliver-bugfix`, which has no `confirm-ship` gate.
- Any change to CI workflows (`.github/workflows/**`) — that is approval gate #7.
- Updating `shared/DOMAIN_DICTIONARY.md` with the new "Pre-ship check" term — `shared/` sits outside this feature's `scope.paths`, and the spec's own Definition of Done forbids changing any file outside it. The term is recorded below under `retrieval.domainTerms` instead; a human or a later feature can promote it to the dictionary.
- Updating `shared/policies/README.md` or `shared/templates/delivery-policy.template.yaml` — both are outside `scope.paths`; `docs/MIGRATION.md` (inside `docs/**`) is the one place tech-writer documents the new key.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Agent Orchestration
- **Context Crossings**: internal/policy (delivery-policy.yaml schema, PreShipCheck decode) -> internal/orchestrator (stage execution, failure/artifact recording), cmd/loom/cmd (CLI wiring: loadPolicies, executeRun) -> internal/policy and internal/orchestrator

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `internal/policy/config.go` — Add `PreShipChecks []PreShipCheck` to `Config`, add `"preShipChecks"` to `keptKeys`, and call the new strict decode step from `LoadConfig` after `checkValues`, wrapping any failure in `ErrInvalidConfig`.
- `internal/policy/preship.go` — New file: the `PreShipCheck{Name, Command string; Timeout time.Duration}` type, `defaultPreShipCheckTimeout = 10 * time.Minute`, and a strict per-entry YAML-node decoder (mirroring decode.go's `decodeCondition` walk) that rejects an unknown key or a missing name/command, naming the entry by its name, else its command, else its position.
- `internal/policy/preship_test.go` — New file: decode success (name/command/timeout, default timeout when omitted), and every rejection case — no name, no command, unknown key — each asserting the entry is named in the error, plus the absent-key case decoding to an empty slice.
- `internal/policy/switch.go` — No functional change expected — `Enabled()`'s tolerant `decodeConfig` call is unaffected since `PreShipChecks` decodes separately in `LoadConfig`. Re-run `go test ./internal/policy/...` to confirm the kill switch stays orthogonal to this feature.
- `internal/orchestrator/plan.go` — Add `const PreShipChecksStageID = "pre-ship-checks"`; insert it into `DefaultDeliverFeaturePlan`'s `agents` slice between `"tech-writer"` and `"devops-engineer"`; extend the `Internal:` expression (currently `id == RouterStageID`) to also mark `PreShipChecksStageID` internal. Leave it out of `defaultSkippableStages` so it always reaches the internal dispatch (its own "nothing configured" self-skip is independent of the router).
- `internal/orchestrator/router.go` — Generalize `runInternalStage` from its current switch-of-one (router only) into a two-case dispatch keyed on `stage.ID`, per the function's own comment that a second internal stage should make this a registry. Widen its signature to accept `ctx context.Context` and `*RunState` so the new case can read `state.StartCommit` and respect cancellation.
- `internal/orchestrator/preship.go` — New file: `runPreShipChecks` (loops `e.preShipChecks`, running each via `exec.CommandContext` with `bash -c <command>`, `Dir: input.ProjectRoot`, `Env` including `LOOM_BASE_COMMIT=<StartCommit>`, and a per-check `context.WithTimeout`); renders a markdown report (name, exit status, duration, last 50 lines on pass, full output on the failing check) to `<WorkspaceDir>/pre-ship-checks.md`; returns `StageOutput{Skipped:true, SkipReason:"no preShipChecks configured"}` when `e.preShipChecks` is empty, and returns the written artifact path alongside a non-nil error on the first failing or timed-out check.
- `internal/orchestrator/preship_test.go` — New file: run the internal stage via the mock-provider harness (newHarness pattern from executor_test.go) with stub commands `true`, `false`, a `sleep` that outlives a short configured timeout, and `echo $LOOM_BASE_COMMIT` — asserting COMPLETED+artifact content on all-pass, FAILED+terminal+preserved full output+later checks never run on a non-zero exit or timeout, and SKIPPED with the exact reason string when no checks are configured.
- `internal/orchestrator/provider.go` — Add `Skipped bool` and `SkipReason string` to `StageOutput`, so an internal stage can report "nothing to do" without going through the router's route-based skip (which is keyed to the analyst's typed state, not to delivery-policy.yaml).
- `internal/orchestrator/failure.go` — Add an `artifactPath string` field to `stageFailure` so a failing check's full-output markdown file is still recorded as the stage's artifact (`record.ArtifactPath`/`ArtifactSHA256`) even though the stage ends FAILED, not COMPLETED. `invocationFailureClass` already returns `FailureTerminal` for `stage.Internal` stages, so no change needed there for the "no supervisor retry" requirement.
- `internal/orchestrator/executor.go` — Add `preShipChecks []policy.PreShipCheck` field and `WithPreShipChecks([]policy.PreShipCheck) *Executor`. Widen `runOrInvoke`'s and `executeStage`'s calls to `runInternalStage` to pass `ctx` and `state` through. Add a `persistSkippedInternal` path inside `persistCompletion` for `output.Skipped`, writing `StageStatusSkipped` + `SkipReason` and emitting `EventStageSkipped` instead of completing normally. Thread `output.ArtifactPath` into the `stageFailure.artifactPath` built in `executeStage`'s failure branch.
- `internal/orchestrator/plan_test.go` — Add (or extend an existing plan test) asserting `pre-ship-checks` is present, positioned between `tech-writer` and `devops-engineer`, `Internal: true`, `Gate: ""`, and `Skippable: false` in `DefaultDeliverFeaturePlan()`.
- `internal/orchestrator/testdata/shape-deliver-feature.json` — Regenerate via `go test ./internal/orchestrator -run TestShapeOfTheBuiltInPlanIsStable -update` once the stage exists — it will add one new step, `{"id": "pre-ship-checks", "iteration": 1, "status": "SKIPPED"}`, right after the `tech-writer` step, because the shape test's mock harness never calls `WithPreShipChecks`. Put the diff and the one-sentence reason in the commit per that test's own discipline.
- `cmd/loom/cmd/run_policies.go` — `loadPolicies` already calls `policy.LoadConfig(policy.ConfigFile)` and discards everything but the kill switch and `[]policy.Policy`. Change its return signature to `([]policy.Policy, []policy.PreShipCheck, error)` and return `config.PreShipChecks` on every path, including the `policiesEnabled: false` early return — `preShipChecks` is orthogonal to that kill switch.
- `cmd/loom/cmd/run.go` — Update `runRun`'s call site for `loadPolicies`'s new return value; add `preShipChecks []policy.PreShipCheck` to the `runSetup` struct; in `executeRun`, call `executor.WithPreShipChecks(setup.preShipChecks)` alongside the existing `executor.WithPolicies(setup.policies)` line.
- `cmd/loom/cmd/run_test.go` — No code change expected: `assertEveryStageSettled` and `stageArtifactPath` already iterate `DefaultDeliverFeaturePlan().Stages` generically via `IsStageSettled`/`StateKind`, and `stagesRoutedOut` is a non-exhaustive allow-list, so the new stage (self-skipped, since the test's temp project has no `.claude/delivery-policy.yaml`) needs no new assertion. Re-run `TestRunApproveEachGateToCompletion` and `TestRunMockProviderHaltsAtFirstGate` to confirm.
- `scripts/pre-ship-check.sh` — New file, in this repo's own house style (see scripts/ci-diff-base.sh): `set -euo pipefail`; refuse with a clear message when `$LOOM_BASE_COMMIT` is empty; `go test ./... -coverprofile=$(mktemp)`; then `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile <that file>`, propagating either command's exit code.
- `docs/MIGRATION.md` — Add a short, additive note near the existing "Removed: auto-proceed keys in delivery-policy.yaml (2026-10-24, ADR-010)" section (lines 163-185) documenting the new, non-breaking `preShipChecks` key: shape (`name`, `command`, optional `timeout`), default 10-minute timeout, and where it runs in the pipeline.
- `docs/ARCHITECTURE.md` — Extend the one-paragraph `internal/orchestrator/` description in the Directory Reference (around line 250-263, which already mentions "a router that decides which stages a run needs") to also name the second executor-internal stage, `pre-ship-checks`, and what it guards.
- `docs/features/pre-ship-checks/` — This run's own delivery record — the one exception the AC's own Definition of Done makes to "no file outside scope.paths changes".

### Data Model Changes

None

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. internal/policy/preship.go: add the PreShipCheck type, the default 10-minute timeout constant, and a strict per-entry YAML-node decoder that rejects a missing name/command or an unknown key, naming the entry.
2. internal/policy/config.go: add PreShipChecks to Config, add preShipChecks to keptKeys, and wire the new decoder into LoadConfig after checkValues, wrapped in ErrInvalidConfig on failure.
3. internal/orchestrator/plan.go: add PreShipChecksStageID, insert the stage between tech-writer and devops-engineer in DefaultDeliverFeaturePlan's agent list, and mark it Internal alongside the router.
4. internal/orchestrator/provider.go: add Skipped and SkipReason to StageOutput.
5. internal/orchestrator/failure.go: add artifactPath to stageFailure.
6. internal/orchestrator/router.go: widen runInternalStage's signature to accept ctx and *RunState, and dispatch on stage.ID between the router's computeRoute and the new pre-ship-checks routine.
7. internal/orchestrator/preship.go: implement runPreShipChecks and its per-check runner (bash -c, project-root working directory, LOOM_BASE_COMMIT env var, per-check timeout, markdown report render/write), returning the self-skip output when no checks are configured and the failing check's artifact path alongside a non-nil error otherwise.
8. internal/orchestrator/executor.go: thread ctx/state through runOrInvoke and executeStage's call to runInternalStage; add the preShipChecks field and WithPreShipChecks; add the SKIPPED branch inside persistCompletion for output.Skipped; pass output.ArtifactPath into the failure branch's stageFailure.
9. cmd/loom/cmd/run_policies.go: change loadPolicies to also return []policy.PreShipCheck.
10. cmd/loom/cmd/run.go: update the loadPolicies call site, add preShipChecks to runSetup, and call executor.WithPreShipChecks in executeRun.
11. scripts/pre-ship-check.sh: write the coverage-gate script per the spec's exact two-command sequence, refusing an empty LOOM_BASE_COMMIT.
12. Regenerate internal/orchestrator/testdata/shape-deliver-feature.json with go test ./internal/orchestrator -run TestShapeOfTheBuiltInPlanIsStable -update once the stage exists, and state the one-sentence reason in the commit.

### QA Tasks

1. internal/policy: table-driven tests for every preShipChecks decode/rejection case listed in affectedComponents, plus the shipped-template-stays-valid regression (TestTheShippedDeliveryPolicyTemplateIsValid continues to pass unmodified).
2. internal/orchestrator: new preship_test.go exercising all-pass (artifact has name/exit/duration/last-50-lines per check), a failing check (stage FAILED, terminal, full output kept, later checks never run), a timed-out check (sleep past a short configured timeout), the empty-config self-skip (exact reason string), and that LOOM_BASE_COMMIT genuinely reaches the child process (echo $LOOM_BASE_COMMIT).
3. Add or extend a plan-shape test asserting pre-ship-checks' exact position and Internal/Gate/Skippable values in DefaultDeliverFeaturePlan.
4. Run go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/... (the AC's own done-when) and also go test ./internal/planfile/... (unrestricted by this feature but exercised by it, since planfile.Parse round-trips the built-in plan dynamically).
5. Manually exercise scripts/pre-ship-check.sh three ways: LOOM_BASE_COMMIT unset (expect refusal), LOOM_BASE_COMMIT set to a real ancestor commit in this repo (expect it to reproduce CI's coverage gate), and a deliberately broken coverage profile path (expect a non-zero exit).

### Tech Writer Tasks

1. docs/MIGRATION.md: add a short additive note near the ADR-010 "Removed: auto-proceed keys" section documenting the new preShipChecks key, its shape, and its default timeout.
2. docs/ARCHITECTURE.md: extend the internal/orchestrator/ paragraph in the Directory Reference to name the second executor-internal stage.

### DevOps Tasks

None

## Edge Cases and Risks

- A preShipChecks entry omits timeout.: Decode to the 10-minute default rather than an unbounded run.
- A preShipChecks entry has no name.: Reject with ErrInvalidConfig, naming the entry by its command (or its position if command is also absent) since there is no name to use.
- A preShipChecks entry has no command.: Reject with ErrInvalidConfig, naming the entry by its name.
- A preShipChecks entry sets an unrecognized key (e.g. retries).: Reject with ErrInvalidConfig naming both the entry and the unknown key, matching the top-level keptKeys/removedKeys discipline Config already has.
- No preShipChecks key is present at all.: Config.PreShipChecks decodes to an empty slice; the stage records itself SKIPPED with reason "no preShipChecks configured", never COMPLETED.
- The first check passes and the second fails.: The stage's artifact records the first check with its last 50 lines, the second with its full output and non-zero exit; the stage fails naming the second check; any checks after it never run.
- A check hangs past its configured or default timeout.: The per-check context deadline kills the process; the stage fails naming the check as timed out, with the classification terminal (no supervisor retry).
- LOOM_BASE_COMMIT would be empty (e.g. no commits, no work tree) when the stage runs.: The executor always sets the environment variable (to the run's StartCommit, which may itself be empty per executor.go's startCommit() comment on an absent work tree); scripts/pre-ship-check.sh carries its own independent guard and refuses to run when it receives an empty value, regardless of what the executor passed.
- delivery-policy.yaml sets policiesEnabled: false.: preShipChecks still run — that kill switch only gates .policy.yaml gate-automation evaluation (ADR-010), not this feature, which is read from the same Config but is orthogonal to PoliciesEvaluated().
- pre-ship-checks fails on a resumed run.: Same terminal-failure semantics as any other FAILED stage: a human fixes the underlying check or code and resumes; this is not loom's own bounded contract-retry loop (that is explicitly out of scope, roadmap L2.18).

## Definition of Done

- go build ./... succeeds.
- go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/... passes (the AC's own done-when), and go test ./internal/planfile/... still passes unmodified.
- Every new function has cyclomatic complexity below 7.
- Every changed line is covered as cmd/diff-coverage measures it (>=85% per ADR-008).
- No file outside scope.paths (internal/orchestrator/**, internal/policy/**, cmd/loom/cmd/**, scripts/pre-ship-check.sh, docs/**) changes, excepting this run's own delivery record under docs/features/pre-ship-checks/.
- internal/orchestrator/testdata/shape-deliver-feature.json is regenerated with the -update flag and the diff explained in the commit.
- scripts/pre-ship-check.sh is executable, refuses an empty LOOM_BASE_COMMIT, and reproduces CI's own diff-coverage invocation exactly.
- code-reviewer and security-reviewer have both passed (this stage executes arbitrary project-declared shell commands via bash -c, which is a trust-boundary-relevant change even though the commands themselves come only from committed, CI-equivalent-trust configuration).
