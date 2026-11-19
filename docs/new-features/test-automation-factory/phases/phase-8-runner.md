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
3. Work on a branch the standing approval covers. Create `factory/<id>` from `base-branch` when the approval names it; otherwise work on the branch the approval names (for example `main`) and say so in the ledger `note`.
4. Start the skill that matches `test-type`, with the order as input.
5. After each stage, append a ledger line and run `check-budget`.
6. At a gate, stop and wait for a human answer.
7. Run the pre-ship checks (built in phase 9; if it does not exist yet, run the checks that do and say so). Open a pull request, or, with no remote, stop at a local branch and record that.
8. Append a `shipped` ledger line with the pull request URL, or with `url: local` when there is no remote.

### Supervisor mode (build second, optional)
Write `scripts/factory-run.<ext>` as a loop. It MUST:
- read orders from the committed `base-branch`, not from a working copy, and take only `status: ready` orders;
- skip orders whose `depends-on` is unmet, and order the rest by priority, then `created`, then `id`;
- claim an order by writing a `claimed` ledger line and a copy of the order as the run's spec;
- run each order in its own git worktree on `factory/<id>`; make one sequential pass and cap the orders in it with `--max-orders` (default 1). True concurrency is not part of this phase;
- call `check-budget` before every stage, and stop on `stop`;
- park on a gate, a budget stop, or two stage timeouts, and add the order to `inbox/` with the reason;
- never write to the order file; record all progress in `ledger.jsonl` (append-only);
- never approve a gate. Only a human command, `factory-approve approve <id> <gate>`, records an approval. The approval stores the digest of the committed order, so any change to the order voids it and the order starts again with a new claim. Provide `factory-approve shipped <id> <url>` so a human records the pull request;
- use two gates: `commit` after the stages, and `design` before any stage for an order with `trust: external` or `autonomy: strict-human`;
- support `--once` (run one pass and exit) and `--dry-run` (print what would run).

Define small interfaces for the clock, the ledger, the order source (read only), the workspace, the inbox, and the agent call, so tests use fakes. The agent call returns only `ok`, `usd` (or unknown), `seconds`, and `timedOut`, so agent output has no field that could carry an approval.

The real agent is a command you configure (for example `FACTORY_AGENT_COMMAND`). It receives `ORDER_ID`, `STAGE`, `WORKSPACE`, and `ORDER_FILE` in its environment and prints one JSON line. Report spend as unknown when the line has none.

State plainly what you did not build: loop mode, concurrency, recovery from a crash mid-order, opening pull requests, and a bundled agent command.

## Files
`docs/test-factory/RUNBOOK.md`; optional `scripts/factory-run.<ext>`, `scripts/factory-approve.<ext>`, `inbox/`, tests.

## Verification
- Manual mode: a person completes the sample order from phase 2 using only `RUNBOOK.md`.
- Supervisor mode, only if you built it (it is optional; skipping it does not fail this phase): with fake agent and git, tests show these cases: order claimed once; dependency respected; budget stop parks the order;
  a gate stops the run; approval is not created by agent output; the order file is unchanged after a run.
- Adapters: run the real ledger, git order source (reads committed content, not an uncommitted edit), worktree, inbox, and command agent (success, failure, timeout) in tests against a temporary git repository.
- Run the supervisor once end to end in a scratch repository with a stub agent: dry run, real pass, approval, second pass, shipped.
- Changed-line coverage is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 8`, `max-attempts: 2`, `max-hours: 4`. Split supervisor mode into two pull requests if needed.

## Time-box
Stage timeout 30 minutes. The supervisor MUST apply the same limits to itself: it stops at `max-hours` of its own loop time unless a human extends it.
