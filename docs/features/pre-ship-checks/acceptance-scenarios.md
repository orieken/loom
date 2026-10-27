---
feature: "pre-ship-checks"
bounded_context: ""
domain_terms: ["pre-ship checks", "PreShipCheck", "confirm-ship gate", "LOOM_BASE_COMMIT", "diff-coverage", "ErrInvalidConfig"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Acceptance Scenarios: pre-ship-checks

## A complete pre-ship check entry decodes with the default timeout

Verifies: Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list where each entry has a `name`, a `command`, and an optional `timeout`, when the policy config loads, then `policy.LoadConfig` decodes every entry. It MUST reject an entry with no `name`, no `command`, or an unknown key with `ErrInvalidConfig` naming the entry and the offending key.

```gherkin
Scenario: A complete pre-ship check entry decodes with the default timeout
  Given A delivery policy file whose preShipChecks list has one entry with a name and a command, and no timeout
  When The policy configuration loads
  Then The entry decodes as a pre-ship check with a ten-minute default timeout
```

## A pre-ship check entry without a name is rejected

Verifies: Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list where each entry has a `name`, a `command`, and an optional `timeout`, when the policy config loads, then `policy.LoadConfig` decodes every entry. It MUST reject an entry with no `name`, no `command`, or an unknown key with `ErrInvalidConfig` naming the entry and the offending key.

```gherkin
Scenario: A pre-ship check entry without a name is rejected
  Given A delivery policy file whose preShipChecks list has one entry with a command but no name
  When The policy configuration loads
  Then Loading fails with an invalid-configuration error that identifies the entry by its command
```

## A pre-ship check entry without a command is rejected

Verifies: Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list where each entry has a `name`, a `command`, and an optional `timeout`, when the policy config loads, then `policy.LoadConfig` decodes every entry. It MUST reject an entry with no `name`, no `command`, or an unknown key with `ErrInvalidConfig` naming the entry and the offending key.

```gherkin
Scenario: A pre-ship check entry without a command is rejected
  Given A delivery policy file whose preShipChecks list has one entry with a name but no command
  When The policy configuration loads
  Then Loading fails with an invalid-configuration error that identifies the entry by its name
```

## A pre-ship check entry with an unrecognized key is rejected

Verifies: Given `.claude/delivery-policy.yaml` contains a `preShipChecks` list where each entry has a `name`, a `command`, and an optional `timeout`, when the policy config loads, then `policy.LoadConfig` decodes every entry. It MUST reject an entry with no `name`, no `command`, or an unknown key with `ErrInvalidConfig` naming the entry and the offending key.

```gherkin
Scenario: A pre-ship check entry with an unrecognized key is rejected
  Given A delivery policy file whose preShipChecks list has one entry named "coverage" with a command and an extra key such as retries
  When The policy configuration loads
  Then Loading fails with an invalid-configuration error that names both the entry "coverage" and the unknown key
```

## The delivery plan runs pre-ship checks before the ship gate can halt the run

Verifies: Given the built-in `deliver-feature` plan, then it MUST contain an executor-internal stage with ID `pre-ship-checks` placed after `tech-writer` and before `devops-engineer` (the stage carrying the `confirm-ship` gate), so the stage runs before that gate can halt the run.

```gherkin
Scenario: The delivery plan runs pre-ship checks before the ship gate can halt the run
  Given The built-in deliver-feature plan
  When The plan's stage order is inspected
  Then An executor-internal stage named pre-ship-checks appears after tech-writer and before devops-engineer
```

## Configured checks run in order from the project root with the run's base commit available

Verifies: Given one or more configured `preShipChecks`, when the stage runs, then it MUST run each command in order through `bash -c`, with the working directory set to the project root, the environment variable `LOOM_BASE_COMMIT` set to the run's `StartCommit`, and an explicit timeout per check (the entry's own, or 10 minutes when the entry sets none).

```gherkin
Scenario: Configured checks run in order from the project root with the run's base commit available
  Given Two configured pre-ship checks that each echo the LOOM_BASE_COMMIT environment variable and the current working directory
  When The pre-ship-checks stage runs
  Then Both checks run in their configured order, each sees the project root as its working directory, and each sees LOOM_BASE_COMMIT set to the run's start commit
```

## A check without its own timeout is bounded by the ten-minute default

Verifies: Given one or more configured `preShipChecks`, when the stage runs, then it MUST run each command in order through `bash -c`, with the working directory set to the project root, the environment variable `LOOM_BASE_COMMIT` set to the run's `StartCommit`, and an explicit timeout per check (the entry's own, or 10 minutes when the entry sets none).

```gherkin
Scenario: A check without its own timeout is bounded by the ten-minute default
  Given A configured pre-ship check with no timeout of its own
  When The pre-ship-checks stage runs that check
  Then The check is subject to a ten-minute deadline rather than running unbounded
```

## All checks passing completes the stage with a per-check summary

Verifies: Given every configured check exits 0, then the stage MUST complete, and its artifact MUST record each check's name, exit status, duration, and the last 50 lines of its output.

```gherkin
Scenario: All checks passing completes the stage with a per-check summary
  Given Two configured pre-ship checks that each exit zero
  When The pre-ship-checks stage runs
  Then The stage completes
  And The stage's artifact records each check's name, exit status, duration, and the last 50 lines of its output
```

## A failing check stops the run before the ship gate without retry

Verifies: Given a check exits non-zero or times out, then the stage MUST fail, name the check, and keep that check's full output in the stage's artifact. The failure MUST be terminal (no supervisor retry), the run MUST NOT reach `confirm-ship`, and checks listed after the failing one MUST NOT run.

```gherkin
Scenario: A failing check stops the run before the ship gate without retry
  Given Three configured pre-ship checks where the first passes, the second exits non-zero, and the third would pass if it ran
  When The pre-ship-checks stage runs
  Then The stage fails naming the second check
  And The second check's full output is kept in the artifact
  And The third check never runs
  And The failure is terminal, with no supervisor retry
  And The run does not reach confirm-ship
```

## A check that exceeds its timeout fails the stage as a terminal timeout

Verifies: Given a check exits non-zero or times out, then the stage MUST fail, name the check, and keep that check's full output in the stage's artifact. The failure MUST be terminal (no supervisor retry), the run MUST NOT reach `confirm-ship`, and checks listed after the failing one MUST NOT run.

```gherkin
Scenario: A check that exceeds its timeout fails the stage as a terminal timeout
  Given A configured pre-ship check with a short timeout whose command sleeps past that timeout
  When The pre-ship-checks stage runs
  Then The stage fails naming the check as timed out
  And The failure is terminal, with no supervisor retry
  And The run does not reach confirm-ship
```

## An unconfigured pre-ship-checks stage is skipped, not completed

Verifies: Given no `preShipChecks` are configured, then the stage MUST be recorded SKIPPED with the reason "no preShipChecks configured", and MUST NOT report COMPLETED for checks that never ran.

```gherkin
Scenario: An unconfigured pre-ship-checks stage is skipped, not completed
  Given A delivery policy configuration with no preShipChecks entries
  When The pre-ship-checks stage runs
  Then The stage is recorded SKIPPED with the reason "no preShipChecks configured"
  And The stage is never recorded COMPLETED
```

## The pre-ship check script measures changed-line coverage against the base commit

Verifies: Given the change is complete, then `scripts/pre-ship-check.sh` MUST exist, run `go test ./... -coverprofile=<a temporary file>` followed by `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile <that file>`, exit non-zero when either step fails, and refuse to run when `LOOM_BASE_COMMIT` is empty.

```gherkin
Scenario: The pre-ship check script measures changed-line coverage against the base commit
  Given scripts/pre-ship-check.sh exists and LOOM_BASE_COMMIT is set to a real ancestor commit
  When The script runs
  Then It runs the test suite with a coverage profile and then diff-coverage against that profile and the base commit
  And It exits non-zero if either step fails
```

## The pre-ship check script refuses to run without a base commit

Verifies: Given the change is complete, then `scripts/pre-ship-check.sh` MUST exist, run `go test ./... -coverprofile=<a temporary file>` followed by `go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile <that file>`, exit non-zero when either step fails, and refuse to run when `LOOM_BASE_COMMIT` is empty.

```gherkin
Scenario: The pre-ship check script refuses to run without a base commit
  Given LOOM_BASE_COMMIT is empty or unset
  When scripts/pre-ship-check.sh runs
  Then The script refuses to run and exits non-zero without attempting the coverage steps
```

## The feature's own quality gates pass before shipping

Verifies: Given the change is complete, then `go test ./internal/orchestrator/... ./internal/policy/... ./cmd/loom/...` MUST pass, every new function MUST have cyclomatic complexity below 7, every changed line MUST be covered as `cmd/diff-coverage` measures it, and no file outside `scope.paths` MUST change (the run's own delivery record under `docs/features/pre-ship-checks/` excepted).

```gherkin
Scenario: The feature's own quality gates pass before shipping
  Given The completed pre-ship-checks feature change
  When The orchestrator, policy, and loom command test suites run, complexity is measured, and changed-line coverage is measured
  Then All three test suites pass
  And Every new function has cyclomatic complexity below 7
  And Every changed line is covered
  And No file outside the declared scope paths changed, except this run's own delivery record
```
