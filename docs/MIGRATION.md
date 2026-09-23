# Migration: v1 -> v2 (Canonical `shared/` Layer)

This document is for anyone who cloned or forked this repo **before** the `shared/` canonical layer
restructure (commit `cc841a8`, "establish canonical shared/ layer with cross-platform install"). If you
cloned recently, none of this applies to you — skip straight to [README.md](../README.md).

## What changed

### Before (v1)
```
.claude/
  agents/*.md        <- real files, the only source of truth
  skills/*/SKILL.md   <- real files
  rules/*.md          <- real files
ARCHITECTURE_RULES.md  <- real file, repo root
DOMAIN_DICTIONARY.md   <- real file, repo root
```
Every other platform's config (`.cursorrules`, `.openai.md`, etc.) was hand-maintained separately and
regularly drifted from `.claude/`'s content — there was no single source of truth and no way to detect
drift.

### After (v2)
```
shared/
  agents/*.md         <- real files, THE source of truth
  skills/*/SKILL.md    <- real files
  rules/*.md           <- real files
  contracts/           <- new: required-section contracts (Epic 5)
  knowledge/            <- new: Knowledge Items (Epic 14)
  templates/            <- new: tutorial content (Epic 19)
  ARCHITECTURE_RULES.md
  DOMAIN_DICTIONARY.md
  platform-registry.json <- new: tier/capability/format per platform

.claude/{agents,skills,rules}/  <- now symlinks to shared/ equivalents
ARCHITECTURE_RULES.md            <- now a symlink to shared/ARCHITECTURE_RULES.md
DOMAIN_DICTIONARY.md             <- now a symlink to shared/DOMAIN_DICTIONARY.md
.cursor/rules/*.mdc, .windsurfrules, .github/copilot-instructions.md,
.github/instructions/*.instructions.md, AGENTS.md, .openai.md   <- now generated from shared/, never hand-edited
```

## Is this a breaking change for me?

- **If you only ever used Claude Code** and never hand-edited `.claude/agents/*.md` directly (only read
  them): nothing breaks. The content is identical, just relocated with a symlink pointing back at it.
- **If you hand-edited `.claude/agents/*.md` or `.claude/skills/*/SKILL.md` directly**: your edits are
  preserved by the migration (it moves files, never deletes them), but going forward you must edit
  `shared/agents/`/`shared/skills/` instead — editing through the `.claude/` symlink still works (it's the
  same file), but editing a *generated* platform config (anything under `.cursor/`, `.github/`, etc.) will
  now be silently overwritten the next time `scripts/generate-configs.sh` runs.
- **If you hand-maintained `.cursorrules`, `.openai.md`, or similar separately**: those are now generated
  from `shared/` — any manual edits to them will be lost on the next `generate-configs.sh` run. Move that
  content into the relevant `shared/rules/` file instead (see
  [CONTRIBUTING.md](CONTRIBUTING.md), "Adding a new rule").

## How to migrate

```bash
# Preview what would change first
bash scripts/migrate-v1-to-v2.sh --dry-run

# Apply it — moves .claude/{agents,skills,rules} and root ARCHITECTURE_RULES.md/DOMAIN_DICTIONARY.md
# into shared/, then symlinks the old locations back to the new ones. Never deletes anything; if a
# shared/ target already exists, that item is skipped rather than overwritten.
bash scripts/migrate-v1-to-v2.sh

# Verify the result
bash scripts/health-check.sh --verbose

# Regenerate every other platform's config from the now-canonical shared/ source
bash scripts/generate-configs.sh
```

The script is idempotent — running it again after a successful migration reports everything as already
migrated (`[skip]`) rather than erroring or duplicating anything.

## Tags

- `v1.0.0` — the last commit before the restructure began (`e7d5557`).
- `v2.0.0` — the completed canonical `shared/` layer, including everything built across Epics 1-21 in
  `docs/features/context-engineering-framework/TODO.md`.

---

# Framework Version Marker (v3.3+, Epic 68)

Starting with v3.3, every non-dry-run `install.sh` run writes a **version marker** into the installed
target. This makes drift between an install and the upstream framework repo detectable without manual
archaeology.

## Marker location

| Install mode | Marker path |
|---|---|
| `--global` | `~/.claude/framework-install.json` |
| `--project <path>` | `<path>/.claude/framework-install.json` |

## Marker format

```json
{
  "source_repo": "/path/to/ai-assistant-dot-files",
  "git_tag": "v3.3.0",
  "commit_sha": "abc1234def5678901234567890abcdef12345678",
  "installed_at": "2026-08-01T12:00:00Z",
  "mode": "symlink",
  "framework_level": "base",
  "platforms": ["claude-code", "cursor"]
}
```

Fields:
- `source_repo` — absolute path to the framework clone at install time
- `git_tag` — the most recent git tag in the framework repo at install time
- `commit_sha` — full HEAD SHA at install time
- `installed_at` — ISO 8601 UTC timestamp
- `mode` — `"symlink"` (default) or `"copy"` (when `--copy` was passed)
- `framework_level` — `"base"` or `"full"` (AOS layers)
- `platforms` — platforms installed; the specific platform if `--platform` was used, or all
  auto-detected platforms if not

## Drift detection

Run `bash <framework>/scripts/health-check.sh` from within an installed project directory. The
"Install Version Marker" section reads the marker and compares `git_tag` against the source repo's
current HEAD tag:

- **PASS** — installed tag matches source repo: no drift.
- **WARN** — tags differ: re-run `install.sh` to update the install and refresh the marker.
- **PASS** (silent skip) — `source_repo` path no longer resolves (repo moved or deleted).
- **PASS** (skipped) — no marker present (pre-v3.3 install, see below).

## Pre-marker installs (before v3.3)

Installs done before Epic 68 / v3.3 have no `framework-install.json`. They are not broken — the
framework operates normally without the marker. Drift detection simply skips silently when no
marker is found. To gain drift detection: re-run `install.sh` against the project and the marker
will be written.

For update detection on pre-marker installs, use the filesystem forensic path documented in
`docs/prompts/update-installed-framework.md` (the `done/` version covers this in detail).

---

# Removed: `test-driven-developer` and `TDDWorkflow` (2026-09-22, ADR-009)

**Breaking**, and deliberately without a deprecation release: a release in between would have shipped
the contradiction ADR-009 removes. If you invoke either by name, this is what to do instead.

| Removed | Use instead |
|---|---|
| `test-driven-developer` agent | `developer` — it now writes the unit tests for the code it changes |
| `/orchestrate --workflow tdd` (`shared/workflows/tdd-workflow.md`) | `/deliver-atdd` for acceptance-first delivery, or `developer` directly |
| `.claude/feature-workspace/tdd-state.json` | nothing — safe to delete |

**Why.** TDD's design benefit comes from a test writer who does not yet know the implementation, and an
agent writing both always does. What the framework requires instead is the result, measured on the
change (ADR-008): changed lines at least 85% covered, no test without a path to a failure, mutants on
changed lines killed. Test-first remains a technique anyone may use; it is no longer a rule.
