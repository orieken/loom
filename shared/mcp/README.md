# ai-assistant-dotfiles MCP Server

A reference scaffold exposing the framework's deterministic tools as an
[MCP](https://modelcontextprotocol.io/) server over stdio transport.

## Tools

| Tool | Description |
|---|---|
| `analyze_complexity` | Cyclomatic complexity + LOC per function against framework thresholds (< 7 / < 30) |
| `check_accessibility` | Semantic-HTML / ARIA violations in HTML, Vue, JSX, TSX, Svelte files |
| `check_ubiquitous_language` | Synonym violations against a `DOMAIN_DICTIONARY.md` |
| `verify_dependencies` | Clean Architecture layer-boundary violations in Go and TypeScript imports |
| `search_ki` | Lexical-ranked search of the framework's Knowledge Items and ADRs |
| `search_docs` | BM25 search (sqlite-fts5) of the installed project's `docs/` corpus |
| `validate_artifact` | Structural contract validation of a pipeline artifact against `shared/contracts/` — required-heading presence plus WARN-level retrieval frontmatter checks; returns typed violations |

All tools are deterministic and stateless — no LLM is required.

`validate_artifact` infers the contract from the artifact's filename (e.g.
`analysis.md` → `shared/contracts/analysis-contract.md`) using
`AI_ASSISTANT_DOTFILES_PATH` as the install root; pass `contractPath`
explicitly to override or when the env var isn't set. It is structural only —
the contract-specific prose content rules and any qualitative judgment remain
with the `validate-artifact` skill until L2.11 lands semantic validation.

## Quick start

The supported way to run this server is the `loom` binary itself:

```bash
brew install orieken/tap/loom   # or: go install github.com/orieken/loom/cmd/loom@latest
loom mcp serve                  # stdio transport; logs to stderr or --log-file
```

> **Deprecated**: the standalone `cmd/mcp-server` entrypoint is retained for
> one release cycle only. It still builds from the repo root
> (`go build ./shared/mcp/cmd/mcp-server`), but new setups should use
> `loom mcp serve`. Since the module merge into `github.com/orieken/loom`,
> this directory is part of the root Go module and no longer carries its own
> `go.mod`.

## Configuration

Copy `.env.example` to `.env` and set:

| Variable | Purpose | Default |
|---|---|---|
| `AI_ASSISTANT_DOTFILES_PATH` | Root of this framework checkout; enables `search_ki` corpus | _(required for search_ki)_ |
| `DOCS_FTS_PATH` | Absolute path to the FTS5 sqlite index for `search_docs` | `.claude/rag/docs-fts5.sqlite` if `.claude/` exists |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | _(optional)_ |

## Wire into Claude Code

Add to `.claude/mcp.json` (or `~/.claude/mcp.json` for global):

```json
{
  "mcpServers": {
    "loom": {
      "command": "loom",
      "args": ["mcp", "serve"],
      "env": {
        "AI_ASSISTANT_DOTFILES_PATH": "/absolute/path/to/loom-checkout"
      }
    }
  }
}
```

Or register it with the Claude Code CLI:

```bash
claude mcp add loom -- loom mcp serve
```

## Installing into a downstream project

If you already have an MCP server, see
[`docs/prompts/done/install-framework-with-mcp-bridge.md`](../../docs/prompts/done/install-framework-with-mcp-bridge.md)
for the bridge-prompt approach (no Go required). It shipped as a handoff prompt, not a skill; the
link previously named a skill that never existed.

If you do not have an MCP server, you don't need a scaffold anymore — install
the `loom` binary and point your MCP host at `loom mcp serve`. The
`--with-mcp` scaffold copy (`install.sh` / `loom install`) still works but now
produces reference source only: since the module merge it has no `go.mod` of
its own and is not standalone-buildable. It is deprecated alongside
`cmd/mcp-server`.

## Embedding loom's tools

If you write your own Go MCP server, embed loom's tools through the public
API instead of the deprecated `register.FrameworkTools`:

- **`github.com/orieken/loom/tools`** — the transport-free `Tool` interface,
  `ToolRegistration` (timeout budget, retry class, permission scope), and a
  name-keyed `Registry` with `Register` and `Merge`. This package imports the
  standard library only (enforced by a fitness test), so nothing in its
  signatures ties you to loom's `mcp-go` or `jsonschema` version pins.
- **`github.com/orieken/loom/shared/mcp/register`** — `Frameworks(logWriter)`
  returns the built-in framework tools as a `*tools.Registry`.

Merge, add your own tools, and adapt the result to *your* MCP library and
version:

```go
registry := tools.NewRegistry()
if err := registry.Register(mytool.Registration()); err != nil { ... }
if err := registry.Merge(register.Frameworks(nil)); err != nil { ... }

for _, registration := range registry.All() {
    s.AddTool(wireDefinition(registration.Tool), wireHandler(registration.Tool))
}
```

`wireDefinition` / `wireHandler` are ~30 lines you own, written against your
own `mcp-go` (or any other MCP library): map `Name`/`Description`/schemas onto
your wire type and convert `tools.ToolResult` content blocks back. A complete,
buildable server — custom `echo` tool included, with its own `go.mod` — lives
in [`examples/embedding/`](../../examples/embedding/). Note the custom tool's
package imports **only the stdlib and `github.com/orieken/loom/tools`** — no
loom internals, no MCP library.

**Compatibility promise**: the `tools` package (and `register.Frameworks`)
follow semantic versioning from their first tagged release onward — no
breaking changes to exported signatures within a major version. Until that
first tag lands, treat the API as v0: stable in intent, but the import path
and signatures may still shift.

## Layout

```
shared/mcp/
├── cmd/mcp-server/main.go       # standalone stdio entrypoint (deprecated — use `loom mcp serve`)
├── register/register.go         # FrameworkTools — embedding entry point
├── internal/
│   ├── analyzers/               # complexity, accessibility, language, deps analyzers
│   ├── domain/
│   │   ├── tool.go              # transport-free Tool interface (stdlib-only, enforced by test)
│   │   └── registration.go      # ToolRegistration metadata + name-keyed Registry (Register/Merge)
│   ├── logging/logger.go        # slog-backed Logger
│   ├── server/
│   │   ├── handler.go           # Handler struct owning the registry
│   │   ├── mcp_adapter.go       # domain ↔ mcp-go wire-type conversion (only place mcp types touch tools)
│   │   ├── registration.go      # registry → AddTool loop
│   │   └── tool_provider.go     # buildFrameworkRegistry wiring (add a tool = one Register entry)
│   └── tools/                   # 6 MCP tool implementations + retriever
└── .env.example
```

These packages live in the root Go module (`github.com/orieken/loom`) under
`shared/mcp/`; lint config (`.golangci.yml`, gocyclo cap at 7) sits at the
repo root.

## Dependencies

- [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) v0.57.0 — MCP SDK
- [`invopop/jsonschema`](https://github.com/invopop/jsonschema) v0.14.0 — output schema reflection
- [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) v1.55.0 — pure-Go sqlite for BM25 retrieval
