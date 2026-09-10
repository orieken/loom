---
feature: "<feature-name>"
bounded_context: "<owning-bounded-context>"
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

<!--
Template for context-manifest.md. Consumed by the context-engineer agent.
Structure defined here; contract in shared/contracts/context-manifest-contract.md validates
that these headings survive intact. Preserve every heading exactly.
-->

# Context Manifest: [Feature/Task Name]

## 1. Scope and Boundaries
- **Target Component**: [e.g. user-auth, billing]
- **Relevant Layers**: [e.g. Domain Entities, Application Use Cases]
- **Bounded Context**: [e.g. Identity & Access]

## 2. Pinpoint Files (To Keep Open)
List specific files that must be opened or referred to, specifying line ranges where appropriate. Prefer
high-fidelity references (interfaces, tests, schemas, fixtures, mockups, rubrics) over prose paraphrases:
- [File Name](file://<absolute_path>#L10-L45) -- [Reference type] -- [Reason, e.g., "Defines the IUser repository interface"]
- [File Name](file://<absolute_path>) -- [Reference type] -- [Reason]

## 3. Global Rules and Constraints
List reference files that establish the patterns:
- [ARCHITECTURE_RULES.md](file:///<absolute_path_to_ARCHITECTURE_RULES.md>)
- [DOMAIN_DICTIONARY.md](file:///<absolute_path_to_DOMAIN_DICTIONARY.md>)

## 4. Knowledge Items & ADRs (To Load)
- [KI Name](file://<path_to_ki>) -- [Why it is relevant, e.g., "Contains database mock patterns"]
- [ADR Name](file://<path_to_adr>) -- [Why it is relevant, e.g., "Defines why we use Vitest instead of Jest"]

## 5. Prior Deliveries in This Bounded Context
- [Feature Name](docs/features/<name>/) -- [delivered date if known] -- [key lesson from its
  retrospective.md's "What Went Poorly"/"What To Improve", e.g. "Missed the user-enumeration edge case on
  first pass — check for it explicitly this time"]
- [Feature Name](docs/features/<name>/) -- [no retrospective.md exists for this one — note that plainly rather than skipping it silently]
— or "No prior deliveries found in this bounded context" if none match

## 6. Prune Recommendations (To Close)
List files currently open or under consideration that must be closed to avoid context drift:
- [ ] [File Name](file://<absolute_path>) -- [Unrelated context]
- [ ] [File Name](file://<absolute_path>) -- [Different architecture layer]

## 7. Token Budget
- **Target agent tier**: [analyst (analyst/architect, ≤60%) | developer (≤80%) | reviewer (≤40%)] of a 200k-token context window
- **Estimated total tokens**: _measured by loom from the pinned files (bytes ÷ 4) — leave this blank_
- **Status**: _measured by loom — leave this blank_

Do not fill in the total or the status. They are computed from the files pinned above and written
here for you. The heuristic this template used to ask for was ~7× under on real prose, and it
reported `OK` with a per-file breakdown and a percentage behind it (see the analyst's counterpart
note and roadmap L3.25).
- **Cut recommendations (if WARNING)**: [file] -- [reason it's the lowest-signal pin]
