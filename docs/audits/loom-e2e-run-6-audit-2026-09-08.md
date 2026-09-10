# Loom End-to-End Run 6 — Audit

**Status**: COMPLETE. Six invocations across two conditions, all succeeded. **$4.3767** for
condition S against a $4.30 projection; condition L was already collected by run 5A.

**Protocol**: [`loom-e2e-run-6-brief-2026-09-08.md`](./loom-e2e-run-6-brief-2026-09-08.md),
committed in `5b8c2fd` **before the first condition-S invocation**, including the predicted
direction. No rule was written or edited after a number was known.

---

## 1. Headline

**L3.19's conclusion is upheld, and now on evidence that actually tests it.** A repository with
**176× fewer lines of source** costs **8.2%** less to build context for — comfortably inside the
noise band, and nowhere near the 40% drop the rule required to call a source term real.

| Condition | Repository | cache_read mean | vs L |
|---|---|---:|---|
| **S** | 6 files, 124 lines | **1,858,579** | 0.918× |
| **L** | 1,547 files, 21,826 lines | **2,024,888** | — |

Run 3 reached the same conclusion from an experiment that changed the repository, the spec and the
build simultaneously at n=1 per condition. That reasoning was not sound; the answer happens to be
right.

**And a finding nobody was looking for**: the two conditions do not merely differ in mean, they
differ **6× in stability**. Condition S's spread is **4.6%**; condition L's is **27.4%**. The
variance run 5A measured is not instrument noise — it is the repository.

---

## 2. Design

The independent variable is repository size and nothing else.

| | Condition S | Condition L |
|---|---|---|
| Target file | `console-logger.ts`, md5 `4e67dda0…` | **same bytes, same path** |
| Spec | md5 `b16ccb20d09e524602f9495f372e1819` | **same bytes** |
| Stage | `context-engineer` alone | same |
| Tracked files | 6 | 1,547 |
| First-party TS lines | 124 | 21,826 |
| n | 3 | 3 (run 5A) |

Condition S was built by **reduction** from `saturday-monorepo`: the target file and its spec file
copied unchanged at their original paths, plus the minimum scaffolding that makes a plausible
package. Nothing was authored, so the file the feature touches is identical in both conditions.

---

## 3. Results

| Run | cache_read | cache_write | output | cost | secs |
|---|---:|---:|---:|---:|---:|
| S-1 | 1,860,443 | 116,710 | 21,369 | $1.5790 | 216 |
| S-2 | 1,814,557 | 82,535 | 17,118 | $1.2965 | 184 |
| S-3 | 1,900,738 | 99,336 | 22,322 | $1.5012 | 346 |
| **S mean** | **1,858,579** | 99,527 | 20,270 | $1.4589 | 249 |
| L-1 | 2,238,979 | 114,599 | 17,346 | $1.6196 | 198 |
| L-2 | 1,683,662 | 85,931 | 15,118 | $1.2476 | 186 |
| L-3 | 2,152,023 | 84,648 | 19,542 | $1.4468 | 218 |
| **L mean** | **2,024,888** | 95,059 | 17,335 | $1.4380 | 201 |

All six produced schema-valid `ContextState` documents.

### The rule, applied as written

`|S − L| / L = 8.2%`

- ≤ 30% → **no detectable source term.** ✅ **This branch fires.**
- S ≤ 0.60 × L → real source term. S/L is 0.918, nowhere close.

**Verdict: no detectable source term.** A 176× difference in source volume moves cache_read by
8.2%, which is smaller than the run-to-run spread of the larger condition. The direction predicted in
advance was correct — S is lower than L — but the magnitude is inside noise, so even the 8.2% cannot
be claimed as real.

**`aider-repo-map` and `repomix-codebase-packing` stay unbuilt**, now on a controlled measurement
rather than on run 3's confounded one.

---

## 4. The unlooked-for result: the noise is the repository

| Condition | spread (max−min)/mean | sd as % of mean |
|---|---|---|
| S — 124 lines | **4.6%** | 2.3% |
| L — 21,826 lines | **27.4%** | 14.8% |

Run 5A reported 27.4% and read it as a property of the instrument, concluding the measurement was
too noisy to support run 3's n=1 claim. **That reading is now wrong in an interesting way.** The same
stage, same prompt, same build, same model, measured against a tiny repository, is stable to within
4.6%.

So the variance is not the model and not the framework prompt. It is what the agent chooses to
explore, and there is only variance to have when there is something to explore.

This is *consistent with* — and does not prove — L3.19's stated mechanism, that a stage reads the
files its manifest pins rather than the repository broadly. Under that mechanism the repository
contributes a small, variable term on top of a large fixed prompt cost, which is exactly the shape
observed: means 8.2% apart, spreads 6× apart.

**It also revises run 5A's R1 finding.** Run 5A's inconclusive 27.4% should be read as "exploring a
large repository varies by a quarter", not "loom's cost measurement is unreliable". The instrument is
fine. The thing being measured is variable, and only in one condition.

---

## 5. What this does not establish

- **Not a mechanism proof.** A null result is consistent with L3.19's explanation and with any other
  explanation predicting no large source term. Nothing here inspects what the agent actually read.
- **Not sensitive to small effects.** §6.3 of the brief: n=3 against a 27.4% spread detects large
  effects only. A source term worth 10–15% of cache_read is invisible to this design, and the
  measured 8.2% sits precisely in that blind spot — it may be entirely real and this run cannot say.
- **Not a statement about pipeline cost.** One stage, as in run 5A.
- **Not an explanation of the 2.77× level shift** between run 3 and run 5. Eleven items shipped;
  no rule here assigns a cause.

---

## 6. What an auditor should challenge

**6.1 — The conditions were measured on different commits.** Condition L is run 5A at `2fb95c4`;
condition S ran at `5b8c2fd`. Between them: L3.33's architecture-schema conditional and four
documentation commits. None touches `context-engineer`, the context schema, its routing, or the
provider. Declared in the brief before the run and still believed inert — and the result is far from
both thresholds, so §6.1's own escape clause ("re-measure L at the same commit if the result lands
near a threshold") is not triggered.

**6.2 — Condition S is constructed, not found.** A reduced repository is not a naturally small one:
no CI config, no sibling packages, a thin README. If `context-engineer` responds to repository
*shape* rather than volume, S understates a real small project. The brief said this in advance.

**6.3 — The 8.2% gap is unresolvable here and may be the real answer.** It has the predicted sign and
a plausible size for a modest source term. Calling it "no detectable term" is correct as stated and
should not harden into "no term". Resolving it needs n large enough to see through condition L's
spread — roughly n≥12 per condition at this variance, about $35.

**6.4 — Both conditions ran their three invocations concurrently.** Inherited from run 5A §5.1
deliberately, so the two conditions share the confound rather than differ by it. Condition S's 4.6%
spread under the same concurrency is itself evidence the confound is small.

**6.5 — Six runs is still six runs.** Two conditions × n=3 is enough for a 176× manipulation. It
would not be enough for a 2× one.

---

## 7. Cost

| Item | Cost |
|---|---|
| Condition S — 3 invocations | **$4.3767** |
| Condition L — reused from run 5A | $0 (already spent: $4.3140) |
| **Run 6 marginal** | **$4.3767** |
| Brief's projection | $4.30 |

Runs 5A and 6 together — **$8.69** — answered both the variance question and the scale question that
runs 3 and 4 spent **$34.96** failing to answer.

---

## 8. Disposition for L3.19

The `RESOLVED` block should now read:

- **Conclusion: upheld.** No detectable source term at 176× source volume, n=3 per condition,
  measured on one build with the spec and target file held byte-identical.
- **Run 3's evidence: superseded, not merely stale.** Its 0.76× came from an n=1 comparison across
  two builds and two specs. Run 6 replaces it with a controlled measurement reaching the same
  conclusion.
- **The residual 8.2% is open** and needs ~n≥12 per condition to resolve. It is not a blocker for
  the repo-map decision, which turned on whether a *large* source term exists.

---

*Run 6 audit. Complete. L3.19 is answered on its own terms for the first time.*
