# Blueprint: Saturday — E2E Test Framework

**Registry id**: `saturday`
**Primary language**: TypeScript (reference implementation)
**Supported languages**: TypeScript, Python, C#, Java, Go (port maturity varies — see below)
**Testing levels covered**: E2E/UI, Acceptance, Unit
**Status**: Stable — this is the reference E2E pattern for the ecosystem.

## When To Use

- You are building a project whose primary output is **automated UI/E2E test coverage** for one or more web applications.
- You need cross-application user journeys (e.g., "log into App A, hand off to App B, verify state in App C").
- You need first-class OpenTelemetry (OTel) traces for every BDD scenario and Playwright action.
- Traditional Page Object Model (POM) has failed you — pages are brittle, tests break constantly, and Page Objects have grown into God Objects.

Do NOT use when:
- You need API-level testing only → use `sunday` blueprint.
- You need a single-app test suite with no cross-context flows → Saturday still works but is overkill; a plain Playwright + `describe`/`test` setup is lighter.

## Port Maturity

| Language   | Repo | Test framework | BDD runner | Notes |
|------------|------|----------------|------------|-------|
| TypeScript | reference | Vitest | Cucumber.js | Reference implementation. All patterns originate here. |
| C#         | `saturday-monorepo-csharp` | xUnit | Reqnroll | Most mature port. Has dedicated `Saturday.Reporting` package. |
| Python     | `saturday-monorepo-python` | pytest | pytest-bdd | Not yet published. Async-first. Reporting package still missing. |
| Java       | not yet built | JUnit 5 | Cucumber-JVM | Provisional — no reference repo yet. |
| Go         | not yet built | `testing` | godog | Provisional — no reference repo yet. |

## Layer Structure

| Layer | Responsibility | Example files |
|---|---|---|
| Domain | Site/Page/Element/Flow abstractions, filter definitions | `base-site.ts`, `base-page.ts`, `base-element.ts`, `base-flow.ts`, `filter.ts` |
| Use-Cases | Concrete site, page, flow, and partial implementations for the target apps | `admin-site.ts`, `login-page.ts`, `checkout-flow.ts`, `header-partial.ts` |
| Adapters | Playwright browser context management, OTel span emission, screenshot/video capture | `playwright-adapter.ts`, `otel-adapter.ts`, `site-manager.ts`, `tab-manager.ts` |
| Test Runner | Cucumber.js step definitions binding Gherkin to flows/pages | `steps/authentication.steps.ts`, `steps/checkout.steps.ts` |
| Reporting | Cucumber JSON output feeding the Friday dashboard | `friday-reporter.ts`, `cucumber.config.ts` |

## Directory Tree (TypeScript reference)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── base-site.ts
│   │   ├── base-page.ts
│   │   ├── base-element.ts
│   │   ├── base-flow.ts
│   │   └── filter.ts
│   ├── sites/                     # one per target application
│   │   ├── admin/
│   │   │   ├── admin.site.ts
│   │   │   ├── pages/
│   │   │   ├── flows/
│   │   │   └── partials/
│   │   └── storefront/
│   │       └── ...
│   ├── adapters/
│   │   ├── playwright.adapter.ts
│   │   ├── otel.adapter.ts
│   │   ├── site-manager.ts
│   │   └── tab-manager.ts
│   └── reporting/
│       └── friday.reporter.ts
├── features/                      # Gherkin .feature files
│   ├── authentication.feature
│   └── checkout.feature
├── steps/                         # Cucumber.js step definitions
│   ├── authentication.steps.ts
│   └── checkout.steps.ts
├── cucumber.config.ts
├── playwright.config.ts
├── vitest.config.ts               # for unit tests on flows/filters
├── package.json
├── tsconfig.json                  # strict: true, no `any`
└── .env.example
```

## Key Abstractions (non-negotiable — do not bypass)

- **`BaseSite`** — represents one application. Owns its own Playwright `BrowserContext`. Registered with `SiteManager` for cross-app flows.
- **`BasePage`** — one URL/route within a site. Never talks to Playwright directly; delegates to `BaseElement`.
- **`BaseElement`** — a single interactive locator with retry, wait, and OTel span emission built in.
- **`BasePartial`** — reusable UI fragments (headers, footers, modals) that appear on multiple pages. See `docs/patterns/saturday-partials.md`.
- **`BaseFlow`** — orchestrates multiple pages into a business scenario. Called from step definitions.
- **`Filter`** — decorator applied to page/flow methods for cross-cutting concerns (retry, screenshot on failure, OTel tags).
- **`SiteManager`** — creates and hands out `BaseSite` instances for cross-app flows.
- **`TabManager`** — manages multi-tab flows within a single site.

Reject any PR that reintroduces the traditional POM (a "PageObject" class holding a `page` reference and mixing locators + assertions + flows).

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Vitest | Filters, custom matchers, `BaseFlow` composition logic, `SiteManager` routing |
| Acceptance | `qa-engineer` inside `/deliver-atdd` | Gherkin | The `.feature` files — scenario IS the acceptance criterion |
| E2E / UI | `qa-engineer` following Saturday conventions | Cucumber.js + Playwright | Full user journeys, cross-app flows |

No API contract or integration level — Saturday is UI-only. If you need API testing, add `sunday` alongside.

## Integration Map (typical)

- **Target applications** — one or more running web apps. Each gets a `BaseSite` subclass. Requires `TARGET_APP_URL_*` env vars.
- **OTel collector** — Saturday emits spans for every scenario, every filter, every element interaction. Requires `OTEL_EXPORTER_OTLP_ENDPOINT`.
- **Friday dashboard** — Cucumber JSON summaries POSTed here (per `shared/rules/approval-gates.md` gate #1 — explicit approval required to ship). Requires `FRIDAY_INGEST_URL` and `FRIDAY_INGEST_TOKEN`.
- **Auth service** — most projects need a way to obtain a session cookie or JWT for logged-in flows. Abstract behind an `AuthProvider` interface, per `architecture-guardrails.md` #1.

## OTel Instrumentation Plan

- **Scenario span** — one per Gherkin scenario. Tags: `scenario.name`, `feature.file`, `tags`.
- **Step span** — one per `Given`/`When`/`Then` step. Child of scenario span.
- **Element span** — one per `BaseElement` interaction (click, fill, waitFor). Child of step span. Tags: `element.selector`, `element.action`, `element.result`.
- **Site handoff span** — one per `SiteManager.handoff()` call. Records source and destination sites.
- **Never instrument inside domain classes.** OTel emission lives in the adapter layer per `architecture-guardrails.md` #8. Domain classes call a `Tracer` interface; the adapter provides the concrete OTel-backed implementation.

## Scaffold Recipe (plan-and-scaffold mode)

Files to create in addition to the planning artifacts:

- `package.json` — pinned versions of `@orieken/saturday-core`, `@orieken/saturday-cucumber`, `@playwright/test`, `vitest`, `@cucumber/cucumber`, `zod`.
- `tsconfig.json` — `strict: true`, `noImplicitAny: true`, `target: ES2022`, `moduleResolution: bundler`.
- `.eslintrc.json` — complexity max 6, plus `@typescript-eslint/no-explicit-any: error`.
- `playwright.config.ts` — projects for chromium/firefox/webkit, OTel reporter wired.
- `cucumber.config.ts` — step definition glob, JSON formatter → `.reports/cucumber.json`.
- `vitest.config.ts` — coverage threshold 85%.
- `src/domain/base-site.ts` — stub with the abstract methods per the reference.
- `features/hello-world.feature` — one scenario using placeholder steps.
- `steps/hello-world.steps.ts` — one step definition that intentionally fails (Red).
- `.env.example` — placeholders for `TARGET_APP_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `FRIDAY_INGEST_URL`, `FRIDAY_INGEST_TOKEN`.
- `.gitignore` — `node_modules/`, `.reports/`, `playwright-report/`, `test-results/`, `.env`.

## ADR-000 Seed Context

When `/adr` is invoked for ADR-000, pass this context:

> We are building an E2E test project using the Saturday framework. Rationale: the Site-Centric pattern (`BaseSite` / `BasePage` / `BaseElement` / `BaseFlow`) prevents the God-Object drift that traditional POM produces at scale, particularly for cross-application user journeys. Cucumber.js + Playwright driven by Site-Centric abstractions gives us first-class OTel instrumentation and Friday dashboard integration out of the box. Alternatives considered: raw Playwright with `describe`/`test` (rejected — no OTel, no cross-app orchestration, no Friday integration); Cypress (rejected — no cross-origin support, no OTel-native integration); WebDriverIO (rejected — Playwright's auto-waiting is more reliable and its trace viewer is superior). Consequences: all UI test code must follow the Site-Centric pattern; the traditional POM is explicitly banned; every scenario emits OTel spans; Friday ingestion is gated by `approval-gates.md` #1.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each seed scenario into an acceptance-criteria breakdown.
2. `architect` — validate the layer boundaries for the target app(s) being tested.
3. `qa-engineer` — write the `.feature` files and step definitions.
4. `developer` — implement any custom filters, flows, or `BaseElement` extensions, with their unit tests.
5. `code-reviewer` — enforce the Site-Centric pattern; reject POM regressions.
6. `sre-engineer` — validate OTel span naming and cardinality.
7. `devops-engineer` — wire Playwright projects into CI, configure Friday ingestion.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
