# Automated Deliver-Feature Workflow Design

> **Superseded in part by [ADR-010](../adrs/ADR-010-factory-order-state-and-policy-configuration.md)
> (decision 4B, 2026-10-24).** The markdown pipeline no longer auto-proceeds, and
> `.claude/delivery-policy.yaml` no longer has `mode`, `autoProceed*`, `gates`, `stages`,
> `escalations` or `defaults`. Automation lives only in `.claude/policies/*.policy.yaml`, evaluated by
> `loom run`, which records every decision. Read the rest of this document as history.

> **Goal**: Evolve the `/deliver-feature` pipeline from prose-instructed human PAUSE checkpoints to policy-driven graduated automation while preserving the framework's core discipline ("Stage-3 excellence over Stage-4 fantasy").

---

## 1. Vision & Automation Tiers

Fully removing human oversight trades engineering discipline for raw throughput. This framework adopts **policy-driven graduated automation**, where pipeline stages have distinct risk profiles and project-level policies control when auto-approval is permitted.

### Scope of Automation Tiers

- **Tier A — Auto-Continue on Green (Scoped for v1)**:
  If a stage's output passes contract validation (`validate-artifact`), all fitness functions pass, diff size is below a policy threshold, and no risk flags are raised, the pipeline automatically proceeds to the next step without pausing for manual confirmation.
- **Tier B — Auto-Retry on Structural Contract Failure (Scoped for v1)**:
  If `validate-artifact` rejects an artifact for missing structural contract sections, the pipeline automatically re-invokes the producing agent with the explicit list of section violations (up to $N=3$ attempts) before prompting a human operator.
- **Tier C — Unattended-Until-Blocked (Deferred to Post-Phase 4)**:
  The pipeline executes end-to-end without any human prompts unless an explicit escalation rule or non-negotiable gate fires. Deferred until telemetry demonstrates multi-month pipeline stability across projects.

---

## 2. Classification of Non-Negotiable Approval Gates

[`shared/rules/approval-gates.md`](../../shared/rules/approval-gates.md) establishes 8 non-negotiable gates. These gates are classified into policy-eligible candidates versus permanently human-controlled actions:

| Gate | Action | Irreversible Risk | Policy Eligible? | Automation Tier |
|---|---|---|---|---|
| **1. Shipping to Friday** | POST Cucumber JSON summary | Modifies external reporting metrics | ❌ Always Human | None (Requires explicit "ship") |
| **2. Creating a Git Commit** | Commit to active branch | Alters repository history | ✅ Policy Eligible | Tier A (If diff < threshold & tests green) |
| **3. DB Migrations (Expand/Migrate)** | Execute SQL against remote DB | Modifies stateful infrastructure data | ❌ Always Human | None (Requires explicit "run migration") |
| **4. DB Migrations (Contracting Phase)** | `DROP` / `RENAME` table/column | Data loss risk | ❌ Always Human | None (Requires explicit "confirm contract phase") |
| **5. Posting to External APIs** | Third-party API mutation | External side-effects | ❌ Always Human | None (Requires explicit "approve request") |
| **6. Writing Files out of Boundary** | Edits outside `.claude/feature-workspace/` or source dirs | System structure risk | ✅ Policy Eligible | Tier A (If path in `allowedPaths` whitelist) |
| **7. Wiring a New Fitness Function** | Modifying CI/CD pipeline | May break CI build | ✅ Policy Eligible | Tier A (If fitness check validates dry-run) |
| **8. Deploying to Environment** | Triggering deployment | Production downtime risk | ❌ Always Human | None (Requires explicit "deploy") |

---

## 3. Delivery Policy Schema Sketch (`.claude/delivery-policy.yaml`)

Project teams opt into automation by defining a `.claude/delivery-policy.yaml` configuration file in their repository root:

```yaml
version: "1.0"
mode: "policy-driven" # Options: "strict-human", "policy-driven", "unattended"

defaults:
  maxContractRetries: 3
  maxDiffLines: 200
  requireGreenTests: true

gates:
  gitCommit:
    autoProceed: true
    maxDiffLines: 300
    requireCleanStatus: true

  outOfBoundaryWrite:
    autoProceed: true
    allowedPaths:
      - "src/**"
      - "tests/**"
      - "docs/**"

  fitnessFunctionWiring:
    autoProceed: true
    requireDryRunPass: true

stages:
  phase1_discovery:
    autoProceedAnalyst: true
    autoProceedArchitect: false # Pause if RFC or structural change flagged

  phase2_implementation:
    autoProceedDeveloper: true
    autoProceedCodeReviewer: true # Auto-proceed if verdict is APPROVED & 0 Criticals

  phase3_verification:
    autoProceedQA: true
    autoProceedTechWriter: true
    autoProceedDevOps: true

escalations:
  haltOnSecurityCritical: true
  haltOnArchitecturalRFC: true
  haltOnMaxRetriesExceeded: true
```

---

## 4. Interim Automation (Pre-Phase 4)

Full AOS Phase 4 provides runtime state machines. However, interim automation can be implemented directly within the current [`shared/skills/deliver-feature/SKILL.md`](../skills/deliver-feature/SKILL.md) skill today:

1. **Policy Detection**: `deliver-feature` checks for `.claude/delivery-policy.yaml`. If absent, defaults to standard human-prompt mode (100% backward compatible).
2. **Interim Tier B Retry Loop**: When `validate-artifact` returns `FAIL`, the skill automatically re-runs the producing agent with the specific error snippet up to `maxContractRetries` before executing a `PAUSE`.
3. **Interim Tier A Auto-Proceed**: Replaces prose `PAUSE` instructions with a policy evaluation step:
   ```
   If policy.stages.<stage>.autoProceed == true AND artifact validation == PASS:
       Log telemetry event -> Proceed to next step without pausing.
   Else:
       Execute PAUSE prompt to human.
   ```

---

## 5. Rollback & Recovery Strategy

Automation must never leave a repository in an ambiguous state if a downstream step fails:

1. **Workspace Rollback**: When a downstream validation fails (e.g. `code-reviewer` flags `CHANGES REQUESTED`), the pipeline reverts `.claude/feature-workspace/` to `.claude/feature-workspace/.history/pipeline-state.json.<last-pass>`.
2. **Git Worktree Isolation**: Automated developer edits occur inside isolated git worktrees. If tests fail or code review is rejected repeatedly ($N > 3$), the worktree is discarded without polluting the main working branch.

---

## 6. Telemetry & Auditability Requirements

Every policy decision MUST emit a telemetry event to `.claude/telemetry/events.jsonl` (using `shared/telemetry/event-schema.md` standards):

- `policy.evaluated`: Records stage name, policy rule matched, decision (`AUTO_PROCEED` | `PAUSE_HUMAN`), and evaluation duration.
- `contract.retry`: Records producing agent, attempt number ($1..N$), contract name, and violation list.

These events enable [`agent-scorecard`](../skills/agent-scorecard/SKILL.md) and [`pipeline-retrospective`](../skills/pipeline-retrospective/SKILL.md) to measure whether auto-approval improved throughput without degrading quality.

---

## 7. Escalation & Emergency Halt Criteria

The pipeline IMMEDIATELY halts and prompts a human operator regardless of policy when:
1. `validate-artifact` structural retries exceed `maxContractRetries` ($N=3$).
2. `security-reviewer` flags a Critical security finding.
3. `architect` produces an Architectural RFC requiring structural changes.
4. An unhandled exception or script failure occurs in any verification command.
5. The total diff exceeds `maxDiffLines`.
