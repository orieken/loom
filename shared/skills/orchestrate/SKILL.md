---
name: orchestrate
description: AOS Phase 3 runtime entry point. Loads a Workflow definition and executes its stages with checkpoint support, parallel branch handling, and automatic audit invocation. Teams that type /deliver-feature continue to work identically — /orchestrate is the opt-in runtime path.
triggers:
  keywords: ["orchestrate", "run workflow", "pipeline runtime"]
  intentPatterns: ["/orchestrate *", "run with orchestration", "orchestrate feature delivery"]
standalone: true
---

## When To Use

Use when you want:
- **Resumable runs**: pick up a pipeline from its last checkpoint after an interruption
- **Parallel stages**: run independent stages concurrently (security-reviewer + accessibility-engineer)
- **Automatic auditing**: counter agents invoked after each producing stage
- **Policy-driven auto-proceed**: skip human gates when `.claude/delivery-policy.yaml` allows

Do NOT use when:
- You want the standard, human-gated pipeline — use `/deliver-feature` directly (unchanged from v3.1).
- You haven't set up a workflow definition — `/orchestrate` requires a workflow file to load.

## Invocation

```
/orchestrate --workflow <workflow-id> [--spec <feature-file>] [--resume] [--legacy]
```

| Flag | Effect |
|---|---|
| `--workflow <id>` | Load `shared/workflows/<id>.md` or `docs/workflows/<id>.md` (project takes precedence) |
| `--spec <file>` | Feature spec file to pass to the entry skill (e.g. `features/user-auth.md`) |
| `--resume` | Resume from last checkpoint in `pipeline-state.json` (same as checking for existing state) |
| `--legacy` | Invoke `legacyFallback` skill directly, bypassing the workflow runtime entirely |
| `--dry-run` | Print the stage execution plan without running any stages |

## Context To Load First

1. The workflow file: `shared/workflows/<workflow-id>.md` or `docs/workflows/<workflow-id>.md`
2. `shared/orchestration/interface.md` — workflow contract
3. `shared/orchestration/pipeline-schema.md` — field reference
4. `checkpointStore` file (default `.claude/feature-workspace/<feature-name>/pipeline-state.json`) — if it exists and `--resume` is set or `resumable: true`

## Process

### 0. Validate and Load

1. Read the workflow file. Confirm it has required frontmatter: `workflow`, `version`, `entry`, `stages`.
2. If `--legacy` is set: invoke `legacyFallback` skill directly and exit. This is the behavior-preservation escape hatch.
3. If `--dry-run`: print the stage execution plan (ordered stages, parallel groups, audit assignments) and exit.
4. **Context-engineer gate**: for any workflow whose `type` is `feature-delivery` or `refactoring`,
   verify that the first non-setup stage (i.e. the first stage with `checkpoint: true` or `role` set)
   has `role: context-engineer`. If not, halt with:
   > "Workflow '<id>' is missing context-engineer as its first stage. Add a `context-engineer` stage
   > before '<first-stage-id>' and re-run. Context scoping is mandatory for feature-delivery and
   > refactoring workflows."
   Workflows of other types (e.g. `maintenance`, `report`) are exempt from this check.

### 1. Resume Check

4. Read `checkpointStore`. If a state file exists for this workflow:
   - If `resumable: true` and `--resume` is set (or user did not explicitly say "start over"): resume from `lastCompletedStep` — skip already-completed stages.
   - If user says "start fresh": archive old state to `.history/pipeline-state.<timestamp>.json` and start from stage 0.

### 2. Execute Stages

For each stage in order:

5. **Condition check**: if the stage has a `condition` expression, evaluate it against the pipeline context. If false: mark stage SKIPPED, log to `pipeline-trace.json`, proceed to next stage.

6. **Parallel group**: collect adjacent `parallel: true` stages into a group. Execute them via `parallelStrategy`:
   - `sequential-simulation`: execute each in sequence but without reading each other's in-progress output
   - `fork`: spawn sub-agents (via Agent tool) and merge results when all complete

7. **Invoke role**: call the stage's `role` (agent or skill) with the feature context.

8. **Audit** (if defined): after the producer completes, invoke `audit.agent`. On FAIL:
   - `retry`: send artifact back to producer with specific violations; increment retry count; halt at `maxRetries`
   - `halt`: block pipeline, surface audit findings to human
   - `skip`: log violation, continue (use sparingly — creates audit debt)

9. **Checkpoint**: if `checkpoint: true`, write updated `pipeline-state.json` and `pipeline-trace.json`.

### 3. Complete

10. Write delivery summary to `.claude/feature-workspace/<feature-name>/delivery-summary.md`.
11. Persist all artifacts to `docs/features/<feature-name>/` (if this is a feature workflow).
12. Log `workflow.completed` telemetry event.

## Output

At the end of each stage, print:
```
[orchestrate] Stage N/M: <stage-id> — <role> → <produces> [PASS | FAIL | SKIPPED | AUDITED]
```

On completion:
```
[orchestrate] Workflow <workflow-id> complete in Ns. N stages executed, N audits passed, N checkpoints saved.
```

## Guardrails

- Never bypass a non-negotiable human gate (Gate #1 Friday ship, Gate #3 DB migrations, Gate #4 Contracting, Gate #5 External APIs, Gate #8 Deploy) — even in policy-driven mode. These are defined in `shared/rules/approval-gates.md`.
- Never execute a `feature-delivery` or `refactoring` workflow whose first executable stage is not `context-engineer`. The validation in step 4 enforces this at load time — do not bypass it via `--legacy` to skip context scoping.
- `--legacy` always works and routes to the skill named in `legacyFallback`. Never remove this escape hatch.
- On any unhandled error in a stage: checkpoint current state, surface the error, and stop. Never silently skip.
- Parallel execution never shares in-progress state between concurrent stages.

## Standalone Mode

Reads workflow definition files and delegates to agents/skills. No external services required.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 3 Runtime layer. CC BY 4.0.*
