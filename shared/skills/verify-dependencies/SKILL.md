---
name: verify-dependencies
description: Enforces Clean Architecture boundaries by analyzing imports. Fails if the Domain layer imports from outer layers (like adapters, external APIs, etc.).
triggers:
  keywords: ["clean architecture", "dependency inversion", "import directions", "domain boundaries", "layer violation"]
  intentPatterns: ["check that imports are valid", "is the domain layer cleanly separated"]
standalone: true   # must work without MCP/external systems
---

## When To Use
Use this skill when reviewing code to ensure the dependency rule is respected. 
Specifically apply it to classes residing in the Entities / Domain Layer and Use Cases / Application Layer.
Do NOT use when modifying UI code or end-to-end testing frameworks.

## Context To Load First
1. `ARCHITECTURE_RULES.md`
2. `.claude/feature-workspace/<feature-name>/architecture-notes.md` (if available)

## Process
1. Determine the files or directories to check.
   - What to do: Find the root folder of the application or the specific domain directory being modified.
   - What to produce: A list of target directories.
   - What to check: Verify targets exist.
1b. Query the repo graph (when available).
   - What to do: If the `loom` MCP server is registered and the repo was indexed (`loom index`), call
     `mcp__loom__dependencies_of` for each target directory. Every edge carries `confidence` (`exact` or
     `approximate`) and the `layer` the context index recorded for its target (empty when undocumented).
     Layers run innermost to outermost: `domain` → `application` → `adapter` → `framework`; `shared`, `test`,
     `docs`, `config`, `tooling` and `unknown` are neutral.
   - What to produce: a violation for each `exact` edge from a `domain` directory to an `application`,
     `adapter` or `framework` one, or from an `application` directory to an `adapter` or `framework` one. An
     `approximate` crossing is reported as a *possible* violation to confirm, never as a certain one.
   - What to check: `fresh` on every answer. `false` means the graph predates the working tree, so confirm
     with step 2 before reporting. An edge into an undocumented directory has no layer and cannot be judged:
     list it as unclassified instead of passing it.
   - This replaces reading imports in prose; step 2 remains the check for third-party libraries imported into
     the domain, which the graph does not cover.
2. Run dependency analysis.
   - What to do: Run `./.claude/skills/verify-dependencies/check.sh [target]`
   - What to produce: The console output of dependency-cruiser or equivalent checking tool.
   - What to check: Verify if the Domain layer imports anything from Use Cases, Adapters, or Infrastructure.
3. Handle dependency violations.
   - What to do: If external dependencies (e.g., `axios`, `express`, UI libraries) are imported into the Domain, you MUST REJECT the code.
   - When to pause: If complex dependency inversions require substantial architectural refactoring, pause for Architect or human approval.

## Output Format
```markdown
# Dependency Verification Report

## Violations
- `src/domain/payment.ts`: Imports from outer layer `src/infrastructure/stripe-client.ts`.

## Recommended Fixes
- Create an interface `IPaymentGateway` in the domain layer. 
- Implement `StripePaymentGateway` in the infrastructure layer. 
- Inject the dependency instead of directly importing it.
```

## Guardrails
- You MUST NOT approve code that couples core domain log to third-party frameworks or databases.
- The dependency rule (dependencies point ONLY inward) is rigid and non-negotiable.

## Standalone Mode
If the repo-graph tools are unavailable, or `check.sh` is unavailable (e.g., `dependency-cruiser` is not installed), manually read the `import` statements at the top of all modified files. Flag any imports in `src/domain/` that reference `src/infrastructure/`, `src/adapters/`, or specific vendor libraries (like React, TypeORM, Axios).

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
