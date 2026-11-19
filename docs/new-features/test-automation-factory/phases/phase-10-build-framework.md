# Phase 10 — Build the Framework Orders

## Goal
Ship the framework orders that phase 4 wrote, then finish everything that was deferred because the framework did not exist: the pending examples, the API and UI exemplars, and the skill dry-runs.
Skip this phase when phase 4 found every framework block.

## Inputs
- `docs/test-factory/FRAMEWORK-STATUS.md` and the `framework-*` orders from phase 4.
- The skills (phase 5), exemplars manifest (phase 6), budget check (phase 7), runbook (phase 8), and pre-ship (phase 9).
- A human review of the orders: only a human sets `status: ready`.

## Steps
1. **Confirm the human's go-ahead.** List the framework orders and their `depends-on`. Do not start an order whose `status` is not `ready`. If none is ready, stop and ask.
2. **Ship each ready order in dependency order**, using `RUNBOOK.md` in manual mode:
   - validate the order, check the budget, work on the branch the standing approval covers;
   - run the skill that matches the order's `test-type` (the orders build framework code, so use `unit-test` for its tests);
   - after each stage append a `stage-end` ledger line (spend from the tool's report, or a human estimate), and write a `can-fail-shown` line for each test file;
   - run `preship --order <id> --base <ref>`; fix what it reports; stop at gates;
   - record `shipped` (with `url: local` when there is no remote).
   Do the minimum that satisfies each order's acceptance criteria.
3. **Finish phase 4.** Replace each `Pending` marker in the pattern documents with a real, running example built on the shipped framework. Run it; record the command and result. Update `FRAMEWORK-STATUS.md` to `found`.
4. **Finish phase 5.** Dry-run `api-test` and `ui-e2e-test` on a small case each. Each writes a test, shows it failing once (a change to product code), then passing.
5. **Finish phase 6.** Choose, qualify, and mark the API and UI exemplars. Add their manifest entries with `can-fail` evidence, update the blueprints' `Exemplars` sections, and re-run `check:exemplars` and `test-audit`.
6. **Close the deferred list.** Search the project and the pack for `deferred to phase 10`. Each hit is now done or is recorded as still open, with a reason.

## Files
The framework code and tests from each order, their ledger lines, updated pattern documents, `FRAMEWORK-STATUS.md`, the two new exemplars and manifest entries, updated blueprints.

## Verification
- Every framework order has a `shipped` ledger line, and `preship --order <id>` passed for it (or the failure is recorded and the order is parked).
- `FRAMEWORK-STATUS.md` shows every block `found`, with no `Pending` marker left in a pattern document.
- Each of the three levels (`unit`, `api`, `ui-e2e`) has an exemplar that `check:exemplars` accepts, and each failed when its product code was broken.
- `api-test` and `ui-e2e-test` dry-runs each produced a test that failed once and then passed.
- No `deferred to phase 10` text remains without a recorded reason.
- Changed-line coverage of the framework code is at least 85%, and each test was shown to fail once.

## Budget
Each order carries its own budget. This phase itself: `max-usd: 10`, `max-attempts: 2`, `max-hours: 4` beyond the orders.

## Time-box
Stage timeout 30 minutes per stage. One order per stage where possible. Check each order's budget before every stage; park an order that runs out and report it.
