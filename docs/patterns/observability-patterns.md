# Observability Patterns

`sre-engineer.md`'s entire mandate is this territory ("You believe that code without telemetry is a
black box"), with four governing principles already fully worked out in its prompt. This file extracts
those as a standalone reference, and adds the one layer above them (SLO / Error Budget) that this repo
uses the foundation of (SLIs) without yet naming the concept that sits on top.

## Service Level Indicator (SLI)

**Context**: A specific, measurable signal that defines what "healthy" looks like for a feature —
already a required deliverable in this repo: `sre-engineer.md`'s own Output Format requires an
Availability SLI and a Latency SLI for every feature it reviews.

**Example**: "Payment endpoint returns 2xx or 4xx (non-500) > 99.9% of the time" (Availability), "Payment
API p95 latency < 800ms" (Latency) — both taken directly from `sre-engineer.md`'s own contract.

## Service Level Objective (SLO)

**Context**: The target value or range for an SLI over a defined period — the difference between
*measuring* something (the SLI) and *committing* to a bar for it (the SLO). Not yet a named concept
anywhere in this repo, though the SLI half of the pair is already load-bearing practice.

**Structure**: An SLO is always stated against a time window: "99.9% of requests succeed, measured over
a rolling 30 days," not just "99.9% success" with no window. Without the window, there's no way to say
whether a given bad day actually violates the objective or is well within its normal variance.

**Trade-offs**: Setting an SLO too strict (99.99% when 99.9% would serve users fine) forces expensive
engineering effort chasing reliability nobody would notice the absence of. Setting it too loose defeats
the point of having one. The right number comes from what users actually need, not from what looks
impressive.

## Error Budget

**Context**: The inverse of the SLO — how much unreliability is *allowed* before the objective is
violated. A 99.9% availability SLO has a 0.1% error budget over its measurement window.

**Structure**: The error budget is a resource that can be deliberately spent, not just an accident to
avoid. A team that hasn't touched its error budget in a month has room to ship a riskier change or run a
chaos experiment (see `stability-patterns.md`); a team that's already burned its whole budget for the
window should freeze risky changes until the window resets.

**Trade-offs**: Without an explicit error budget, "how much risk can we take on right now" has no
concrete answer — it becomes a judgment call made fresh every time, usually under time pressure. An
error budget turns that judgment call into a number anyone can check.

**Related**: This is the same "irreversibility is the organizing principle, not blanket caution" logic
`shared/rules/approval-gates.md` already applies to actions — an error budget applies the same idea to
risk over time instead of risk per action.

## Low-Cardinality Logging

**Context**: Logging unstructured, highly variable strings makes aggregation impossible — the log
message text has to be stable and groupable; the actual variable data belongs in structured context
fields, not interpolated into the message string.

**Example** (from `sre-engineer.md` directly):
- **Bad**: `logger.info(f"User {user_id} successfully paid ${amount} for order {order_id}")` — every
  distinct combination of values is a different string, impossible to aggregate or alert on.
- **Good**: `logger.info({ user_id, amount, order_id }, "Payment processed successfully")` — the message
  text is stable and groupable across every occurrence; the variable data lives in structured fields.

## Structured Tracing (OpenTelemetry)

**Context**: Traces and spans must tell a complete story — significant asynchronous boundaries, external
network calls, and complex domain logic all need explicit spans, not just the request's outermost
boundary.

**Structure**: `shared/rules/architecture-guardrails.md` #8 (Observability Boundaries) constrains *where*
this instrumentation is allowed to live: only the adapter or interceptor layer, never inside domain
entities or use cases — see `clean-architecture-layers.md`'s Framework Layer section for the full rule.
This pattern is about *what* to instrument; that guardrail is about *where* the instrumenting code is
allowed to live.

## No PII or Secrets in Telemetry

**Context**: Traces, logs, and metrics must never contain cleartext passwords, tokens, or PII (unmasked
emails, SSNs) unless explicitly approved and tagged for compliance routing.

**Structure**: `shared/rules/architecture-guardrails.md` **#9** is the hard constraint this pattern
describes, and it is wider than secrets: it covers prompt and completion text, retrieved document
bodies, and any field value the code owning it has not declared non-sensitive. Permission to record
a value is opt-in per field — a denylist of sensitive-looking names cannot cover a field it has
never been told about. Record a salted hash, a length, a count, an ID, a status, or a version
instead. loom's own tool-call spans implement this in `internal/telemetry/tool.go`, with the
per-tool declaration expressed as the optional `tools.SafeArguments` interface.

**Related**: This is the Information Disclosure category of STRIDE (see `security-patterns.md`) applied
specifically to the observability pipeline — the same category that already flags "PII in OTel traces"
as a concrete example finding in `security-reviewer.md`'s own threat table. Observability and security
review overlap here on purpose: a trace that leaks a password is both an SRE finding and a security
finding.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
