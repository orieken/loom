# Loom End-to-End Run 5A — Audit

**Status**: COMPLETE. Three invocations, all succeeded, **$4.3140** against a $4.20 projection.

**Protocol**: [`loom-e2e-run-5-brief-2026-09-08.md`](./loom-e2e-run-5-brief-2026-09-08.md), committed
in `2fb95c4` **before the first invocation**. No rule below was written or edited after a number was
known.

**Executed** 2026-09-08 against `ai-assistant-dot-files` @ `2fb95c4`, targeting `saturday-monorepo`
@ `9e9daa2`.

---

## 1. Headline

The variance question, open since run 3's §9.1 and deferred three times, now has a number:
**27.4%**. Under the brief's own rules that is the inconclusive band — too noisy to call the
instrument stable, not noisy enough to reopen L3.19 on noise alone.

**But R2 is decisive, and it is the more consequential rule.** Run 3's `context-engineer` cache_read
of **729,694** — the single measurement L3.19's `RESOLVED` block rests on — does not reproduce. The
mean of three runs on the identical spec and repository is **2,024,888**, which is **1.03× run 4's**
figure and **2.77× run 3's**.

Run 4's 2.71× was not an outlier. It is now the reproduced level, measured three times
independently, agreeing with run 4 within 3%.

---

## 2. Setup

Three fresh clones of `saturday-monorepo` @ `9e9daa2`, `loom install --target . --platform
claude-code` in each, and the byte-exact run-3 spec copied in.

| Clone | HEAD | Spec md5 | Deleted tracked files |
|---|---|---|---|
| `loom-r5-1` | 9e9daa2 | `b16ccb20d09e524602f9495f372e1819` | 0 |
| `loom-r5-2` | 9e9daa2 | `b16ccb20d09e524602f9495f372e1819` | 0 |
| `loom-r5-3` | 9e9daa2 | `b16ccb20d09e524602f9495f372e1819` | 0 |

Spec md5 matches run 3's, so no spec confound. **L3.26 confirmed for the second consecutive run** —
0 deleted tracked files across three more clones, where run 3's install left 124.

---

## 3. Results

| Clone | cache_read | cache_write | output | cost | seconds | document valid |
|---|---:|---:|---:|---:|---:|---|
| `loom-r5-1` | 2,238,979 | 114,599 | 17,346 | $1.6196 | 198 | yes |
| `loom-r5-2` | 1,683,662 | 85,931 | 15,118 | $1.2476 | 186 | yes |
| `loom-r5-3` | 2,152,023 | 84,648 | 19,542 | $1.4468 | 218 | yes |
| **mean** | **2,024,888** | 95,059 | 17,335 | $1.4380 | 201 | 3/3 |

All three produced a schema-valid `ContextState`. **L3.28's fix holds against a real model three
times over** — this is the first evidence of that from anything other than the single contract test.

### R1 — the spread: **27.4%**, inconclusive band

`(2,238,979 − 1,683,662) / 2,024,888 = 27.4%`

The rule was ≤15% stable, >30% reopens L3.19, between → *"report the number, claim nothing, and say
the question is still open."* Applying it as written: **no claim, and the question is still open.**

27.4% is uncomfortably close to the 30% line and the rule was fixed in advance, so it is not moved
now. What can be said without a rule: an instrument with a quarter-spread was never capable of
supporting run 3's n=1 conclusion at the precision that conclusion was stated with.

### R2 — the level: **run 4's figure reproduces**

| | cache_read | vs run 5 mean |
|---|---:|---|
| Run 3 (v3.7.0, 2026-09-07) | 729,694 | 0.36× |
| Run 4 (attempt 2, 2026-09-08) | 1,974,775 | 0.98× |
| **Run 5 mean (n=3)** | **2,024,888** | — |

R2's rule: *"If the mean sits near run 4's, the 2.71× is reproducible on the current build, and run
3's 0.76× describes a system that no longer exists."* It sits within **3%** of run 4's. **The rule
fires.**

And the shift is not explicable by the noise R1 measured: the spread is 27.4%, the run-3-to-run-5
difference is **177%**. A gap six times the observed noise band is a real change in level, not
sampling.

---

## 4. What this does and does not do to L3.19

**Does not refute L3.19's conclusion.** L3.19 says source scale is not the cost driver, on the
evidence that cache_read did not grow when source grew ~770×. **Run 5 did not vary source scale at
all** — same repository, same spec, three times. It cannot speak to that question, and does not.

**Does invalidate L3.19's evidence.** The `RESOLVED` block quotes 729,694 and a 0.76× ratio as the
measurement that closed the repo-map question. That number does not reproduce on the current build,
and the current level is 2.77× higher. The stated evidence describes a system that no longer exists.

**Adds a defect the original never accounted for.** Run 3 drew a conclusion from one measurement
per condition with no variance estimate; the instrument turns out to carry ~27% spread. Run 3's own
§9.1 called this *"the single most load-bearing untested assumption in the report"* and it was
right to.

**Correct disposition**: L3.19's resolution is neither upheld nor reversed. It should be marked
**evidence stale, conclusion untested on the current build**. Re-testing it means repeating run 3's
actual experiment — the same spec against a small repository and a large one, on today's build, with
n≥3 per condition — which is run 6, not this run.

---

## 5. What an auditor should challenge

**5.1 — The three runs were concurrent, not sequential.** Clone 1 started ~2 minutes before 2 and 3,
which then ran alongside each other. Prompt-cache state is therefore not identical across the three,
and clone 1's cache_write of 114,599 against ~85k for the others is consistent with it having
populated a cache the others read. This is a genuine uncontrolled variable and the brief did not
anticipate it.

It does not explain the spread away. The two fully concurrent runs — clones 2 and 3, warm and
simultaneous — differ by **24.4%** on their own. Most of the observed variance survives the
strictest available comparison within this data.

**5.2 — n=3 is a spread, not a distribution.** Three points give a range. 27.4% is one sample of
what the spread is, and a fourth run could move it either side of both thresholds. The rule's
asymmetry was chosen for this reason and the inconclusive band is doing exactly its job.

**5.3 — R2's "near" was not given a number in advance.** The rule said "sits near run 4's" without
a threshold. 3% is unambiguous by any reading, so the judgement is not load-bearing here — but it
is a rule that could have been gamed and was not specified as tightly as R1.

**5.4 — Eleven items shipped between run 3 and run 5.** R2 establishes the level moved. It assigns
no cause, and none of `context-engineer` becoming typed, L3.25's budget measurement, or L3.28's
schema change is separated from the others. §6.1 of run 4's brief applies unchanged.

**5.5 — This measures one stage.** The design's advantage is that stage-1 cache_read cannot be
affected by later stages; its limitation is that nothing here speaks to total run cost, the routing
saving, or any stage after the first.

---

## 6. Run 5B — not run

The brief made 5B conditional on R1 returning ≤15%. It returned 27.4%. **5B is not run**, per the
rule as written: comparing one pipeline total against run 3's $9.49 is not meaningful when the first
stage alone varies by more than a quarter.

The A2 routing-saving question therefore remains open, and now has a known obstacle rather than an
assumed one.

---

## 7. Cost

| Item | Cost |
|---|---|
| 3 × `context-engineer` | **$4.3140** |
| Brief's projection | $4.20 |

Against the run-3-and-4 design — three full pipelines — this answered the same question for roughly
an eighth of the cost and required no human gate approvals.

---

## 8. Reproduction

```bash
for n in 1 2 3; do
  git clone -q ~/Projects/Rieken/saturday-monorepo /tmp/loom-r5-$n
  (cd /tmp/loom-r5-$n && git checkout -q 9e9daa2 \
     && loom install --target . --platform claude-code \
     && cp <repo>/docs/audits/loom-e2e-run-4-specs/console-log-filtering.md docs/features/)
done
```

Then invoke `context-engineer` alone against each clone and read
`StageOutput.Usage.CacheReadTokens`. The harness used was a temporary
`//go:build integration` test in `internal/provider/claude/`, removed after the run; it did nothing
the exported provider API does not already allow.

---

*Run 5A audit. Complete. The variance question is answered as far as n=3 answers it; the scale
question it was blocking is now the open one.*
