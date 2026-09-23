---
name: validate-migrations
description: Validates database migration files against the Expand/Contract (Parallel Change) architectural rules. Rejects destructive operations like DROP COLUMN or RENAME COLUMN.
triggers:
  keywords: ["validate migration", "check database", "schema change", "sql", "drop column", "rename column"]
  intentPatterns: ["review migration script for destructive operations", "does this sql violate expand contract pattern"]
standalone: true   # must work without MCP/external systems
---

## When To Use
Use this skill when reviewing code that includes database schema changes, SQL migration scripts, or ORM model updates.
Do NOT use when reviewing application logic with no database structural alterations.

## Context To Load First
1. `ARCHITECTURE_RULES.md`
2. `.claude/feature-workspace/<feature-name>/data-engineering-notes.md` (if available)

## Process
1. Identify database migration files.
   - What to do: Find `.sql`, `.ts`, `.py`, or other files in the `migrations/` directory that are added or modified.
   - What to produce: A list of targets.
   - What to check: Verify targets exist and contain schema definitions.
2. Run migration checking script.
   - What to do: Run `./.claude/skills/validate-migrations/check.sh [target]`
   - What to produce: The console output indicating if destructive operations were found.
   - What to check: Read the script exit code. If non-zero, destructive keywords were detected without an explicit Contract phase.
3. Handle violations.
   - What to do: Instruct the developer to use the Expand/Contract pattern instead of `DROP` or `RENAME`.
   - When to pause: If complex multi-phase migrations are needed but not fully documented in the data engineering notes, pause for DBA approval.

## Output Format
```markdown
# Migration Validation Report

## Violations
- `migrations/20260307_v2.sql`: Destructive operation `DROP COLUMN legacy_id` detected.

## Recommended Fixes
- **Expand Phase**: Add the new column, dual write, backfill.
- **Contract Phase**: (Separate deployment) Drop the legacy column.
```

## Guardrails
- You MUST REJECT any migration introducing `DROP COLUMN` or `RENAME COLUMN` if it is not explicitly marked and isolated in a Contract phase deployment.
- Never approve `NOT NULL` constraints on new columns without a `DEFAULT` value.

## Standalone Mode
If `check.sh` is unavailable, use `grep` or manual file inspection to manually scan all migration target files for banned terms (`DROP`, `RENAME`, `NOT NULL` without `DEFAULT`).

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
