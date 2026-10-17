# Engineering Report: week of [YYYY-MM-DD] ([team or repo])

Generated: [timestamp] | Data window: [start] to [end] | Prior report: [link or "first report"]
Data quality: [each source that was missing, partial or estimated — or "none"]

> **Measured** tables are counted from records by a collector. No model derived them.
> **Written** sections are a model reading the tables. Every claim cites a table row or a source file.
> A value that was never measured is an em dash, not a zero.

## 1. Headline (written, max 5 lines)
- [What improved, what degraded, the one thing to look at. Cite the row for each.]

## 2. Delivery (measured)
| Metric | This week | 4-week median | Trend | Source |
|---|---|---|---|---|
| Lead time for changes, p50 / p90 | | | | git + deploy log |
| Deployment frequency | | | | deploy log |
| Change failure rate | | | | deploy log + incident tracker |
| Time to restore, p50 | | | | incident tracker |
| Rework rate (reverted or rewritten within 14 days) | | | | git |

## 3. Flow (measured)
| Stage | p50 | p90 | Share of cycle time |
|---|---|---|---|
| Coding (first commit to PR open) | | | |
| Waiting for review (PR open to first review) | | | |
| In review (first review to approval) | | | |
| Merge to deploy | | | |

Median PR size: [lines] | Work in progress per person: [n] | Items older than 14 days: [n]

## 4. Quality (measured)
- Escaped defects traced to this week's changes: [n]
- Change coverage: [%] | Mutation score on changed lines: [%]
- Flaky-test rate: [%] | CI failures not caused by code: [%]
- Unplanned work, share of capacity: [%]

## 5. Agentic pipeline (measured)
> From `loom memory agents --json`. Rows marked "not collected" stay as em dashes until a source exists.

| Metric | This week | 4-week median | Notes |
|---|---|---|---|
| Runs started / completed / abandoned | | | |
| Retry rate, failure rate, human-correction rate (per agent) | | | |
| First-pass acceptance (`code-reviewer`) | | | |
| Latency p50 / p95 (slowest 3 stages) | | | |
| Cost per accepted change (failed runs included) | | | Only when `costReported` is true |
| Gate rejections by cause (security / architecture / test quality / template gap / other) | | | |
| Share of rejections traced to a template gap | | | |
| Human review minutes per PR | — | — | Not collected |
| Escape rate (defects after every agent review passed) | — | — | Not collected |
| Guardrail triggers, and rules with zero triggers in 90 days | — | — | Not collected |

## 6. Outcomes (measured result, human-supplied)
| Shipped item | Hypothesis (written before the work) | Target metric | Result | Verdict |
|---|---|---|---|---|
| | | | | moved / flat / worse / too early |

Share of shipped work that moved its target metric (trailing 8 weeks): [%]

## 7. Developer experience (survey; quarterly, optional weekly pulse)
- Build time, test time, CI queue time: p50 / p95
- Time to first PR for new joiners
- Pulse, 1–5: feedback loops, cognitive load, flow state, hours lost to friction

## 8. Analysis (written)
- **Biggest bottleneck:** [stage, evidence]
- **Rejection patterns:** [recurring causes, each linked to a spec, prompt or rule gap]
- **Outliers worth a human look:** [runs or PRs, with links]
- **Looks good, may not be:** [for example acceptance up while review burden or rework is also up]
- **Unknowns this week:** [what the data cannot show]

## 9. Proposed changes (written; a human approves, nothing here is applied)
| # | Proposal | Evidence | Expected effect | Metric to watch | Approved? |
|---|---|---|---|---|---|
| | | | | | |

## 10. Last week's proposals
| Proposal | Applied? | Metric before → after | Verdict (worked / no effect / inconclusive) |
|---|---|---|---|
| | | | |

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
