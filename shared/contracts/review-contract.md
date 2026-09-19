# Contract: code-review-report.md

**Produced by**: code-reviewer
**Consumed by**: security-reviewer, qa-engineer, orchestrator (deliver-feature)

## Typed State (`loom run`)

Under `loom run` this artifact is typed state conforming to
`shared/schemas/pipeline/review.schema.json` (generated from `internal/state/` —
never hand-edit it), and `code-review-report.md` is rendered from it. The
verdict is a field, which is what the review loop's exit condition reads
(roadmap L2.17). The rendered view still carries the bolded literal under
`## Overall Status`, because the markdown pipeline still parses that string —
the validation rule below describes that path and remains authoritative for it.

One rule the typed form adds: a `CHANGES_REQUESTED` verdict must carry at least
one blocking finding. A rejection with nothing actionable would send the loop
round again with nothing for the developer to do.

## Required Sections (exact heading text and level)
- `## Overall Status`
- `## Design Narrative`
- `## Design Score`
- `## Security Surface`
- `## Performance Surface`
- `## Test Design Review`
- `## Verification of Developer Self-Review`
- `## Feedback for the Developer`

## Validation Rule
`validate-artifact` checks presence of every heading above, plus:
- `## Overall Status` must contain exactly one of `APPROVED` or `CHANGES REQUESTED` (bolded, per the agent's own template) — anything else is a FAIL, since the orchestrator's CHANGES REQUESTED loop parses this literal string.
- `## Design Score` must contain all four dimensions (Clarity, Cohesion, Coupling, Craft) with a numeric 1-5 rating each — a missing dimension is a FAIL.
- `## Design Narrative` ends with `Behaviour change: yes | no | cannot tell` for the diff as a whole. `cannot tell` is a valid answer and is treated as unknown; it is never read as "no". Under the executor this becomes `ReviewState.BehaviorChange`, which a policy may read only to require human attention, never to auto-approve (`shared/policies/policy-schema.md`).
- `## Test Design Review` is never empty. When the diff adds or modifies no tests, it says so explicitly. Under the executor this is enforced by `ReviewState.Validate()`; in the markdown pipeline it is the reviewer's discipline, because an empty section still renders the heading and passes a presence check. "There were none to judge" and "I did not look" are different facts.

This is a structural check only. It does not re-judge whether APPROVED was the right call — that's the security-reviewer and qa-engineer's job to catch downstream, and the human's job at the CHANGES REQUESTED checkpoint.

## Retrieval Frontmatter (WARN)

Pipeline artifacts should include a YAML frontmatter block at the very top of the file. Missing or incomplete retrieval frontmatter triggers a **WARN** from `validate-artifact` — not a FAIL. Existing archived artifacts without frontmatter are unaffected.

```yaml
---
feature: "<feature-name>"             # kebab-case slug derived from the feature file name
bounded_context: "<context>"          # owning bounded context (from DOMAIN_DICTIONARY.md domain list)
domain_terms: []                      # canonical terms from DOMAIN_DICTIONARY.md used in this feature
files_touched: []                     # repo-relative paths of files created or modified
issue_refs: []                        # ticket/issue references (e.g., PROJ-123, #456)
linked_adrs: []                       # repo-root-relative paths to referenced ADRs
linked_kis: []                        # repo-root-relative paths to referenced Knowledge Items
---
```

Once frontmatter adoption is visible across a project's feature archive, this check will be promoted to FAIL in a future release.
