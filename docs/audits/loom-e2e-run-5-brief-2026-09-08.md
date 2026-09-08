# Loom End-to-End Run 5 — Brief

**Status**: written before any run-5 number exists. That property is the only reason runs 3 and 4
are worth citing, and it is preserved here deliberately: the rules below were committed before the
first invocation.

**Written** 2026-09-08 against `ai-assistant-dot-files` @ `5093280`.

---

## 1. The one question this run exists to answer

**Does `context-engineer` cache_read vary enough between identical runs to invalidate run 3's
result?**

This has been deferred three consecutive runs, each with a good local reason, and it is now the
oldest open item from the run-3 audit (§9.1). L3.19's `RESOLVED` block — the decision not to build
`aider-repo-map` or `repomix-codebase-packing` — rests on **one** measurement: 729,694, reported as
0.76× of run 2's baseline. The single attempted replication, run 4, came back at **1,974,775 —
2.71×**, on a different build with three changes in between.

Two measurements, opposite directions, no variance estimate. Everything else on this roadmap is
downstream of knowing whether that instrument is stable.

---

## 2. Design change: measure the stage, not the pipeline

Runs 3 and 4 both treated variance as something obtained from full pipelines. **It is not.**
`context-engineer` is stage 1; its cache_read is fixed by its own invocation and cannot be affected
by stages that run after it. Three full pipelines to measure one stage three times is the expensive
way to get the same number.

**Run 5A is three invocations of `context-engineer` alone**, same spec, same clone procedure, no
downstream stages.

| | Full pipeline ×3 (runs 3–4 design) | Stage 1 ×3 (this design) |
|---|---|---|
| Cost | ~$30–45 | **~$4.20** at run 4's observed $1.36–1.41 |
| Human gates | 9 approvals | **none** |
| Confounds | every downstream stage, gate latency, review loops | none |
| Answers A1 | yes | yes — *the identical number* |

This is strictly better, not a compromise. The only thing it gives up is measuring whether *total
run* cost varies, which is a different question and one no rule below asks.

### Why this was not obvious before

Because run 3's audit framed A1 as a property of runs, and runs 4's brief inherited the framing
without re-examining it. Recording that here rather than presenting the cheaper design as if it had
always been the plan.

---

## 3. Decision rules, fixed now

**Target**: `saturday-monorepo` @ `9e9daa2`, three fresh clones, `loom install` in each.
**Spec**: `loom-e2e-run-4-specs/console-log-filtering.md`, md5
`b16ccb20d09e524602f9495f372e1819` — the byte-exact run-3 spec, so comparisons carry no spec
confound.
**Measurement**: `context-engineer` cache_read, cost, and output tokens per invocation.

**R1 — the spread.** Report `(max − min) / mean` across the three.

- **≤ 15%** → the instrument is stable. Run 3's 0.76× and run 4's 2.71× cannot both be sampling
  noise, so the difference is attributable to the build, and L3.19's resolution stands **on the
  understanding that it describes v3.7.0 and not the current build**.
- **> 30%** → the instrument is unstable at the scale the decision was made on. **L3.19's `RESOLVED`
  block is reopened**, and the repo-map question returns to open. This is the expensive outcome and
  it must not be argued away.
- **Between** → report the number, claim nothing, and say the question is still open.

**R2 — the level.** Report the mean against run 3's 729,694 and run 4's 1,974,775.

- If the mean sits near run 4's, the 2.71× is **reproducible on the current build**, and run 3's
  0.76× describes a system that no longer exists. L3.19's evidence is then stale regardless of R1.
- If it sits near run 3's, run 4's figure was an outlier and R1's spread says how surprising one.

**R3 — no rule is applied to a stage that failed.** A failed invocation is recorded, its cost
counted against the budget, and excluded from the spread. If two of three fail, the run reports
nothing and says why.

---

## 4. What run 5A does not answer

- **Total run cost variance.** Not measured, not claimed. A2's routing-saving question from run 4
  remains unanswered and needs full pipelines.
- **Whether the eleven items shipped since run 3 changed the level.** R2 will show *that* the level
  moved; §6.1's attribution problem is unchanged and no rule here assigns a cause.
- **Anything about stages after the first.** Run 4's Experiment B already demonstrated the full
  pipeline end to end; this run deliberately does not repeat it.

---

## 5. Run 5B — optional, only if 5A clears

If and only if R1 returns ≤15%, one full-pipeline run of the run-3 spec becomes worth its cost,
because a stable stage-1 instrument makes a total comparable to run 3's $9.49 and finally answers
A2. **If R1 is >30%, 5B is not run**: comparing one total against another is meaningless when the
first stage alone varies by a third.

Both routing bugs found in run 4's data (`a96b6aa`) mean the run-3 spec should now route **zero**
optional stages, against run 3's four no-op stages. That is the A2 prediction, and 5B is what tests
it.

---

## 6. Budget

**5A: ~$4.20** (3 × $1.40 observed). Stop and report if the first invocation exceeds $2.50 — that
would itself be a finding about the level.

**5B, if reached: ~$8–12.**
