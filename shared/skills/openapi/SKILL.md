---
name: openapi
description: Deliberate API contract design before a line of implementation code is written.
triggers:
  keywords: ["API", "OpenAPI", "contract", "swagger", "endpoint"]
  intentPatterns: ["Design the API for *", "Write an OpenAPI spec for *", "What should the API look like for *?", "Define the contract for *"]
standalone: true
---

## When To Use
Triggered when an API change is requested or `analysis.md` has API Changes.
When NOT to use: Internal method signatures, test helper APIs, or any interface that isn't crossed over HTTP.

## Context To Load First
1. `DOMAIN_DICTIONARY.md`
2. `.claude/feature-workspace/<feature-name>/analysis.md`

## Process
1. "What resource does this endpoint operate on?"
2. "What HTTP verb best describes the operation? Walk through the options together if unsure."
3. "What does a successful response look like? What fields does the consumer actually need?"
4. "What can go wrong? List the failure cases — each needs a distinct status code."
5. "Does this operation have side effects? If so, does it need an idempotency key?"
6. "What authentication/authorization is required?"
7. "Are there any performance constraints? (pagination, rate limits, timeouts)"
8. Draft the OpenAPI spec section and show it for approval
9. On approval, write to `docs/api/[resource].openapi.yaml` (or append to existing spec)

## Output Format
`.claude/feature-workspace/openapi-[resource].yaml` and `.claude/feature-workspace/api-design-notes.md`

```markdown
# API Design Notes: [Resource]

## Design Decisions
| Decision | Rationale |
|---|---|
| POST /sessions not /login | Resource-oriented — session is the resource |
| 429 with Retry-After | Communicates lockout duration without revealing account state |

## Security Surface
- [Auth required: yes/no + mechanism]
- [User enumeration risk: addressed how]
- [Rate limiting: yes/no + threshold]

## Breaking Change Assessment
- Additive only: yes/no
- Breaking changes: [list — requires version bump]

## Sunday Framework Mapping
- Client class: `[ClassName]` extends `BaseApiClient`
- Zod schema: `[SchemaName]` validates response shape
- Auth provider: `[Provider]`
```

## Guardrails
- Never return error messages that reveal whether an account exists (user enumeration).
- Every collection endpoint must have pagination.
- Every mutation must specify idempotency strategy.
- OpenAPI spec must be shown to the user for approval before being written to disk.
- Never use 200 for error responses.

## Standalone Mode
Pure design reasoning. No external tools required. Produces YAML spec locally.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
