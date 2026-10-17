# Contract: feature spec (delivery template)

**Produced by**: spec-writer (Write Mode), or a human copying `features/TEMPLATE.md` (source: `templates/claude-feature-team/features/TEMPLATE.md`)
**Consumed by**: analyst, and every agent after it through `analysis.md`
**Artifact**: `features/<kebab-title>.md`

This is the code layer of validation for the delivery template. See `docs/patterns/delivery-template.md`
for the pattern. It checks structure only. Whether a criterion is testable, or a root cause is the real
one, stays with `spec-writer`'s readiness critique and the human who approves the spec.

## Required Sections (FAIL when missing)

Exact heading text and level. Both `shared/agents/spec-writer.md`'s output format and the template
`templates/claude-feature-team/features/TEMPLATE.md` produce all of these.

- `## Summary`
- `## Acceptance Criteria`
- `## Out of Scope`
- `## Hypothesis and Target Metric`
- `## Edge Cases and Failure Modes`

The last two are not required when the frontmatter has `type: spike` or `type: chore`. Those work items
have no user-observable outcome to measure. A spec with no `type` is treated as a feature.

## Content Rules (FAIL when broken)

1. **Acceptance criteria are checkbox lines.** `## Acceptance Criteria` MUST contain at least one line
   that starts with `- [ ]` or `- [x]`.
2. **Every criterion is Given / when / then.** Each checkbox line MUST contain all three words
   `given`, `when` and `then`, in any letter case.
3. **No placeholders.** A required section MUST NOT contain an empty checkbox (`- [ ]` with no text),
   a bracketed placeholder word such as `[context]` or `[Feature Name]` (the checkbox `[ ]` and `[x]`
   do not count), the word `TBD`, or `TODO`.
4. **Out of Scope is not blank.** The section MUST list at least one item, or say "None identified".
   An empty section cannot be told apart from an unfinished one.
5. **Ready means resolved.** If the frontmatter has `status: ready`, then `## Open Questions` (when
   present) MUST contain no item other than "None". A spec with an open question that blocks a
   downstream agent is not ready.

## Hypothesis and Edge Cases

Both sections were promoted from WARN to FAIL on 2026-10-17, when `shared/agents/spec-writer.md` 1.3.0
began to produce them. Specs written before that date do not have them. Treat the FAIL on an old spec
as a prompt to add the section, not as a defect in the spec.

- The hypothesis section MUST contain text and no placeholder (Content Rule 3). It SHOULD name a
  metric, an amount and a date. Code cannot judge whether it does. The readiness critique does.
- The edge-cases section MUST contain at least one item, or say "None identified".

The weekly report's outcome rows read the hypothesis section. Without it the report cannot say whether
shipped work mattered (`docs/patterns/engineering-metrics.md`).

## Bug specs (WARN)

Apply when the frontmatter has `type: bug`. A bug spec SHOULD also contain these headings. A missing
one is a WARN. These stay WARN until `spec-writer` has a bug-specific interview that produces them.

| Heading | Content |
|---|---|
| `## Symptom` | What was observed |
| `## Reproduction` | Steps that show the bug |
| `## Root Cause` | The cause at the bottom of the why chain, not the proximate one (`five-whys`) |
| `## Regression Test` | A test that fails before the fix |
| `## Prevention` | The rule, test or process change that would have caught this |

## Validation Rule

`validate-artifact` reads the spec, checks each Required Section by exact heading, then applies the
Content Rules. A Required Section that is missing, or a Content Rule that fails, is a FAIL. The WARN
checks are listed separately and do not change PASS or FAIL.

The loop that follows a FAIL is the gap-filling prompt in `docs/patterns/delivery-template.md`: one
question per gap, never a guess. The agent MUST NOT set `status: ready` while the status is FAIL.

## Retrieval Frontmatter

Not applicable. A spec is an input to the pipeline, not a pipeline artifact. The seven retrieval fields
apply to the artifacts the agents write from it.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
