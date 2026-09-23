# Blueprint: Scribe — Content Publishing CLI

**Registry id**: `scribe-cli`
**Primary language**: Go (only supported language)
**Testing levels covered**: E2E/UI (CLI-driven), Integration, Unit
**Status**: Stable — single-binary distribution is core to the pattern.

## When To Use

- You are building a **command-line tool that publishes content across multiple channels** — blog (Markdown → static site), RSS, S3, Bluesky, LinkedIn, Mastodon, Dev.to, Medium, etc.
- You need a single-binary distribution model (users run `brew install scribe` or download a release, not `npm install`).
- You need each publishing destination to be pluggable — adding a new channel is one adapter, not a rewrite.
- You need dry-run mode for every destructive action.

Do NOT use when:
- You need a long-running service (this is a one-shot CLI) → use `clean-arch-service`.
- You need a general-purpose CLI unrelated to content publishing → use `clean-arch-cli`.
- You only publish to one channel and never plan to add more — a plain script is lighter.

## Layer Structure

| Layer | Responsibility | Example files |
|---|---|---|
| Domain | Content model (Post, Publication, Draft), channel-agnostic formatting rules, dry-run semantics | `post.go`, `publication.go`, `formatter.go` |
| Use-Cases | Publish orchestration, channel selection, dry-run vs. commit paths, retry policy | `publisher.go`, `publish_workflow.go` |
| Adapters | One per destination — RSS, S3, Bluesky, LinkedIn, Mastodon, filesystem, etc. | `rss_adapter.go`, `s3_adapter.go`, `bluesky_adapter.go` |
| Frameworks | Cobra CLI wiring, config loading, logging, OTel wiring | `cmd/root.go`, `cmd/publish.go`, `internal/config/config.go` |

## Directory Tree

```
<project-root>/
├── cmd/
│   └── scribe/
│       └── main.go                     # entrypoint (Cobra)
├── internal/
│   ├── domain/
│   │   ├── post.go                     # Post entity with computed properties
│   │   ├── publication.go              # Publication (result of publishing)
│   │   ├── draft.go
│   │   └── formatter.go                # channel-agnostic content formatting rules
│   ├── usecases/
│   │   ├── publisher.go                # main use-case interface
│   │   ├── publish_workflow.go         # orchestrates multiple adapters
│   │   └── dry_run.go
│   ├── adapters/
│   │   ├── rss/
│   │   │   ├── rss_adapter.go
│   │   │   └── rss_adapter_test.go
│   │   ├── s3/
│   │   │   └── s3_adapter.go
│   │   ├── bluesky/
│   │   │   └── bluesky_adapter.go
│   │   ├── linkedin/
│   │   │   └── linkedin_adapter.go
│   │   ├── filesystem/
│   │   │   └── filesystem_adapter.go
│   │   └── otel/
│   │       └── otel_tracer.go
│   ├── config/
│   │   └── config.go                   # loads ~/.scribe.yaml + env vars
│   └── cmd/
│       ├── root.go
│       ├── publish.go
│       ├── validate.go                 # lint content before publishing
│       └── channels.go                 # list available channels
├── testdata/
│   ├── sample-post.md
│   └── expected-rss.xml
├── .goreleaser.yaml                    # multi-platform builds + homebrew tap
├── go.mod
├── go.sum
├── .golangci.yml
├── .env.example
├── .gitignore
└── README.md
```

## Key Abstractions (non-negotiable — do not bypass)

- **`Post`** — immutable content entity. All formatting/validation logic lives on the entity or on `Formatter` — never in adapters.
- **`Publisher`** interface — the main use-case. Takes a Post + a set of destinations, returns a `Publication` (or a list of errors per destination).
- **`ChannelAdapter`** interface — one per destination. Each adapter implements `Publish(ctx, post) (PublicationResult, error)` and `DryRun(ctx, post) (DryRunResult, error)`.
- **`DryRun` is a first-class mode**, not an afterthought. Every adapter MUST implement it. It logs what would happen without side effects.
- **External SDKs stay in their adapter.** The domain never imports `aws-sdk-go`, the Bluesky SDK, etc.

Reject any code that:
- Uses `os.Exit()` outside `main.go`.
- Puts formatting logic in an adapter (it belongs on `Post` or `Formatter`).
- Skips the `DryRun` implementation on a new adapter.
- Hardcodes API tokens (per `architecture-guardrails.md` #3).

## Testing Pyramid Coverage

| Level | Written by | Framework | What it tests here |
|---|---|---|---|
| Unit | `developer` | Go `testing` table-driven + `testify` | `Post` validation, `Formatter` output, `Publisher` orchestration with mock adapters, dry-run semantics |
| Integration | `qa-engineer` | Same as unit + testcontainers-go | Each adapter against a real service (S3: LocalStack; Bluesky: sandbox account; RSS: filesystem write + XML validation) |
| E2E / UI | `qa-engineer` | Shell-based (e.g., `bats-core` or Go test invoking the built binary) | Full CLI runs: `scribe publish sample.md --dry-run --channel=rss,bluesky` and assert on the printed output + exit code |

## Integration Map (typical)

- **RSS output** — writes to filesystem or S3. Requires either a local path or S3 credentials.
- **S3** — for hosting the RSS/JSON feed or the static site. Requires `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `S3_BUCKET`.
- **Bluesky** — requires `BLUESKY_HANDLE`, `BLUESKY_APP_PASSWORD`.
- **LinkedIn** — requires `LINKEDIN_ACCESS_TOKEN` (OAuth 2.0 flow handled separately).
- **Mastodon** — requires `MASTODON_INSTANCE`, `MASTODON_ACCESS_TOKEN`.
- **OTel collector** — every publish action emits a span. Optional; degrades gracefully if `OTEL_EXPORTER_OTLP_ENDPOINT` is unset.

Every mutating external call is behind `shared/rules/approval-gates.md` gate #5 (posting to external APIs) when the CLI is invoked interactively — `--yes` flag required for automation.

## OTel Instrumentation Plan

- **CLI invocation span** — one per `scribe` command execution. Tags: `cli.command`, `cli.dry_run`, `cli.channels_requested`.
- **Publish span** — one per channel. Child of the CLI span. Tags: `channel.name`, `channel.success`, `channel.duration_ms`, `channel.retry_count`.
- **External API span** — one per outbound HTTP call. Child of the publish span. Standard `http.*` tags.
- **Domain never emits spans.** All OTel emission is in adapters or the Cobra command handler.

## Scaffold Recipe (plan-and-scaffold mode)

- `go.mod` with `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/stretchr/testify`, `go.opentelemetry.io/otel`, testcontainers-go (dev dep).
- `.goreleaser.yaml` — builds for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`, `windows/amd64`; homebrew tap config.
- `.golangci.yml` — gocyclo max 6, revive, errcheck, staticcheck enabled.
- `cmd/scribe/main.go` — Cobra root, one subcommand (`publish`).
- `internal/domain/post.go` — Post struct with a `Validate()` method.
- `internal/usecases/publisher.go` — `Publisher` interface.
- `internal/adapters/filesystem/filesystem_adapter.go` — the simplest adapter, both `Publish` and `DryRun` implemented.
- `internal/adapters/filesystem/filesystem_adapter_test.go` — one failing table-driven test (Red).
- `.env.example` — placeholders for every supported channel's credentials + `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `.gitignore` — `/scribe`, `/dist`, `coverage.out`, `.env`.

## ADR-000 Seed Context

> We are building a content publishing CLI. Rationale: publishing the same post to N destinations manually is repetitive and error-prone; a CLI with pluggable channel adapters lets us add destinations without touching existing ones (Open-Closed Principle). Single-binary distribution via GoReleaser means users install with `brew install` or a curl download — no runtime dependencies. Dry-run as a first-class mode ensures we never accidentally publish. Alternatives considered: shell scripts (rejected — no schema validation, no dry-run, no OTel); n8n/Zapier (rejected — vendor lock-in, no version control on the flow, subscription cost); a full CMS (rejected — massively overbuilt for one author). Consequences: every new channel is one adapter + tests; the domain has zero SDK dependencies; every mutating action supports dry-run; credentials live in `~/.scribe.yaml` or env, never in code.

## Downstream Agents (typical invocation plan)

1. `analyst` — turn each seed channel into acceptance criteria (per channel: what content shape, what auth model, what retry story).
2. `architect` — validate adapter interface uniformity, especially for channels with wildly different SDKs.
3. `developer` — implement each channel test-first (Red-Green-Refactor).
4. `code-reviewer` — enforce domain isolation from SDKs.
5. `security-reviewer` — credential handling (`~/.scribe.yaml` permissions, env var handling, token scoping).
6. `qa-engineer` — integration tests against LocalStack / sandbox accounts.
7. `sre-engineer` — OTel span cardinality, log format for machine parsing.
8. `devops-engineer` — GoReleaser config, homebrew tap, release automation.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
