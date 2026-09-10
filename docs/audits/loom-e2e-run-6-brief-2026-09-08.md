# Loom End-to-End Run 6 — Brief

**Status**: written and committed before any run-6 number exists.

**Written** 2026-09-08 against `ai-assistant-dot-files` @ `0a6d974`.

---

## 1. The question

**Does `context-engineer` cache_read scale with the size of the repository it runs in?**

This is L3.19's actual question, and after run 5A it is the only open one that matters: run 5A
showed the *evidence* for L3.19's answer is stale (729,694 does not reproduce; the level is now
2.77× higher) while saying nothing about the *answer*, because it held source scale constant.

L3.19's `RESOLVED` block currently says do not build `aider-repo-map` or `repomix-codebase-packing`.
That instruction is presently supported by nothing measured on the current build.

---

## 2. Why run 3's version of this experiment was weak

Run 3 compared two conditions, and **changed three things between them**:

| | Run 2 | Run 3 |
|---|---|---|
| Repository | 2 files, 26 lines | 1,547 files, ~22k lines |
| **Spec** | a Go handler | a TypeScript class method |
| **Build** | earlier | v3.7.0 |
| n per condition | 1 | 1 |

Only the first row is the independent variable. The spec and the build were confounds, and n=1 per
condition against an instrument now measured at **27.4% spread** (run 5A) cannot resolve anything
smaller than a large effect.

## 3. Design: vary the repository and nothing else

**Condition L (large)** — already collected. Run 5A's three invocations against
`saturday-monorepo` @ `9e9daa2`: 1,547 tracked files, 21,826 lines of first-party TypeScript.
Mean cache_read **2,024,888**, spread 27.4%, build `2fb95c4`.

**Condition S (small)** — this run. A repository containing **only the files the feature touches**,
at the same paths, with the **byte-identical spec** (md5 `b16ccb20d09e524602f9495f372e1819`) and
the same install procedure, on the same build. Three invocations.

Constructed by reduction rather than invention: `packages/saturday-core/src/utils/console-logger.ts`
and its spec file are copied from `saturday-monorepo` unchanged, with the minimum scaffolding that
makes it a plausible package. **The target file is byte-identical.** The spec resolves to a real
file at the same path in both conditions.

So exactly one thing differs: how much *other* source surrounds the target.

| | Condition S | Condition L |
|---|---|---|
| Target file | identical bytes, identical path | identical bytes, identical path |
| Spec | identical bytes | identical bytes |
| Build | `0a6d974` | `2fb95c4` (schema/routing identical; see §6.1) |
| Surrounding source | ~2 orders of magnitude less | full monorepo |

---

## 4. Decision rules, fixed now

Let **S** be condition S's mean cache_read and **L** = 2,024,888.

The instrument's measured spread is 27.4%, so nothing inside that band is a signal.

- **|S − L| / L ≤ 30%** → **no detectable source term.** A repository two orders of magnitude
  smaller costs the same to build context for. L3.19's conclusion is upheld **on the current build**,
  its `RESOLVED` block gets fresh evidence, and `aider-repo-map` / `repomix-codebase-packing` stay
  unbuilt on something measured.
- **S ≤ 0.60 × L** → **a real source term exists.** Cost falls materially with repository size, so
  source discovery is a cost axis after all. **L3.19's conclusion is reversed**, the repo-map KIs
  return to the roadmap as candidates, and run 3's finding is recorded as an artefact of n=1.
- **Anything between** → inconclusive; report the number, claim nothing, and say what n would settle
  it.

**R-fail**: a failed invocation is recorded, counted against budget, and excluded. Fewer than three
successes in condition S reports nothing.

**Direction is predicted in advance**: if source scale drives cost, S is *lower* than L. An S
materially *higher* than L would falsify the mechanism both hypotheses share and is a finding in its
own right.

---

## 5. What this cannot answer

- **Total run cost.** One stage, as in run 5A. Nothing here speaks to a pipeline total or to the
  routing saving.
- **Why the level moved 2.77× between run 3 and run 5.** Eleven items shipped; this run does not
  separate them and no rule assigns a cause.
- **Whether the mechanism is what L3.19 claims.** L3.19 asserts stages read only the files the
  manifest pins. A null result is *consistent* with that and does not prove it; a positive result
  would refute it.

---

## 6. What an auditor should challenge

**6.1 — The two conditions were measured on different commits.** Condition L is run 5A at `2fb95c4`;
condition S runs at `0a6d974`. Between them: L3.33's conditional (architecture schema only) and
three documentation commits. Neither touches `context-engineer`, the context schema, the routing it
uses, or the provider. The confound is believed inert and is **stated rather than controlled** — if
the result lands near a threshold, condition L must be re-measured at the same commit before
anything is claimed.

**6.2 — Condition S is constructed, not found.** A reduced repository is not the same object as a
naturally small one: no CI config, no sibling packages, a thinner README. If `context-engineer` reads
repository *shape* rather than volume, S understates what a real small repo would cost.

**6.3 — n=3 per condition against a 27.4% spread detects large effects only.** A source term
contributing, say, 15% of cache_read is invisible to this design. A null result means "no *large*
source term", never "no source term".

**6.4 — Run 5A's runs were concurrent.** Condition L inherits that caveat (run 5A §5.1). Condition S
will be run the same way so the two conditions share the confound rather than differ by it.

---

## 7. Budget

**~$4.30** — three invocations at run 5A's observed $1.44 mean. Stop and report if condition S's
first invocation exceeds $2.50.
