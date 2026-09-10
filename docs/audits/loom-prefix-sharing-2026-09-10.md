# L3.19c — Why stages pay cache-creation, and what the only fix costs

**Date**: 2026-09-10 · **Method**: controlled invocation matrix against `claude -p`, n=1 per cell
**Model**: `haiku` · **Follows**: `loom-prompt-tax-attribution-2026-09-10.md` (L3.19a)

---

## 1. The question L3.19a left

L3.19a attributed a stage's prompt: **68% is the CLI's own baseline**, ~38.5k tokens that loom does
not control, re-paid on every one of a run's fifteen stages. It left one question, which is the only
one that matters for the largest line item in the system:

> Why is that baseline cache-**creation** on stages 2–15 rather than cache-**read**?

## 2. Result: repetition alone never amortizes

| Invocation | created | read |
|---|---:|---:|
| R1 fresh, no tools | 30,092 | 17,927 |
| **R2 byte-identical immediate repeat** | **30,092** | 17,927 |
| R3 `--allowed-tools Read,Glob,Grep` | 49,258 | 0 |
| R4 `--allowed-tools Read,Glob,Grep,Write,Edit` | 30,090 | 19,169 |
| R5 repeat of R3 | 30,088 | 19,169 |

**R1 vs R2 is the finding.** Two byte-identical consecutive invocations, and the second pays the
same 30,092 in creation. A separate `claude -p` process cannot reuse the previous process's cached
prefix, regardless of whether anything changed.

This kills the hypothesis I went in with. I expected the per-stage `--allowed-tools` allowlist
(different for every agent, and part of the cached prefix) to be the cause. It is not: R2 changed
nothing at all and still missed. The tool set does perturb the prefix — R3 shows a completely fresh
lineage, read=0 — but it is not what prevents amortization.

Only ~18–19k is ever read across processes. In an **empty** directory a process still creates
20,592, so the un-amortizable block is CLI content, not anything loom installs.

## 3. Result: one session amortizes completely

| Invocation | created | read |
|---|---:|---:|
| S1 fresh session | 30,090 | 17,927 |
| **S2 same session, `--resume`, new prompt** | **51** | 48,017 |
| **S3 same session, `--resume` again** | **50** | 48,068 |

Cache-creation falls from 30,090 to **51 — a 99.8% reduction**. The prefix amortizes perfectly when
a stage continues an existing session.

Weighted for billing (creation 1.25x, read 0.1x), per stage:

- fresh: `30,090x1.25 + 17,927x0.1` = **39,406** input-equivalent units
- resumed: `51x1.25 + 48,017x0.1` = **4,866** units

**8.1x cheaper per stage.** Over fifteen stages, 591,090 units against 107,530 — **5.5x** on the
prefix term.

## 4. Result: forking does NOT amortize

`--fork-session` ("when resuming, create a new session ID") looked like prefix sharing without
conversation sharing — one primer session paying the prefix, each stage forking from it.

| Invocation | created | read |
|---|---:|---:|
| primer | 30,092 | 17,927 |
| fork 1 | 30,241 | 17,927 |
| fork 2 | 30,242 | 17,927 |
| fork 3 | 30,241 | 17,927 |

**A fork pays full price.** Each one starts a new cache lineage; it is a cold start that happens to
inherit some history. The cheap option does not exist.

## 5. What this means for L3.19

The mechanism is real, cheap to adopt mechanically, and **conflicts directly with a design property
loom deliberately has**.

Amortizing requires every stage to run as a turn in **one continuous conversation**. That means each
stage sees every prior stage's full output — which is exactly what L2.9's typed state exists to
prevent. Today a stage receives *projected upstream fields*, chosen per consumer; under a shared
session it would receive the raw transcript of everything that came before. The `code-reviewer`
would see the `developer`'s reasoning rather than its artifact.

Three further costs, stated because they are not obvious:

- **Context grows linearly.** S2→S3 added only 51 read tokens because the replies were trivial. Real
  stage outputs run 1.3k–11.6k tokens, so a fifteen-stage conversation accumulates roughly 70k. The
  5.5x above is therefore an **upper bound**, not a projection — the true figure needs measuring
  against real outputs, which this experiment did not do.
- **The context window becomes a run-length limit.** Fifteen stages of accumulated output against a
  200k window is a ceiling the current architecture does not have.
- **Failure blast radius.** One session for a whole run makes a corrupted or poisoned context a
  run-level failure rather than a stage-level one, and `--resume` is also how L2.15's resume works.

## 6. The options, honestly costed

1. **One session per run.** ~5.5x on the prefix (upper bound). Costs stage isolation, bounded
   context, and per-stage failure containment. Mechanically small — the provider already shells out;
   this is threading a session id.
2. **Direct API provider with explicit `cache_control`.** Keeps full isolation *and* gets the
   sharing, because loom would place the breakpoint itself after the shared preamble. Costs
   implementing the agent tool-use loop that the CLI currently provides. Large, and overlaps L4.8.
3. **Selective merging.** Share a session only among stages where isolation genuinely does not
   matter, paying cold starts at the boundaries. Keeps the property where it is load-bearing and
   captures part of the saving.
4. **Do nothing structural; trim content.** `.claude/rules/` is 7,707 tokens loaded in full on every
   stage — the only per-byte win L3.19a found. Small, safe, and does not touch the 68%.

**Recommendation: (3), with (4) alongside.** (1) trades a correctness property for cost, which is
the wrong direction for a system whose last nine runs were spent establishing that its stages report
honestly. (2) is the right end state and is too large to start here.

## 7. What this does not establish

- **n=1 per cell**, haiku, and a trivial project. The ratios are large enough to survive noise; the
  absolute numbers are not a cost model.
- **Growth under real stage outputs is unmeasured.** §5's first bullet is the load-bearing gap: the
  5.5x could fall substantially once a conversation carries fifteen real artifacts.
- **Why** a separate process cannot reuse the cache is still unknown. Something in the CLI's own
  ~20.6k block varies per invocation; the session transcripts do not contain the system prompt, so
  this was established behaviourally rather than forensically. It does not change the options.
