# Audit-After-Producer Composition Pattern

**Status**: Default for all workflow stages in AOS Phase 3 (v3.2).

## What This Is

Every workflow stage that ends with a contract-bound artifact automatically invokes the corresponding
counter agent from Phase 2 (Op 2.1) before the pipeline proceeds. This is the "workflow-invokes-audit-
after-producer" pattern — the default composition behavior for all workflows in `shared/workflows/`.

## Default Behavior

When a workflow stage has an `audit` block and the config knob `workflowAuditsEnabled` is not set to
`false`, the `/orchestrate` runtime executes:

```
Producer stage finishes → artifact written → auditor invoked → result evaluated
```

**On PASS**: pipeline proceeds normally.
**On FAIL**:
- `retry`: artifact sent back to producer with specific violations; attempt again up to `maxRetries`
- `halt`: block pipeline, surface audit findings to human for resolution
- `skip`: log violation as audit debt, continue (use sparingly)

## Producer → Auditor Mapping

| Producer Role | Artifact | Default Auditor | onFail |
|---|---|---|---|
| context-engineer | context-manifest.md | context-auditor | retry |
| analyst | analysis.md | context-auditor | retry |
| architect | architecture-notes.md | rule-auditor | halt |
| developer | implementation-notes.md | code-reviewer | retry |
| code-reviewer | code-review-report.md | *(no audit — code-reviewer IS an auditor)* | n/a |
| security-reviewer | security-report.md | privacy-auditor | halt |
| accessibility-engineer | accessibility-report.md | documentation-auditor | skip |
| qa-engineer | qa-report.md | tool-validator | retry |
| tech-writer | docs-report.md | documentation-auditor | retry |
| unit-tester (red) | test suite | tool-validator | retry |
| developer (refactor) | refactored code | code-reviewer | retry |

## Config Knob

To disable audit invocation for a project:

```yaml
# .claude/delivery-policy.yaml
workflowAuditsEnabled: false
```

When `workflowAuditsEnabled: false`, the runtime skips all `audit:` blocks in workflow stages.
Producers still run; auditors are simply not invoked. This is a project-level override — it
does not affect teams that don't set the flag (default is "audits run").

To disable per-stage (more granular):

```yaml
# In a project-specific workflow override at docs/workflows/<name>.md
stages:
  - id: security
    audit:
      enabled: false   # disable audit for this stage only
```

## Auditor-on-Failure Protocol

When an auditor returns FAIL on a `retry` stage:

1. The auditor's findings report is written to `.claude/feature-workspace/<stage>-audit-findings.md`.
2. The producer is reinvoked with the findings as additional context:
   > "Your previous `<artifact>` failed the `<auditor>` audit. The specific violations are: [findings]. Fix these violations and produce a new `<artifact>`."
3. The retry counter is incremented. On `maxRetries` exhausted: fall back to `halt`.
4. Neither the findings nor the retry are recorded anywhere. The `audit.fail` / `audit.retry` / `audit.halt` events this step described were specified and never emitted, and the file they targeted was retired in roadmap L3.9. Giving audit composition a real execution record is **L3.12**
   (event type: `audit.fail`, `audit.retry`, `audit.halt`).

## Why This Is the Default

Audit-after-producer prevents the accumulation of undetected quality violations across pipeline stages.
Without it, a context manifest that misses key bounded-context files propagates uncorrected through
analysis → architecture → implementation, where it becomes expensive to fix. With it, the auditor
catches the miss at the earliest possible stage (context), when correction is cheap (re-run context-
engineer with specific instructions).

This is the same design principle as "fail fast" in resilience engineering: surface violations at the
narrowest scope, closest to the source.

The `onFail: skip` option exists for non-blocking auditors (e.g., accessibility on a non-UI feature
where the auditor returns UNCONFIGURED). It is not an invitation to suppress findings; it is a
mechanism for graceful degradation when the auditor legitimately cannot evaluate the artifact.

## Relationship to validate-artifact

`validate-artifact` checks **structural compliance** against a contract file. The audit-after-producer
pattern invokes a **counter agent** that applies qualitative judgment. These are complementary:

- `validate-artifact` answers: "Does this artifact have the required sections in the right format?"
- Counter agent answers: "Does this artifact's content meet the quality bar for its domain?"

A passing structural check does not guarantee a passing audit. A failing structural check short-circuits
before the audit runs (no point auditing a malformed artifact).

## References

- `shared/orchestration/interface.md` — audit block schema in the Workflow contract
- `shared/orchestration/pipeline-schema.md` — `audit.*` field reference
- `shared/workflows/feature-delivery-workflow.md` — producer→auditor assignments in action
- `docs/aos/governance-pairs.md` — all 15 governance pairs (the source of the mapping above)
