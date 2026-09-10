# L3.19a — Where the per-stage prompt tax actually comes from

**Date**: 2026-09-10 · **Method**: controlled ablation against `claude -p`, n=1 per condition
**Cost**: ~$0.85 (8 recorded calls $0.4754, ~6 probe/verification calls unrecorded)
**Model**: `haiku` throughout — token *counts* are what this measures, and haiku makes the
measurement cheap. See §5 for what that limits.

---

## 1. The question

L3.19 says the prompt tax is the largest line item in a run: 74–97k cache-creation tokens per
stage, fifteen stages, ~$0.50–0.74 for a stage that correctly decides it has nothing to do. The item
named two levers — share the cached prefix, and make a decline cheap — and I proposed a third ahead
of both: **install only the agents a plan uses**, on the reasoning that 40 agents and 70 skills
(872KB on disk) are loaded fifteen times and mostly never invoked.

L3.19a exists to check that reasoning before anyone builds on it. It does not survive.

## 2. Method

Cumulative conditions, each a directory containing progressively more of a level-1 install, each
measured with one `claude -p --output-format json` call reading `cache_creation_input_tokens` and
`cache_read_input_tokens` from the envelope.

**A confound was caught and corrected mid-measurement.** The first pass showed agents and skills
costing *zero*, which was wrong: the user's global `~/.claude` already contains all 40 agents and
70 skills, so the "bare" baseline was never bare and the project-level copies were duplicates of
things already loaded. A clean config directory has no credentials, so isolation that way was not
available. The corrected measurement installs the same surface under **non-colliding names**
(`zz-` prefix) and reads the marginal cost, which needs no clean baseline. Registration was
verified rather than assumed — asked for the count, got `69`.

## 3. Result

| Component | On disk | Tokens | Per unit |
|---|---:|---:|---:|
| CLI system prompt + tools (empty directory) | — | **38,519** | — |
| `CLAUDE.md` | 8KB | +1,793 | — |
| `.claude/rules/` (5 core files) | 32KB | **+7,707** | — |
| `ARCHITECTURE_RULES.md` + `DOMAIN_DICTIONARY.md` | 36KB | +21 | ~0 |
| `.claude/agents/` (39) | 320KB | +3,637 | 93/agent |
| `.claude/skills/` (69) | 552KB | +458 | 6.6/skill |
| loom's own per-stage prompt (agent definition + schema) | 17.5KB | +4,573 | — |
| **Total per stage** | | **~56,700** | |

Raw ablation, cumulative (`created` + `read`):

```
c0_bare          20592 + 17927 = 38519
c1_claudemd      22385 + 17927 = 40312
c2_rules         30092 + 17927 = 48019
c3_projectdocs   30113 + 17927 = 48040
c4_agents        30089 + 17927 = 48016
c5_skills        30092 + 17927 = 48019
c6_full          30090 + 17927 = 48017
```

Noise is small enough to trust the deltas: c3 through c6 span **24 tokens (0.05%)** while on-disk
content grows 14x.

## 4. What this changes

**Agent and skill *bodies* never enter the prompt.** 872KB of agents and skills — ~218k tokens if
loaded — costs **4,095 tokens**, under 2% of their size. Only names and descriptions are loaded;
bodies arrive on invocation. `ARCHITECTURE_RULES.md` and `DOMAIN_DICTIONARY.md` are not loaded at
all despite being installed.

**So "install only what a plan uses" is a rounding error.** `deliver-feature` invokes ~12 of 40
agents; dropping the other 28 saves **~2,600 tokens per stage, ~4.6% of the prefix**. That is real
and nearly free to implement, but it is not a lever — I proposed it as one and was wrong.

**The dominant term is the CLI's own baseline, and loom cannot touch it.** 38,519 tokens — **68% of
a stage's prefix** — is the cost of starting a `claude -p` process at all, before any framework
content exists. Across fifteen stages that is **~578,000 tokens of byte-identical repetition per
run**.

**Therefore sharing the prefix across stages is not one lever among three. It is the only one that
addresses the majority term.** Everything controllable — rules, agents, skills, loom's own prompt —
sums to ~18k, and the largest single controllable item is `.claude/rules/` at 7,707, which *is*
loaded in full and is the one place trimming pays per byte.

## 5. What this does not establish

- **n=1 per condition.** Defensible for the prefix (0.05% spread across four conditions) and not
  defensible for anything else here.
- **Measured on haiku.** The CLI baseline may differ by model, and the runs this item cites used
  `inherit`. The 38.5k figure should be re-measured on the run model before it is used in a cost
  projection.
- **~17–40k of the real runs' 74–97k is unaccounted here.** The remainder is the target project's
  own `CLAUDE.md` and rules, the upstream-state section (which grows through a run), tool-use
  iterations, and any model difference. This ablation measures a clean project, not
  `saturday-monorepo`.
- **`created` and `read` are summed as "prefix".** Correct for size, wrong for cost — creation bills
  at 1.25x and read at 0.1x. A cost projection must split them.
- The constant 17,927-token `cache_read` across every condition is an already-cached CLI component.
  It is included in the size totals and is not attributable to anything loom installs.

## 6. Recommended re-ordering of L3.19

1. **(c) Share the prefix across stages** — promoted to first. It is the only lever that reaches the
   68% term. The crux question it must answer: *why is the baseline cache-**creation** rather than
   cache-**read** on stages 2–15 of a run?* If the CLI's cache does not survive across processes,
   the fix is architectural — one process serving many stages.
2. **(d) Cheap decline** — unchanged in position. A skipped stage still pays the full 38.5k baseline
   today, which is precisely why declining cheaply requires not starting the process.
3. **(b) Install less** — demoted to a papercut. ~4.6%, worth doing when convenient, never worth
   scheduling.
4. **Trim `.claude/rules/`** — new, and the only per-byte win available: 7,707 tokens, loaded in
   full, on every stage.
