# Orchestration Workflow Interface

## Workflow Contract

A Workflow is a document in `shared/workflows/` (framework built-in) or `docs/workflows/` (project-specific) that conforms to this interface. The `/orchestrate` skill reads it at invocation time.

### Required Fields (frontmatter)

```yaml
---
workflow: <workflow-id>          # kebab-case identifier, unique across shared/workflows/ and docs/workflows/
version: <semver>                # e.g. "1.0.0"
description: <one-line summary>
entry: <skill or agent name>     # the thin-caller skill/agent that teams invoke directly
stages:                          # ordered list of pipeline stages
  - id: <stage-id>
    role: <agent-name>           # agent or skill that executes this stage
    produces: <artifact-name>    # output artifact (matches a contract file in shared/contracts/)
    parallel: false              # true = can run concurrently with other parallel:true stages
    checkpoint: true             # true = save pipeline-state.json after this stage completes
    audit:                       # optional: counter agent to invoke after this stage
      agent: <counter-agent>
      onFail: retry | halt | skip  # what happens when auditor returns FAIL
      maxRetries: 3
---
```

### Optional Fields

```yaml
resumable: true           # default true; false = pipeline must restart from scratch on interruption
parallelStrategy: fork    # fork (spawn sub-agents) | sequential-simulation (default; LLM simulates parallel)
checkpointStore: .claude/feature-workspace/<feature-name>/pipeline-state.json  # where state is persisted
legacyFallback: <skill>   # if set, /orchestrate --legacy routes here instead of the workflow
```

## How `/orchestrate` Uses This

1. Load the workflow file matching the `--workflow <id>` argument.
2. Read `checkpointStore` — if a checkpoint exists and `resumable: true`, resume from `lastCompletedStep`.
3. Execute stages in order. For `parallel: true` stages adjacent to each other, execute them concurrently (via `parallelStrategy`).
4. After each `checkpoint: true` stage, write updated state.
5. If an `audit` is defined for a stage, invoke it after the producer finishes. On FAIL: apply `onFail` strategy.
6. On completion, write final state and surface a delivery summary.

## Built-In Workflows

| Workflow ID | File | Entry Skill/Agent |
|---|---|---|
| `feature-delivery` | `shared/workflows/feature-delivery-workflow.md` | `deliver-feature` |
| `tdd` | `shared/workflows/tdd-workflow.md` | `test-driven-developer` |

## Custom Workflows

Teams can define project-specific workflows in `docs/workflows/`. The format is identical.
Custom workflows override built-in ones if they share the same `workflow` ID.

## Legacy Fallback

Every workflow that wraps an existing skill MUST define `legacyFallback`. Teams can route to the
pre-workflow behavior via `/orchestrate --legacy --workflow <id>` through v3.x without waiting for
a framework patch.

## Fitness Function

The following check can be added to CI to enforce workflow conformance:

```bash
# verify every workflow file has required frontmatter fields
for f in shared/workflows/*.md docs/workflows/*.md 2>/dev/null; do
  python3 scripts/validate-frontmatter.py "$f" --required workflow version entry stages
done
```
