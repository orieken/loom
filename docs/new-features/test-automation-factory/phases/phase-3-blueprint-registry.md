# Phase 3 — Blueprint Registry and Custom Blueprints

## Goal
Ship three built-in blueprints, a folder where teams add their own, a registry that lists both, and a validator.

## Inputs
- `BLUEPRINT.md` and `templates/BLUEPRINT-TEMPLATE.md` in this pack.
- The intake validator from phase 2.

## Steps
1. Create `blueprints/ui-e2e-saturday-like.md` from `templates/BLUEPRINT-TEMPLATE.md`. Content:
   - Fixed pattern: `BaseSite`, `BasePage`, `BaseElement`, `BasePartial`, `BaseFlow`, a `Filter` decorator for
     cross-cutting behavior, and a site manager for multi-site flows. No classic page object model.
   - Layers: Domain (abstractions), Use-Cases (concrete sites, pages, flows), Adapters (browser driver, telemetry),
     Test Runner (Gherkin steps), Reporting.
   - Soft choice: the browser driver (Playwright, Cypress, Selenium) and the language.
   - Guardrails: Gherkin scenario names are the acceptance criteria; telemetry only in adapters; pages never call the driver directly.
2. Create `blueprints/api-sunday-like.md`. Content:
   - Fixed pattern: a `BaseApiClient` per service, an `HttpAdapter` interface, fluent matchers (`toHaveStatus`,
     `toBeSuccessful`, `toRespondWithin`), schema validation on every response, and resilience primitives
     (circuit breaker, exponential backoff).
   - Guardrails: no custom retry loops; every call has a timeout; models and factories live in separate files; no raw `any`.
3. Create `blueprints/unit.md`. Content: Arrange, Act, Assert; one concept per test; test names describe behavior;
   doubles only at interface boundaries; fixtures in a `fixtures/` folder.
4. Create `blueprints/custom/README.md`. Explain: copy the template, name the file `<id>.md`, add a registry entry with
   `"custom": true`, mark an exemplar, run the validator.
5. Create `blueprints/registry.json`. Each entry:
   ```json
   {
     "id": "api-sunday-like",
     "name": "API tests, Sunday-like",
     "oneLiner": "Declarative API clients, adapter interface, fluent matchers, schema validation.",
     "blueprint": "blueprints/api-sunday-like.md",
     "testType": "api",
     "languages": { "primary": "typescript", "supported": ["typescript", "python", "java", "csharp", "go"] },
     "custom": false,
     "status": "stable"
   }
   ```
   List all three built-ins. Add a top-level `"schema": 1`.
6. Write `scripts/validate-blueprints.<ext>`. It MUST:
   - reject an unknown registry key;
   - check every registry `blueprint` path exists;
   - check each blueprint file has these headings: `When To Use`, `Layer Structure`, `Key Abstractions`,
     `Testing Pyramid Coverage`, `Exemplars`, `Budgets and Time-Boxes`, `Guardrails`;
   - check each blueprint file has an `**Registry id**` line that matches its registry entry;
   - check a custom blueprint lives under `blueprints/custom/`, and a built-in does not;
   - exit 1 on any failure and name the file.
7. Write tests for the validator: one passing case and one failing case per check.
8. Connect the intake validator: an order's `blueprint` MUST exist in the registry, and its `test-type` MUST equal the entry's `testType`.

## Files
`blueprints/{ui-e2e-saturday-like,api-sunday-like,unit}.md`, `blueprints/custom/README.md`, `blueprints/registry.json`,
`scripts/validate-blueprints.<ext>`, tests, and the intake validator change.

## Verification
- The validator passes on the three built-ins.
- Add a temporary custom blueprint with a missing heading. The validator fails and names the heading. Remove the temporary file.
- The intake sample from phase 2 now validates against the registry.
- Changed-line coverage is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 5`, `max-attempts: 2`, `max-hours: 2`.

## Time-box
Stage timeout 30 minutes. Writing three blueprints is documentation work. Cap that step at 20 minutes; park the phase if it runs over.
