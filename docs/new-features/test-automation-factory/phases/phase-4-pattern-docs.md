# Phase 4 — Pattern Documents

## Goal
Write the two pattern documents that the blueprints and skills cite. A reader MUST be able to write a correct test from one.

## Inputs
- `blueprints/ui-e2e-saturday-like.md` and `blueprints/api-sunday-like.md` from phase 3.
- `.claude/rules/` from phase 1.
- The project's language and test runner from `docs/test-factory/BASELINE.md`.

## Steps
1. Write `docs/patterns/ui-e2e-site-centric.md` (the Saturday-like pattern). Sections:
   - **Mental model**: a user visits a Site, uses Pages made of Elements and Partials, and completes a business goal in a Flow.
   - **Why not the classic page object model**: pages grow into god objects; this pattern splits them.
   - **Site**: owns one browser context. **Page**: one route; delegates to elements. **Element**: one locator with wait and retry.
     **Partial**: a reusable fragment such as a header. **Flow**: chains pages into a business scenario; step definitions call flows.
     **Filter**: gates or selects by state, for example a base filter class plus a decorator that blocks an Element action until a page condition holds. **Site manager**: hands out sites for multi-site flows.
   - **Gherkin**: the scenario name is the acceptance criterion. Keep the interesting user action visible in the scenario;
     hide setup noise in flows and factories ("moist, not fully dry").
   - **Layers and imports**: which layer may import which.
   - **Example**: one login page, one login flow, one scenario, in the project's language. Check the real framework before you write it: a base class may read abstract members in its own constructor, so use getters there, not fields. List any class the pattern names that the framework package does not export.
2. Write `docs/patterns/api-declarative-client.md` (the Sunday-like pattern). Sections:
   - **Mental model**: tests describe what to call and what to expect. The client owns how.
   - **Base client**: one per service, built on an `HttpAdapter` interface. Tests use a fake adapter for unit speed and a real one for contract tests.
   - **Fluent matchers**: `toHaveStatus`, `toBeSuccessful`, `toRespondWithin`. Add your own in one file.
   - **Schema validation**: validate every response body against a schema. Use `unknown` plus runtime validation. Never `any`.
   - **Resilience**: circuit breaker and exponential backoff from a library. Never a hand-written loop with `sleep`.
   - **Models and factories**: separate files. Factories use a seeded fake-data library.
   - **Coverage matrix**: for each endpoint, list happy path, validation error, auth failure, not found, and timeout. Mark gaps.
   - **Example**: one create-order client method, one test, one schema, in the project's language.
3. Write `docs/patterns/testing-pyramid.md`: the five levels, who writes them, the speed budget for each, and which blueprint applies.
4. Write `docs/patterns/test-annotation.md`: every test carries an issue reference and an acceptance-criterion reference in
   the language's own mechanism (JSDoc `@issue` and `@ac`, pytest markers or docstring, JUnit `@Tag` and `@DisplayName`,
   xUnit `[Trait]`, Go comment, Gherkin tag). Gherkin scenarios need no AC annotation, because the scenario name is the criterion.
5. Each pattern document ends with a "What this does not establish" table. State plainly what following the pattern does not prove.

## Files
`docs/patterns/{ui-e2e-site-centric,api-declarative-client,testing-pyramid,test-annotation}.md`.

## Verification
- Each example compiles or runs in the project's toolchain. Run it. Record the command and result in the pattern file.
- Every class and term used in a pattern document also appears in `DOMAIN_DICTIONARY.md` or is defined in the document.
- Search the pack for conflicting names: `BasePage` is spelled one way everywhere.

## Budget
`max-usd: 5`, `max-attempts: 2`, `max-hours: 2`.

## Time-box
Stage timeout 30 minutes. Write one pattern document per stage so each stays inside the timeout.
