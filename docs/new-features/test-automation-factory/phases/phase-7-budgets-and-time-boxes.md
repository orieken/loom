# Phase 7 — Budgets and Time-Boxes

## Goal
Define how the factory limits cost, effort, and elapsed time, then enforce those limits with a script. Budgets stop runaway work. Time-boxes stop slow work.

## Inputs
- The intake template and validator from phase 2.
- `BLUEPRINT.md`, section "Budgets and Time-Boxes".

## The policy to write into `budgets.yaml` and `docs/test-factory/BUDGETS.md`

### Order budget (required on every order)
| Field | Meaning | Default |
|---|---|---|
| `max-usd` | Park the order when recorded spend crosses this value | 5 |
| `max-attempts` | Whole-run retries for transient failures before the order is parked | 2 |
| `max-hours` | Ceiling on total stage run time across all attempts | 2 |
| `stage-timeout-minutes` | Hard limit for one stage | 30 |
| `deadline` | Optional wall-clock stop, ISO 8601 with offset | none |

`max-hours` counts time spent running stages. It does not count an order waiting in the inbox. `deadline` counts wall-clock time from any
cause, including waiting. Use `deadline` when a human needs the result by a set time.

### Behavior at the limits
1. **80% of any ceiling:** write a warning line to the ledger and to the run summary. Keep working.
2. **Ceiling reached or deadline passed:** stop before the next stage starts. Write a checkpoint note (what is done, what is left, files touched).
   Park the order with `reason: budget-exhausted` or `reason: deadline-passed`. Add it to the inbox. Do not retry.
3. **Resume:** a human raises the ceiling or extends the deadline in the order, then sets `status: ready`. The runner resumes from the checkpoint.
4. **Stage timeout:** kill the stage, record the elapsed time, and count one attempt. Two timeouts in a row park the order.
5. **Unmeasured usage counts as unknown, not zero.** When the tool cannot report spend, the order halts at the next stage
   boundary and asks a human to accept the unknown cost. Never assume it was free.
6. **No agent raises its own ceiling.** Changing a budget is a human action.

### Speed budgets per test level
| Level | Budget per test |
|---|---|
| Unit | under 100 ms |
| Integration | under 5 s |
| API contract | under 5 s |
| Acceptance scenario | under 60 s |
| E2E suite total | a few minutes |

A new test that breaks its speed budget fails the pre-ship check. Widening a budget needs a measured p95 and a stated reason.

### Context budget
Each task declares a context tier before work starts: `light` (one file), `default` (a feature area), or `heavy` (cross-cutting).
Load only the files the tier needs. Do not state a token count from memory.

### Model tiers
Map each job to a cost tier in `budgets.yaml`: `light` for audits and document edits, `default` for writing tests, `heavy` for design and
legacy characterization. Teams set the model names in one place.

### Time-boxed tasks (new mechanism, built in this phase)
This is not assumed to exist. Build it:
- Add `deadline` and `stage-timeout-minutes` to the intake validator (phase 2 already accepts them).
- Write `scripts/check-budget.<ext>`. Input: an order file and a ledger file (JSON Lines). Output: `ok`, `warn`, or `stop`, with the reason.
  It MUST compute spend, attempts, stage hours, and deadline status, and treat missing spend data as unknown.
- Write a `time-box` section in each skill: "Stop at the stage timeout. Write a checkpoint. Do not extend it."

## Steps
1. Write `budgets.yaml` with the defaults above and the model-tier map.
2. Write `docs/test-factory/BUDGETS.md` from the policy above.
3. Write `scripts/check-budget.<ext>` and its tests: one case per behavior in "Behavior at the limits".
4. Add a ledger format to the doc: one JSON object per line with `time`, `order`, `event`, `usd`, `stage`, `seconds`, `note`.
5. Add the `time-box` section to each skill from phase 5.

## Files
`budgets.yaml`, `docs/test-factory/BUDGETS.md`, `scripts/check-budget.<ext>`, tests, ledger format, skill updates.

## Verification
- Feed `check-budget` a ledger at 79%, 80%, and 100% of `max-usd`: it returns `ok`, `warn`, `stop`.
- Feed it an order with a `deadline` in the past: `stop` with `deadline-passed`.
- Feed it a ledger with a stage lacking `usd`: it returns `stop` with `unknown-usage`, not `ok`.
- Changed-line coverage is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 5`, `max-attempts: 2`, `max-hours: 2`.

## Time-box
Stage timeout 30 minutes. This phase defines the limits it must itself obey. Check its own ledger with `check-budget` before you finish.
