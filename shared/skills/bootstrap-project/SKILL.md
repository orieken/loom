---
name: bootstrap-project
description: Guided greenfield project creation — interviews the user, classifies against known ecosystem patterns (Saturday / Sunday / MCP server / Scribe CLI / Clean Architecture service, CLI, or library / Frontend), produces a planning blueprint, and optionally scaffolds the initial file structure with ADR-000, DOMAIN_DICTIONARY.md, and TEAM_TOPOLOGY.md stubs before the delivery agents take over.
triggers:
  keywords: ["bootstrap-project", "bootstrap project", "new project", "start a project", "create a project", "project template", "starter template", "greenfield"]
  intentPatterns: ["Bootstrap a new *", "Start a new project *", "I want to build a new *", "Create a starter *", "/bootstrap-project *"]
standalone: true
---

## When To Use

When the user is starting a project from an empty (or near-empty) directory and needs the ecosystem's patterns applied from day one — Clean Architecture layout, ADR-000, ubiquitous language dictionary, testing framework choice, first Gherkin scenarios — before any code is written.

Do NOT use when:
- A project already exists and the user wants to add a **feature** to it → use `/new-feature`.
- The user just wants a spec, not a project → use `/spec-writer`.
- The user is doing domain modeling on an existing bounded context → use `/event-storm`.
- The user only wants an ADR for a decision → use `/adr`.

## Context To Load First

1. `shared/project-patterns.json` — the registry of known blueprints. This is the source of truth for which patterns to offer in the interview.
2. `shared/rules/architecture-guardrails.md` — non-negotiable hard constraints every project inherits.
3. `shared/rules/design-principles.md` — Simple Design, Fowler refactorings, Sandi Metz limits.
4. `shared/rules/testing-conventions.md` — testing pyramid, Saturday/Sunday framework rules.
5. `shared/rules/<language>-conventions.md` — loaded once the language is picked in Phase 1 (TypeScript / Go / Python / Java / C#).
6. `shared/blueprints/<pattern-id>.md` — loaded once the pattern is picked in Phase 1.
7. If a project-local `DOMAIN_DICTIONARY.md`, `TEAM_TOPOLOGY.md`, or `ARCHITECTURE_RULES.md` exists, read them and treat as canonical. If they don't, this skill scaffolds stubs when in `plan-and-scaffold` mode.

## Process

### Phase 1 — Discovery Interview (one question at a time)

Follow the same one-question-per-message discipline as `spec-writer` and `new-feature`. Do not batch. Wait for each answer before continuing.

1. **What are we building?** — Push past "a tool" or "a service." Require: what problem it solves, who uses it, primary action performed.

2. **What type of project is this?** — Read `shared/project-patterns.json` and present each pattern's `name` + `oneLiner`. Currently registered:
   - `saturday` — E2E test framework
   - `sunday` — API test framework
   - `mcp-server` — Model Context Protocol server
   - `scribe-cli` — Content publishing CLI
   - `clean-arch-service` — polyglot backend service
   - `clean-arch-cli` — CLI tool
   - `clean-arch-library` — publishable library
   - `frontend` — web frontend (Vue 3 recommended)
   - `other` — custom, falls back to Clean Architecture generic
   
   If the user names something not in the registry, check `unblueprintedIdeas`. If it's there, offer to either fall back to the closest match or defer (blueprint the pattern first as a separate task).

3. **What language?** — Present the picked pattern's `languages.supported`. If `languages.primary` exists, mark it as recommended (e.g., "Go is recommended for MCP servers"). For `frontend`, ask the framework question here instead: "Vue 3 is recommended; supported alternatives are React, Svelte, Angular, SolidJS." Language stays TypeScript for all frontend choices.

4. **Greenfield or extending?** — If extending, ask for the target repo path and what's already there. Read what exists before proceeding to Phase 2.

5. **What are the first three things it needs to do?** — Concrete user-facing actions, not technical tasks. These become the seed scenarios that feed `/new-feature` or `/deliver-atdd` after bootstrap.

6. **What does "done" look like for phase one?** — The minimum shippable slice.

7. **Any known constraints or non-negotiables?** — Auth pattern, integration target, performance SLA, deployment target, licensing.

### Phase 2 — Architecture Reasoning (show your thinking)

Reason through each of these out loud in the response before producing artifacts. Do not skip to conclusions.

- **2a. Pattern confirmation.** Load `shared/blueprints/<pattern-id>.md`. Confirm the blueprint's layer structure fits what the user described. If the user's answer to Phase 1 Q1 needs layers the blueprint doesn't cover, flag that as a gap.

- **2b. Layer boundary table.** Adapt the blueprint's layer table to the user's specific project. One line per layer: `Layer → Responsibility → Example files (using the language's naming convention from CLAUDE.md)`.

- **2c. Dependency & integration map.** External systems the project touches, which need interface abstractions (per `architecture-guardrails.md` #1 and #5), which need OTel instrumentation (per #8 — adapter/interceptor layer only, never domain).

- **2d. Testing pyramid coverage.** Cross-reference the blueprint's `testingLevels` against the user's project. Per `shared/rules/testing-conventions.md`, name the writing agent for each level:
  - Unit → `developer`, with the code (greenfield) or `unit-tester` (backfill)
  - Integration → `qa-engineer` (or `developer` when the integration IS the feature)
  - API Contract → `api-test-generator` (from OpenAPI) or `qa-engineer` (hand-written)
  - Acceptance → `qa-engineer` inside `/deliver-atdd` or `/deliver-feature`
  - E2E / UI → `qa-engineer` following Saturday conventions

- **2e. Agent invocation plan.** Ordered sequence of *real* agents to run after planning. Only reference agents that exist in `shared/agents/`. Typical sequence: `analyst` → `architect` → (`data-engineer` if schema involved) → `performance-engineer` → `developer` → `code-reviewer` → `security-reviewer` → `qa-engineer` → `tech-writer` → `devops-engineer`. Prune to what the project actually needs.

- **2f. Risk & unknowns.** Tag each with `[AMBIGUOUS]`, `[RISK-HIGH]`, or `[ASSUMPTION]`.

### Phase 3 — Blueprint & Scaffold Output

Ask the user to pick a mode:
- **Plan only** — write the three planning documents (3a–3c), no source files, no directories beyond `docs/`.
- **Plan and scaffold** — write the planning documents *and* lay down the initial file skeleton per the blueprint's scaffold recipe: language-specific project files (`package.json` / `go.mod` / `pyproject.toml` / etc.), directory tree matching the layer table, a failing first test, `.env.example`, `.gitignore`, `DOMAIN_DICTIONARY.md` stub, `TEAM_TOPOLOGY.md` stub.

Then produce these files:

**3a. `PROJECT_BLUEPRINT.md`** (project root)
```markdown
# Project Blueprint: <Name>

**Type**: <pattern.name from registry>
**Pattern ID**: <pattern.id>
**Language**: <language from Phase 1 Q3>
**Framework**: <if frontend, the framework picked in Phase 1 Q3>
**Status**: Planning — not yet implemented
**Generated by**: bootstrap-project skill — <YYYY-MM-DD>

## Problem Statement
<Phase 1 Q1 answer, cleaned up>

## Phase 1 Scope
<Phase 1 Q6 answer — minimum shippable slice>

## Architecture
### Layer Structure
<Phase 2b table>

### Package / Directory Structure
<Tree from the blueprint, tailored to the language>

### Key Abstractions
<Base classes / interfaces the pattern requires>

## Integration Map
<Phase 2c — external deps and abstraction strategy>

## Testing Pyramid Coverage
<Phase 2d — level → agent → framework>

## OTel Instrumentation Plan
<What gets traced, measured, logged. Adapter/interceptor layer only.>

## Craftsmanship Constraints
Inherits all rules from `shared/rules/`. Project-specific additions:
<list, or "none">

## Open Questions
<All [AMBIGUOUS] and [ASSUMPTION] items from 2f>

## Agent Invocation Plan
<Ordered slash commands from 2e — copy-pasteable>
```

**3b. `SEED_SCENARIOS.md`** (project root — feeds the first `/new-feature` or `/deliver-atdd` run)
```markdown
# Seed Scenarios: <Name>

Generated by: bootstrap-project skill
Source: Phase 1 Q5 answers

## Scenarios
[HIGH] <scenario title — plain language, user-facing>
[HIGH] <scenario title>
[MED]  <scenario title>

## Next Step
Run one of:
  /new-feature — turn a scenario into a full feature spec, then deliver
  /deliver-atdd — jump straight into ATDD from these scenarios
```

**3c. `docs/adrs/ADR-000-project-foundation.md`** — delegate to `/adr` skill for consistent formatting. Pass Phase 2 context.

**3d. Plan-and-scaffold mode only:**
- `DOMAIN_DICTIONARY.md` stub with ubiquitous terms surfaced in Phase 1 (per `shared/rules/design-principles.md` #6).
- `TEAM_TOPOLOGY.md` stub with a single stream-aligned team entry (so `team-topology-check` has something to bite on).
- Language-specific bootstrap files from the blueprint's scaffold recipe.
- Directory tree matching the layer table.
- One failing first test wired to the language's test runner.
- `.env.example` with placeholder keys for every external dep in the integration map (per `architecture-guardrails.md` #3).
- `.gitignore` from the blueprint recipe.

### Phase 4 — Confirmation Gate

Present a summary and require explicit "yes" before invoking any downstream agent. Any edit to a pending artifact resets the gate (per `shared/rules/approval-gates.md` gate #6 — writing files out of boundary).

```
BOOTSTRAP SESSION COMPLETE
==========================
Project:        <name>
Pattern:        <pattern.name>
Language:       <language>
Framework:      <if frontend>
Phase 1 scope:  <one sentence>
Mode:           <plan-only | plan-and-scaffold>

Files produced:
  PROJECT_BLUEPRINT.md
  SEED_SCENARIOS.md
  docs/adrs/ADR-000-project-foundation.md
  <scaffolded files if applicable>

Testing pyramid coverage: <levels covered>
Risks flagged:            <n>
Assumptions:              <n>
Open questions:           <n>

Recommended next command:
  /new-feature — pick a HIGH-priority seed scenario and turn it into a feature spec

Type 'yes' to confirm, or ask questions / request changes.
```

## Output Format

### Files Created (plan-only mode)
- `PROJECT_BLUEPRINT.md`
- `SEED_SCENARIOS.md`
- `docs/adrs/ADR-000-project-foundation.md`

### Files Created (plan-and-scaffold mode)
All of the above, plus everything defined in the blueprint's scaffold recipe for the chosen `<pattern, language>` pair.

### Confirmation Message
Exactly as shown in Phase 4 above.

## Guardrails

- **No source code without explicit user confirmation.** Even in `plan-and-scaffold` mode, the scaffolded files must be listed and approved before writing.
- **Never invoke a downstream agent (`/analyst`, `/architect`, `/deliver-feature`, etc.) without explicit user confirmation.** Bootstrap ends at the confirmation gate.
- **Never reference an agent or skill that isn't in `shared/agents/` or `shared/skills/`.** If Phase 2e wants to invoke something that doesn't exist, that's a bug in this skill's flow — flag it, don't fabricate.
- **Never scaffold a pattern that isn't in `shared/project-patterns.json`.** Either fall back to a registered pattern or defer.
- **Never silently override an ecosystem standard.** If the user's Phase 1 answers conflict with `shared/rules/*`, surface the conflict and let the user decide.
- **Ensure every testing pyramid level the blueprint calls for has a named writing agent in Phase 2d.** No coverage gaps snuck past by omission.
- **File paths use ecosystem conventions.** ADRs go to `docs/adrs/`, not `docs/adr/`. Feature docs go to `docs/features/<name>/`. Pipeline artifacts go to `.claude/feature-workspace/`.
- **Writing scaffolded files outside of `docs/` and the project root counts as "writing files out of boundary"** per `shared/rules/approval-gates.md` gate #6 — explicit approval required.

## Standalone Mode

Works entirely offline. All pattern recipes live in `shared/project-patterns.json` and `shared/blueprints/*.md`. No external services required. The one hard dependency is the `/adr` skill (for writing ADR-000 consistently) — if unavailable, fall back to writing ADR-000 inline following the format in `shared/skills/adr/SKILL.md`.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
