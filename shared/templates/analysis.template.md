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
Template for analysis.md. Consumed by the analyst agent.
Structure defined here; contract in shared/contracts/analysis-contract.md validates
that these headings survive intact. Preserve every heading exactly.
-->

# Feature Analysis: [Feature Name]

## Summary
One paragraph plain-English summary of what this feature does and why it matters.

### Acceptance Criteria
List criteria that must be true for this feature to be considered complete. Use BDD given/when/then format where helpful. Focus on *what*, never *how* (Dave Farley).
**Specification by Example (SBE)**: Ambiguity hides in abstractions. You MUST provide concrete examples and data tables for complex business rules, not just abstract Gherkin scenarios.
**MANDATORY**: For any feature containing User Interface (UI) elements, you MUST explicitly define an Accessibility (a11y) requirement (e.g., "Keyboard Navigation must work", "Screen readers must announce X").

### Surfaces
- **UI Surface**: [yes/no — does this feature render something a person looks at?]
- **Served Runtime Surface**: [yes/no — does this run in a deployed process whose availability or latency someone operates? An in-process library or test utility does not.]
- These decide whether `accessibility-engineer`, `visual-qa-engineer` and `sre-engineer` run at all.

### Non-Functional Requirements
- Performance (Must explicitly define SLAs and Timeout thresholds for all external or long-running calls)
- Security considerations (auth, data privacy)
- Scaling considerations (e.g., will this generate millions of rows?)
- **A threshold is a number and a unit** — `p99 request latency: 200ms`. If a requirement has no
  measurable limit, describe it in the requirement text and give it no threshold. A threshold
  routes in both the architect and the performance-engineer, so prose in that field bills for two
  reports that will say "not applicable".

## Proposed Fitness Functions
For every Non-Functional Requirement identified above, propose a measurable fitness function:
- **[Property Name]**: [What is the property to measure?]
- **Verification**: [How would CI verify it? tool, threshold, command]
- **Owner**: [Architect or DevOps]

## Out of Scope
- Things this feature explicitly does NOT do

## Technical Breakdown

### Bounded Context
- **Owning Context**: [Which domain/bounded context owns this feature?]
- **Context Crossings**: [Does this feature require crossing a boundary? e.g., Billing reaching into Identity. If yes, flag as an architectural concern.]

### Domain Events (Event Storming Lite)
- **Commands**: [e.g., `ProcessPayment`]
- **Events Produced**: [e.g., `PaymentProcessed`]
- **Owning Aggregates**: [e.g., `Invoice`]
- **Read Models / Projections**: [What does the UI need to display?]

### Affected Components
- List each file/module likely to be touched and why

### Data Model Changes
- New tables/collections, modified schemas, migration needs
- MUST specify if this is an **Expand** phase (additive/safe, runs before deploy) or a **Contract** phase (destructive/cleanup, runs after deploy). Destructive changes cannot happen in the same release as the code they support.
- Leave empty if not applicable — never write "None" (see the analysis contract: a prose "none" reads as work to the router and invokes an agent to discover it meant zero)

### API Changes
- New endpoints, modified signatures, new request/response shapes
- Leave empty if not applicable — never write "None" (see the analysis contract: a prose "none" reads as work to the router and invokes an agent to discover it meant zero)

### New Dependencies
- Any new packages, services, or external integrations needed
- Leave empty if not applicable — never write "None" (see the analysis contract: a prose "none" reads as work to the router and invokes an agent to discover it meant zero)

## Task List

### Developer Tasks
1. [Specific, actionable task]
2. ...

### QA Tasks
1. [What tests need to be written]
2. [What edge cases to cover]
3. ...

### Tech Writer Tasks
1. [What docs need updating]
2. ...

### DevOps Tasks
1. [Any CI changes, env vars, deploy steps]
2. ...

## Edge Cases and Risks
- [Edge case]: [how to handle it]
- [Risk]: [mitigation strategy]

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Unit tests written and passing
- [ ] Integration tests written and passing
- [ ] Docs updated
- [ ] CI pipeline green
- [ ] Code reviewed (if applicable)
- [ ] No new linting errors
