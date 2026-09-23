# Blueprint: Clean Architecture CLI

**Registry id**: `clean-arch-cli`
**Primary language**: Go (via Cobra)
**Supported languages**: Go, TypeScript (via Commander)
**Testing levels covered**: E2E (CLI-driven), Integration, Unit
**Status**: Stable — the generic CLI blueprint. Use `scribe-cli` instead if the CLI's purpose is content publishing.

## When To Use

- You are building a **command-line tool** with more than one subcommand and non-trivial domain logic.
- Domain logic should be testable **without** invoking Cobra/Commander (Clean Architecture applied to CLIs).
- You want single-binary distribution (Go) or npm-installable (TypeScript).

Do NOT use when:
- The tool is a content publisher → use `scribe-cli`.
- The tool has one flag and one output — a shell script is lighter.
- The tool needs to run as a long-lived server → use `clean-arch-service`.

## Layer Structure

| Layer | Responsibility | Cannot import from |
|---|---|---|
| Domain | Business rules, entities, value objects | Anything except stdlib |
| Use-Cases | Command handlers, orchestration | Adapters, Frameworks |
| Adapters | Filesystem access, HTTP clients, external tool wrappers (git, docker, kubectl, etc.), OTel emission | Frameworks (except through DI) |
| Frameworks | Cobra/Commander wiring, config loading, logging, terminal I/O | (outermost) |

Same dependency direction rule as `clean-arch-service`: outer → inner only.

## Directory Tree (Go — reference)

```
<project-root>/
├── cmd/
│   └── <cli-name>/
│       └── main.go                     # Cobra root
├── internal/
│   ├── domain/
│   │   ├── project.go
│   │   ├── project_factory.go
│   │   └── errors.go
│   ├── usecases/
│   │   ├── init_project.go
│   │   ├── init_project_test.go
│   │   ├── list_projects.go
│   │   └── project_repository.go       # interface
│   ├── adapters/
│   │   ├── filesystem/
│   │   │   ├── filesystem_repository.go
│   │   │   └── filesystem_repository_test.go
│   │   ├── git/
│   │   │   └── git_adapter.go          # wraps `git` shell calls
│   │   ├── http/
│   │   │   └── registry_client.go
│   │   └── otel/
│   │       └── tracer.go
│   ├── cmd/                            # Cobra command definitions
│   │   ├── root.go
│   │   ├── init.go                     # thin, delegates to use-case
│   │   ├── list.go
│   │   └── version.go
│   ├── config/
│   │   └── config.go                   # loads ~/.<cli>.yaml
│   └── output/
│       ├── text_formatter.go
│       ├── json_formatter.go
│       └── formatter.go                # interface
├── testdata/
│   └── sample-project.yaml
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── .golangci.yml
├── .env.example
├── .gitignore
└── README.md
```

## Directory Tree (TypeScript)

```
<project-root>/
├── src/
│   ├── domain/
│   │   ├── project.model.ts
│   │   ├── project.factory.ts
│   │   └── errors.ts
│   ├── usecases/
│   │   ├── init-project.usecase.ts
│   │   ├── init-project.spec.ts
│   │   ├── list-projects.usecase.ts
│   │   └── project-repository.interface.ts
│   ├── adapters/
│   │   ├── filesystem/
│   │   │   ├── filesystem.repository.ts
│   │   │   └── filesystem.repository.spec.ts
│   │   ├── git/
│   │   │   └── git.adapter.ts
│   │   ├── http/
│   │   │   └── registry-client.adapter.ts
│   │   └── otel/
│   │       └── tracer.adapter.ts
│   ├── cli/                            # Commander wiring
│   │   ├── index.ts
│   │   ├── init.command.ts
│   │   └── list.command.ts
│   ├── output/
│   │   ├── text.formatter.ts
│   │   ├── json.formatter.ts
│   │   └── formatter.interface.ts
│   └── index.ts                        # entrypoint with shebang
├── package.json                        # "bin" field points at dist/index.js
├── tsconfig.json
├── .eslintrc.json
├── vitest.config.ts
├── .env.example
├── .gitignore
└── README.md
```

## Key Abstractions (non-negotiable — do not bypass)

- **Cobra/Commander commands are thin.** They parse flags, invoke a use-case, format the result. They contain zero business logic.
- **The use-case layer never touches `os.Args`, `os.Exit`, `console.log`, or terminal I/O.** Output goes through the `Formatter` interface.
- **Output formatters live in the adapter layer.** Support at minimum: text (human) and JSON (machine).
- **External tools (git, docker, kubectl) go behind adapters.** Never shell out from the use-case layer.
- **`--dry-run` for every destructive command.** Follows the same discipline as `scribe-cli`.
- **Exit codes are meaningful and documented.** 0 = success, 1 = general error, 2 = invalid input, etc.

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Go `testing` table-driven / Vitest | Use-cases with mocked adapters, formatters, domain logic |
| Integration | `qa-engineer` | Same as unit + testcontainers if needed | Adapters against real filesystem, real git, real registry |
| E2E | `qa-engineer` | Go: invoke built binary via `os/exec`; TS: invoke via `child_process` | Full CLI runs, exit-code assertions, output-format assertions |

## Integration Map (typical)

- **Filesystem** — behind a `Filesystem` interface. Every read/write is testable with an in-memory implementation.
- **Git / Docker / Kubectl** — one adapter per external tool. Each wraps a subprocess.
- **HTTP registry** (if the CLI talks to a package registry, config service, etc.) — behind a client interface.
- **User config** — `~/.<cli-name>.yaml`, loaded via config layer. Never read directly from use-cases.
- **OTel collector** (optional) — CLI operations can emit spans if `OTEL_EXPORTER_OTLP_ENDPOINT` is set; degrades gracefully.

## OTel Instrumentation Plan

- **Command invocation span** — one per CLI command. Tags: `cli.command`, `cli.args_hash` (never log raw args — could contain secrets), `cli.exit_code`.
- **Use-case span** — one per use-case call.
- **Adapter span** — one per external subprocess or HTTP call.
- **Domain never emits spans.**

## Scaffold Recipe (plan-and-scaffold mode)

**Go:**
- `go.mod` — `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/stretchr/testify`, OTel.
- `.goreleaser.yaml` — multi-platform builds, homebrew tap.
- `.golangci.yml` — gocyclo max 6.
- `cmd/<cli-name>/main.go` — Cobra root, one subcommand.
- `internal/usecases/init_project.go` — one use-case interface + implementation.
- `internal/usecases/init_project_test.go` — one failing test (Red).
- `internal/adapters/filesystem/filesystem_repository.go` — implements the domain interface.

**TypeScript:**
- `package.json` — `commander`, `vitest`, OTel. `"bin"` field. `"type": "module"`.
- `tsconfig.json` — `strict: true`, `module: NodeNext`.
- `.eslintrc.json` — complexity max 6, no-explicit-any.
- `vitest.config.ts` — 85% coverage.
- `src/index.ts` — shebang + Commander wiring.
- `src/usecases/init-project.usecase.ts` — one use-case.
- `src/usecases/init-project.spec.ts` — one failing test.

**Both:**
- `.env.example` — `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — language-appropriate.

## ADR-000 Seed Context

> We are building a CLI tool following Clean Architecture. Rationale: Cobra/Commander is a framework concern; putting business logic in command handlers makes it untestable without spawning subprocesses and makes reuse (as a library from another tool) impossible. Layer separation means the CLI is one adapter over a testable core. Language choice: <language from Phase 1>, chosen because <reason>. Alternatives considered: script-first (rejected — no dry-run, no exit-code discipline, no testability); a "fat main.go" with all logic inline (rejected — untestable, unreusable). Consequences: commands are thin; use-cases have zero terminal I/O; every external tool is behind an adapter; every destructive command supports `--dry-run`; exit codes are documented.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each subcommand into acceptance criteria.
2. `architect` — validate the command/use-case boundary, formatter interface design.
3. `developer` — implement use-cases test-first.
4. `code-reviewer` — enforce thin commands, no business logic in Cobra handlers.
5. `qa-engineer` — E2E CLI tests (invoke built binary, assert on output and exit code).
6. `security-reviewer` — argument handling, subprocess injection risk.
7. `sre-engineer` — OTel opt-in behavior, log format.
8. `devops-engineer` — GoReleaser / npm publishing config, homebrew tap.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
