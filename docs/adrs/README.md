# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) documenting significant technical and structural decisions made in this repository.

---

## Purpose

ADRs capture the context, decision, and consequences of architectural choices. They exist so that future contributors and agents understand not just what was decided, but why. ADRs are immutable once accepted -- if a decision is reversed, a new ADR supersedes the original.

---

## Format

Each ADR follows this structure:

```
# ADR-NNN: Title

## Status
Accepted | Superseded by ADR-NNN | Deprecated

## Date
YYYY-MM-DD

## Context
What is the issue? What forces are at play?

## Decision
What is the change that is being proposed or has been agreed upon?

## Consequences
What becomes easier? What becomes harder? What are the trade-offs?
```

---

## Numbering Convention

ADRs use a three-digit sequential number prefixed with `ADR-`:

- `ADR-001`, `ADR-002`, `ADR-003`, etc.

File names follow the pattern: `ADR-NNN-short-description.md` in kebab-case.

---

## Index

| ADR     | Title                                           | Status   | Date       |
|---------|-------------------------------------------------|----------|------------|
| ADR-001 | Adopt RAG-friendly documentation structure      | Accepted | 2026-04-08 |
| ADR-002 | Corpus-aware retrieval strategy (graduated RAG) | Accepted | 2026-07-22 |
| ADR-003 | Enterprise memory sync protocol                 | Proposed | 2026-07-31 |
| ADR-004 | Bugfix artifacts use the features namespace     | Accepted | 2026-08-05 |
| ADR-005 | Use explicit `.sh` install script paths          | Accepted | 2026-08-15 |
| ADR-006 | Loom executes pipelines                         | Accepted | 2026-08-29 |
| ADR-007 | UI test location and the evidence-bundle contract | Accepted | 2026-08-31 |
| ADR-008 | Define "tested" by what the tests can catch, measured on the change | Accepted | 2026-09-22 |
| ADR-009 | Retire the unit-level TDD ritual for agents; keep spec-derived acceptance tests | Accepted | 2026-09-22 |

---

## Usage

- **Agents** read this directory before proposing architectural changes to check for prior decisions and constraints.
- **Humans** review ADRs during onboarding and design reviews to understand the rationale behind the current structure.
- **The delivery pipeline** links relevant ADRs in architecture-notes.md and docs-report.md artifacts for each feature.
