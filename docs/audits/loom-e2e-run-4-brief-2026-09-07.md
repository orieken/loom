# Loom End-to-End Run 4 — Brief

**Status**: written before the run, so the decision rules below cannot be chosen after seeing the
numbers. That property is what made run 3's primary result worth anything, and it is the only reason
to write this down in advance rather than describe the runs afterwards.

**Written** 2026-09-07, against `ai-assistant-dot-files` @ `c2e0aa2`, after eight roadmap items
shipped from the run-3 audit.

---

## 1. What changed since run 3, and why that makes this run awkward

| Item | Change | Verified how, so far |
|---|---|---|
| L3.21 | `extractJSON` takes the last fenced block | unit tests |
| L3.22 | run total sums every attempt, not the last | unit tests reproducing run 3's $8.7978 vs $9.4923 |
| L3.24 + L3.18 | routing reads facts prose cannot satisfy | unit tests over run 3's own analysis shape |
| L3.25 | executor measures the context budget | unit tests + a mock run |
| L3.26 | install owns files per path; cache read-only | reproduced the clobber, then the fix |
| L2.22 | provider passes each stage its declared tools | **the real CLI** — denied without, writes with |
| L2.25 | schema enums derived from the constants | fitness function + reproduction |
| — | `context-engineer` is a typed stage | mock run |

Eight changes against a single n=1 baseline. That is the awkwardness: run 4 measures a large delta,
and if the total moves, attributing the movement to any one item is guesswork. Splitting the runs
below is how that is partly managed; it is not fully solved, and §6.1 says so.

---

## 2. Target: `saturday-monorepo`, again — for one experiment, not both

**Yes for the repeat runs.** Variance is only meaningful under identical conditions, so §9.1 of the
run-3 audit ("three repeat runs on the same spec") requires the same repository, the same spec, and
the same protocol. Changing the target would produce three numbers that cannot be compared to
729,694 / $9.49 / 12 calls, which is the whole point of measuring them.

**And that is exactly why the repeat runs cannot be the only experiment.** Holding the spec constant
is required for variance and fatal for generalisation — run 3's §9.3 already recorded that one spec
shape was tested and nothing was said about any other. Conflating the two questions in one run
answers neither.

So run 4 is two experiments with different targets and different rules.

---

## 3. Experiment A — variance and the routing saving

**Target**: `saturday-monorepo`, cloned fresh per run.
**Spec**: `loom-e2e-run-4-specs/console-log-filtering.md`, the original run-3 file recovered intact.
Copy it to `docs/features/` in each clone before the run so all three read the same bytes.
**Runs**: three, sequential, no changes between them.

### Decision rules, fixed now

**A1 — variance.** Report the spread of `context-engineer` cache_read across the three runs as a
percentage of their mean.
- Spread ≤ 15% → run 3's 0.76× result discriminates as claimed, and L3.19's `RESOLVED` block stands.
- Spread > 30% → the repo-map conclusion rests on noise; reopen L3.19's resolution and say so in the
  roadmap. **This is the outcome that costs the most to accept and must not be argued away.**
- Between the two → report the number, claim nothing.

**A2 — the routing saving.** Run 3 spent **$2.63** on four stages that had nothing to do. L3.24
projects that to zero for this spec.
- Mean total ≤ $7.20 (run 3's $9.49 − $2.63, plus 10% slack) → the saving is real at this shape.
- Mean total ≥ $9.00 → the saving did not materialise; find out why before claiming it anywhere.
- The comparison is only valid if A1's spread is ≤ 15%. If it is not, A2 reports a number and no
  verdict.

**A3 — did anything stop running that should not have?** Record which stages the router included.
Expected: the four L3.24 names out, and `developer`, `code-reviewer`, `security-reviewer`,
`qa-engineer`, `tech-writer`, `context-engineer`, `analyst` in. **Any review stage skipped is a
failure of this run regardless of what it saved.**

---

## 4. Experiment B — the risk the narrowing introduced

**One run. Different spec. This is the more important experiment, and it is not about cost.**

L3.24 narrowed four routing predicates. Every observation of that change so far — unit tests, the
mock run, run 3's replayed analysis — has watched it **skip** things. Nothing has yet watched it
**include** things from a real analyst's output.

The specific failure mode: `Surfaces{UI, Runtime}` is a new field, and absent means false. If the
analyst does not emit it — because the schema is new, the template changed this week, and nothing
forces the field — then `architect`, `performance-engineer`, `visual-qa-engineer` and
`sre-engineer` all skip **on every feature**, including ones that need them. The same holds for the
typed `Threshold`: an analyst that keeps writing prose gets no threshold at all, and the architect
never runs.

**Under-routing is a worse failure than the over-routing L3.24 fixed.** Over-routing costs $2.63.
Under-routing silently skips review on work that needed it, and produces no artifact saying so.

**Spec shape**: a feature with a genuine UI surface, a served runtime surface, a real latency budget
with a number and a unit, and a multi-file change. Something an SRE, a performance engineer and a
visual QA engineer each have real work on.

### Decision rules, fixed now

**B1** — the analyst emits `surfaces` with at least one true, and at least one
`nonFunctionalRequirements[].threshold` with `metric`, `value` and `unit` all populated.
- If it does not: **L3.24 is a regression, not an improvement.** File it, and treat the narrowing as
  unsafe until the analyst reliably produces the facts the predicates read.

**B2** — `architect`, `performance-engineer`, `sre-engineer` and `visual-qa-engineer` all route IN.
- Any of them skipped on this spec is the under-routing failure. Record which, and why the route
  file says so.

**B3** — the four L3.24 predicates are judged on B1 and B2 together, not on cost. A cheaper run that
skipped a stage it needed is a worse result than run 3.

**B4 — the controls.** B2 alone is passable by a router that includes everything, which is the bug
L3.24 removed. The spec therefore carries work that must stay **out**: `data-engineer` (the store
stays in memory — no schema, no migration) and `devops-engineer` (no CI, deployment or environment
change is asked for).
- Both skipped, and B2's four included → routing discriminates.
- All six included → routing has stopped discriminating; B2 passing means nothing.
- Report both halves. A run that only records the inclusions has not tested the predicate.

### The specs are committed

Both live in [`loom-e2e-run-4-specs/`](./loom-e2e-run-4-specs/), so neither run depends on a `/tmp`
clone surviving. Experiment A's spec is **byte-identical to run 3's**, recovered from
`/tmp/loom-e2e3` before it was reaped (md5 `b16ccb20d09e524602f9495f372e1819`) — it is the original,
not a reconstruction, so §3's comparison to $9.49 carries no spec confound. That was luck; committing
them is the fix.

---

## 5. Protocol — what run 3 did that must not be repeated

Three deviations from run 3 are corrective, and each closes a caveat that weakened its findings.

1. **No hand-patching.** Run 3 worked around L2.22 (`settings.local.json`) and L2.25 (the STRIDE
   enum) *before* the run, so neither could be reported on (§9.5). Both are now fixed. **Apply no
   workarounds.** If either still needs one, that is the run's most valuable finding and it must be
   recorded rather than patched past.

2. **The developer must not install dependencies.** Run 3's developer ran `pnpm install` at 00:07:18
   unplanned, which meant `qa-engineer` had real test data and the L2.24 "no repro" tested "does it
   report honestly when it can measure" rather than "does it fabricate when it cannot" (§9.2).
   Install dependencies **before** the pipeline in Experiment A so this is not left to chance, and
   in **one** of the three runs deliberately leave them absent to test what L2.24 actually describes.

3. **Cite the real scale.** 1,547 tracked files, ~22k lines of first-party TypeScript. Not 5,130
   files or 1.4M lines — those counts included `node_modules`, which is gitignored and which no
   stage reads (§9.6).

Standard setup, unchanged from run 3: clone to `/tmp`, never the live repository, so `git status` is
a trustworthy record of what the pipeline wrote. Commit the post-install state as a baseline before
running. Approve gates one at a time after reading the pending artifacts.

**New in this run** — L3.26 changed what install does, so record it:
- `git status` after `loom install` should show **no deleted tracked files**. Run 3's install left
  124. Any deletion is an L3.26 regression.
- Confirm the framework cache is read-only (`ls -l ~/Library/Caches/loom/<version>/shared/`).

---

## 6. What an auditor should challenge before the run happens

**6.1 — Eight changes, one baseline.** If the total moves, this run cannot say which item moved it.
Experiment A isolates the routing saving only because L3.24 is the only shipped item that changes
*which stages run*; everything else changes what a stage costs or whether it fails. That reasoning
is an argument, not a control, and it fails if two items interact.

**6.2 — Three runs is a small n for a variance claim.** Three points give a spread, not a
distribution. A ≤15% spread across three runs is weak evidence of a stable process; a >30% spread is
strong evidence of an unstable one. The rule is deliberately asymmetric for that reason.

**6.3 — Experiment B is n=1 and judged on qualitative criteria.** B1 and B2 are yes/no readings of
one analyst's output. An analyst that emits the fields once is not an analyst that emits them
reliably. A pass here lowers the risk; it does not close it.

**6.4 — The A2 saving is a projection being tested against itself.** $2.63 came from summing run 3's
four no-op stages. If the routing fix works, the saving is roughly that by construction. The
informative outcomes are the ones where it *is not*.

**6.5 — L2.22 is verified at the CLI, not end to end.** Its done-when asks for a non-empty `git
diff` from a fresh install plus run. Nothing yet demonstrates that. If Experiment A's developer
produces an empty diff, L2.22 is not fixed regardless of the probe result.

---

## 7. Budget

Run 3 cost $9.49. Three A runs at the projected saving ≈ $21, plus Experiment B's larger feature
≈ $12–15. **Expect $33–40.** If A's first run comes in above $11, stop and find out why before
spending the rest.

---

## 8. Attempt 1 — aborted at stage 1 (2026-09-08)

Setup verified clean against every §5 check: 1,547 tracked files / 21,826 lines, spec md5
`b16ccb20d09e524602f9495f372e1819` in all three A clones, **0 deleted tracked files** across four
clones where run 3 left 124 (L3.26 confirmed), cache read-only, no hand-patching, A3 left bare.

Both A runs then died identically on the first stage:

```
stage "context-engineer" returned invalid state: field "schemaVersion" is 1, this build supports 2
```

**A1 $0.8226, A2 $1.2632. $2.09 spent, zero stages completed.** No workaround applied, per §5.1.

**Cause**: `schemaVersion` reflected as a bare `{"type": "integer"}` while the validator refused
anything but the constant. Latent for as long as the field has existed and harmless while the
constant was 1; the bump to 2 in `bf302c8` activated it across all eight schemas at once. Fixed in
`9580614` by deriving the `const` from the constant, with a test that builds each document from the
schema the agent receives — the boundary a mock provider cannot reach, and the one §1 recorded the
typed context-engineer as "verified by" .

### What attempt 1 says about A1, despite measuring nothing

The two `context-engineer` cache_read figures — **1,000,545 (A1) and 2,005,958 (A2)** — are from
failed stages and are **not** an A1 measurement. They are still worth reading as a caution: same
spec, same repository, same stage, and a **2.0× spread**.

Part of that is explained by different amounts of work before failing (4,541 vs 10,638 output
tokens), so it is not a clean variance sample. But A1's rule asks whether the spread of a *completed*
stage is ≤15%, and the only same-condition pair observed so far is 100% apart. **Lower confidence
that A1 clears 15%** — and note that a >30% spread is the outcome that reopens L3.19's `RESOLVED`
block. Do not treat that as the unlikely branch.

### Ordering for attempt 2

**Experiment B first**, then A×3 only if B completes.

B is already the brief's more important experiment, it is n=1 so a further confound costs it least,
and it is the better canary: it routes in more stages than A, so it exercises more typed schemas per
dollar. If another model-boundary defect exists, B surfaces it for ~$12 rather than ~$35.

**On the confound**: the `schemaVersion` fix does not sharpen §6.1. It changes no routing predicate,
nothing about what a stage reads, and nothing about what a stage costs — only whether a document
validates. Every axis A1, A2, B1 and B2 measure is untouched by it. It is a ninth commit, not a
ninth variable.
