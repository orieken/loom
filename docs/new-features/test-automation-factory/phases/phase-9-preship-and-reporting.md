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
   | Unit and API tests | all pass | 10 min |
   | Browser tests | all pass (needs a Chromium; see the note below) | 10 min |
   | Complexity | no function at 7 or above | 5 min |
   | Changed-line coverage | at least 85% of added or modified lines | 10 min |
   | Can-fail check | each added or changed test file has a `can-fail-shown` ledger record, or a mutation tool kills the mutants on its changed lines | 15 min |
   | Speed budgets | no new test breaks its level's budget | 5 min |
   | Budget check | the order's ledger is `ok` or `warn` | 1 min |
   A ledger record proves someone wrote the claim down, not that it is true; say so in the script's comment and in the backlog. Prefer a mutation tool once you have verified it on one sample.
   Pick the mutation tool for the language and verify it on one sample before you trust its score. A mutant that did not compile or that timed
   out MUST NOT count as killed.
   **Complexity gate.** Wire the linter's complexity rule (capped at 6) over the factory's own scripts first and run it. Expect violations in code the earlier phases wrote; refactor them (extract functions, lookup tables) and keep the tests green before you go on.
   **Changed-line coverage.** Compute it from the diff against the base plus the runner's JSON coverage report: count only executable statements on changed lines, and ignore files absent from the report.
   **Browser tests.** Installing a Chromium is a download. Ask a human before you add it to CI. Locally, allow an environment variable that points at an existing Chromium.
2. **Record the pre-ship list in `budgets.yaml`** as `preShipChecks` entries with `name`, `command`, and `timeout`.
3. **Reporting.** Write test results as Cucumber-compatible JSON, or add a converter to it, so every language feeds one format.
   Add a short `delivery-summary.md` generator: order id, tests added, changed-line coverage, speed results, budget used, parked or shipped.
4. **Approval gate.** Posting the summary to any external dashboard is an irreversible action. The script prints the summary and stops.
   Only a human answer ("ship") continues. Any edit to the summary voids the answer.
5. **Telemetry rule.** Reports record properties, not payloads: counts, durations, ids, statuses. Never record secret values, tokens, or user
   personal data.
6. **Propose CI, do not apply it.** Write `docs/test-factory/CI-PROPOSAL.md` with the workflow, least-privilege permissions, and the decisions for a human (browser download, how a pull request names its order, where the ledger lives). Use placeholders for action commit SHAs; never guess one.

## Files
`scripts/pre-ship.<ext>`, `scripts/delivery-summary.<ext>`, `budgets.yaml` update, tests, CI proposal (not applied).

## Verification
- On a clean branch, `pre-ship` exits 0.
- Add a new test file with no can-fail record: `pre-ship` fails at the can-fail check.
- Add a function with complexity 8: it fails at complexity.
- Lower coverage of changed lines under 85%: it fails at coverage.
- Make a check exceed its timeout: it fails and reports the timeout.
- The summary contains no secret values. Make the generator refuse to write when it holds the value of a variable named in `.env.example`, and test that.
- Changed-line coverage of the scripts is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 6`, `max-attempts: 2`, `max-hours: 3`.

## Time-box
Stage timeout 30 minutes. `pre-ship` has its own per-check timeouts, listed above. Their total, 59 minutes with the browser check, MUST stay under the order's `max-hours`. Test that the sum in `budgets.yaml` stays below the default.
