---
name: agent-scorecard
description: Scores each pipeline agent on measured metrics counted by `loom memory agents` (attempts, failures, human corrections, latency, cost) plus judged metrics read from artifacts in docs/features/, compares against the previous month's scorecard to flag improving/degrading agents, and persists the result to docs/agent-metrics/scorecard-YYYY-MM.md.
triggers:
  keywords: ["agent-scorecard", "score agents", "agent metrics", "agent performance"]
  intentPatterns: ["Score the agents", "How are the agents performing", "Generate this month's scorecard", "/agent-scorecard *"]
standalone: true
---

## When To Use
Monthly cadence (manually triggered, or wire up via the host tool's own scheduling capability — e.g. Claude
Code's `schedule` skill — if the user wants it automatic; this is a platform capability, not one of this
repo's `shared/skills/`, so it isn't auto-invoked by `deliver-feature` itself the way `/retrospective` is).
Also on-demand whenever the user asks how a specific agent, or the pipeline generally, is performing.

Do NOT use for a single delivery's story (what happened on this one feature) — use `/retrospective`. Do NOT
use for cross-delivery timing/iteration trends without a quality judgment attached — use
`pipeline-retrospective` (this skill is complementary to it, not a replacement: `pipeline-retrospective` says
*how long/how many retries*, this skill says *was the output actually good*).

## Metric Definitions

Two kinds, and the difference is the point (roadmap L3.13). **Measured** metrics are counted from
records by `loom memory agents`; a model reads the number, it does not derive it. **Judged** metrics
are a model reading artifacts, which is legitimate for questions no counter can answer and is the
weaker evidence of the two. Never present them as the same thing — label every score with its kind.

Don't invent new metrics without updating these tables first: an undocumented metric can't be
tracked for a trend.

### Measured — from `loom memory agents --json`

Every agent that ran gets these. They need no artifact-reading and are available for any delivery
that ran under `loom run`.

| Metric | Field | Underperforming Floor | What it does NOT mean |
|---|---|---|---|
| Retry rate | `retried` / `stages` | > 40% | Not a quality verdict on its own — the review loop sending a stage back IS the pipeline working |
| First-pass acceptance (`code-reviewer`) | `1 - (retried / stages)` on the `code-reviewer` row | < 50% | Every stage in a loop's span carries the same iteration count (`internal/orchestrator/loop.go`, `reenter`), so a `code-reviewer` stage with `iterations > 1` is a round the loop went again — the same fact `changesRequestedCount > 0` recorded, counted instead of read |
| Failure rate | `failed` / `stages` | > 10% | A stage that failed on an invalid payload is a contract problem, not necessarily a bad agent |
| Human-correction rate | `corrections` / `stages` | > 30% | A correction was **recorded, not adopted** (roadmap L4.5) — evidence the output needed fixing, NOT that anything shipped fixed |
| Latency p50 / p95 | `durationP50Ms`, `durationP95Ms` | judgment-only | Nearest-rank over stages that **finished**. Absent (`null`) means none did — never read it as fast |
| Cost / tokens | `costUsd`, `tokens` | judgment-only | Valid only when `costReported` is true. `costReported: false` means the provider reported **nothing**, which is not zero — never average it in |

**Report an absent value as absent.** A `null` percentile and a 0ms percentile are different facts,
and so are "never failed" and "never ran". The guardrail below about `n/a` versus `0%` applies to
every field here.

### Judged — a model reading artifacts

| Agent | Metric | Computed From | Underperforming Floor |
|---|---|---|---|
| `security-reviewer` | True positive rate (proxy) | `security-report.md`: fraction of CRITICAL/HIGH findings with a non-"Recommendation only" `Fix applied` line, adjusted down for any finding a later `retrospective.md` explicitly disputes as a false alarm | < 80% |
| `analyst` | Completeness score | `analysis.md` vs. `shared/contracts/analysis-contract.md`: fraction of required sections present **and** containing real content (not leftover `[...]` template placeholders) | < 90% |
| `architect` | Fitness function coverage | `architecture-notes.md`: fraction of `## Structural Decisions` entries with a concrete `**Fitness Function**` + `**Enforcement**` line, vs. entries explicitly flagged `judgment-only` | < 70% |

**Known limitation**: security-reviewer's "true positive rate" is a proxy, not a real confirmed/false-positive
rate — there's no mechanism yet for a human or downstream agent to formally dispute a finding after the fact.
Treat this metric as directional, not exact, until that dispute-tracking mechanism exists (tracked as an open
item — see `docs/features/context-engineering-framework/TODO.md`, Epic 15).

## Context To Load First
0. **Start with the measured metrics. They are not optional and not a fallback.**

   ```bash
   loom memory agents --json
   ```

   One row per agent with every field in the measured table above, counted from run records. The
   command prints the same figures as a table, with its caveats, if you want to read it directly.

   `loom memory corrections --json` and `loom memory retries --json` remain for the narrower
   questions they answer — which agents a human corrected most, and which individual stages went
   round more than N times.

   **When the store is empty or the binary is absent**, say so in the scorecard's Methodology
   section and score only the judged metrics. Do not reconstruct the measured ones by reading
   markdown: that is the substitution this item removed, and a derived figure presented in a
   measured column is worse than a missing one.

1. `docs/features/*/delivery-summary.md` — determine which features fall in the scoring period
2. `docs/features/*/pipeline-trace.json` — model-written estimates, and now only for deliveries that
   did NOT run under the executor. `changesRequestedCount` and `estimatedCostUsd` both have measured
   counterparts in step 0; prefer those whenever the store has the run
3. `docs/features/*/security-report.md`, `analysis.md`, `architecture-notes.md` — per-agent artifacts
4. `docs/features/*/retrospective.md` (if present) — for disputed-finding signals
5. `shared/contracts/analysis-contract.md` — required section list for the analyst completeness score
6. The most recent `docs/agent-metrics/scorecard-*.md` (if one exists) — for trend comparison

## Process
1. **Determine scope**: all features in `docs/features/` whose `delivery-summary.md` falls in the current
   calendar month. If fewer than 3 features exist this month, widen to the last 3 delivered features instead
   (say so explicitly in the output — don't silently score on 1 data point).
2. **Compute each metric** per the table above, across every feature in scope.
3. **Compare to the previous scorecard** (`docs/agent-metrics/scorecard-<prior-YYYY-MM>.md`), if one exists.
   Trend = `IMPROVING` / `STABLE` / `DEGRADING` (>10 percentage-point swing = not stable; smaller threshold
   than `pipeline-retrospective`'s 15% because these are already rate/percentage metrics, not raw durations).
4. **Flag underperforming agents** — any metric below its floor in the table above.
5. **Write the scorecard** to `docs/agent-metrics/scorecard-[YYYY-MM].md`.

## Output Format
```markdown
# Agent Scorecard: [YYYY-MM]

## Scope
- Features scored: [N] — [list feature names]
- Period: [start date] to [end date]
- Note: [if scope was widened due to <3 features this month]

## Measured Metrics
> Counted by `loom memory agents` from run records. No model derived these.

| Agent | Stages | Retry rate | Failure rate | Correction rate | p50 | p95 | Status |
|---|---|---|---|---|---|---|---|
| [agent] | [N] | [X%] | [X%] | [X%] | [Ns] / — | [Ns] / — | OK/UNDERPERFORMING |

- Corrections were **recorded, not adopted** — evidence an agent's output needed fixing, not that
  anything shipped fixed.
- An em dash is a value that was never measured, not a zero. Report it as such; do not fill it in.
- [Omit the cost columns, or state `costReported: false`, when no run reported usage — unmeasured,
  not free.]

## Judged Metrics
> A model reading artifacts. Weaker evidence than the table above; kept for the questions no counter
> answers.

| Agent | Metric | Score | Prior Month | Trend | Status |
|---|---|---|---|---|---|
| security-reviewer | True positive rate (proxy) | [X%] | [Y%] / n/a | IMPROVING/STABLE/DEGRADING/n/a | OK/UNDERPERFORMING |
| analyst | Completeness score | [X%] | [Y%] / n/a | ... | ... |
| architect | Fitness function coverage | [X%] | [Y%] / n/a | ... | ... |

## Underperforming Agents
- [agent]: [metric] at [X%], below the [floor]% floor. [Brief diagnosis grounded in the actual artifacts —
  e.g. "3 of 5 features this month had analysis.md missing a non-empty Edge Cases and Risks section"]
— or "None this period"

## Cost Summary (when data available)
| Agent | Total Cost (USD) | Tokens |
|---|---|---|
| analyst | $[N] | [N] |
| ... | ... | ... |

> Populated from `loom memory agents --json`, and only for agents whose `costReported` is true.
> Omit the section entirely when no agent reported usage — a table of zeroes asserts the pipeline
> was free, which is a different claim from having no figures. Not a scored metric. For trend
> analysis across periods, see `pipeline-retrospective`.

## Methodology & Known Limitations
- Measured metrics: counted by `loom memory agents` over [N] ingested runs. Judged metrics: a model
  reading artifacts from [N] features.
- [Say so if the store was empty or `loom` was absent, and that only judged metrics were scored.]
- [Restate the security-reviewer proxy caveat if it's scored this period]
- [Any other scope caveats, e.g. widened window]

## Recommendations
- [Specific, tied to a specific metric/agent — e.g. "Review analyst.md's Output Format template; the Edge
  Cases section may need a stronger prompt nudge since it's the most frequently thin section"]
```

## Guardrails
- **Never** score an agent that wasn't invoked in any feature this period — report `n/a`, not `0%`
  (architect and security-reviewer are conditional; a period with no architecturally-flagged features
  should not show architect as "failing").
- **Never** fabricate a prior-month comparison if no prior scorecard exists — say "n/a, first scorecard".
- **Never** compute a measured metric by reading markdown when the store is unavailable. Report it
  missing and say why. A derived figure in a measured column claims a provenance it does not have,
  which is the defect roadmap L3.13 exists to remove.
- **Never** present a judged metric as measured, or merge the two tables. The distinction is the
  reader's only way to know how much weight a number carries.
- **Never** silently change a metric's definition or floor between runs — if you believe a metric needs to
  change, say so explicitly and note it breaks trend continuity with prior scorecards.
- This is a read-only analysis — it does not modify any agent prompt, contract, or artifact.

## Standalone Mode
Local file reads and aggregation, plus `loom memory agents` when the binary is present. No external
calls. Without `loom`, the judged metrics still score and the measured ones are reported missing.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
