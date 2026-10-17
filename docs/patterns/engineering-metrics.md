# Engineering Metrics

Which numbers to track for a team that ships with agents, which of them loom already counts, and
which live outside loom. PR count and feature count are not on the list. They measure activity,
not value, and agents make them cheaper to inflate (Goodhart's law).

This file is the rationale. The weekly layout is `shared/templates/weekly-engineering-report.template.md`.
The prompts that fill it are in `docs/prompts/weekly-engineering-report.md`.

---

## Principles

1. **Measure teams and systems. Never measure individuals.** Individual metrics destroy collaboration
   and invite gaming.
2. **Pair metrics that pull against each other.** Speed with stability. Throughput with quality. One
   number alone is gameable; the pair is much harder to fake. `test-suite-health-metrics.md` makes the
   same argument for flake rate and escaped defects.
3. **Track trends against the team's own past.** Do not compare teams to each other.
4. **Never tie a metric to a performance review.** It stops being accurate the day you do.
5. **Numbers come from records. Agents explain them.** A model never computes, estimates or recalls a
   metric. This is the measured/judged split `agent-scorecard` already enforces, applied to the whole
   report.

---

## The metric set

### Delivery (DORA)

| Metric | Why |
|---|---|
| Lead time for changes (first commit to production), p50 and p90 | How fast value reaches users |
| Deployment frequency | Batch size and pipeline health |
| Change failure rate | Share of deploys that cause a rollback, hotfix or incident |
| Time to restore, p50 | Resilience |
| Rework rate (reverted or rewritten within 14 days of merge) | Plausible-looking agent code that did not hold |

Define "failure" before you collect. Rollback, hotfix and incident are three different numerators.

### Flow

Break cycle time into stages: coding, waiting for review, in review, merge to deploy. The largest stage
is the bottleneck, and it is usually the wait for review. Also track median PR size, work in progress
per person, and items older than 14 days. Use PR size as a diagnostic, not a target.

### Quality

Escaped defects, change coverage (coverage of changed lines, not the global figure), mutation score on
changed lines, flaky-test rate, and the share of capacity spent on unplanned work. The four test-suite
numbers and the shape that means a suite is going green and blind are in `test-suite-health-metrics.md`.

### Agentic pipeline

| Metric | Source today |
|---|---|
| Retry rate, failure rate, human-correction rate, latency p50/p95, cost | `loom memory agents --json` (measured) |
| First-pass acceptance (`code-reviewer`) | Same command, derived from retries (measured) |
| Corrections by agent, stages that went round more than N times | `loom memory corrections`, `loom memory retries` |
| Human review minutes per PR | Not collected. Needs PR timeline events |
| Escape rate (defects found after every agent review passed) | Not collected. Needs the bug tracker |
| Gate rejections by cause, including **template gap** | Partly. See `delivery-template.md` |
| Share of rejections traced to a template gap | Not collected |
| Guardrail triggers, and rules with zero triggers in 90 days | Not collected |

A correction is recorded, not adopted. It shows that output needed fixing, not that anything shipped
fixed. Keep that caveat next to the number.

### Outcomes

Output is cheap with agents, so outcomes carry the weight. Record a hypothesis and a target metric on
every work item before it starts. Then track the share of shipped work that moved its target metric,
over a trailing eight weeks. Verdicts: moved, flat, worse, too early.

### Developer experience

Build, test and CI-queue time at p50 and p95. Time to first PR for a new joiner. A short quarterly
survey on feedback loops, cognitive load, flow state and hours lost to friction. Perception data is the
only source for the last three.

---

## What loom collects and what it does not

| Layer | State |
|---|---|
| Agent stages, retries, failures, corrections, latency, cost | Collected by the `loom memory` store |
| Lead time, deploy frequency, change failure, restore time | Not collected. Lives in git, the deploy log and the incident tracker |
| Flow stages, PR size, rework | Not collected. Lives in the PR API and git |
| Escaped defects, flaky rate | Not collected. Lives in the bug tracker and CI history (`test-suite-health-metrics.md` says the same) |
| Outcomes | Not collected. A human supplies the result |
| Developer experience | Not collected |

Most of the table is therefore **judgment-only** under `architecture-guardrails.md` #7: the source
sits in a system loom does not read, so no fitness function can check it today. Say so in the report's
data-quality line. Do not fill a gap with an estimate.

---

## Cautions specific to agentic work

- Throughput gets cheap, so volume is even less informative than before.
- Perceived speed-up can differ from measured speed-up. METR's 2025 randomised study of experienced
  open-source developers reported slower work with early-2025 AI tools while the developers believed
  they were faster. Check the study before citing it. Measure end-to-end cycle time and review burden
  instead of asking how it feels.
- Review burden can absorb the saving. Agents can speed up writing and move the cost to reviewers.
- Capture two or three weeks of pre-agent data before you claim an improvement.
- Benchmarks such as SWE-bench measure models. They do not predict your lead time or defect rate.

---

## Sources

Cited from memory. Check titles, years and figures before quoting them externally.

| Topic | Source |
|---|---|
| DORA metrics | Forsgren, Humble, Kim, *Accelerate* (2018). DORA *State of DevOps* reports (2014 onward), dora.dev |
| SPACE | Forsgren, Storey, Maddila, Zimmermann, Houck, Butler, "The SPACE of Developer Productivity," *ACM Queue* 19(1), 2021 |
| DevEx | Noda, Storey, Forsgren, Greiler, "DevEx: What Actually Drives Productivity," *ACM Queue*, 2023 |
| Gaming of measures | Goodhart (1975); Strathern (1997) |
| Outcomes over output | Seiden, *Outcomes Over Output* (2019); Doerr, *Measure What Matters* (2018) |
| Experiments | Kohavi, Tang, Xu, *Trustworthy Online Controlled Experiments* (2020) |
| Flow metrics | Vacanti, *Actionable Agile Metrics for Predictability* (2015); Kersten, *Project to Product* (2018); Little's Law (1961) |
| Code review latency | Sadowski et al., "Modern Code Review: A Case Study at Google," ICSE-SEIP 2018 |
| Coverage as a quality signal | Inozemtseva, Holmes, "Coverage Is Not Strongly Correlated with Test Suite Effectiveness," ICSE 2014 |
| Mutation testing in industry | Petrović, Ivanković, "State of Mutation Testing at Google," ICSE-SEIP 2018 |
| Cognitive load | Skelton, Pais, *Team Topologies* (2019) |

Known weak points:

- Kohavi's often-quoted split is about one third of ideas improve the target metric, one third are
  neutral and one third hurt it (Microsoft). Bing and Google report 10–20% success. The figure covers
  experimented online features, not all engineering work.
- "Flow efficiency is under 20%" is lean and Kanban folklore. Measure your own.
- The review-size finding (effectiveness drops past about 400 lines) comes from a 2006 vendor study.
  Use it as a heuristic.
- DORA findings are correlational and survey-based. In 2023 McKinsey published a developer
  productivity article, and Beck and Orosz published a widely read rebuttal in *The Pragmatic
  Engineer*. Read both before presenting metrics to leadership.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
