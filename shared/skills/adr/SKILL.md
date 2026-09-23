---
name: adr
description: Effortless, consistent Architecture Decision Records.
triggers:
  keywords: ["ADR", "decision", "record", "document"]
  intentPatterns: ["Write an ADR for *", "Document the decision to *", "We decided to *, record it", "Create an architecture decision record"]
standalone: true
---

## When To Use
When the user explicitly asks to document an architectural decision or asks to write an ADR.

## Context To Load First
1. All existing ADRs in `docs/adrs/` (for next number + tone)
2. `ARCHITECTURE_RULES.md`
3. `DOMAIN_DICTIONARY.md`
4. `.claude/feature-workspace/<feature-name>/architecture-notes.md` if it exists.

## Process
1. Determine next ADR number (scan `docs/adrs/` for existing files)
2. Ask: "What was decided?" (one sentence, active voice) — one question at a time
3. Ask: "What was the context that made this decision necessary?"
4. Ask: "What alternatives were considered and why were they rejected?"
5. Ask: "What are the consequences — easier, harder, changed?"
6. Ask: "Does this decision produce a fitness function? If so, how is it enforced?"
7. Draft the ADR and show it for approval
8. On approval, write to `docs/adrs/ADR-[NNN]-[kebab-title].md`
9. Update `docs/adrs/README.md` (index) — create it if it doesn't exist

## Output Format
```markdown
# ADR-[NNN]: [Title — active voice: "Use X" not "Decision to use X"]

Date: YYYY-MM-DD
Status: Accepted
Deciders: [from git config user.name]
Technical Story: [feature or ticket that prompted this]

## Context
[What situation, constraint, or force prompted this decision]

## Decision
[What was decided — specific, concrete, active voice]

## Alternatives Considered
| Option | Why Rejected |
|---|---|

## Consequences
- **Easier**: [what this unlocks]
- **Harder**: [what this constrains]
- **Changed**: [what is different going forward]

## Fitness Function
[How this decision is enforced in CI — or "Judgment-only: [reason]"]
```

## Guardrails
- Never write an ADR without user approval of the draft
- Never number an ADR without checking the existing sequence
- ADR titles use active voice
- Never write the consequences as implementation details — write effects, not mechanisms

## Standalone Mode
Fully conversational. Generates the Markdown logic locally.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
