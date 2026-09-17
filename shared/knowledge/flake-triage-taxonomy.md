---
name: flake-triage-taxonomy
tags: [testing, flaky-tests, triage, quarantine, ci]
domain: testing
created: 2026-09-14
---

Fourteen flaky tests is usually not fourteen problems. In practice they resolve to two or three
underlying causes — a shared fixture that does not reset, a service slow to warm, an undeclared
ordering dependency. **Triage by cause, not by test.** The unit of work is the cause; the tests are
symptoms. A list of flaky test names with a ticket each guarantees fourteen investigations of what
is often one bug, which is why flake backlogs never shrink.

## Four conditions filed under one word

| Category | What it is | Who fixes it | The danger |
|---|---|---|---|
| **A — System non-determinism** | The system really is racy; the test is correctly reporting it | The team owning the code | **Quarantining this hides a real defect** |
| **B — Test timing defect** | Fixed sleeps, missing waits, a polling ceiling too low | You | Masking it with a longer sleep |
| **C — Shared state** | Passes alone, fails in suite, or depends on order | You | Fixing one test instead of the topology |
| **D — Environment** | Runner variance, network, cold starts, contention | Platform / CI | Debugging your own code for hours |

The fix, the owner and the urgency differ for all four. **Category A is the expensive one to get
wrong**: a test that intermittently catches a real race is providing exactly the signal you want,
and it is the one most often quarantined because it "looks flaky".

## Evidence that separates them

Triage from failure history, not from one failed run. One run tells you almost nothing; ninety days
usually tells you the category without opening the test.

| Evidence | Points to |
|---|---|
| Identical failure messages across tests | One cause |
| Always fail together in the same run | Shared cause, or environment |
| Concentrated on one runner or time of day | D |
| Passes alone, fails in suite | C |
| Correlates with a deploy or dependency bump | A regression |
| Duration at the timeout ceiling | A or B |

Failure-message variance is the underrated row: fourteen tests failing with byte-identical messages
is one cause; fourteen distinct messages is fourteen problems, or an environment issue affecting
everything — which the timing column will settle.

## Dispositions — exactly one per cluster

- **Fix** — cause understood, and the fix is in the test or the system
- **Escalate** — category A. It is a product defect the test found; hand it to the code owner *with
  the failure history attached*, which is the argument for doing the clustering properly
- **Retire** — it covers behavior that no longer exists
- **Quarantine** — cause not yet understood

There is no fifth disposition called "keep an eye on it".

## How this framework narrows quarantine

**This is a deliberate divergence from the source material, recorded rather than silently applied.**

The wider practice treats quarantine as a disposition an engineer applies. Here, removing a test
from the gating suite is a one-way door on regression signal and is **approval gate #9** in
`shared/rules/approval-gates.md`. An agent **proposes** a quarantine and a human applies it;
`shared/rules/test-repair-contract.md` forbids an agent marking a test skipped, pending, excluded or
quarantined on its own.

A proposal carries four fields or it rots:

```
owner:            payments-team
expiry:           2026-11-01
cause:            suspected race in price recalculation; see CI-4471
evidence_needed:  does it fail with recalc disabled?
```

Then make the expiry real — a scheduled job that fails the build when a quarantine passes its date.
Without that, everything above is a naming convention, and the quarantine list becomes a graveyard
nobody can explain two years later.

### That job is yours to build, and loom does not ship it

Stated plainly because the sentence above reads like an instruction the framework carries out.

A quarantine lives in the test file of the project whose suite is quarantined. `health-check.sh`
runs against the framework repository; `loom health` verifies an *installation* — manifest, version,
paths, symlinks, agent counts. Neither reads your test files, so neither can see an expiry, let
alone enforce one. And `shared/hooks/scheduled-monthly.yaml` declares schedules for a runner you
supply; loom dispatches no hooks (see that file's header, and `shared/hooks/README.md`).

So this is **judgment-only in loom, and enforceable in your project**. The job is roughly twenty
lines. A starting point that fails the build the day an expiry passes:

```bash
#!/usr/bin/env bash
# Fail when any quarantine has passed its expiry. Run on a schedule, not only on PRs —
# an expiry passes on a calendar date, not when someone opens a pull request.
set -euo pipefail

today=$(date -u +%Y-%m-%d)
expired=0

# Adjust the pattern to your annotation. Matches `expiry: "2026-11-01"` or `expiry: 2026-11-01`.
while IFS=: read -r file line _; do
  date_found=$(sed -n "${line}p" "$file" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
  [[ -z "$date_found" ]] && continue
  if [[ "$date_found" < "$today" ]]; then
    echo "EXPIRED  $file:$line — quarantine expired $date_found"
    expired=1
  fi
done < <(grep -rn --include='*test*' -E 'expiry:\s*"?[0-9]{4}-[0-9]{2}-[0-9]{2}' . || true)

if [[ "$expired" -eq 1 ]]; then
  echo
  echo "A quarantine past its expiry is a deleted test with extra steps."
  echo "Fix it, escalate it, retire it deliberately (gate #9), or extend the expiry with a reason."
  exit 1
fi
```

Two properties worth keeping if you rewrite it. **Run it on a schedule, not only on pull requests** —
an expiry passes on a date, and a repository with no PRs that week would never notice. And **make
the failure name the file and the date**, so the fix is obvious without reading the job.

## Related

- `shared/rules/test-repair-contract.md` — what an agent may and may not change repairing a test
- `shared/rules/approval-gates.md` gate #9 — removing test coverage
- `docs/patterns/test-suite-health-metrics.md` — the numbers that tell you whether triage worked
