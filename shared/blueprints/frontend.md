# Blueprint: Frontend Application

**Registry id**: `frontend`
**Language**: TypeScript (mandatory across all framework choices)
**Recommended framework**: Vue 3 (Composition API)
**Supported frameworks**: Vue 3, React, Svelte, Angular, SolidJS
**Testing levels covered**: E2E/UI, Acceptance, Integration, Unit
**Status**: Stable — Vue 3 is what the rest of this ecosystem uses (Friday dashboard, Saturday reporting UI), so it's recommended for consistency. But the framework is genuinely the user's choice.

## When To Use

- You are building a **web frontend** — a single-page app, a static-generated site with dynamic islands, or a full-stack app's frontend portion.
- You need proper testing coverage at every level (unit component tests, integration tests, acceptance, E2E).
- You want strong opinions on tooling but freedom on framework.

Do NOT use when:
- You need a mobile app → this blueprint is web-only.
- You need a CLI → use `clean-arch-cli`.
- You need a backend service that happens to serve HTML → use `clean-arch-service` and pick your HTML rendering per that pattern.

## Framework Choice

The bootstrap interview MUST ask which framework. Present them in this order with these framings:

1. **Vue 3** (recommended) — matches the rest of this ecosystem. Composition API, `<script setup>`, first-class TypeScript. If undecided, pick this.
2. **React** — largest ecosystem, most hiring pool. Use function components + hooks; class components are banned.
3. **Svelte** (SvelteKit) — smallest bundles, compile-time reactivity. Good for content-heavy sites.
4. **Angular** — pick this only if the team has strong Angular experience or an enterprise standard requires it. Higher ceremony, larger initial bundle.
5. **SolidJS** — React-like ergonomics with fine-grained reactivity. Small ecosystem; pick for greenfield projects where the team wants React DX with better performance.

Once picked, the scaffold recipe below adapts.

## Layer Structure (framework-agnostic — this is the point)

| Layer | Responsibility | Cannot import from |
|---|---|---|
| Domain | Business rules, entities, value objects, pure computation | Anything framework-specific (no React, no Vue, no fetch) |
| Use-Cases / Composables / Hooks / Services | Reactive orchestration: fetches, mutations, derived state | UI components |
| UI Components | Presentation only — receive props, emit events | Direct DB/HTTP calls, business logic |
| Adapters | HTTP clients, storage, feature flags, analytics | UI components (they consume via composables/hooks) |
| Frameworks | Router, DI, SSR, build tooling | (outermost) |

**The "no business logic in components" rule is non-negotiable regardless of framework.** A component either renders UI or delegates. This is what makes multi-framework portability real — the domain and use-case layers survive a framework migration.

## Directory Tree (Vue 3 — recommended reference)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── user.model.ts
│   │   ├── user.factory.ts
│   │   └── errors.ts
│   ├── usecases/                       # Vue: composables live here
│   │   ├── use-user.composable.ts
│   │   ├── use-user.spec.ts
│   │   └── user-repository.interface.ts
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── user.client.ts
│   │   │   └── user.client.spec.ts
│   │   ├── storage/
│   │   │   └── local-storage.adapter.ts
│   │   └── analytics/
│   │       └── plausible.adapter.ts
│   ├── components/                     # UI only
│   │   ├── UserCard.vue
│   │   ├── UserCard.stories.ts         # Storybook
│   │   └── UserCard.spec.ts
│   ├── pages/                          # route-level components
│   │   ├── HomePage.vue
│   │   └── UsersPage.vue
│   ├── router/
│   │   └── router.ts
│   ├── App.vue
│   └── main.ts
├── tests/
│   ├── unit/                           # already colocated .spec.ts also count
│   ├── integration/                    # Vue Testing Library
│   └── e2e/                            # Saturday-pattern (Cucumber.js + Playwright)
├── public/
├── index.html
├── package.json
├── tsconfig.json                       # strict: true
├── vite.config.ts
├── vitest.config.ts                    # 85% coverage
├── playwright.config.ts
├── cucumber.config.ts                  # for E2E acceptance tests
├── tailwind.config.ts
├── .eslintrc.json                      # complexity max 6, no-explicit-any
├── .env.example
├── .gitignore
└── README.md
```

## Directory Tree Adjustments Per Framework

- **React**: replace `usecases/*.composable.ts` with `hooks/use-*.hook.ts`, `.vue` files become `.tsx`, add `postcss.config.cjs` if using Tailwind.
- **Svelte (SvelteKit)**: `usecases/` becomes `lib/composables/`, components are `.svelte`, use SvelteKit's `+page.svelte` / `+layout.svelte` conventions.
- **Angular**: swap composables for injectable services, use standalone components, angular.json instead of vite.config.
- **SolidJS**: hooks become `createResource`/`createSignal` factories, components are `.tsx`.

The layer structure is identical across all five. The file extensions and idioms differ.

## Key Abstractions (non-negotiable — do not bypass regardless of framework)

- **No business logic in components.** Components render and emit; that's it.
- **No direct HTTP calls from components.** Everything goes through a composable/hook, which goes through an adapter.
- **No `any` in TypeScript** (per `architecture-guardrails.md` #4).
- **Every external dep behind an interface.** The HTTP client, storage, analytics, feature flags — all injected, all mockable.
- **State management is a use-case layer concern, not a component concern.** Pinia (Vue), Zustand/Redux (React), Svelte stores, RxJS/Signals (Angular) — pick per framework, but state lives in the use-case layer.
- **Design tokens live in one place.** Tailwind config or a design-token file. Never magic hex codes in components.
- **Accessibility is enforced by the `accessibility-engineer` agent.** Semantic HTML, no `onClick` on `<div>`, ARIA where needed. See `check-accessibility` skill.

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Vitest | Composables/hooks, adapters, domain, pure component render logic |
| Integration | `qa-engineer` | Vitest + Testing Library (Vue Testing Library / React Testing Library / etc.) | Component + composable + mocked adapter — full render trees with user interactions |
| Acceptance | `qa-engineer` inside `/deliver-atdd` | Gherkin scenarios | Business-language scenarios that drive real browser interactions |
| E2E / UI | `qa-engineer` following Saturday conventions | Cucumber.js + Playwright + Site-Centric pattern | Full user journeys against a real backend |

**All four levels are expected.** A frontend project that ships without integration + acceptance + E2E has a coverage gap the bootstrap should flag.

## Integration Map (typical)

- **Backend API** — behind an HTTP client adapter. Requires `VITE_API_BASE_URL` (or framework equivalent).
- **Auth provider** — OAuth/OIDC/session. Behind an `AuthProvider` interface.
- **Analytics** — Plausible/PostHog/GA. Behind an adapter, off-by-default in dev.
- **Feature flags** — LaunchDarkly/GrowthBook/Unleash. Behind a `FeatureFlagProvider` interface.
- **CDN / static hosting** — Cloudflare Pages, Vercel, S3+CloudFront. Deployment target, not runtime.
- **OTel browser SDK** (optional) — for real user monitoring. Adapter-layer only.

## OTel Instrumentation Plan

- **Route change span** — one per navigation. Emitted by a router middleware.
- **User interaction span** — one per major interaction (form submit, primary CTA click). Emitted by composables/hooks, not components.
- **HTTP span** — one per outbound API call. Standard `http.*` tags.
- **Component render spans are usually not worth the noise.** Emit only for known-expensive components under a feature flag.
- **Domain never emits spans.**

## Scaffold Recipe (plan-and-scaffold mode)

**Common to all frameworks:**
- `tsconfig.json` — `strict: true`, `noImplicitAny: true`.
- `.eslintrc.json` — complexity max 6, `@typescript-eslint/no-explicit-any: error`, plus framework-specific rules.
- `vitest.config.ts` — 85% coverage threshold.
- `playwright.config.ts` — projects for chromium/firefox/webkit.
- `cucumber.config.ts` — Saturday-pattern E2E.
- `tailwind.config.ts` (if using Tailwind).
- `.env.example` — `VITE_API_BASE_URL` (or equivalent), auth config, feature flag SDK key, analytics site ID, `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — `node_modules/`, `dist/`, `coverage/`, `playwright-report/`, `test-results/`, `.env`.

**Vue 3-specific:**
- `package.json` — Vue 3, Vite, `@vue/test-utils`, `@testing-library/vue`, Pinia (state), Vue Router.
- `vite.config.ts` — Vue plugin, path aliases.
- `src/App.vue` — root component with `<script setup lang="ts">`.
- `src/components/HelloWorld.vue` — one component with props/emits.
- `src/components/HelloWorld.spec.ts` — one failing test (Red).
- `src/usecases/use-hello.composable.ts` — one composable.
- `src/usecases/use-hello.spec.ts` — one failing test.
- `features/hello.feature` — one Gherkin scenario for the E2E path.
- Storybook config — every component gets a `.stories.ts`.

**React-specific:**
- `package.json` — React 18+, Vite, `@testing-library/react`, Zustand or TanStack Query for state, React Router.
- `src/App.tsx`, `src/hooks/use-hello.hook.ts`, `src/components/HelloWorld.tsx` + `.spec.tsx`.

**Svelte-specific (SvelteKit):**
- `package.json` — SvelteKit, `@testing-library/svelte`.
- `src/routes/+page.svelte`, `src/lib/composables/hello.ts` + `.spec.ts`.

**Angular-specific:**
- Angular CLI-generated project structure.
- Karma → Jest migration or use Vitest via `@analogjs/vitest-angular`.

**SolidJS-specific:**
- `package.json` — Solid, Vite, `@solidjs/testing-library`.
- Similar structure to React.

## ADR-000 Seed Context

> We are building a frontend application. Rationale: <problem>. Framework choice: <chosen framework> because <reason>. Vue 3 was <chosen | considered and rejected because ...>. Layer separation (Domain / Use-Cases / Adapters / Components) means the framework is one adapter over a testable core — components can be replaced or the framework itself can be migrated without rewriting business logic. Testing coverage: unit (Vitest), integration (Testing Library), acceptance (Gherkin), E2E (Saturday-pattern Cucumber.js + Playwright). Alternatives considered: server-rendered HTML (rejected — <reason>); mobile-first (rejected — <reason>). Consequences: components contain no business logic; every external dep is behind an interface; all four testing levels are non-negotiable; accessibility is verified by the `accessibility-engineer` agent.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each seed scenario into UI acceptance criteria.
2. `architect` — component boundary decisions, state management shape, routing structure.
3. `developer` — implement composables/hooks test-first, then components.
4. `accessibility-engineer` — semantic HTML, ARIA, keyboard nav, screen reader compat.
5. `code-reviewer` — enforce "no business logic in components," design token discipline.
6. `security-reviewer` — XSS, CSRF, auth token storage, CSP.
7. `performance-engineer` — bundle size budgets, LCP/CLS/INP targets, image optimization.
8. `qa-engineer` — integration + acceptance + E2E tests.
9. `sre-engineer` — real user monitoring, error tracking, log cardinality.
10. `tech-writer` — README, Storybook docs, design system docs.
11. `devops-engineer` — CI/CD to CDN, preview deployments per PR.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
