# Blueprint: MCP Server

**Registry id**: `mcp-server`
**Suggested language**: Go (see language note below)
**Supported languages**: Go, TypeScript
**Testing levels covered**: API Contract, Integration, Unit
**Status**: Stable — pattern extracted from existing MCP servers in the ecosystem.

> **The design pattern is non-negotiable. The language is a preference.** What makes this blueprint valuable is the Clean Architecture separation between tools (use-case layer), external APIs (adapter layer), and MCP transport (framework layer) — and the tool / persona / workflow trinity as the domain vocabulary. Language choice is soft: Go is suggested because it matches `CLAUDE.md`'s Backend/MCP declaration and produces a single deployable binary, but TypeScript is fully first-class — the MCP SDK is mature in both languages. Pick whichever fits your team and existing codebase; the pattern is identical either way.

## When To Use

- You are building a **Model Context Protocol server** that exposes tools, personas (prompts), or workflows (compositions of tools) to MCP clients (Claude Desktop, Claude Code, Cursor, etc.).
- You need to wrap an existing internal API, database, or service and make it callable by an LLM in a structured, schema-validated way.
- You need long-running workflows composed of multiple tool calls with explicit state.

Do NOT use when:
- You need a plain HTTP API for humans → use `clean-arch-service`.
- You need a CLI for humans → use `clean-arch-cli` or `scribe-cli`.
- You just want to expose one function to Claude and it's already an HTTP endpoint — an inline tool definition in your Claude project is lighter than a full MCP server.

## Layer Structure

| Layer | Responsibility | Example files (Go / TypeScript) |
|---|---|---|
| Domain | Tool definitions (name, description, JSON schema for inputs/outputs), Persona definitions, Workflow state machines | `tool.go` / `tool.ts`, `persona.go` / `persona.ts`, `workflow.go` / `workflow.ts` |
| Use-Cases | Tool execution logic — what the tool actually does when invoked | `create_issue_tool.go` / `create-issue.tool.ts` |
| Adapters | MCP transport (stdio, SSE, streamable HTTP), external API clients, database clients | `stdio_transport.go` / `stdio-transport.adapter.ts`, `github_client.go` / `github.client.ts` |
| Frameworks | MCP SDK bindings, HTTP server, logging | `main.go` / `index.ts` |

## Directory Tree (Go primary)

```
<project-root>/
├── cmd/
│   └── mcp-server/
│       └── main.go                     # entrypoint, wires transport + tools
├── internal/
│   ├── domain/
│   │   ├── tool.go                     # Tool interface
│   │   ├── persona.go                  # Persona interface
│   │   └── workflow.go                 # Workflow state machine
│   ├── tools/                          # one file per tool
│   │   ├── create_issue_tool.go
│   │   ├── search_repos_tool.go
│   │   └── list_prs_tool.go
│   ├── personas/                       # one file per persona
│   │   └── code_reviewer_persona.go
│   ├── workflows/                      # one file per workflow
│   │   └── triage_issue_workflow.go
│   ├── adapters/
│   │   ├── transport/
│   │   │   ├── stdio_transport.go
│   │   │   └── sse_transport.go
│   │   ├── github/
│   │   │   ├── github_client.go        # implements domain interface
│   │   │   └── github_client_test.go
│   │   └── otel/
│   │       └── otel_tracer.go
│   └── config/
│       └── config.go
├── go.mod
├── go.sum
├── .golangci.yml                       # gocyclo max 6
├── .env.example
├── .gitignore
└── README.md
```

## Directory Tree (TypeScript alternative)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── tool.interface.ts
│   │   ├── persona.interface.ts
│   │   └── workflow.interface.ts
│   ├── tools/
│   │   ├── create-issue.tool.ts
│   │   ├── search-repos.tool.ts
│   │   └── list-prs.tool.ts
│   ├── personas/
│   │   └── code-reviewer.persona.ts
│   ├── workflows/
│   │   └── triage-issue.workflow.ts
│   ├── adapters/
│   │   ├── transport/
│   │   │   ├── stdio-transport.adapter.ts
│   │   │   └── sse-transport.adapter.ts
│   │   ├── github/
│   │   │   ├── github.client.ts
│   │   │   └── github.client.spec.ts
│   │   └── otel/
│   │       └── otel-tracer.adapter.ts
│   ├── config/
│   │   └── config.ts
│   └── index.ts                        # entrypoint
├── package.json
├── tsconfig.json
├── vitest.config.ts
├── .eslintrc.json
├── .env.example
├── .gitignore
└── README.md
```

## Key Abstractions (non-negotiable — do not bypass)

- **`Tool` interface** — every exposed tool implements this. Carries: name, description, input JSON schema, output JSON schema, execute function.
- **`Persona` interface** — a prompt template + optional tool-set. Represents a "role" the LLM can adopt.
- **`Workflow` interface** — a state machine composed of tool calls. Explicit state, resumable, testable.
- **`Transport` interface** — stdio / SSE / streamable HTTP. Injected at the entrypoint. Domain never depends on transport.
- **External API clients live in adapters and are hidden behind domain-defined interfaces** (per `architecture-guardrails.md` #1). Never call GitHub/Jira/Slack SDKs directly from a tool.

Reject any code that:
- Puts business logic in `main.go` / `index.ts` (the entrypoint should be wiring only).
- Calls an external API SDK directly from a tool's execute function — must go through an adapter.
- Hardcodes credentials or tokens (per `architecture-guardrails.md` #3).

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Go: `testing` + table-driven / TS: Vitest | Tool execute logic with mocked adapters, workflow state transitions, persona template rendering |
| Integration | `qa-engineer` | Same as unit + test containers | Tool + adapter + real external service (staging or test container) |
| API Contract | `qa-engineer` (or `api-test-generator` if the external API has an OpenAPI spec) | Sunday framework via `api` fixture | Schema conformance for every tool's input/output; external API client's contract with the upstream service |

## Integration Map (typical)

- **MCP client** — Claude Desktop / Claude Code / Cursor. Communicates via stdio or SSE. No auth typically needed for stdio; SSE needs a token.
- **External APIs / DBs** — whatever the tools wrap. Each gets an adapter behind a domain interface. Requires per-service env vars.
- **OTel collector** — every tool invocation emits a span. Requires `OTEL_EXPORTER_OTLP_ENDPOINT`.
- **Secrets management** — API tokens, DB credentials via `.env` locally, secure vault in production. Never hardcoded.

## OTel Instrumentation Plan

- **Tool span** — one per tool invocation. Tags: `tool.name`, `tool.duration_ms`, `tool.success`, `tool.error_class` (if any).
- **Workflow span** — one per workflow run. Child spans per tool called within the workflow. Tags: `workflow.name`, `workflow.state`.
- **External API span** — one per outbound HTTP/gRPC call. Child of the tool span. Tags: `http.method`, `http.url`, `http.status_code`.
- **Domain layer never emits spans.** Tracing lives in the adapter layer per `architecture-guardrails.md` #8.

## Scaffold Recipe (plan-and-scaffold mode)

**Go:**
- `go.mod` with `github.com/modelcontextprotocol/go-sdk` (or the current canonical Go SDK), `github.com/stretchr/testify`, `go.opentelemetry.io/otel`.
- `.golangci.yml` — gocyclo max 6, revive, errcheck, staticcheck enabled.
- `cmd/mcp-server/main.go` — wires stdio transport, registers one example tool.
- `internal/tools/hello_tool.go` — one tool that returns "hello world".
- `internal/tools/hello_tool_test.go` — one failing table-driven test (Red).
- `.env.example` — placeholders for external API tokens, `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — `/mcp-server` (built binary), `coverage.out`, `.env`.

**TypeScript:**
- `package.json` with `@modelcontextprotocol/sdk`, `vitest`, `zod`, `@opentelemetry/api`.
- `tsconfig.json` — `strict: true`, `noImplicitAny: true`.
- `.eslintrc.json` — complexity max 6, no-explicit-any error.
- `vitest.config.ts` — coverage threshold 85%.
- `src/index.ts` — wires stdio transport, registers one example tool.
- `src/tools/hello.tool.ts` — one tool returning "hello world".
- `src/tools/hello.tool.spec.ts` — one failing test (Red).
- `.env.example` — same placeholders as Go variant.
- `.gitignore` — `node_modules/`, `dist/`, `coverage/`, `.env`.

## ADR-000 Seed Context

> We are building an MCP server to expose <domain> capabilities as tools/personas/workflows for MCP-compatible LLM clients. Rationale: MCP is the emerging standard for LLM-to-tool integration; a proper server (rather than inline tool definitions) gives us versioning, schema validation, workflow state, and reusability across multiple client applications. We chose <Go | TypeScript> as the language because <reason from Phase 1>. Clean Architecture separation between tools (use-case layer) and external API clients (adapter layer) means we can swap upstream services without rewriting tools, and we can unit-test tool logic without hitting the network. Alternatives considered: inline function-calling in each client (rejected — no versioning, no reuse); HTTP API (rejected — no schema-first tool discovery, requires custom client code per LLM). Consequences: every external dependency goes behind an adapter interface; every tool has a Zod/JSON schema; every tool call emits an OTel span; no credentials in code.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each seed tool/workflow into acceptance criteria.
2. `architect` — validate the tool/persona/workflow layering, especially for workflows that span multiple tools.
3. `data-engineer` — if the server wraps a database, review schema access patterns.
4. `developer` — implement tools test-first.
5. `code-reviewer` — enforce adapter isolation for external APIs.
6. `security-reviewer` — token handling, secret redaction, workflow authorization.
7. `qa-engineer` — integration tests against test containers or staging.
8. `sre-engineer` — OTel span cardinality per tool.
9. `devops-engineer` — packaging (single binary for Go, npm package for TS), stdio vs. SSE deployment story.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
