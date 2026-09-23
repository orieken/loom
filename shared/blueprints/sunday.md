# Blueprint: Sunday — API Test Framework

**Registry id**: `sunday`
**Primary language**: TypeScript (only supported language today)
**Testing levels covered**: API Contract, Integration, Unit
**Status**: Stable — this is the reference API test pattern for the ecosystem.

## When To Use

- You are building a project whose primary output is **automated API test coverage** for one or more HTTP/REST/GraphQL/gRPC services.
- You need declarative, fluent testing (`await api.get('/users').expect.toHaveStatus(200)`) rather than imperative HTTP calls.
- You need strict schema validation on every response (Zod).
- You need resilience primitives (CircuitBreaker, ExponentialBackoff) instead of custom retry loops.
- You need contract coverage (Pact-style consumer-driven contracts optional but supported).

Do NOT use when:
- You need UI test coverage → use `saturday`.
- You need a running production API — Sunday tests APIs, it does not build them. For building the API, use `clean-arch-service`.

## Layer Structure

| Layer | Responsibility | Example files |
|---|---|---|
| Domain | Fluent matcher definitions, resilience strategy interfaces, schema types | `matchers.ts`, `resilience-strategy.interface.ts`, `api-response.model.ts` |
| Use-Cases | Domain-specific API client classes extending `BaseApiClient` | `users-api.client.ts`, `orders-api.client.ts` |
| Adapters | HTTP transport (`IHttpAdapter` implementations), OTel span emission, resilience implementations | `fetch-http.adapter.ts`, `otel-http.adapter.ts`, `circuit-breaker.ts`, `exponential-backoff.ts` |
| Fixtures | The `api` fixture that wires everything together for a test | `api.fixture.ts` |
| Test Files | Vitest specs consuming the fixture | `users.spec.ts`, `orders.spec.ts` |

## Directory Tree

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── matchers.ts                     # toHaveStatus, toBeSuccessful, toRespondWithin
│   │   ├── resilience-strategy.interface.ts
│   │   └── api-response.model.ts
│   ├── clients/
│   │   ├── base-api.client.ts
│   │   ├── users-api.client.ts
│   │   └── orders-api.client.ts
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── i-http.adapter.ts
│   │   │   ├── fetch-http.adapter.ts
│   │   │   └── playwright-http.adapter.ts  # for e2e API tests
│   │   ├── otel/
│   │   │   └── otel-http.adapter.ts
│   │   └── resilience/
│   │       ├── circuit-breaker.ts
│   │       └── exponential-backoff.ts
│   ├── schemas/                             # Zod schemas per endpoint
│   │   ├── user.schema.ts
│   │   └── order.schema.ts
│   └── fixtures/
│       └── api.fixture.ts
├── tests/
│   ├── unit/                                # Vitest, no network
│   │   └── matchers.spec.ts
│   ├── integration/                         # Vitest, hits test doubles or in-process API
│   │   └── users.integration.spec.ts
│   └── e2e/                                 # Playwright, hits deployed API
│       └── users.e2e.spec.ts
├── contracts/                               # optional Pact-style contracts
│   └── users.pact.json
├── vitest.config.ts
├── playwright.config.ts
├── package.json
├── tsconfig.json
└── .env.example
```

## Key Abstractions (non-negotiable — do not bypass)

- **`BaseApiClient`** — every domain-specific client extends this. Handles auth injection, base URL, default headers, resilience strategy attachment.
- **`IHttpAdapter`** — interface that abstracts the transport. Implementations: `FetchHttpAdapter` (unit/integration), `PlaywrightHttpAdapter` (E2E with browser cookies).
- **`api` fixture** — the Vitest/Playwright test fixture. Test bodies read `({ api }) => { ... }` and call `api.users.getById(1).expect.toHaveStatus(200)`.
- **Fluent matchers** — `toHaveStatus`, `toBeSuccessful`, `toRespondWithin`, `toMatchSchema`. Extend Vitest's `expect`.
- **`validateSchema(schema, response)`** — Zod validation. Every response is validated; a schema mismatch fails the test.
- **`CircuitBreaker`, `ExponentialBackoffStrategy`** — the ONLY approved retry mechanisms per `architecture-guardrails.md` #5. Custom `for`/`while` + `sleep` loops are banned.

Reject any test that:
- Calls `fetch` or `axios` directly instead of going through `api`.
- Uses `expect(response.status).toBe(200)` instead of the fluent matcher.
- Writes its own retry loop.
- Skips Zod validation.

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Vitest | Matchers, resilience strategies, Zod schema logic, `BaseApiClient` composition |
| Integration | `qa-engineer` | Vitest | Client + adapter wiring, against test doubles or a Docker-composed dependency |
| API Contract | `api-test-generator` (from OpenAPI) or `qa-engineer` | Playwright + Vitest via `api` fixture | Consumer-driven contract verification, endpoint schema conformance |

No acceptance or E2E-UI level — Sunday is API-only. Pair with `saturday` for full-stack coverage.

## Integration Map (typical)

- **Target API(s)** — the services under test. Requires `API_BASE_URL_*` env vars.
- **Auth provider** — token minting for authenticated calls. Abstract behind an `AuthProvider` interface.
- **OTel collector** — every API call emits a span with method, URL, status, duration. Requires `OTEL_EXPORTER_OTLP_ENDPOINT`.
- **Contract broker** (optional) — if using Pact, requires `PACT_BROKER_URL` and `PACT_BROKER_TOKEN`.

## OTel Instrumentation Plan

- **Test span** — one per Vitest test / Playwright test. Tags: `test.name`, `test.file`.
- **HTTP span** — one per `api.*` call. Child of test span. Tags: `http.method`, `http.url`, `http.status_code`, `http.duration_ms`, `resilience.retries`, `resilience.circuit_state`.
- **Schema validation span** — one per `validateSchema` call. Tags: `schema.name`, `schema.valid`.
- **All OTel emission in the adapter layer.** Domain classes (matchers, schemas) never emit spans.

## Scaffold Recipe (plan-and-scaffold mode)

- `package.json` — pinned: `vitest`, `@playwright/test`, `zod`, `@opentelemetry/api`, `@opentelemetry/sdk-node`, `@pact-foundation/pact` (optional).
- `tsconfig.json` — `strict: true`, `noImplicitAny: true`.
- `.eslintrc.json` — complexity max 6, no-explicit-any error, custom rule banning `fetch`/`axios` outside `src/adapters/`.
- `vitest.config.ts` — coverage threshold 85%, projects for `unit` and `integration`.
- `playwright.config.ts` — projects for `contract` and `e2e`, OTel reporter wired.
- `src/clients/base-api.client.ts` — stub with the mandatory injection points.
- `src/adapters/http/fetch-http.adapter.ts` — concrete implementation.
- `src/domain/matchers.ts` — the four fluent matchers.
- `src/fixtures/api.fixture.ts` — the fixture factory.
- `tests/unit/matchers.spec.ts` — one failing test (Red).
- `.env.example` — placeholders for `API_BASE_URL`, `AUTH_TOKEN`, `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — `node_modules/`, `coverage/`, `test-results/`, `playwright-report/`, `.env`.

## ADR-000 Seed Context

> We are building an API test project using the Sunday framework. Rationale: fluent matchers (`toHaveStatus`, `toBeSuccessful`, `toRespondWithin`) + Zod schema validation + resilience primitives (`CircuitBreaker`, `ExponentialBackoff`) give us declarative, self-documenting tests that catch schema drift and flaky network behavior without imperative retry loops. `BaseApiClient` + `IHttpAdapter` enforces Clean Architecture separation between test logic and transport. Alternatives considered: raw supertest (rejected — no schema validation, no OTel); Postman/Newman (rejected — no TypeScript, poor version control story); Karate (rejected — Java-based, doesn't integrate with the rest of the TypeScript ecosystem). Consequences: all API test code must go through the `api` fixture; direct `fetch`/`axios` is banned outside the adapter layer; every response is Zod-validated; retries are `CircuitBreaker` or `ExponentialBackoff`, never hand-rolled.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each seed scenario into API-level acceptance criteria.
2. `architect` — validate `BaseApiClient` layering for the target API(s).
3. `api-test-generator` — if an OpenAPI spec exists, generate the initial client + schema files.
4. `qa-engineer` — write the integration and contract specs.
5. `developer` — implement any custom matchers or resilience extensions, with their unit tests.
6. `code-reviewer` — enforce the Sunday pattern; reject direct `fetch` calls.
7. `security-reviewer` — verify auth token handling, secret redaction.
8. `sre-engineer` — validate OTel span cardinality.
9. `devops-engineer` — wire into CI, configure Pact broker if used.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
