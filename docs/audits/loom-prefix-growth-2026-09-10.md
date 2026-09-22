# L3.19c growth — what prefix sharing is actually worth at run length

**Date**: 2026-09-10 · **Model**: `haiku` · **Cost**: $1.91 measured across both arms
**Closes**: the load-bearing gap in `loom-prefix-sharing-2026-09-10.md` §7

---

## 1. The gap this closes

The prefix-sharing audit measured `--resume` at **8.1x cheaper per stage** and projected **~5.5x**
over fifteen stages — then flagged that figure as an **upper bound**, because it was measured with
trivial replies. Real stage outputs run 1.3k–11.6k tokens and accumulate in a shared conversation.

That caveat was correct and it was decisive. The 5.5x does not survive.

## 2. Method

Twelve turns matching the stage sequence of the real run in
`docs/audits/loom-e2e-run-2026-09-07/` (12 stages, outputs 2,573–10,429, mean 5,093, on
`claude-sonnet-5`). Each turn is loom's actual prompt shape — the real agent definition, the task
line, an output contract — with the target output size scaled to that stage's real output.

Two arms, same twelve prompts:

- **SHARED** — one session, every turn via `--resume`
- **COLD** — twelve separate `claude -p` processes (today's architecture)

## 3. Result

| | created | read | output | prefix units¹ | measured cost |
|---|---:|---:|---:|---:|---:|
| **SHARED** | 131,966 | 1,141,406 | 83,315 | 279,098 | **$0.7947** |
| **COLD** | 388,542 | 520,496 | 56,531 | 537,727 | **$1.1121** |
| ratio | 2.94x less | 2.19x more | — | **1.93x less** | **1.40x cheaper** |

¹ input-equivalents, creation x1.25 + read x0.1.

**The saving is 1.93x on the prefix term, not 5.5x.** Measured end-to-end cost is 1.40x, and output
tokens — which sharing does nothing about — are the largest single line item ~~in both arms~~ **in
the SHARED arm**.

> **Corrected 2026-09-22.** "In both arms" was wrong, and wrong about the arm that matters: COLD is
> the architecture in production. Applying the 5x output weight every Claude model bills at (Sonnet
> $3/$15, Haiku 4.5 $1/$5) to the table above:
>
> | Arm | cache creation (x1.25) | cache read (x0.1) | output (x5) | largest line item |
> |---|---:|---:|---:|---|
> | COLD | **485,678** | 52,050 | 282,655 | **cache creation** |
> | SHARED | 164,958 | 114,141 | **416,575** | output |
>
> In COLD, cache creation alone exceeds output and the whole prefix term is **1.90x** it. Output
> would have to bill at **8.6x** input to overtake creation and **9.5x** to overtake the prefix —
> nearly twice the real rate — so this does not turn on which model was used.
>
> The sentence was load-bearing: it is the stated reason §6 ranks "reduce stage count" above the
> structural fix, and it is how the roadmap summarises this whole audit. The ranking still holds —
> cutting a stage removes its prefix *and* its output — but not for the reason given, and the case
> for a direct-API provider is stronger than this line implied.
>
> **What does not reconcile**, recorded rather than buried: these units do not reproduce the measured
> dollar figures at any published rate card (COLD's 820,382 input-equivalents against $1.1121 implies
> ~$1.36/M, which matches no Claude model). The arms are internally consistent and their *ratio* is
> unaffected, but the cost column should not be treated as rate-checked.

### Why the estimate was 3x too high

Two effects the trivial-reply test could not show:

- **Read grows.** Turn 2 read 50,979; turn 12 read **142,321**. Cache reads bill at only 0.1x, but
  the volume compounds across every subsequent turn.
- **Each turn's own content must be cached.** The trivial test showed creation of ~50 per resumed
  turn. With real outputs it is **6,407–12,343 per turn** — the conversation's new content, paid at
  the 1.25x creation rate every turn.

```
turn   1  created 33,052   read  68,265
turn   2  created  6,407   read  50,979
turn   6  created 10,074   read  88,369
turn  12  created  7,572   read 142,321
```

## 4. The context ceiling is real and close

Read reaches **142,321 at turn 12 — 71% of a 200k window**, for a twelve-stage run on a small
feature. The real pipeline has fifteen stages, and its `developer` stage alone drew 2.59M cache
reads through tool use. A shared session does not merely risk the window; on this evidence a
full-length run does not fit.

## 5. Confound, stated plainly

**The arms did not produce equal output.** SHARED generated 83,315 output tokens against COLD's
56,531 — **47% more** from identical prompts. The plausible cause is that a shared session lets each
turn see prior stages and write more, which is itself a finding about the design, but it makes the
end-to-end cost comparison uncontrolled.

The 1.93x prefix-unit ratio is the defensible number: it counts only cached input. The 1.40x
end-to-end figure should be treated as indicative. Neither is n>1.

## 6. What this changes

**Option 1 — one session per run — is now clearly a bad trade.** It costs stage isolation, bounded
context, and per-stage failure containment, and buys **under 2x on the prefix term**. The 5.5x that
made it tempting was an artifact of measuring with trivial replies. *(This sentence also read "of a
bill that output tokens dominate" — corrected in §3. Output dominates in SHARED only; in COLD the
prefix is 1.90x output. Option 1 remains a bad trade on the isolation cost alone.)*

**Option 2 — a direct-API provider with explicit `cache_control` — gets *better*, not worse.** It
was previously "keeps isolation, same saving, more work". The growth measurement changes the
comparison: a direct-API stage is its **own** conversation that merely shares a cached prefix, so it
pays no accumulation penalty and stays near the **8.1x per-stage** figure rather than decaying to
1.93x. Sharing the prefix without sharing the conversation is exactly what the CLI cannot do and an
API client can.

**Revised recommendation.** Drop selective session merging. The ranking is now:

1. **Direct-API provider with explicit `cache_control`** — the only path to the full saving with
   isolation intact. Large, overlaps L4.8, and now the only structural option worth the work.
2. **Trim `.claude/rules/`** — 7,707 tokens on every stage, loaded in full. Unchanged, still the
   cheapest real win.
3. **Reduce stage count** — a stage that does not run pays neither prefix nor output, so not asking
   at all (L3.24's lever) beats making the ask cheaper. *(The reason originally given here — "output
   tokens dominate the bill in both arms" — is corrected in §3: in COLD, cache creation is the
   largest line item. The ranking survives; the reasoning did not.)*
4. **Install less** — still a ~4.6% papercut.

## 7. What this does not establish

- **n=1 per arm**, haiku, one project, twelve turns.
- **The output confound above** is not controlled, and it affects the headline cost ratio.
- Output sizes were *elicited by word count*, not produced by real stage work, so they approximate
  the real run's shape rather than reproducing it.
- The COLD arm's read varies between 17,927 and 121,351 across turns because some stages made
  internal tool calls. That is realistic, and it adds variance the single run cannot separate.
