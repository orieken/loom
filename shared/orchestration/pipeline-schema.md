# Declarative Pipeline Schema

Workflows in `shared/workflows/` and `docs/workflows/` use this schema.

> **Note (roadmap L2.9).** The *artifact* schemas for typed stages are no longer prose: they are
> generated from Go structs in `internal/state/` into `shared/schemas/pipeline/`, and a test fails
> if the committed copies drift. The workflow schema described in this file — stages, roles,
> audits, checkpoints — is still hand-maintained; generating it belongs to the planner/router work
> (L3.1), not here. What a stage `produces` is the contract's filename either way, since typed
> stages render their markdown view under that name.

## Full Example

```yaml
---
workflow: feature-delivery
version: 1.0.0
description: Full feature delivery pipeline — analyst → architect → developer → review → QA → ship
entry: deliver-feature
resumable: true
parallelStrategy: sequential-simulation
checkpointStore: .claude/feature-workspace/<feature-name>/pipeline-state.json
legacyFallback: deliver-feature
stages:
  - id: context
    role: context-engineer
    produces: context-manifest.md
    parallel: false
    checkpoint: true
    audit:
      agent: context-auditor
      onFail: retry
      maxRetries: 3

  - id: analysis
    role: analyst
    produces: analysis.md
    parallel: false
    checkpoint: true
    audit:
      agent: context-auditor
      onFail: retry
      maxRetries: 3

  - id: architecture
    role: architect
    produces: architecture-notes.md
    parallel: false
    checkpoint: true
    condition: "analysis.architecturalFlags != 'None'"

  - id: development
    role: developer
    produces: implementation-notes.md
    parallel: false
    checkpoint: true

  - id: code-review
    role: code-reviewer
    produces: code-review-report.md
    parallel: false
    checkpoint: true

  - id: security
    role: security-reviewer
    produces: security-report.md
    parallel: true
    checkpoint: true

  - id: accessibility
    role: accessibility-engineer
    produces: accessibility-report.md
    parallel: true
    checkpoint: true
    condition: "feature.hasUI == true"

  - id: qa
    role: qa-engineer
    produces: qa-report.md
    parallel: false
    checkpoint: true

  - id: tech-writer
    role: tech-writer
    produces: docs-report.md
    parallel: false
    checkpoint: true

  - id: devops
    role: devops-engineer
    produces: devops-report.md
    parallel: false
    checkpoint: true
---
```

## Field Reference

### Top-Level Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `workflow` | string | yes | — | Unique kebab-case identifier |
| `version` | semver | yes | — | Semantic version of this workflow definition |
| `description` | string | yes | — | One-line summary |
| `entry` | string | yes | — | Skill or agent name teams invoke directly (`/deliver-feature`, not `/orchestrate`) |
| `resumable` | boolean | no | `true` | Whether checkpointed state allows resume on interruption |
| `parallelStrategy` | enum | no | `sequential-simulation` | `fork` (real parallel) or `sequential-simulation` (LLM simulates; default) |
| `checkpointStore` | path | no | `.claude/feature-workspace/<feature-name>/pipeline-state.json` | Where to persist state |
| `legacyFallback` | string | no | — | Skill to invoke on `--legacy` flag; mandatory for wrappers of existing skills |

### Stage Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | string | yes | — | Unique stage identifier within this workflow |
| `role` | string | yes | — | Agent or skill that executes this stage |
| `produces` | string | yes | — | Artifact filename produced; must match a contract in `shared/contracts/` (or be documented as unconstrained) |
| `parallel` | boolean | no | `false` | Adjacent `parallel: true` stages run concurrently |
| `checkpoint` | boolean | no | `true` | Write pipeline state after this stage completes |
| `condition` | expression | no | — | If present and evaluates false, stage is skipped; string evaluated at runtime |
| `audit.agent` | string | no | — | Counter agent to invoke after this stage |
| `audit.onFail` | enum | no | `halt` | `retry`, `halt`, or `skip` |
| `audit.maxRetries` | integer | no | `3` | Max retry count before falling back to `halt` |

## Condition Expressions

Conditions are simple dot-path equality checks evaluated against the pipeline context:

```
"analysis.architecturalFlags != 'None'"   # check analysis.md architectural flags
"feature.hasUI == true"                    # check feature spec metadata
"policy.autoProceed == true"               # check delivery policy
```

Condition expression language is intentionally minimal — no loops, no function calls, no side effects.

## `sequential-simulation` vs `fork`

- **`sequential-simulation`** (default): the LLM invokes parallel stages sequentially but treats them as logically parallel — each stage starts without reading the others' in-progress output. This avoids agent spawning overhead and works in single-agent environments. Correct for most cases.
- **`fork`**: spawns sub-agents (via Agent tool). Use only when stages are genuinely independent and the overhead of spawning is worth the wall-clock speedup.
