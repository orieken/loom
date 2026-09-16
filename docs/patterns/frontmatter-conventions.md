# Frontmatter Conventions

Every markdown file the framework treats as structured — agents, skills, Knowledge Items — uses YAML frontmatter to declare identity, capabilities, and metadata. This doc is the reference for what fields exist, what values are valid, and why.

Enforcement today: `scripts/health-check.sh` validates **field presence** for all three shapes AND
value-level shape (via [JSON schemas](../../shared/schemas/) — kebab-case patterns, semver on agent
`version`, ISO 8601 on KI `created`, valid Claude Code tool names on agent `tools`, `standalone: true`
on skills). The schema step is opt-in in the sense that it degrades to a WARN if `python3` +
`jsonschema` + `PyYAML` aren't installed; field-presence still runs regardless. See "Gaps and
follow-ups" at the bottom for the remaining contract-file work.

---

## 1. Agent frontmatter (`shared/agents/*.md`)

### Required fields

| Field | Type | Values / notes | Rationale |
|---|---|---|---|
| `name` | string | kebab-case; must match filename base (`analyst.md` → `name: analyst`) | Referenced by the `Task` tool's `subagent_type` parameter and by other agents citing this one by role |
| `description` | string | One sentence. Include `PROACTIVELY` when the pipeline should invoke unconditionally; include `MUST` for hard-required ordering (e.g., "MUST be invoked before developer") | Consumed by the Claude Code / Cursor UI to help users pick agents; also read by the orchestrator when deciding pipeline sequencing |
| `tools` | comma-separated string | Claude Code tool names (Read, Write, Edit, MultiEdit, Bash, Glob, Grep). Use least-privilege, and declare every capability the prompt uses. `Bash` is a write and egress channel, not a read tool | Describes real capability — a counter agent with `Read, Glob, Grep` and no `Bash` cannot modify what it audits. An agent holding `Bash` can, whatever else the list says |
| `model` | string | Usually `inherit`. Can pin (`claude-opus-4-8`, `claude-sonnet-5`, `claude-haiku-4-5`) for cost/quality tradeoffs on specific agents | `inherit` is the norm because the parent session's model choice usually applies; pin only when there's a real reason (e.g., agent-eval agent might pin to a cheaper model since it's called at scale) |
| `version` | semver string (`X.Y.Z`) | Bump minor on behavior-relevant change (output-format refactor, tool-list change); patch on prose-only edits; major only if the agent's contract with callers breaks | Downstream installs pull versioned agents; the changelog tracks bumps |

### Optional fields

| Field | Type | Values / notes |
|---|---|---|
| `isolation` | string | `worktree` — agent runs in a temporary git worktree isolated from the main working copy. See `shared/agents/developer.md` for the reference use |

### Example

```yaml
---
name: analyst
description: Use PROACTIVELY as the first step of any feature implementation...
tools: Read, Glob, Grep, Bash
model: inherit
version: 1.2.0
---
```

### Template

`shared/templates/agent.template.md` (deliberately outside `shared/agents/` so the Claude Code loader doesn't register the template itself as a real agent — same "escape hatch" reason `SKILL_TEMPLATE.md` lives at `shared/skills/SKILL_TEMPLATE.md` above the loader's `*/SKILL.md` search path).

---

## 2. Skill frontmatter (`shared/skills/*/SKILL.md`)

### Required fields

| Field | Type | Values / notes | Rationale |
|---|---|---|---|
| `name` | string | kebab-case; must match the parent directory name (`shared/skills/adr/SKILL.md` → `name: adr`) | Referenced by the `Skill` tool when invoking, and by slash-command routing (`/adr`) |
| `description` | string | One sentence. What the skill does and when Claude should auto-trigger it | Consumed by the Claude Code UI for slash-command discovery and auto-trigger routing |
| `triggers` | object | See below — has two sub-keys | The auto-trigger routing table |
| `standalone` | boolean | Must be `true` in this framework. Skills that require MCP servers or external network calls to function are not portable across installs | Enforces the "everything must work offline" discipline |

### `triggers` sub-structure

```yaml
triggers:
  keywords: [list, of, single, words, or, short, phrases]
  intentPatterns: ["Natural language sentence pattern *", "Another pattern *"]
```

- `keywords`: literal string matches — routing fires when any keyword appears in user input
- `intentPatterns`: fuzzy sentence patterns with `*` wildcards for user-supplied nouns

### Example

```yaml
---
name: adr
description: Effortless, consistent Architecture Decision Records.
triggers:
  keywords: ["ADR", "decision", "record", "document"]
  intentPatterns: ["Write an ADR for *", "Document the decision to *", "We decided to *, record it", "Create an architecture decision record"]
standalone: true
---
```

### Template

`shared/skills/SKILL_TEMPLATE.md`.

---

## 3. Knowledge Item frontmatter (`shared/knowledge/*.md`, `.claude/knowledge/*.md`)

### Required fields

| Field | Type | Values / notes | Rationale |
|---|---|---|---|
| `name` | string | kebab-case; must match filename base | Referenced by `[[link]]` syntax from other KIs and by the memory-registry index |
| `tags` | list | Free-form tag list. Convention: use lowercase kebab-case tags (`tag-name` not `TagName`); reuse existing tags in the corpus when possible before inventing new ones | Consumed by `search-ki` for tag-based filtering and by `memory-auditor` for coverage analysis |
| `domain` | string | Which bounded context this KI applies to (e.g., `testing`, `architecture`, `deployment`, `retrieval`) | Consumed by `context-engineer` when building context manifests for a feature in a matching domain |
| `created` | ISO date (`YYYY-MM-DD`) | Immutable — set once at authoring time | Consumed by the forgetting engine (future) for staleness calculations |

### Example

```yaml
---
name: workflow-tool-wraps-domain-workflow-for-mcp
tags: [mcp, trinity, workflow, adapter]
domain: mcp-server-pattern
created: 2026-07-19
---
```

### Template

[`shared/templates/ki.template.md`](../../shared/templates/ki.template.md).

---

## Universal conventions across all three

- **Frontmatter comes first.** Nothing (not even a comment) can precede the opening `---`.
- **Fields are lowercase.** `name` not `Name`; `tools` not `Tools`.
- **Values that contain YAML-special chars** (colons, brackets, quotes) must be quoted or escaped.
- **No commented-out fields.** If a field isn't used, omit it — YAML comments in frontmatter confuse some parsers.
- **File body starts immediately after the closing `---`.** No blank line requirement, but common style adds one for readability.

---

## Related industry conventions

The framework's agent frontmatter shape (`name`, `description`, `tools`, `model`) was deliberately aligned with Anthropic Claude Code's agent spec. That alignment lets install scripts symlink `shared/agents/` into `.claude/agents/` without generation. The framework adds `version` on top of Claude Code's spec.

Cross-tool state: there is no universal cross-vendor standard for these files. Cursor uses `.mdc` files with a different frontmatter shape (`description`, `alwaysApply`, `globs`). Windsurf and GitHub Copilot use flat markdown with no frontmatter. This framework's install strategy generates or inlines platform-specific formats from the shared markdown source of truth — see `shared/platform-registry.json` and `scripts/generate-configs.sh`.

---

## Gaps and follow-ups

These are known improvements to the frontmatter story, currently open as handoff prompts under `docs/prompts/`:

- **Formal contracts** — DONE. Three contract files landed:
  [`shared/contracts/agent-frontmatter-contract.md`](../../shared/contracts/agent-frontmatter-contract.md),
  [`shared/contracts/skill-frontmatter-contract.md`](../../shared/contracts/skill-frontmatter-contract.md),
  and [`shared/contracts/ki-frontmatter-contract.md`](../../shared/contracts/ki-frontmatter-contract.md).
  `validate-artifact` now accepts frontmatter files as valid artifacts (see the last three rows of its
  Contract Mapping table). The contracts are 1:1 with the field-presence checks in `scripts/health-check.sh`
  steps 2, 3, and 8 — no stricter, no looser — plus a few cheap structural rules (semver shape on agent
  `version`, ISO-8601 shape on KI `created`, kebab-case + filename-match on all three `name` fields).
- **JSON schemas** — DONE. Three schemas landed under [`shared/schemas/`](../../shared/schemas/):
  [`agent-frontmatter.schema.json`](../../shared/schemas/agent-frontmatter.schema.json),
  [`skill-frontmatter.schema.json`](../../shared/schemas/skill-frontmatter.schema.json), and
  [`ki-frontmatter.schema.json`](../../shared/schemas/ki-frontmatter.schema.json). Wired into VS
  Code and Cursor via `.vscode/settings.json.example` / `.cursor/settings.json.example` — copy the
  template to enable in-editor validation (README has the walkthrough under "IDE Setup"). Also
  wired into `scripts/health-check.sh` via a dedicated "Frontmatter JSON Schema Validation" step
  backed by `scripts/validate-frontmatter.py`, which runs when `python3 jsonschema + PyYAML` are
  available and warn-skips otherwise. `tools: WhatEverRandomName` is now flagged at author time
  in-IDE and at check time in the script; both agree on shape by design.
- **KI template** — DONE. [`shared/templates/ki.template.md`](../../shared/templates/ki.template.md) landed. Conforms to `shared/schemas/ki-frontmatter.schema.json` with required fields (`name`, `tags`, `domain`, `created`) and body sections (`## Context`, `## Pattern`).

