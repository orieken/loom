# Blueprint: Clean Architecture Library

**Registry id**: `clean-arch-library`
**Primary language**: TypeScript
**Supported languages**: TypeScript, Go, Python, Java, C#
**Testing levels covered**: Unit only
**Status**: Stable — the generic library blueprint. Use when you're building code to be *consumed* by other projects, not run standalone.

## When To Use

- You are building a **publishable library** (npm package, Go module, PyPI package, Maven artifact, NuGet package).
- The library has **domain and use-case layers only** — no adapters, because the consumer provides those.
- The library will be depended on by multiple downstream projects and needs a stable public API.

Do NOT use when:
- The code is only used by one project → put it in that project's `internal/` and skip publishing.
- The code has HTTP handlers, DB access, or other adapter concerns → use `clean-arch-service`.
- The code is a CLI → use `clean-arch-cli`.

## Layer Structure

| Layer | Responsibility | Cannot import from |
|---|---|---|
| Domain | Entities, value objects, factories, domain services | Anything except stdlib |
| Use-Cases | High-level orchestration exposed to consumers | Adapters (there are none in this pattern) |
| Public API | Curated re-exports; the only thing consumers see | Nothing internal without going through the API surface |

The absence of an adapter layer is the whole point. Consumers inject their own implementations of any interfaces the use-case layer defines.

## Directory Tree (TypeScript — reference)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── user.model.ts
│   │   ├── user.factory.ts
│   │   └── errors.ts
│   ├── usecases/
│   │   ├── calculate-score.usecase.ts
│   │   ├── calculate-score.spec.ts
│   │   └── score-provider.interface.ts    # consumer must implement
│   └── index.ts                            # curated public API
├── tests/
│   └── integration/
│       └── public-api.spec.ts              # verifies public API stability
├── package.json                            # "main", "types", "exports" fields
├── tsconfig.json                           # composite: true, declaration: true
├── tsconfig.build.json                     # what actually gets shipped
├── .eslintrc.json                          # complexity max 6, no-explicit-any
├── vitest.config.ts                        # 85% coverage
├── .gitignore
├── .npmignore
├── README.md                               # usage examples
├── CHANGELOG.md                            # semver-tracked changes
└── LICENSE
```

## Directory Tree (Go)

```
<project-root>/
├── domain/
│   ├── user.go
│   └── errors.go
├── usecases/
│   ├── calculate_score.go
│   ├── calculate_score_test.go
│   └── score_provider.go                   # interface
├── doc.go                                  # package-level docs
├── go.mod
├── go.sum
├── .golangci.yml
├── .gitignore
├── README.md
├── CHANGELOG.md
└── LICENSE
```

Note: Go libraries put nothing under `internal/` unless it truly shouldn't be importable — public packages live at the module root.

## Key Abstractions (non-negotiable — do not bypass)

- **The public API surface is curated in one file** (`index.ts` for TS, package exports for Go, `__init__.py` for Python). Nothing else is publicly consumable.
- **Every dependency the library needs from the outside world is an interface**, defined in the use-case layer. The consumer implements it. This is DI without a DI framework.
- **No side effects at import time.** No `console.log`, no config reads, no network calls. Importing the library is free.
- **Semver strictly enforced.** Breaking public API changes are major version bumps. This is why the public API surface is one file — reviewers can see every breaking change in one diff.
- **No dependencies on framework-shaped things.** No React, no Express, no Cobra. If the library needs to work "inside React," it exposes hooks that call pure functions — React is the consumer's problem.

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Language default | Domain entities, use-cases with stubbed interfaces, factory behavior |

Libraries have no other test levels. If you need integration or E2E, you're not writing a library — you're writing an application that happens to be reusable. Split it.

The consumer is responsible for integration testing the library within their own project.

## Integration Map

None. That's the point.

The library defines interfaces (e.g., `ScoreProvider`). The consumer implements them (e.g., `PostgresScoreProvider`, `RedisScoreProvider`, `InMemoryScoreProvider` for tests).

## OTel Instrumentation Plan

Libraries do **not** emit OTel spans directly. Instead:

- Define an optional `Tracer` interface in the domain.
- Consumers who want observability inject an OTel-backed implementation.
- Consumers who don't inject a no-op implementation (or the library's default no-op).

This keeps the library free of OTel dependencies (which are heavy).

## Scaffold Recipe (plan-and-scaffold mode)

**TypeScript:**
- `package.json` — proper `main`/`module`/`types`/`exports` fields, `sideEffects: false`, `files: ["dist"]`. Peer-deps for anything the consumer needs to bring (e.g., `zod` if used in schemas).
- `tsconfig.json` — `declaration: true`, `declarationMap: true`, `strict: true`.
- `tsconfig.build.json` — what actually gets bundled.
- `.eslintrc.json` — complexity max 6, no-explicit-any, plus a rule banning imports of `fs`, `http`, `child_process` from `src/` (libraries don't do I/O).
- `vitest.config.ts` — 85% coverage threshold.
- `src/index.ts` — one exported function, one exported type.
- `src/usecases/calculate-score.usecase.ts` — the exported function.
- `src/usecases/calculate-score.spec.ts` — one failing test (Red).
- `README.md` — installation, one usage example, API reference link.
- `CHANGELOG.md` — Keep-a-Changelog format, `## [Unreleased]` section.

**Go:**
- `go.mod` with the module path matching the intended import path.
- `.golangci.yml` — gocyclo max 6, godot (enforce doc comments end in a period), godox (flag TODOs in exported code).
- `doc.go` — package-level documentation.
- `usecases/calculate_score.go` — one exported function.
- `usecases/calculate_score_test.go` — one failing table-driven test.

**Both:**
- `LICENSE` — CC BY 4.0 or MIT depending on intent.
- `.gitignore` — appropriate for the language.
- `.npmignore` / no equivalent for Go (Go modules ship the whole repo).

## ADR-000 Seed Context

> We are building a library, not an application. Rationale: <problem> is common across N of our projects; extracting it prevents copy-paste drift. Clean Architecture applies but at a smaller scale: domain + use-cases only, adapters are the consumer's responsibility (dependency injection without a framework). The public API surface is deliberately curated in one file so semver breaking changes are visible in one diff. Language: <language>, chosen because the primary consumers are <TS/Go/Python/etc.> projects. Alternatives considered: put the code in one project and copy-paste to others (rejected — drift within a quarter); build it as a service (rejected — network hop for what is CPU-bound domain logic). Consequences: no adapters in this codebase; every external dep is an interface; no side effects at import time; strict semver; the library never depends on a framework.

## Downstream Agents (typical invocation plan)

1. `analyst` — identify the public API surface and stability guarantees.
2. `architect` — interface design, versioning strategy, backwards-compat approach.
3. `developer` — implement use-cases test-first.
4. `code-reviewer` — enforce zero side effects at import, no I/O in domain/use-cases, public API discipline.
5. `qa-engineer` — verify the public API contract via a `public-api.spec` that imports only from the package root.
6. `tech-writer` — README, API reference generation (TypeDoc / godoc / Sphinx).
7. `release-manager` — semver bump decision, changelog generation, publish to registry.
8. `devops-engineer` — CI/CD for automated publishing on tag push.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
