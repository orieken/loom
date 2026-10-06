# Contract: context-manifest.md

**Produced by**: context-engineer
**Consumed by**: analyst, architect, developer, and every other pipeline agent (all read it first, per their
own "Context To Load First" sections)

## Required Sections (exact heading text and level)
- `## 1. Scope and Boundaries`
- `## 2. Pinpoint Files (To Keep Open)`
- `## 3. Global Rules and Constraints`
- `## 4. Knowledge Items & ADRs (To Load)`
- `## 5. Prior Deliveries in This Bounded Context`
- `## 6. Prune Recommendations (To Close)`
- `## 7. Token Budget`

## Optional Section: Blast Radius
When the repo graph is available (`loom index`), `context-engineer` records what the change reaches under an
**unnumbered** `## Blast Radius` heading, before `## 7. Token Budget`. It is optional, so existing manifests stay
valid, and absent when the repository has no graph.

Under `loom run` the section is typed: the agent supplies only `blastRadius.seeds` and `depth`, and the executor
computes the rest (`graphSha`, `fresh`, `total`, `impacted`) from the graph and replaces anything the agent wrote
there, as it does the token budget. It lists each reached directory with its distance and whether the chain of
edges is `exact` or `approximate`, says when the graph was stale, and says why when it could not be resolved. Outside
`loom run` the agent copies the output of `mcp__loom__impact` and marks it as not executor-verified.

## Validation Rule
- `## 7. Token Budget` must contain a `**Status**` line with value `OK` or `WARNING` — never blank and
  never any other value. If `WARNING`, a `**Cut recommendations**` line must also be present (per
  `context-engineer`'s own guardrail: never report a budget without cut recommendations to back it up).
- `## 5. Prior Deliveries in This Bounded Context` must not be empty — it must contain either at least one
  feature reference or the literal phrase "No prior deliveries found in this bounded context" (per
  `context-engineer`'s own guardrail: never skip this section silently).

This is a structural check only. It does not verify the Pinpoint Files are actually the *right* files, or
that the token estimate is accurate — that's judgment, caught downstream if analyst or developer finds the
manifest missing something relevant (see `context-audit` for after-the-fact waste analysis).

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
