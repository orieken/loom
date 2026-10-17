# AOS Governance Pairs & Counter-Auditors

This document maps the **15 Producer / Counter Governance Pairs** established in `docs/aos/AOS_Governance_Design_Pack/01-Governance-Checks-and-Balances.md` to concrete agents, skills, and invocation entrypoints in `ai-assistant-dot-files`.

> **Core Rule**: Every producer role (an agent generating an artifact or a human authoring a markdown specification) is paired with an independent counter role (an auditor agent or opposing-force skill) that inspects and reports findings without mutating the producer's artifact.

> **Terms and pattern**: the dictionary calls these **Producer / Counter Pairs** and the checking role a
> **Counter Agent**. Pairs 1–11 are audit pairs and pairs 12–15 are opposing-force pairs. The structure,
> trade-offs and how to add a pair are in `docs/patterns/producer-counter-pairs.md`.

---

## The 15 Governance Pairs Inventory

| # | Producer Role | Producer Artifact / Action | Counter Role (Auditor / Opposing-Force) | Entrypoint / Invocation |
|---|---|---|---|---|
| 1 | `context-engineer` | `context-manifest.md` | `context-auditor` | `shared/agents/context-auditor.md` |
| 2 | `memory-engineer` | `shared/knowledge/*.md` | `memory-auditor` | `shared/agents/memory-auditor.md` |
| 3 | `create-ki` (skill) | New KI frontmatter & body | `knowledge-auditor` | `shared/agents/knowledge-auditor.md` |
| 4 | Prompt Author (human/agent) | `shared/agents/*.md`, `shared/skills/*` | `prompt-evaluator` | `shared/agents/prompt-evaluator.md` |
| 5 | Agent Author (human/agent) | Agent frontmatter & persona prompts | `agent-evaluator` | `shared/agents/agent-evaluator.md` |
| 6 | Rule Author (human/agent) | `shared/rules/*.md` | `rule-auditor` | `shared/agents/rule-auditor.md` |
| 7 | Pattern Author (human/agent)| `docs/patterns/*.md` | `pattern-reviewer` | `shared/agents/pattern-reviewer.md` |
| 8 | Skill Author (human/agent) | `shared/skills/*/SKILL.md` | `tool-validator` | `shared/agents/tool-validator.md` |
| 9 | `tech-writer` / Prose Author | `README.md`, `docs/AGENT_REFERENCE.md` | `documentation-auditor` | `shared/agents/documentation-auditor.md` |
| 10 | Retrieval Engine (ADR-002) | `search-ki`, `query-memory` queries | `retrieval-evaluator` | `shared/agents/retrieval-evaluator.md` |
| 11 | `security-reviewer` / Developer | `.claude/feature-workspace/` artifacts | `privacy-auditor` | `shared/agents/privacy-auditor.md` |
| 12 | `memory-expansion` (skill) | Promotes retrospectives to KIs | `memory-compression` (skill) | `shared/skills/memory-compression/` |
| 13 | `learning-engine` (skill) | Proposes lessons from retrospectives | `forgetting-engine` (skill) | `shared/skills/forgetting-engine/` |
| 14 | `cost-optimizer` (skill) | Recommends lower-cost model tiers | `quality-optimizer` (skill) | `shared/skills/quality-optimizer/` |
| 15 | `deliver-feature` (skill) | Manages feature delivery pipeline | `scheduler` (skill) | `shared/skills/scheduler/` |

---

## Invocation Modes

1. **Standalone Audit Pass**: Any counter-auditor agent can be invoked standalone (e.g. `context-auditor`, `privacy-auditor`) to produce an audit report.
2. **Opt-In `validate-artifact` Integration**: When configured via `.claude/validate-artifact.yaml` or `.claude/delivery-policy.yaml`, `validate-artifact` automatically triggers the corresponding counter auditor after structural validation passes.
3. **Event-Driven Hooks**: Hooks in `.claude/hooks/` (or `shared/hooks/examples/`) trigger counter agents on pipeline events (such as `on-validation-pass` or `on-ki-created`).
