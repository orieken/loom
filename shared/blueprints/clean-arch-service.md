# Blueprint: Clean Architecture Backend Service

**Registry id**: `clean-arch-service`
**Primary language**: Go
**Supported languages**: Go, TypeScript, Python, Java, C#
**Testing levels covered**: E2E, API Contract, Integration, Unit
**Status**: Stable — the generic backend blueprint. When a project doesn't fit a more specific blueprint (Saturday / Sunday / MCP / Scribe), this is the fallback.

## When To Use

- You are building a **long-running backend service** — HTTP/REST, gRPC, or queue consumer.
- You need strict Clean Architecture layer separation from day one.
- You need multi-language flexibility (this blueprint works for any of the five supported languages).

Do NOT use when:
- The service's primary purpose is testing → use `saturday` (UI) or `sunday` (API).
- You're building an MCP server → use `mcp-server`.
- You're building a CLI → use `clean-arch-cli` or `scribe-cli`.
- You're building a library with no adapters layer → use `clean-arch-library`.

## Layer Structure (language-agnostic)

| Layer | Responsibility | Cannot import from |
|---|---|---|
| Domain (Entities) | Pure business rules, entity invariants, value objects | Anything except stdlib |
| Use-Cases | Orchestrates domain operations, defines interfaces for adapters | Adapters, Frameworks |
| Adapters | HTTP handlers, DB repositories, external API clients, queue consumers/producers, OTel emission | Frameworks (except through defined seams) |
| Frameworks | HTTP server, DI container, config loader, logging setup, migration runner | (outermost — everyone else may not import back in) |

Dependency direction: **outer → inner only.** Enforced by `architecture-guardrails.md` #1 and the `verify-dependencies` skill.

## Directory Tree (Go — reference)

```
<project-root>/
├── cmd/
│   └── <service-name>/
│       └── main.go                     # entrypoint, DI wiring
├── internal/
│   ├── domain/
│   │   ├── user.go                     # entity with methods
│   │   ├── user_factory.go             # per CLAUDE.md: name.type.ext
│   │   └── errors.go                   # domain-level error types
│   ├── usecases/
│   │   ├── create_user.go
│   │   ├── create_user_test.go         # unit test with mocked repo
│   │   └── user_repository.go          # interface, defined by the consumer
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── user_handler.go
│   │   │   └── router.go
│   │   ├── persistence/
│   │   │   ├── postgres_user_repository.go     # implements domain interface
│   │   │   └── postgres_user_repository_test.go
│   │   ├── queue/
│   │   │   └── rabbitmq_publisher.go
│   │   └── otel/
│   │       └── tracer.go
│   ├── config/
│   │   └── config.go
│   └── migrations/
│       ├── 001_create_users_table.up.sql
│       └── 001_create_users_table.down.sql
├── go.mod
├── go.sum
├── .golangci.yml
├── docker-compose.yml                  # local dev deps (Postgres, Redis, etc.)
├── .env.example
├── .gitignore
└── README.md
```

## Directory Tree (TypeScript)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── user.model.ts               # per CLAUDE.md TS naming
│   │   ├── user.factory.ts
│   │   └── errors.ts
│   ├── usecases/
│   │   ├── create-user.usecase.ts
│   │   ├── create-user.spec.ts
│   │   └── user-repository.interface.ts
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── user.controller.ts
│   │   │   └── router.ts
│   │   ├── persistence/
│   │   │   ├── postgres-user.repository.ts
│   │   │   └── postgres-user.repository.spec.ts
│   │   ├── queue/
│   │   │   └── rabbitmq-publisher.adapter.ts
│   │   └── otel/
│   │       └── tracer.adapter.ts
│   ├── config/
│   │   └── config.ts
│   └── index.ts                        # entrypoint
├── migrations/
│   └── 001_create_users.sql
├── package.json
├── tsconfig.json                       # strict: true
├── .eslintrc.json                      # complexity max 6, no-explicit-any
├── vitest.config.ts
├── docker-compose.yml
├── .env.example
├── .gitignore
└── README.md
```

## Directory Tree (Python)

```
<project-root>/
├── src/<package_name>/
│   ├── domain/
│   │   ├── user_model.py               # @dataclass(frozen=True)
│   │   ├── user_factory.py
│   │   └── errors.py
│   ├── usecases/
│   │   ├── create_user.py
│   │   └── user_repository.py          # ABC with @abstractmethod
│   ├── adapters/
│   │   ├── http/
│   │   │   └── user_router.py
│   │   ├── persistence/
│   │   │   └── postgres_user_repository.py
│   │   ├── queue/
│   │   │   └── rabbitmq_publisher.py
│   │   └── otel/
│   │       └── tracer.py
│   └── config/
│       └── config.py
├── tests/
│   ├── unit/
│   ├── integration/
│   └── contract/
├── pyproject.toml                      # uv-managed
├── ruff.toml                           # complexity, imports, formatting
├── docker-compose.yml
├── .env.example
├── .gitignore
└── README.md
```

**Java and C#** follow the same layer structure; see `shared/rules/java-conventions.md` and `shared/rules/csharp-conventions.md` for language-specific tooling picks (Gradle vs. .NET 8, JUnit 5 vs. xUnit, etc.).

## Key Abstractions (non-negotiable — do not bypass)

- **Repositories are interfaces defined in the use-case layer**, implemented in the adapter layer (per `CLAUDE.md`: "Define interfaces in the consumer package").
- **Every external dependency (DB, HTTP client, queue, filesystem, clock) hides behind an interface.** Enforced by `architecture-guardrails.md` #5 and `CLAUDE.md`.
- **Domain entities have no persistence logic.** No `@Entity` decorators from ORMs. No `.save()` methods.
- **Factories are the only place `new` is called on complex domain objects outside of tests** (per `CLAUDE.md`).
- **Migrations follow expand/contract** (per `architecture-guardrails.md` #2). No `DROP COLUMN`, no `RENAME COLUMN`, no `NOT NULL` without `DEFAULT`.
- **All network calls have explicit timeouts** (per `architecture-guardrails.md` #5).
- **Retries use `CircuitBreaker` or `ExponentialBackoff`** — never hand-rolled loops.
- **Collection endpoints are paginated** — cursor-based preferred (per `architecture-guardrails.md` #6).
- **No N+1 queries** — eager loading or DataLoaders required (per `architecture-guardrails.md` #6).

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Language default (Vitest / Go `testing` / pytest / JUnit 5 / xUnit) | Use-cases with mocked adapters, domain entity invariants, factory logic |
| Integration | `qa-engineer` | Same as unit + testcontainers | Adapter + real dependency (Postgres via testcontainers, Redis, etc.) |
| API Contract | `qa-engineer` or `api-test-generator` | Sunday framework | HTTP/gRPC endpoint schema conformance |
| E2E | `qa-engineer` | Saturday framework (if UI in front) or Sunday for API-only | Full request → DB → response paths |

## Integration Map (typical)

- **Primary database** — Postgres/MySQL/whatever. Behind a repository interface. Requires `DATABASE_URL`.
- **Cache** — Redis/Memcached. Behind an interface. Requires `REDIS_URL`.
- **Message queue** — RabbitMQ/Kafka/SQS. Behind publisher and consumer interfaces. Requires broker connection env vars.
- **External APIs** — one adapter per API, behind a use-case-defined interface. Requires per-API tokens.
- **OTel collector** — traces, metrics, logs. Adapter-layer only.
- **Secrets** — via vault or env vars, never hardcoded.

## OTel Instrumentation Plan

- **Request span** — one per inbound HTTP/gRPC/queue message. Emitted in the adapter (HTTP middleware, queue consumer wrapper).
- **Use-case span** — one per use-case invocation. Adapter middleware wraps the call; the use-case itself doesn't know it's traced.
- **Adapter span** — one per outbound call (DB query, HTTP call, queue publish). Standard `db.*` / `http.*` / `messaging.*` tags.
- **Domain layer never emits spans** (per `architecture-guardrails.md` #8).

## Scaffold Recipe (plan-and-scaffold mode)

Language-specific — the skill picks the right one based on Phase 1 Q3.

**Common to all languages:**
- `docker-compose.yml` — local Postgres + Redis + Jaeger (for OTel visualization) + LocalStack (for S3/SQS if used).
- `.env.example` — every external dependency's connection string + `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — language-appropriate.
- `README.md` — quickstart, arch overview, layer boundary explanation.

**Go-specific:** `go.mod` with net/http (or Chi/Echo), `pgx`, `testify`, `testcontainers-go`, OTel packages. `.golangci.yml` with gocyclo max 6.

**TS-specific:** `package.json` with Fastify (or Express), `pg`, `vitest`, `testcontainers`, OTel packages. `tsconfig.json` strict. `.eslintrc.json` complexity max 6, no-explicit-any error.

**Python-specific:** `pyproject.toml` uv-managed, FastAPI, `asyncpg`, pytest, `testcontainers`, OTel. `ruff.toml`.

**Java-specific:** Gradle Kotlin DSL, Spring Boot (or vanilla Javalin), JUnit 5, Testcontainers, OTel Java agent. Checkstyle for complexity.

**C#-specific:** .NET 8, Central Package Management (`Directory.Packages.props`), xUnit, Testcontainers.NET, OTel. Nullable reference types on.

**Every language:** one failing first test (Red) in `usecases/` — a `create_user` use-case with a mocked repository.

## ADR-000 Seed Context

> We are building a backend service following Clean Architecture. Rationale: strict layer separation (Domain / Use-Cases / Adapters / Frameworks) means the domain is testable without any framework, adapters can be swapped without touching business logic, and dependencies flow inward only. This is enforced mechanically via the `verify-dependencies` skill (import-graph analysis) and philosophically via `architecture-guardrails.md`. Language choice: <language from Phase 1>, chosen because <reason>. Alternatives considered: layered monolith (rejected — allows domain to import ORMs, leaks framework concerns); ports and adapters as documented in Alistair Cockburn's original paper (accepted as functionally equivalent to Clean Architecture; using Uncle Bob's naming for consistency with the rest of this ecosystem). Consequences: every external dep is behind an interface; migrations are expand/contract; all network calls have timeouts; retries use resilience primitives, never hand-rolled loops.

## Downstream Agents (typical invocation plan)

1. `analyst` — break seed scenarios into acceptance criteria.
2. `architect` — layer boundary decisions, interface definitions.
3. `data-engineer` — schema design, migration files (expand/contract enforced).
4. `performance-engineer` — N+1 prevention, timeout budgets, pagination strategy.
5. `developer` — implement use-cases test-first.
6. `code-reviewer` — enforce Clean Architecture, SOLID, Sandi Metz limits.
7. `security-reviewer` — STRIDE threat modeling on auth, input validation, secrets.
8. `qa-engineer` — integration and contract tests.
9. `sre-engineer` — OTel span design, SLI definitions, log cardinality.
10. `tech-writer` — README, ADRs, OpenAPI spec.
11. `devops-engineer` — CI/CD, migration runner, deployment topology.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
