# The UI test factory — `deliver-tests-saturday`

Takes a signed spec package and automates its `@level-ui` scenarios on the Saturday framework
(Site → Flow → Page → Element, Playwright underneath, Cucumber or Playwright Test as the runner). It
never edits a scenario and never edits product code. Read `00-factory-blueprint.md` and
`01-shared-intake.md` first; its input is what `02-spec-factory.md` publishes.

**Scope: Phase 0 (fix the exemplar), then DESIGN + plan + contracts.**

## Target repo

The framework side: `/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`.
The proving ground: `/Users/oscarrieken/Projects/Rieken/saturday-monorepo`,
`apps/ye-olde-magic-shop` (Vue app + `apps/mock-api`).

## Prior context (read first)

In `saturday-monorepo`:

- `packages/saturday-core/src/base/` — `base-site.ts`, `base-page.ts`, `base-flow.ts`,
  `base-element.ts`, `base-filter.ts`; `src/managers/` (site and tab managers)
- `packages/saturday-cucumber/` (World, hooks), `packages/saturday-playwright/`
- `packages/saturday-cucumber-indexer/` — indexes existing steps; reuse before writing new ones
- `saturday-mcp/internal/tools/` — deterministic generators: `generate_site`, `generate_page`,
  `generate_element`, `generate_flow`, `generate_steps`, plus `check_ubiquitous_language`
- `apps/ye-olde-magic-shop/lib/site/`, `features/`, `features/step_definitions/`, `features/support/`
- `docs/SITE-TAB-MANAGEMENT.md`, `CLAUDE.md`, `ARCHITECTURE_RULES.md`

In the framework: `shared/agents/qa-engineer.md`, `visual-qa-engineer.md`,
`docs/adrs/ADR-007-ui-test-location-and-artifact-contract.md`, `shared/rules/testing-conventions.md`.

## Phase 0 — the exemplar contradicts the framework. Fix that first.

`context-engineer` pins existing files as examples. Today the only full example teaches the factory
the wrong thing:

- The demo app depends on `file:./saturday-core` and `file:./saturday-cucumber` — vendored copies
  that have drifted from `packages/` (`managers/` is missing; `index.ts` and `requires-filter.ts`
  differ). Tests written against it target a framework that no longer exists.
- `lib/site/CartPage.ts`, `MagicShop.ts` are PascalCase; the naming rule is `cart.page.ts`.
- `MagicShop.initializeFlows()` registers nothing (`// No flows`), so the Flow layer is never shown.
- Step definitions bypass the model: 25 uses of `this.page` / `getByTestId` across five step files.
- `CartPage` guesses a selector, `button:has-text("COMPLETE TRANSACTION")`, with a comment saying
  the test id is unknown.

Phase 0 deliverable: the demo app on workspace packages, renamed files, at least two real flows
(login, checkout), steps that only talk to the Site, a `data-testid` on every element the steps
touch, and a `GOLDEN.md` naming these files as *the* exemplar. Exit: the suite passes, a lint rule
(below) passes, and `context-engineer` pins `GOLDEN.md`'s files for a Saturday order.

## Plan

```
context-engineer        (site model, existing pages/flows, step index, runner config)
  → acceptance-guard    (internal: scenarios byte-identical to the pinned spec package)
  → ui-survey           (drive the running app; map each step to an element; list missing test ids)
  → [halt if a required test id is missing]                    gate: confirm-testability — app owner
  → site-model          (which Site/Pages/Elements/Flows/Filters to add or extend; which steps to reuse)
                                                                 gate: confirm-model       — test architect
  → generate            (internal: run saturday-mcp generators from the site-model state)
  → automation-developer
  ⟲ automation-reviewer (bounded loop, 3 rounds)
  → stability           (internal: each new scenario runs N times, zero failures)
  → can-it-fail         (internal: fault injection, below)
  → pre-ship-checks     (lint, typecheck, unit tests, full suite in a container with the app)
                                                                 gate: confirm-ship        — QA lead
```

Notes on the stages that are new:

- **`acceptance-guard`** makes "never edit a scenario" a check, not an instruction. It runs again
  after `automation-developer`; any diff to a `.feature` file from the package fails the stage.
- **`ui-survey`** uses the Playwright MCP against the running app. Its typed state lists, per step,
  the element and its locator, and `missingTestIds[]`. A missing id is a request to the app's owner,
  The factory may not add it: product code is outside `scope.paths` (gate #6). Instead the run
  emits a **testability order**: a `status: draft` intake file for the target's
  `testability.plan` (normally `deliver-feature`), asking for exactly the missing seams — a
  `data-testid`, a `window.__SATURDAY_*__` hook, a fixture. The test order parks with reason
  `waiting-on-testability`, names the new order, and is re-claimed when that order ships. The
  testability order is reviewed by the target's app owner. The same mechanism covers a missing fixture the tests need (seed data, a
  feature flag, a mock endpoint).
- **`site-model`** is the design gate. A test architect sees the class tree before any code: new
  vs. extended pages, flows and their parameters, and reused steps with their source file.
- **`generate`** makes structure deterministic. The model decides *what*; the generators write the
  skeletons; `automation-developer` fills method bodies only. If the generators cannot express a
  needed shape, fix the generator — do not hand-write around it.

## `automation-reviewer` rules — deterministic first

Encode these as a lint (ESLint rule or a small Go/TS checker) so they are not judgement calls:

- step definitions call only `this.site` (Site → Flow → Page); no `this.page`, no `getByTestId`,
  `locator(` or `$(` in `step_definitions/`;
- locators live only in `initializeElements()`; `data-testid` or role-based, never `has-text` or CSS
  structure;
- no `waitForTimeout`, no `setTimeout`, no retries in test code;
- every Page is registered on the Site, every Flow is registered and used by at least one step;
- file names `name.type.ts` (`cart.page.ts`, `checkout.flow.ts`, `cart.steps.ts`);
- one assertion concept per `Then`; every `Then` asserts something the scenario's criterion names.

Then the model review: does each step do what its Gherkin says, at the level it says.

## The judge: can each test fail?

A UI test that passes against a broken app is worse than none. For each new scenario, `can-it-fail`
injects faults the scenario's `Then` should catch, using Playwright route interception against the
mock API (wrong total, empty order list, 500 on submit) or a feature flag in the demo app, and
asserts the scenario goes red. A scenario no injected fault can fail is reported as unverified, and
blocks ship unless the QA lead accepts it. This is ADR-008 for end-to-end tests. Design how faults
are derived from criteria — by hand in a fixture per feature for the first phase is acceptable.

The end-to-end harness for this factory: a fixture copy of the demo app, a spec package, and a
hidden set of **faulty builds**. The factory's tests must pass on the good build and fail on each
faulty one.

## Environment — decide before building

The suite needs the Vue app and `mock-api` running. Pre-ship checks and `can-it-fail` therefore run
in a container image built from the monorepo (`containers/`, `apps/ye-olde-magic-shop/Dockerfile`).
The ADR must say how the floor starts and stops it, how the base URL reaches the run, and what
happens when the app does not come up (transient, retry; never a test failure).

## Exit criteria for the first phase

1. Phase 0 is merged.
2. A real spec package's UI scenarios are automated to a PR, scenarios unchanged, lint green.
3. Every new scenario passed `stability` and failed at least one injected fault.
4. The harness fails a run that edits a scenario, and a run whose tests pass on a faulty build.
