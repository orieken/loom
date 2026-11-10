# The API test factory — `deliver-tests-sunday`

Takes a signed spec package and automates its `@level-api` scenarios on the Sunday framework
(typed API clients over a swappable HTTP adapter, data factories, custom matchers). It never edits a
scenario and never edits the service under test. Read `00-factory-blueprint.md` and
`01-shared-intake.md` first; its input is what `02-spec-factory.md` publishes. It shares its plan
shape with `03-test-factory-saturday.md` on purpose: same guard, same reviewer loop, same
"can it fail" judge. Only the model of the system differs.

**Scope: Phase 0 (decide what "Sunday" is), then DESIGN + plan + contracts.**

## Target repo

The framework side: `/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`.
The proving ground: `/Users/oscarrieken/Projects/Rieken/sunday-monorepo/api-testing-framework`
(`examples/test-service` as the service under test, `examples/demo-tests` as the suite).

## Prior context (read first)

In `sunday-monorepo/api-testing-framework`:

- `packages/core/src/base/BaseApiClient.ts`, `src/auth/`, `src/retry/`, `src/errors.ts`
- `packages/adapters-playwright/`, `packages/adapters-axios/` — the HTTP port and its adapters
- `packages/test-runner-playwright/src/` — fixtures and matchers (`toHaveStatus`, `toRespondWithin`)
- `packages/mcp-server/src/tools/` — `generateApiClient`, `generateTests`, `scaffoldProject`
- `docs/DESIGN_PATTERNS.md`, `docs/ADR/001`–`005`, `docs/TESTING_BEST_PRACTICES.md`
- `../packages/clients/generated/` — generated clients

## Phase 0 — "Sunday" is three designs today. Pick one.

A factory enforces a pattern; there is no single pattern to enforce yet:

- **Base class.** The code and README use `BaseApiClient`. ADR-005 specifies `BaseService` (with
  interceptors, validation and observability hooks), and is still `Proposed`.
- **Generated clients do not compile against core.** `packages/clients/generated/list-pets.client.ts`
  imports `../../core/base-api.client`, which does not exist, calls `this.get(ctx, …)` with a `ctx`
  argument `BaseApiClient` does not take, and types its response as `z.any()`. The same client exists
  twice, as `list-pets.client.ts` and `listPets.client.ts`.
- **Test data.** `DESIGN_PATTERNS.md` documents fishery factories in `packages/factories`; that
  package is empty, and the demo tests build data inline.
- **No API description.** `examples/test-service` has no OpenAPI file, and `api-survey` below
  depends on one. Phase 0 adds it (generated from the service, or written and checked against it).
- **Runner — decided: Playwright Test.** `test-runner-playwright` provides the fixtures and
  matchers. `examples/demo-tests` runs on Vitest and asserts with plain
  `expect(response.status).toBe(200)`; Phase 0 moves it to Playwright Test and the matchers.

Phase 0 deliverable: an ADR that accepts or rejects ADR-005, one generator whose output compiles and
passes against `examples/test-service`, response schemas generated from OpenAPI (not `z.any()`),
`packages/factories` populated for the demo service's entities, the suite on Playwright Test, and a `GOLDEN.md`
naming the exemplar files. Exit: `generateApiClient` on the test service's OpenAPI produces a client
the golden tests use unchanged.

## Plan

```
context-engineer        (clients, factories, auth strategy, runner config, existing suites)
  → acceptance-guard    (internal: scenarios byte-identical to the pinned spec package)
  → api-survey          (internal: read the OpenAPI description; map each step to an operation)
  → [halt if a step has no operation, or the description is missing]   gate: confirm-testability — service owner
  → client-model        (clients and methods to add, schemas, factories and traits, auth per call)
                                                                          gate: confirm-model       — test architect
  → generate            (internal: generateApiClient + schema and factory generation)
  → automation-developer
  ⟲ automation-reviewer (bounded loop, 3 rounds)
  → contract-check      (internal: every response the tests touch validates against its schema)
  → stability           (internal: N runs, zero failures, run in random order)
  → can-it-fail         (internal: fault injection, below)
  → pre-ship-checks     (lint, typecheck, unit tests, suite in a container with the service)
                                                                          gate: confirm-ship        — QA lead
```

- **`api-survey`** is deterministic: an OpenAPI description is data. A step with no matching
  operation means the spec package asked for behaviour the API does not expose. That is a finding for
  the spec's owner, or a sign the scenario belonged at `@level-ui` — not something to paper over.
- **`contract-check`** validates every response the suite receives against the schema the client
  was generated from. A schema mismatch is reported separately from a scenario failure.

## `automation-reviewer` rules — deterministic first

- tests call only typed clients; no raw `httpAdapter.get('/path')` in a test file;
- every request body comes from a factory or a trait; no inline literal objects for entities;
- assertions use the framework matchers; no hand-rolled status checks;
- no response typed `any`; every client method returns a schema-validated type;
- each test creates and cleans up its own data; no test depends on another's order or output;
- credentials come from the auth strategy and environment; none in test files;
- file names `name.type.ts` (`posts.client.ts`, `post.factory.ts`, `posts.spec.ts`).

## The judge: can each test fail?

For API tests, fault injection is cheap and should be thorough. Run the suite against a mutated
`examples/test-service` — wrong status codes, a dropped field, an off-by-one in a filter, an
authorization check removed — and require each new scenario to fail at least one mutation its
criterion is about. Report scenarios no mutation fails as unverified; they block ship unless the QA
lead accepts them. The harness holds the mutations hidden from the run.

Decide in the ADR whether NFRs in the spec package (`respond within 500ms`) become `toRespondWithin`
assertions here, or belong to a separate performance factory (Saturday already has a k6 exporter).

## Exit criteria for the first phase

1. Phase 0 is merged.
2. A real spec package's API scenarios are automated to a PR, scenarios unchanged, lint green.
3. Every new scenario passed `stability`, `contract-check`, and failed at least one mutation.
4. The harness fails a run that edits a scenario, and a run whose tests pass on a mutated service.
