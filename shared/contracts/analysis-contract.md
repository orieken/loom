# Contract: analysis.md

**Produced by**: analyst
**Consumed by**: architect, performance-engineer, data-engineer, developer, qa-engineer, tech-writer, devops-engineer

## Typed State (`loom run`)

Under `loom run`, this artifact is **typed state**, not a markdown document: the stage returns JSON
conforming to `shared/schemas/pipeline/analysis.schema.json` (generated from `internal/state/` — never hand-edit it),
the executor validates it, and `analysis.md` is *rendered* from that state as a human-readable view
(roadmap L2.9). The rendered view reproduces every heading below, so the structural check in this
contract still describes what a reader sees; the machine handoff no longer goes through it.

Two consequences worth knowing: the view is derived, so editing it changes nothing a downstream
stage reads — the state document under `state/` is what integrity tracks (L2.12). And a downstream
stage receives only the fields its projection declares, not the whole document.

For the markdown pipeline (the `deliver-feature` skill), everything below remains authoritative
exactly as written.

## Required Sections (exact heading text and level)
- `## Summary`
- `### Acceptance Criteria`
- `### Non-Functional Requirements`
- `## Proposed Fitness Functions`
- `## Out of Scope`
- `## Technical Breakdown`
- `### Bounded Context`
- `### Domain Events (Event Storming Lite)`
- `### Affected Components`
- `### Data Model Changes`
- `### API Changes`
- `### New Dependencies`
- `## Task List`
- `### Developer Tasks`
- `### QA Tasks`
- `### Tech Writer Tasks`
- `### DevOps Tasks`
- `## Edge Cases and Risks`
- `## Definition of Done`

## What "nothing to do" looks like

**A section with nothing in it is left empty. Never write "None", "N/A", or "None required by this
spec".**

This is not a style preference; it decides which agents run and what the run costs. The executor
routes from the typed analysis (roadmap L3.0), and a routing predicate that counts list items
cannot tell a prose "none" from work. The second real end-to-end run emitted exactly one DevOps
task — *"None required by this spec — no CI or deployment config changes requested."* — and the
router counted one item and spent **$0.64** invoking `devops-engineer` to discover the sentence
meant zero. An empty list is the only unambiguous way to say there is nothing here.

The router ignores entries opening with "none", "n/a", "nothing", "not applicable" or "not
required" as a defensive net, but do not rely on it: the net exists because models write these
phrases reflexively, not to make writing them acceptable.

## Thresholds are numbers, not sentences

A **Non-Functional Requirement** carries a threshold **only when there is a number and a unit** —
`p99 request latency, 200, ms`. If the requirement has no measurable limit, describe it in the
requirement text and leave the threshold out entirely.

A threshold routes in both the `architect` and the `performance-engineer`. The third real run put
*"O(n) over the captured logs array... no I/O"* in the threshold field of a three-line synchronous
array filter, and both stages ran — **$1.45** — to report that nothing applied. A sentence in the
threshold field is a bill, not a description.

## Surfaces decide who reviews

Declare whether the feature has a **UI surface** (something a person looks at) and a **served
runtime surface** (a deployed process whose availability or latency someone operates). An
in-process library or test utility has neither.

These decide whether `accessibility-engineer`, `visual-qa-engineer` and `sre-engineer` run. Absent
means no surface, which is the honest default: a feature that renders nothing and serves nothing
should not summon reviewers for either.

## Validation Rule
`validate-artifact` checks presence of every heading above, exact string and level match. Missing a heading is a FAIL — even if the content would logically live under a sibling section, downstream agents (developer, qa-engineer, tech-writer, devops-engineer) grep for these exact headings to find "their" task list.

This is a structural check only. It does not verify the content is correct, complete, or non-placeholder — that judgment belongs to the human PAUSE checkpoint after analyst and to the architect/developer who consume it.

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
