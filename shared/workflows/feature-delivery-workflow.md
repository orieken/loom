---
workflow: feature-delivery
version: 1.0.0
description: Full feature delivery pipeline — context-engineer → analyst → architect → developer → review → QA → ship
entry: deliver-feature
resumable: true
parallelStrategy: sequential-simulation
checkpointStore: .claude/feature-workspace/<feature-name>/pipeline-state.json
legacyFallback: deliver-feature
---

# FeatureDeliveryWorkflow

Defines the state machine for `deliver-feature`. The skill (`shared/skills/deliver-feature/SKILL.md`)
remains the **external invocation contract** — teams keep typing `/deliver-feature <spec>`. This
workflow is the internal structure the `/orchestrate` runtime executes when `deliver-feature` is
invoked through the runtime path.

**External contract unchanged**: teams that type `/deliver-feature <spec>` directly continue to work
identically to v3.1. This workflow only activates when `/orchestrate --workflow feature-delivery` is
called or when `orchestrationMode: "runtime"` is set in `.claude/delivery-policy.yaml`.

## Legacy Fallback

`/orchestrate --legacy --workflow feature-delivery --spec <file>` routes to `deliver-feature` directly,
bypassing this workflow entirely. This fallback persists through v3.x.

## Named Internal Roles

| Role Name | Played by | Stage ID |
|---|---|---|
| Context Scoper | `context-engineer` | `context` |
| Analyst | `analyst` | `analysis` |
| Architect | `architect` | `architecture` |
| Performance Engineer | `performance-engineer` | `performance` |
| Data Engineer | `data-engineer` | `data` |
| Developer | `developer` | `development` |
| Code Reviewer | `code-reviewer` | `code-review` |
| Accessibility Engineer | `accessibility-engineer` | `accessibility` |
| Security Reviewer | `security-reviewer` | `security` |
| QA Engineer | `qa-engineer` | `qa` |
| Visual QA Engineer | `visual-qa-engineer` | `visual-qa` |
| SRE Engineer | `sre-engineer` | `observability` |
| Tech Writer | `tech-writer` | `docs` |
| DevOps Engineer | `devops-engineer` | `devops` |

## Stage Definitions

```yaml
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
    audit:
      agent: rule-auditor
      onFail: halt

  - id: performance
    role: performance-engineer
    produces: performance-report.md
    parallel: false
    checkpoint: true
    condition: "analysis.hasPerformanceSLAs == true"

  - id: data
    role: data-engineer
    produces: data-engineering-notes.md
    parallel: false
    checkpoint: true
    condition: "analysis.dataModelChanges != 'None'"

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

  - id: accessibility
    role: accessibility-engineer
    produces: accessibility-report.md
    parallel: true
    checkpoint: true
    condition: "feature.hasUI == true"
    audit:
      agent: documentation-auditor
      onFail: skip

  - id: security
    role: security-reviewer
    produces: security-report.md
    parallel: true
    checkpoint: true
    audit:
      agent: privacy-auditor
      onFail: halt

  - id: qa
    role: qa-engineer
    produces: qa-report.md
    parallel: false
    checkpoint: true

  - id: visual-qa
    role: visual-qa-engineer
    produces: visual-qa-report.md
    parallel: false
    checkpoint: true
    condition: "feature.hasUI == true && project.hasHeatmapData == true"

  - id: observability
    role: sre-engineer
    produces: observability-report.md
    parallel: false
    checkpoint: true

  - id: docs
    role: tech-writer
    produces: docs-report.md
    parallel: false
    checkpoint: true
    audit:
      agent: documentation-auditor
      onFail: retry
      maxRetries: 2

  - id: devops
    role: devops-engineer
    produces: devops-report.md
    parallel: false
    checkpoint: true
```

## Resumability

Checkpoints are written after every stage that has `checkpoint: true`. If the pipeline is interrupted
(network failure, context limit, explicit user pause), resume with:

```
/orchestrate --workflow feature-delivery --spec <file> --resume
```

The runtime reads `.claude/feature-workspace/<feature-name>/pipeline-state.json` and skips already-completed stages.

## Audit-After-Producer Composition

Stages that have an `audit` block automatically invoke the corresponding counter agent after the
producer finishes. On FAIL:
- `retry`: send artifact back to producer with violations; max `maxRetries` attempts
- `halt`: block pipeline, surface findings to human
- `skip`: log violation, continue (creates audit debt — use sparingly)

This default can be disabled per-project:
```yaml
# .claude/delivery-policy.yaml
workflowAuditsEnabled: false
```

## Explicit Stage Boundaries for Phase 4 Policy Hooks

Each stage is a named, typed boundary where Phase 4 policy hooks can evaluate:
- `context` → context-auditor finding → policy: "if context-manifest is PASS and KI coverage ≥ 80%, auto-proceed"
- `analysis` → analyst output → policy: "if scope is small (< 5 files) and no architectural flags, auto-proceed"
- `code-review` → review verdict → policy: "if APPROVED and diff < 200 LOC, auto-proceed"

Policy hook evaluation is deferred to Phase 4 (v3.3). The stage boundaries are defined here now so
Phase 4 hooks don't require structural changes to this workflow.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 3 Runtime layer. CC BY 4.0.*
