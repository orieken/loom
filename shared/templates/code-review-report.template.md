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
Template for code-review-report.md. Consumed by the code-reviewer agent.
Structure defined here; contract in shared/contracts/review-contract.md validates
that these headings survive intact. Preserve every heading exactly.
-->

# Code Review Report: [Feature Name]

## Overall Status
**[APPROVED | CHANGES REQUESTED]**

## Design Narrative
[2-3 sentence plain-English description of what the code is doing architecturally]

Behaviour change: [yes | no | cannot tell]

## Design Score
- **Clarity** [1-5]: Does the code reveal its intent without comments?
- **Cohesion** [1-5]: Does each class/module do one well-defined thing?
- **Coupling** [1-5]: Are dependencies minimal and pointing the right direction?
- **Craft** [1-5]: Was the refactor pass taken seriously? (Checks the developer's Named Refactoring Log)

*(Note: Score of 3+ on all dimensions = APPROVED. Any dimension below 3 = CHANGES REQUESTED. Provide specific Fowler refactoring operations to improve any dimension scoring < 3)*

## Security Surface
- [Auth/Inputs/APIs touched or "None"]

## Performance Surface
- [DB calls/Network calls/Loops or "None"]

## Test Design Review
- [Are tests verifying behaviors instead of implementation details?]

| Test (added/modified in this diff) | Would it fail if the named behavior broke? | Type (if NO) |
|---|---|---|
| [test name] | [YES / NO / UNCERTAIN] | [vacuous / self-fulfilling / unreached / disabled, or the file you would need to read if UNCERTAIN] |

## Verification of Developer Self-Review
- [Did the developer's self-review match reality? If not, explicitly call out the discrepancy]

## Feedback for the Developer

*(If Approved, you can just leave a note of encouragement or minor non-blocking suggestions)*

*(If Changes Requested, provide specific, named refactoring instructions):*

### 1. [Specific Refactoring Operation, e.g., "Extract Function"]
- **File**: `path/to/file.ts` lines X-Y
- **Smell**: This method calculates the outstanding balance AND prints the receipt. Multiple responsibilities.
- **Instruction**: Extract the calculation logic into `calculateOutstanding(invoice)` and reduce this function's length.

### 2. [Specific Architectural Violation]
- **File**: `path/to/file.py`
- **Smell**: The domain entity is directly importing `psycopg2` (Infrastructure).
- **Instruction**: Abstract this behind a repository interface so the domain remains pure.

### 3. ...
