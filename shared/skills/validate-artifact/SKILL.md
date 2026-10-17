---
name: validate-artifact
description: Validates a pipeline artifact against its inter-agent contract in shared/contracts/ — checks required sections are present (and a few contract-specific content rules) before the artifact is allowed to move to the next agent. Invoked automatically between every contract-bound handoff in deliver-feature.
triggers:
  keywords: ["validate-artifact", "validate contract", "check contract", "contract check"]
  intentPatterns: ["/validate-artifact *", "Validate this artifact against its contract", "Check *.md against its contract"]
standalone: true
---

## When To Use
Invoked automatically by `deliver-feature` immediately after every contract-bound agent produces its
artifact — before the pipeline proceeds to the next step. Every pipeline agent now has a contract (Epic 5,
closed 2026-07-04 — see the Contract Mapping below). Can also be run standalone: "validate analysis.md
against its contract".

Also usable outside the pipeline entirely to validate **frontmatter files** — agent files, skill
`SKILL.md` files, and Knowledge Items — against the three frontmatter contracts (see the last three
rows of the Contract Mapping). This is the referenceable, standalone-invocable form of the
field-presence checks that `scripts/health-check.sh` already runs across the whole corpus in bulk;
`validate-artifact` handles the "validate this one file" case.

## Context To Load First
1. The contract file for the artifact being validated (see mapping below)
2. The artifact file itself

## Contract Mapping
| Agent | Artifact | Contract |
|---|---|---|
| spec-writer | `features/<kebab-title>.md` | `shared/contracts/delivery-template-contract.md` |
| context-engineer | `.claude/feature-workspace/<feature-name>/context-manifest.md` | `shared/contracts/context-manifest-contract.md` |
| analyst | `.claude/feature-workspace/<feature-name>/analysis.md` | `shared/contracts/analysis-contract.md` |
| repo-documenter | `.loom/runs/<run>/docs/<dir>.json` | `shared/contracts/repo-doc-contract.md` |
| repo-doc-auditor | typed `repo-doc-audit` state, read by the `loom document` loop | `shared/contracts/repo-doc-audit-contract.md` |
| architect | `.claude/feature-workspace/<feature-name>/architecture-notes.md` | `shared/contracts/architecture-contract.md` |
| performance-engineer | `.claude/feature-workspace/<feature-name>/performance-report.md` | `shared/contracts/performance-contract.md` |
| data-engineer | `.claude/feature-workspace/<feature-name>/data-engineering-notes.md` | `shared/contracts/data-engineering-contract.md` |
| developer | `.claude/feature-workspace/<feature-name>/implementation-notes.md` | `shared/contracts/implementation-contract.md` |
| code-reviewer | `.claude/feature-workspace/<feature-name>/code-review-report.md` | `shared/contracts/review-contract.md` |
| accessibility-engineer | `.claude/feature-workspace/<feature-name>/accessibility-report.md` | `shared/contracts/accessibility-contract.md` |
| security-reviewer | `.claude/feature-workspace/<feature-name>/security-report.md` | `shared/contracts/security-contract.md` |
| qa-engineer | `.claude/feature-workspace/<feature-name>/qa-report.md` | `shared/contracts/qa-contract.md` |
| visual-qa-engineer | `.claude/feature-workspace/<feature-name>/visual-qa-report.md` | `shared/contracts/visual-qa-report-contract.md` |
| sre-engineer | `.claude/feature-workspace/<feature-name>/observability-report.md` | `shared/contracts/observability-contract.md` |
| tech-writer | `.claude/feature-workspace/<feature-name>/docs-report.md` | `shared/contracts/docs-contract.md` |
| devops-engineer | `.claude/feature-workspace/<feature-name>/devops-report.md` | `shared/contracts/devops-contract.md` |
| refactor-engineer | `.claude/feature-workspace/<feature-name>/refactoring-notes.md` | `shared/contracts/refactoring-contract.md` |
| humans (agent author) | `shared/agents/*.md` | `shared/contracts/agent-frontmatter-contract.md` |
| humans (skill author) | `shared/skills/*/SKILL.md` | `shared/contracts/skill-frontmatter-contract.md` |
| humans (KI author) | `shared/knowledge/*.md`, `.claude/knowledge/*.md` | `shared/contracts/ki-frontmatter-contract.md` |

## Process

### Pipeline artifacts (analysis.md, architecture-notes.md, etc.)
1. Read the contract's "Required Sections" list.
2. Grep the artifact for each required heading — exact string and heading level (`##` vs `###`) must match.
3. Apply the contract's "Validation Rule" content checks (e.g., "Overall Status must contain APPROVED or CHANGES REQUESTED", "Test Results must show Failed: 0").
4. Record every miss — missing heading or failed content rule — as a violation.
5. **Check retrieval frontmatter (WARN-level)**: check whether the artifact starts with a YAML frontmatter block (`---` on the first line). If the frontmatter block is missing entirely, record one WARN. If the block is present, check that all seven retrieval fields are present (`feature`, `bounded_context`, `domain_terms`, `files_touched`, `issue_refs`, `linked_adrs`, `linked_kis`); record a WARN for each missing field. WARNs do not affect the PASS/FAIL status — they are surfaced separately in the output.
6. Report `PASS` (zero structural violations) or `FAIL` (one or more structural violations). WARNs appear in the output regardless of PASS/FAIL status.

### Frontmatter files (agents, skills, KIs)
Same PASS/FAIL semantics; the validation switches from heading-grepping to frontmatter-parsing.
1. Read the contract's "Required Fields" table.
2. Isolate the YAML frontmatter block — everything between the opening `---` and the next `---`. If the
   file doesn't start with `---`, that alone is a FAIL (no frontmatter present).
3. For each required field, grep for `^<field>:` inside the frontmatter block.
4. Apply the contract's structural rules (semver shape for agent `version`, ISO-8601 shape for KI
   `created`, sub-key presence for skill `triggers`, kebab-case + filename-match for `name`, etc.).
5. Record every miss as a violation and report `PASS` / `FAIL` in the same output format as pipeline
   artifacts.

The output section headings (`Missing Sections`, `Present Sections`) still apply — read them as
"Missing Fields" / "Present Fields" for the frontmatter case.

### Opt-in Counter Auditor Invocation (AOS Phase 2)
When enabled via project configuration (`.claude/validate-artifact.yaml` or `invokeAuditors: true` in `.claude/delivery-policy.yaml`):
1. After the artifact passes the structural check with status `PASS`, lookup the producer's corresponding counter agent (per `docs/aos/governance-pairs.md`).
2. Invoke the counter auditor agent (e.g. `context-auditor` for `context-manifest.md`, `privacy-auditor` for implementation artifacts, `knowledge-auditor` for KIs).
3. Attach the counter auditor's findings report under `## Counter Auditor Findings` in the output format.
4. **Default Behavior Unchanged**: If no configuration exists, `validate-artifact` runs strictly structural checks without invoking auditors.

## Output Format
```markdown
# Artifact Validation: [artifact filename]
Contract: [contract filename]
Status: PASS | FAIL

## Missing Sections (if any)
- [heading]

## Content Rule Violations (if any)
- [rule] — [what was found instead]

## Retrieval Frontmatter Warnings (if any)
- WARN: Missing retrieval frontmatter block — add a YAML `---` block at the top of the file (see contract's "Retrieval Frontmatter (WARN)" section for required fields)
- WARN: Retrieval frontmatter field `<field>` is missing or empty

## Present Sections
- [heading] ✓
```

## Guardrails
- **Structural only**: this checks presence of headings and the small set of content rules each contract
  spells out explicitly. It never judges whether the content is *correct* — that stays with the human
  PAUSE checkpoints and the downstream agents (code-reviewer, security-reviewer, qa-engineer).
- **Exact match**: a heading at the wrong level (e.g., `### Summary` when the contract requires `## Summary`)
  is a FAIL, not a warning — a level mismatch breaks downstream `grep`-based parsing just as much as a
  missing heading would.
- **Never silently pass**: every miss must be listed explicitly in the output, even if the orchestrator
  chooses to proceed anyway (it shouldn't, except by explicit human override).
- **On FAIL**: the orchestrator sends the artifact back to the producing agent with the specific violations
  listed, and re-validates after the fix. This is a structural loop, distinct from code-reviewer's
  qualitative CHANGES REQUESTED loop — both can fire independently.

## Standalone Mode
No external tools required — this is a Read + Grep pass over two local markdown files. Works identically
inside or outside the `deliver-feature` pipeline.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
