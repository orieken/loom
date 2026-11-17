# Phase 9 — Pre-Ship Checks and Reporting

## Goal
Run one command before any order ships. It proves the tests are real, fast, and clean. Then report results in a form other tools read.

## Inputs
- All earlier phases.
- The project's CI system.

## Steps
1. **Write `scripts/pre-ship.<ext>`.** It runs these checks in order, each with a timeout, and stops at the first failure:
   | Check | Pass condition | Timeout |
   |---|---|---|
   | Intake validation | every order valid | 1 min |
   | Blueprint validation | registry and blueprints valid | 1 min |
   | Exemplar check | manifest and annotations agree | 1 min |
   | Unit and integration tests | all pass | 10 min |
   | Complexity | no function at 7 or above | 5 min |
   | Changed-line coverage | at least 85% of added or modified lines | 10 min |
   | Can-fail check | mutation tool, or a manual break-and-restore record, shows each new test fails when its behavior breaks | 15 min |
   | Speed budgets | no new test breaks its level's budget | 5 min |
   | Budget check | the order's ledger is `ok` or `warn` | 1 min |
   Pick the mutation tool for the language and verify it on one sample before you trust its score. A mutant that did not compile or that timed
   out MUST NOT count as killed.
2. **Record the pre-ship list in `budgets.yaml`** as `preShipChecks` entries with `name`, `command`, and `timeout`.
3. **Reporting.** Write test results as Cucumber-compatible JSON, or add a converter to it, so every language feeds one format.
   Add a short `delivery-summary.md` generator: order id, tests added, changed-line coverage, speed results, budget used, parked or shipped.
4. **Approval gate.** Posting the summary to any external dashboard is an irreversible action. The script prints the summary and stops.
   Only a human answer ("ship") continues. Any edit to the summary voids the answer.
5. **Telemetry rule.** Reports record properties, not payloads: counts, durations, ids, statuses. Never record secret values, tokens, or user
   personal data.
6. **Wire into CI** — gated. Ask a human before you change any pipeline. Show the proposed diff first.

## Files
`scripts/pre-ship.<ext>`, `scripts/delivery-summary.<ext>`, `budgets.yaml` update, tests, CI proposal (not applied).

## Verification
- On a clean branch, `pre-ship` exits 0.
- Add a test with no assertion: `pre-ship` fails at the can-fail check.
- Add a function with complexity 8: it fails at complexity.
- Lower coverage of changed lines under 85%: it fails at coverage.
- Make a check exceed its timeout: it fails and reports the timeout.
- The summary contains no secret values (search it for the names in `.env.example`).
- Changed-line coverage of the scripts is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 6`, `max-attempts: 2`, `max-hours: 3`.

## Time-box
Stage timeout 30 minutes. `pre-ship` has its own per-check timeouts, listed above. Their total, 49 minutes at most, MUST stay under the order's `max-hours`.
