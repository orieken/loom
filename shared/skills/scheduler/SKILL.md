---
name: scheduler
description: Orchestrates scheduled or hook-driven pipeline runs (cron triggers, automated memory audits, or periodic health checks).
triggers:
  keywords: [scheduler, cron trigger, run scheduled task, periodic audit]
  intentPatterns: ["schedule pipeline run", "execute cron task"]
standalone: true
---

## When To Use

Use when:
- Executing recurring framework checks (e.g., weekly `memory-auditor` or `pipeline-retrospective` runs).
- Configuring background event schedules using the system `schedule` tool.

Do NOT use when:
- Running interactive single-feature deliveries (use `deliver-feature`).

## Context To Load First

1. `shared/hooks/README.md`
2. `shared/telemetry/`

## Process

1. **Read Schedule Specification**: Identify target event, cadence, and action script/agent.
2. **Configure Background Timer**: Invoke system `schedule` tool with explicit prompt or cron expression.
3. **Log Scheduled Event**: Nothing to append. The `.claude/telemetry/events.jsonl` layer was retired in roadmap L3.9 — it had no verified writer and no reader. A scheduled run executed by `loom run` records itself on that run's `run-events.jsonl`; a scheduled run of this skill records nothing.
4. **Report Schedule Configuration**: Output status confirmation report.

## Examples

### Monthly exemplar audit

```markdown
Schedule a monthly exemplar-auditor run on the first of the month at 09:00 UTC.
Findings write to: docs/audits/exemplar-audit-YYYY-MM-DD.md
```

Cron expression: `0 9 1 * *`
Action: invoke `exemplar-auditor` agent
Declared in: `shared/hooks/scheduled-monthly.yaml` (disabled by default, like every entry there)

Worth knowing why this one exists: exemplars are deliberately not gated (roadmap L3.46), so nothing
stops an agent editing one, and this audit is the only thing that notices a degraded exemplar. The
digest warning in `health-check` reports that an exemplar's *file* changed; the auditor is what says
whether the exemplar still holds.

### Weekly documentation-auditor run

```markdown
Schedule a weekly documentation-auditor run every Monday at 00:00 UTC.
Findings write to: docs/audits/doc-audit-YYYY-MM-DD.md
```

Cron expression: `0 0 * * 1`
Action: invoke `documentation-auditor` agent
Output convention: `docs/audits/doc-audit-YYYY-MM-DD.md` (dated findings file)

Copy the expression into your `schedule` tool invocation or `.claude/hooks/` entry.

## Output Format

```markdown
# Scheduler Status Report

## Active Timers / Cron Schedules
- Event: `weekly-memory-audit` | Cadence: `0 0 * * 1` | Action: `memory-auditor`
- Event: `weekly-doc-audit`    | Cadence: `0 0 * * 1` | Action: `documentation-auditor`
- Event: `exemplar-auditor-monthly` | Cadence: `0 9 1 * *` | Action: `exemplar-auditor`
```

## Guardrails

- Never launch mutating background commands without explicit human approval.

## Standalone Mode

Delegates timer management to the `schedule` tool or outputs standard shell instructions.
