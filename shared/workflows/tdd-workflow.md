---
workflow: tdd
version: 1.0.0
description: Red-Green-Refactor loop — test-writer → implementer → refactor-reviewer → coverage-auditor, iterated until the suite is green and coverage >= 85%
entry: test-driven-developer
resumable: true
parallelStrategy: sequential-simulation
checkpointStore: .claude/feature-workspace/tdd-state.json
legacyFallback: test-driven-developer
---

# TDDWorkflow

Defines the Red-Green-Refactor loop for `test-driven-developer`. The agent
(`shared/agents/test-driven-developer.md`) remains the **external invocation contract**. This workflow
is the internal structure the `/orchestrate` runtime executes when the agent is invoked through the
runtime path, or when `/orchestrate --workflow tdd` is called explicitly.

**External contract unchanged**: teams that invoke `test-driven-developer` directly continue to work
identically to v3.1. This workflow only activates when `/orchestrate --workflow tdd` is called.

## Legacy Fallback

`/orchestrate --legacy --workflow tdd --spec <file>` routes to `test-driven-developer` directly,
bypassing this workflow entirely. This fallback persists through v3.x.

## Named Internal Roles

| Role Name | Played by | Purpose |
|---|---|---|
| Test Writer | `unit-tester` (in write mode) | Writes failing tests that express acceptance criteria |
| Implementer | `developer` | Writes minimum code to make tests pass (Green) |
| Refactor Reviewer | `developer` + `code-reviewer` audit | Refactors to meet complexity/style rules without breaking tests |
| Coverage Auditor | `unit-tester` (in audit mode) | Verifies coverage ≥ 85% and all tests pass after refactor |

## Loop Structure

The Red-Green-Refactor loop repeats until the exit criteria are met:

```
[RED]     test-writer writes failing tests
[GREEN]   implementer writes minimum code to pass
[REFACTOR] refactor-reviewer improves structure
[AUDIT]   coverage-auditor verifies coverage >= 85% + all tests green
  └─ if coverage < 85% or tests fail → loop again from [GREEN]
  └─ if loop count > maxIterations → halt, surface to human
```

## Stage Definitions

```yaml
stages:
  - id: search-ki
    role: unit-tester
    produces: tdd-ki-search.md
    parallel: false
    checkpoint: false
    description: >
      Invoke search-ki with the feature's domain/keywords to surface existing patterns.
      Read-only; proceeds regardless of results. Notes findings in tdd-report.md preamble.

  - id: red
    role: unit-tester
    produces: test-suite.md
    parallel: false
    checkpoint: true
    description: >
      Write failing tests for all acceptance criteria. Tests must fail before implementation
      starts (Uncle Bob's Law 1). Annotate each test with issue reference + AC reference per
      shared/rules/testing-conventions.md Test Annotation Convention
      (per-language syntax: docs/patterns/test-annotation.md).
    audit:
      agent: tool-validator
      onFail: retry
      maxRetries: 2

  - id: green
    role: developer
    produces: implementation.md
    parallel: false
    checkpoint: true
    description: >
      Write the minimum production code to make the failing tests pass. No more. Tests must
      be green after this stage before refactor begins (Law 3: no more code than to pass the test).

  - id: refactor
    role: developer
    produces: refactor-notes.md
    parallel: false
    checkpoint: true
    description: >
      Refactor to meet the framework's complexity constraints (cyclomatic < 7, functions < 30 LOC,
      Sandi Metz limits). Tests must still be green after refactor.
    audit:
      agent: code-reviewer
      onFail: retry
      maxRetries: 3

  - id: coverage
    role: unit-tester
    produces: coverage-report.md
    parallel: false
    checkpoint: true
    description: >
      Run the full test suite. Verify: (1) all tests pass, (2) coverage >= 85%.
      If either fails, the loop retries from [green] with the failing test output as context.

  - id: documentation
    role: developer
    produces: tdd-report.md
    parallel: false
    checkpoint: true
    description: >
      Generate feature documentation: KI candidates, API changes, edge cases handled,
      implementation approach. This is the final step, executed only after coverage audit passes.
```

## Loop Control

```yaml
loopControl:
  loopStages: [green, refactor, coverage]  # stages that repeat
  exitCondition: "coverage.allTestsPass == true && coverage.coveragePercent >= 85"
  maxIterations: 5   # halt and surface to human if loop doesn't converge
  onMaxIterations: halt
```

## Coverage Gate

The coverage gate (`coveragePercent >= 85`) matches the framework-wide non-negotiable defined in
`CLAUDE.md`. This is the same threshold enforced manually in the standalone agent — extracting it
into the workflow makes it machine-verifiable and consistent.

## Retry Policy

| Stage | On audit FAIL | Max retries |
|---|---|---|
| `red` (tool-validator) | Rewrite tests to fix annotation violations | 2 |
| `refactor` (code-reviewer) | Refactor until APPROVED or complexity < 7 | 3 |
| Coverage loop | Retry green→refactor→coverage | 5 (maxIterations) |

## Behavior Preservation Check

The workflow mirrors the standalone `test-driven-developer` agent's process exactly:

| Agent Step | Workflow Stage | Same? |
|---|---|---|
| Step 2: invoke search-ki | `search-ki` stage | ✓ |
| Step 3: write tests (annotated) | `red` stage | ✓ |
| Step 4: run tests — confirm they fail | `red` stage exit condition | ✓ |
| Step 5-7: implement, run, iterate | `green` + loop | ✓ |
| Step 8: generate documentation | `documentation` stage | ✓ |
| Step 9: final summary | tdd-report.md | ✓ |

No step is removed or reordered. The workflow adds loop termination tracking, coverage gate
enforcement, and audit invocations — all additive, none breaking.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 3 Runtime layer. CC BY 4.0.*
