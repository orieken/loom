# shared/orchestration/ — AOS Orchestration Runtime

**Status**: AOS Phase 3 (v3.2) — opt-in. Teams that don't invoke `/orchestrate` see no change from v3.1.

## What This Is

The orchestration runtime is a thin wrapper that gives first-class support to:

- **Resumable pipelines**: replay from a checkpoint after an interruption (builds on the existing `pipeline-state.json` infrastructure in `deliver-feature`)
- **Parallel branches**: run independent pipeline steps concurrently (e.g., `security-reviewer` + `accessibility-engineer` in parallel, then merge before QA)
- **Workflow definitions**: declarative pipeline definitions that can be composed, versioned, and audited

Skills like `deliver-feature` continue to work identically without the runtime. The runtime wraps them — it doesn't replace them.

## Backward-Compatibility Guarantee

A team that:
- does NOT invoke `/orchestrate`
- does NOT set `orchestrationMode: "runtime"` in `.claude/delivery-policy.yaml`
- does NOT adopt a `FeatureDeliveryWorkflow` object

…sees **zero behavior change from v3.1**. The runtime is pure addition.

## How Workflows Plug In

See [`interface.md`](interface.md) for the Workflow registration contract.

The built-in Phase 3 workflow is:
- `FeatureDeliveryWorkflow` — wraps `deliver-feature` (see `shared/workflows/feature-delivery-workflow.md`)

A second, a Red-Green-Refactor loop, was retired with ADR-009 (2026-09-22).

## Declarative Pipelines

See [`pipeline-schema.md`](pipeline-schema.md) for the YAML pipeline definition format.

## Audit-After-Producer (Default Composition)

Every workflow stage that ends with a contract-bound artifact automatically invokes the corresponding
Phase 2 counter agent before the pipeline proceeds. Failures send the artifact back to the producer
with specific violations.

To disable per-project: `workflowAuditsEnabled: false` in `.claude/delivery-policy.yaml`.

See [`audit-composition-pattern.md`](audit-composition-pattern.md) for the full producer→auditor
mapping, retry protocol, and config knob reference.

## File Map

```
shared/orchestration/
├── README.md                     ← this file
├── interface.md                  ← Workflow plug-in contract
├── pipeline-schema.md            ← declarative pipeline format
└── audit-composition-pattern.md  ← default audit-after-producer pattern + config knob
```

Skills and agents:
- `shared/skills/orchestrate/SKILL.md` — the `/orchestrate` invocation entry point

Workflows (added by Ops 3.11-3.12):
- `shared/workflows/feature-delivery-workflow.md`

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 3 Runtime layer. CC BY 4.0.*
