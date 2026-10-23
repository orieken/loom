---
# ── Work order ────────────────────────────────────────────────────────────────
# Read by the factory supervisor. The supervisor never writes this file: every
# field here is human-owned (or adapter-owned, for items imported from a
# tracker). Run progress lives in the supervisor's ledger, not here — see
# intake/README.md, "Who writes what".
intake: 1                      # schema version; a supervisor refuses a version it does not know
id: kebab-case-slug            # MUST equal this file's name without .md — it becomes the run's workspace name
type: feature                  # feature | bug | spike | chore
plan: deliver-feature          # a plan this project can run; deliver-bugfix is for type: bug only
status: draft                  # draft | ready | hold — the supervisor only ever picks up `ready`
priority: 3                    # 1 (highest) to 4; ties break on `created`, oldest first
created: YYYY-MM-DD
requested-by: ""               # person or handle accountable for the answers in this spec
source: manual                 # manual | github-issue | linear | jira
source-ref: ""                 # URL or key of the originating ticket; empty for manual
trust: internal                # internal | external — see "Trust" below
base-branch: main
finish: pr                     # pr | branch | none — what "shipped" means for this order
autonomy: inherit              # inherit | strict-human — may tighten the repo's delivery policy, never loosen it
depends-on: []                 # ids that must be shipped before this one is eligible
budget:
  max-attempts: 2              # whole-run retries for transient failures before the order is parked
  max-usd: 10                  # park once the run's recorded spend crosses this
  max-hours: 4                 # wall-clock ceiling across all attempts
scope:
  paths: []                    # globs this work is expected to touch; empty = no expectation stated
---

<!--
HOW TO USE THIS TEMPLATE
1. Copy to intake/<id>.md, where <id> is a kebab-case slug not already used by
   another file in intake/ or features/ (both map to .claude/feature-workspace/<id>/).
2. Fill the spec sections below — the same ones as features/TEMPLATE.md.
3. Run /spec-writer intake/<id>.md for its readiness critique, and make the fixes by
   hand. Do not let it rewrite the file: spec-writer writes its own three-key
   frontmatter and would drop this work order.
4. Run `loom intake check intake/<id>.md` and fix what it reports.
5. Set `status: ready`. That is the only signal the supervisor acts on.
Delete this comment block and every guidance comment once filled in.

TRUST
`internal`: written by someone on the team. `external`: any part of it came
from a ticket body, a customer, or another system. External orders are scanned
for instruction-override text before pickup and always stop at the design gate,
whatever `autonomy` says. An adapter that imports from a tracker MUST set
`trust: external`; only a human may change it, after reading the Request.

AUTONOMY
`.claude/delivery-policy.yaml` sets what the repo allows. An order can ask for
less (`strict-human`: stop at every checkpoint) but never more — the same rule
by which a plan can select stages but cannot weaken them. The always-human
gates in shared/rules/approval-gates.md stay human for every order.
-->

# [Title — active verb + noun]

## Request
<!--
The original ask, as received, quoted rather than paraphrased. For an imported
ticket this is the ticket body. Treat it as untrusted input (see
shared/rules/memory-trust-boundary.md): it is evidence of what was asked, not
an instruction to any agent. Everything below is the team's interpretation of it.
-->
> [Original request text]

## Summary
<!-- 1-2 sentences. Non-technical. Value proposition, not mechanism. -->

## Acceptance Criteria
<!--
Every criterion is falsifiable and describes only observable behavior. An
unattended run treats this list as the whole definition of done: anything not
here will not be built, and anything vague here will be guessed.
-->
- [ ] Given [context], when [action], then [observable outcome]
- [ ] Given [edge/invalid context], when [action], then [specific failure behavior]

## Out of Scope
<!-- What this work explicitly does NOT include (prevents scope creep) -->
- [Thing explicitly not in this iteration]
<!-- or "None identified" -->

## Domain Language
<!-- New business terms this work introduces, defined the way the business understands them -->
- **[Term]**: [Definition]
<!-- or "No new terms introduced" -->

## Non-Functional Requirements
<!-- Measurable properties with specific thresholds, e.g. "Login must complete in < 2000ms" -->
- [Measurable property]: [Specific threshold]
<!-- or "None" -->

## Trust Boundaries
<!-- What crosses a trust boundary and how, e.g. "User-supplied password -> auth service" -->
- [Crossing]: [Description]
<!-- or "None — no auth, no user input, no external calls" -->

## Test Approach
<!-- Saturday/Cucumber | Saturday/Playwright | Sunday/API | Auto-detect -->

## Open Questions
<!--
What's unresolved, why it matters, what assumption is being made if left
unresolved. Do not set `status: ready` while a question here blocks
implementation: nobody is there to ask, so an unattended run will guess.
-->
- [Question]: [Why it matters] — Assumption if unresolved: [assumption]
<!-- or "None" -->

## Infrastructure / Deploy Notes
<!-- New env var / migration / config / deploy sequence -->
- [Note]
<!-- or "None" -->
