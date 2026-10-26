---
intake: 1
id: pre-ship-checks
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-25
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 40
  max-hours: 3
scope:
  paths: [internal/orchestrator/**, internal/policy/**, cmd/loom/cmd/**, scripts/pre-ship-check.sh, docs/**]
---

# Run the project's own checks before the ship gate

## Request

> PR #18 reached `confirm-ship` with every agent's tests passing, was approved, and then failed CI's
> changed-line coverage gate (`cmd/diff-coverage`, 85%) on seven uncovered lines. QA had reported
> per-package and per-function coverage, which is not the number CI gates on. A human fixed it after
> the PR was open. Nothing in the pipeline ran the check CI runs, so the ship gate asked a human to
> approve a change CI would reject.

## Summary

A project declares the checks a change must pass before a human is asked to ship it, as commands in
`.claude/delivery-policy.yaml`. `loom run` runs them in an executor-internal stage after the last
reviewing and documenting stage and before the `confirm-ship` gate. A failing check fails the run
with the check's output kept, so `confirm-ship` is never reached with a check failing. Loom's own
repository will declare the coverage gate CI runs. The checks are commands, not Go-specific logic:
loom delivers into projects in any language.

## Acceptance Criteria

- [ ] Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list, where each entry has a
      `name`, a `command` and an optional `timeout`, when the policy config is loaded, then the
      entries are decoded. An entry with no `name` or no `command`, or with an unknown key, is
      rejected with `ErrInvalidConfig` naming the entry. This keeps the strict loading `LoadConfig`
      already has.
- [ ] Given the built-in `deliver-feature` plan, then it contains an executor-internal stage
      `pre-ship-checks` placed after `tech-writer` and before the stage that carries the
      `confirm-ship` gate. The stage runs before that gate halts the run.
- [ ] Given one or more `preShipChecks`, when the stage runs, then it runs each command in order
      through `bash -c` with the working directory at the project root. It gives each command the
      environment variable `LOOM_BASE_COMMIT` set to the run's `StartCommit`, and an explicit
      timeout. The default timeout is 10 minutes when the entry sets none.
- [ ] Given every check exits 0, then the stage completes, and its artifact records each check's
      name, exit status, duration and the last 50 lines of its output.
- [ ] Given a check exits non-zero or times out, then the stage fails, names the check, and keeps
      that check's full output in the stage's artifact. The failure is terminal, so a supervisor
      does not retry it, and the run never reaches `confirm-ship`. Checks after a failing one do not
      run.
- [ ] Given no `preShipChecks` are configured, then the stage is recorded `SKIPPED` with the reason
      "no preShipChecks configured". It never reports COMPLETED for checks that never ran.
- [ ] Given the change is complete, then `scripts/pre-ship-check.sh` exists and runs what CI's
      coverage gate runs:
      - `go test ./... -coverprofile=<a temporary file>`
      - then `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile <that file>`

      It exits non-zero when either fails, and refuses to run when `LOOM_BASE_COMMIT` is empty.
- [ ] Given the change is complete, then `go test ./internal/orchestrator/... ./internal/policy/...
      ./cmd/loom/...` passes. Every new function has cyclomatic complexity below 7. Every changed
      line is covered as `cmd/diff-coverage` measures it. No file outside `scope.paths` changes, the
      run's own delivery record under `docs/features/pre-ship-checks/` excepted.

## Out of Scope

- Writing loom's own `.claude/delivery-policy.yaml` entry. Agents cannot write under `.claude/`; a
  human adds `preShipChecks: [{name: changed-line coverage, command: scripts/pre-ship-check.sh}]`
  after this merges.
- Feeding a failed check back to the developer for another attempt. That is roadmap L2.18's bounded
  contract retry.
- Adding `preShipChecks` to `deliver-bugfix`, which has no `confirm-ship` gate.
- Any change to CI workflows. That is approval gate #7.

## Domain Language

- **Pre-ship check**: a project-declared command that must exit 0 before a human is asked to ship.

## Non-Functional Requirements

- Every check runs under an explicit timeout (architecture-guardrails.md #5).
- A check's output may contain secrets from the project's environment. The artifact keeps the
  output, but telemetry records only the check's name, exit status and duration (guardrail #9).

## Trust Boundaries

- The commands come from the project's committed `.claude/delivery-policy.yaml`, which is the same
  trust level as the project's own CI configuration. They are never read from a work order, which
  can be external.

## Test Approach

Go unit tests:
- the config decoding, including rejected entries;
- the plan's stage order;
- the internal stage, run with stub commands such as `true`, `false`, `sleep` past a short timeout
  and `echo $LOOM_BASE_COMMIT`.

Executor tests use the mock provider.

## Open Questions

- Whether the stage's artifact is typed state or markdown. Assumption if unresolved: markdown, like
  the router's `route.md`, since nothing downstream decodes it.

## Infrastructure / Deploy Notes

After merge, a human adds loom's `preShipChecks` entry to `.claude/delivery-policy.yaml`.
