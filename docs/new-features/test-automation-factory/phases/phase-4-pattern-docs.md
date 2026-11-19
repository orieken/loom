# Phase 4 — Pattern Documents

## Goal
Find out whether the testing framework the patterns need already exists. If it does not, turn the missing pieces into work orders, not into code written here. Then write the pattern documents that the blueprints and skills cite. A reader MUST be able to write a correct test from one.

This phase does not build a framework. A framework is real product work with its own tests, review, and budget, and it belongs in orders.

## Inputs
- `blueprints/ui-e2e-saturday-like.md` and `blueprints/api-sunday-like.md` from phase 3.
- `.claude/rules/` from phase 1.
- The project's language and test runner from `docs/test-factory/BASELINE.md`.
- The language for the testing framework. The human chooses it; it may differ from the product's language. If it is not recorded in `BASELINE.md`, ask now. Do not choose from the tools installed on this machine.
- `templates/INTAKE-TEMPLATE.md` and the intake validator from phase 2.

## Steps
1. **Find the framework layer.** For each test type in scope, look in the project and its dependencies for the building blocks the pattern names.
   - UI end-to-end: Site, Page, Element, Flow, plus Partial, Filter, and a site manager.
   - API: an HTTP adapter interface, a base client, fluent matchers, schema validation, and resilience (circuit breaker, backoff).
   - Write the result to `docs/test-factory/FRAMEWORK-STATUS.md`: one row per block with `found`, `found with gaps`, or `missing`, where it lives, and how you checked (a command or a file path). A block that exists but breaks a rule, for example a raw `any`, is `found with gaps`.
2. **If any block is missing or has gaps, write work orders. Do not write the framework yourself.**
   - Group the blocks into orders that can each ship alone. A typical split: UI base layer (Site, Page, Element, Flow), UI adapters (browser driver, test-runner binding), API base layer (adapter interface, base client, matchers), API schema and resilience, test-data helpers (Model and Factory conventions).
   - Write each order in `intake/<id>.md` from the intake template with `status: draft`. Give each real acceptance criteria taken from the pattern, a `test-type` and `blueprint`, a budget from the blueprint defaults, and `depends-on` where one order needs another (the base layer before anything that uses it). Run the intake validator on each.
   - Add one order for each `found with gaps` row.
   - **Stop here and ask the human** to review the orders and the framework language, and to set `status: ready` on the ones to build. Orders ship through the normal path (the runbook in phase 8, with the skills from phase 5 and the checks from phase 9).
   - Do not mark an order `ready` yourself.
3. Write the pattern documents. The seed files in `examples/` show the shape of an example; adapt them to the framework you found. The design text does not need the framework to exist. The code examples do.
   - If every block for a pattern was `found`: write and run the example as described below.
   - If a block is missing or has gaps: leave the example section as `Pending: written when order <id> ships`, naming the order. Do not invent an example against a framework that does not exist.

   3a. Write `docs/patterns/ui-e2e-site-centric.md` (the Saturday-like pattern). Sections:
   - **Mental model**: a user visits a Site, uses Pages made of Elements and Partials, and completes a business goal in a Flow.
   - **Why not the classic page object model**: pages grow into god objects; this pattern splits them.
   - **Site**: owns one browser context. **Page**: one route; delegates to elements. **Element**: one locator with wait and retry.
     **Partial**: a reusable fragment such as a header. **Flow**: chains pages into a business scenario; step definitions call flows.
     **Filter**: gates or selects by state, for example a base filter class plus a decorator that blocks an Element action until a page condition holds. **Site manager**: hands out sites for multi-site flows.
   - **Gherkin**: the scenario name is the acceptance criterion. Keep the interesting user action visible in the scenario;
     hide setup noise in flows and factories ("moist, not fully dry").
   - **Layers and imports**: which layer may import which.
   - **Example**: one login page, one login flow, one scenario, in the project's language. Check the real framework before you write it: a base class may read abstract members in its own constructor, so use getters there, not fields. List any class the pattern names that the framework package does not export.
   3b. Write `docs/patterns/api-declarative-client.md` (the Sunday-like pattern). Sections:
   - **Mental model**: tests describe what to call and what to expect. The client owns how.
   - **Base client**: one per service, built on an `HttpAdapter` interface. Tests use a fake adapter for unit speed and a real one for contract tests.
   - **Fluent matchers**: `toHaveStatus`, `toBeSuccessful`, `toRespondWithin`. Add your own in one file.
   - **Schema validation**: validate every response body against a schema. Use `unknown` plus runtime validation. Never `any`.
   - **Resilience**: circuit breaker and exponential backoff from a library. Never a hand-written loop with `sleep`.
   - **Models and factories**: separate files. Factories use a seeded fake-data library.
   - **Coverage matrix**: for each endpoint, list happy path, validation error, auth failure, not found, and timeout. Mark gaps.
   - **Example**: one create-order client method, one test, one schema, in the project's language.
   3c. Write `docs/patterns/testing-pyramid.md`: the five levels, who writes them, the speed budget for each, and which blueprint applies.
   3d. Write `docs/patterns/test-annotation.md`: every test carries an issue reference and an acceptance-criterion reference in
   the language's own mechanism (JSDoc `@issue` and `@ac`, pytest markers or docstring, JUnit `@Tag` and `@DisplayName`,
   xUnit `[Trait]`, Go comment, Gherkin tag). Gherkin scenarios need no AC annotation, because the scenario name is the criterion.
   3e. Each pattern document ends with a "What this does not establish" table. State plainly what following the pattern does not prove.

## Files
`docs/test-factory/FRAMEWORK-STATUS.md`, `intake/<id>.md` for each framework order, `docs/patterns/{ui-e2e-site-centric,api-declarative-client,testing-pyramid,test-annotation}.md`.

## Verification
- `FRAMEWORK-STATUS.md` has a row for every block, and every `missing` or `found with gaps` row names an order. Each order passes the intake validator and has `status: draft`.
- For each example that exists: it compiles or runs in the project's toolchain. Run it. Record the command and result in the pattern file.
- For each example that does not exist: the pattern file says `Pending` and names the order.
- Come back to this phase when a framework order ships: replace each `Pending` marker with a real, running example, and update `FRAMEWORK-STATUS.md`.
- Every class and term used in a pattern document also appears in `DOMAIN_DICTIONARY.md` or is defined in the document.
- Search the pack for conflicting names: `BasePage` is spelled one way everywhere.

## Budget
`max-usd: 5`, `max-attempts: 2`, `max-hours: 2`. The framework orders carry their own budgets.

## Time-box
Stage timeout 30 minutes. Write one pattern document per stage so each stays inside the timeout.
