# Test Suite Health Metrics

`CLAUDE.md` gates on coverage ≥ 85% and `run-tests` enforces it. Coverage is the one number that
cannot detect the failure this framework's own agents are capable of producing:

- repairing a vacuous test changes coverage by **zero**
- retiring a dead test moves it **down** while the suite gets better
- deleting an assertion leaves the line **covered**

A suite can get greener and blinder at the same time, and coverage reports that as success. These
four numbers are what detect it. None of them is enforced — see *Judgment-only*, below.

---

## The four numbers

| Metric | What it is | Why it earns its place |
|---|---|---|
| **Flake rate** | % of runs where a test fails without a code change | The headline, and the one teams quote |
| **Mean time to diagnose** | From red build to "we know why" | Improves fastest under this kind of work, and almost nobody tracks it |
| **Escaped defects** | Bugs reaching production a test could have caught | The one that catches a green-and-blind suite |
| **Suite wall-clock** | p50 and p95 of a full run | The one the team feels daily |

**The pairing is the point.** Flake rate falling on its own is ambiguous — you may have fixed the
flakes or deleted them. Flake rate falling *while escaped defects hold steady* means the signal
survived. Either number alone is gameable; the pair is much harder to fake.

### The shape that means something went wrong

```
Flake rate       ↓↓ falling fast
Escaped defects  ↑  rising
```

That is a suite being made green by deleting evidence — exactly what
`shared/rules/test-repair-contract.md` exists to prevent, and the first thing to check if anything
feels off.

**Cheap diagnostic**: pull the last 30 days of test-file diffs and count removed assertions against
added. More removed than added is your answer. `git log -p --since=30.days -- tests/ | grep -c
'^-.*expect('` is the rough version and takes seconds — language-specific and noisy, so read it as a
signal to look, never as a verdict.

## What these are not

- **Not test count.** An agent can add four hundred tests in an afternoon. This measures typing.
- **Not coverage percentage.** See the top of this file.
- **Not "AI-generated tests merged"** or **agent invocations.** These measure adoption of a tool,
  not improvement of a suite. Put one on a dashboard and someone will optimise for it.

## Two numbers that are supposed to go down

At ninety days, a suite this work has improved often looks like:

```
Flake rate            12%   →  4%       ✓ and you can name the causes
Time to diagnose      ~2h   →  ~25m     ✓
Escaped defects       6/qtr →  6/qtr    ✓ flat is a PASS
Suite wall-clock      34m   →  21m      ✓
Coverage              62%   →  59%      ✓ you retired dead tests
Test count           1,840  → 1,790     ✓ you removed more than you added
```

Coverage and test count fall, and both are good news. **Explain that before it shows up on a
dashboard, not after.** Flat escaped defects at ninety days is a pass: holding steady while flake
rate falls by two thirds means noise was removed and signal was not.

---

## Judgment-only, and why

Per `architecture-guardrails.md` #7, a structural decision that cannot produce a fitness function
must say so with a reason. This one cannot:

- **Escaped defects** live in a bug tracker loom does not read.
- **Time to diagnose** lives in ticket timestamps, or in asking three engineers.
- **Flake rate** lives in CI history, and nothing here ingests it.

Only suite wall-clock is reachable, and one of four numbers is not a metric set. Claiming a check
here would repeat the mistake ADR-002 records — a judgment-only fitness function citing a layer that
did not exist.

**These are also not pipeline metrics.** `pipeline-retrospective` and `agent-scorecard` measure the
*pipeline* — which agent is slowest, which loops most. These measure the *suite*. Merging them gives
you one dashboard where flake rate sits beside code-reviewer p95 and neither means anything.

### The cheapest honest way to start capturing escaped defects

Not built, and described rather than specified: one line in each delivery's `retrospective.md` asking
whether a bug reached production since the last delivery that a test could have caught, and which
test would have caught it. That is a quarter's worth of data in a year, gathered where a human is
already writing prose — and "filtered to testable" needs a definition the team agrees on *before*
anyone counts, or the metric gets relitigated the first time it moves unwelcomely.

## Related

- `shared/rules/test-repair-contract.md` — what stops a suite being made green by deletion
- `shared/knowledge/flake-triage-taxonomy.md` — what to do once flake rate is a number you have
- `docs/patterns/testing-pyramid.md` — the levels these metrics are measured across

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
