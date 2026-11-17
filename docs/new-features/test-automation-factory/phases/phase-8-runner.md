# Phase 8 — Runner (Optional)

## Goal
Run orders one after another with state, budgets, and gates. **Manual mode is the default.** An automated supervisor is optional, and you
build it only after manual mode works for several orders.

## Inputs
- Intake validator, budget checker, and skills from phases 2, 5, and 7.
- The approval rules from phase 1.

## Steps
Build manual mode first. Build supervisor mode only after manual mode has shipped several orders.

### Manual mode (build first)
Write `docs/test-factory/RUNBOOK.md`. A human follows it for each order:
1. Pick the oldest `status: ready` order with the highest priority whose `depends-on` orders have shipped.
2. Run the intake validator and `check-budget`. Stop if either fails.
3. Create a branch `factory/<id>` from `base-branch`.
4. Start the skill that matches `test-type`, with the order as input.
5. After each stage, append a ledger line and run `check-budget`.
6. At a gate, stop and wait for a human answer.
7. Run the pre-ship checks (phase 9). Open a pull request.
8. Append a `shipped` ledger line with the pull request URL.

### Supervisor mode (build second, optional)
Write `scripts/factory-run.<ext>` as a loop. It MUST:
- read orders from the committed `base-branch`, not from a working copy, and take only `status: ready` orders;
- skip orders whose `depends-on` is unmet, and order the rest by priority, then `created`, then `id`;
- claim an order by writing a `claimed` ledger line and a copy of the order as the run's spec;
- run each order in its own git worktree on `factory/<id>`; limit concurrent orders with `--max-concurrent` (default 1);
- call `check-budget` before every stage, and stop on `stop`;
- park on a gate, a budget stop, or two stage timeouts, and add the order to `inbox/` with the reason;
- never write to the order file; record all progress in `ledger.jsonl` (append-only);
- never approve a gate. Only a human command, `factory-approve <id> <gate>`, records an approval, and any change to the pending artifact voids it;
- support `--once` (run one pass and exit) and `--dry-run` (print what would run).

Define small interfaces for the clock, the filesystem, git, and the agent call, so tests use fakes.

## Files
`docs/test-factory/RUNBOOK.md`; optional `scripts/factory-run.<ext>`, `scripts/factory-approve.<ext>`, `inbox/`, tests.

## Verification
- Manual mode: a person completes the sample order from phase 2 using only `RUNBOOK.md`.
- Supervisor mode: with fake agent and git, tests show these cases: order claimed once; dependency respected; budget stop parks the order;
  a gate stops the run; approval is not created by agent output; the order file is unchanged after a run.
- Changed-line coverage is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 8`, `max-attempts: 2`, `max-hours: 4`. Split supervisor mode into two pull requests if needed.

## Time-box
Stage timeout 30 minutes. The supervisor MUST apply the same limits to itself: it stops at `max-hours` of its own loop time unless a human extends it.
