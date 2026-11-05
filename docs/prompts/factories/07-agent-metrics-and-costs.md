# Measure what your agents cost and how well they work

A prompt for adding cost and quality metrics to a multi-agent pipeline, written for teams who want
to see how loom does it, or to add the same thing to their own project without adopting loom.

**Scope: add the measurements, then answer three questions with them**: what a run costs, which
agent costs the most, and which agent's output humans correct the most. A team that cannot answer
those after this prompt has built dashboards, not measurement.

## Two ways in

- **You run loom** (`loom run` or `loom factory`): everything below is already on. Skip to "Using
  it" for the commands.
- **You have your own pipeline** (scripts calling `claude -p`, the Agent SDK, or the API directly):
  build the levels below in order. Each one is useful on its own, and each takes an afternoon to a
  few days.

## Reference implementation (read first if you can)

In `github.com/orieken/loom` (ai-assistant-dot-files):

- `internal/provider/claude/envelope.go`: reading cost and tokens from the CLI's JSON result
- `internal/orchestrator/usage.go`: the usage record, and why it is never computed
- `internal/orchestrator/budget.go`, `unmeasured.go`: the budget governor, and unknown spend
- `internal/orchestrator/timeline.go`, `correction.go`: the event log and the human-correction signal
- `internal/orchestrator/tracing.go`, `internal/telemetry/`: OpenTelemetry traces
- `internal/memory/schema.go`, `query_agents.go`: the queryable store and per-agent metrics
- `cmd/loom/README.md`: "Telemetry: what a run cost" and "What past runs did"

## The rules (each one is a mistake loom made or nearly made)

1. **Record what the provider reports. Never compute a price.** `claude -p --output-format json`
   returns `total_cost_usd` and a `usage` block, and the Agent SDK's result message carries the same
   fields. Record them verbatim. A price table in your repository is wrong within a quarter and
   produces a confident number nobody was billed. If you call the API directly, it returns tokens
   but no dollars: keep the tokens as the truth, and label any dollar figure you derive *estimated*,
   with the price table's date.
2. **Unknown is not zero.** A stage that reported nothing (killed by a timeout, cut off by a usage
   limit, run by a mock) has *unknown* cost. Store it as absent, not `0`. Totals say "≥ $X, N stages
   unmeasured", never a smaller number that looks complete.
3. **Never let a model report its own metrics.** Durations, costs and retry counts come from your
   code's clock and the provider's envelope. loom's first pipeline had agents write their own
   timings into `pipeline-trace.json`; those numbers were estimates presented as measurements, and
   were retired.
4. **Measure where it happens, record where it lasts.** Measure at the call. Keep the record next
   to the run's own state, and archive it with the run's other outputs (in git), so history survives
   a deleted workspace.
5. **Telemetry never fails a run.** If a metric cannot be written, say so and carry on.
6. **Local by default, network by opt-in.** Write files beside the run. Send to a collector only
   when someone sets an endpoint.
7. **Record properties, not payloads.** Never record prompt or completion text, command output,
   retrieved documents, user identities, credentials, or a tool argument's value. Record a salted
   hash, a length, a count, a status instead. An unsalted hash of short input can be brute-forced.
   Recording any value is opted into per field by the code that owns it; a denylist of
   secret-looking names misses the field nobody told it about. (loom's
   `shared/rules/architecture-guardrails.md`, #9.)

## Level 1: cost per call and per stage

For every model invocation, record this from the provider's result (loom's shape, from a real run):

```json
{
  "stage": "acceptance-scenarios",
  "agent": "qa-engineer",
  "startedAt": "2026-10-27T04:17:41.247868Z",
  "finishedAt": "2026-10-27T04:18:17.531786Z",
  "usage": {
    "model": "claude-sonnet-5",
    "inputTokens": 10,
    "outputTokens": 7079,
    "cacheReadTokens": 278100,
    "cacheCreationTokens": 118759,
    "costUsd": 0.601466,
    "finishReason": "end_turn"
  }
}
```

- `model` is what *served* the request, which is not always what was asked for.
- Keep cache reads and cache writes separate from fresh input: they price differently, and they
  are most of your input (below).
- `finishReason` tells a completed answer from one cut off at `max_tokens`. The text alone cannot.
- Keep a run-level total alongside the per-stage records, added to as each call finishes, so the
  total survives a stage record being rewritten on retry.
- Retries add to the same stage's usage; they do not replace it. A stage that took three attempts
  cost three attempts.

**Done when** a finished run prints one line like
`usage: 48120 in / 9330 out tokens (31002 cache read, 2048 cache write) — $1.8730`, and prints nothing
(not `$0.00`) when nothing was reported.

## Level 2: budgets that stop a run

- Give each run ceilings: dollars, tokens, and hours. **Hours means time spent running stages**, not
  the run's age: a run waiting three days for a human has not spent three days.
- Check the ceilings **before** each stage, not after. Warn at 80%; at 100%, checkpoint and stop
  cleanly, resumable with a larger budget.
- If any finished stage is unmeasured, a dollar or token ceiling cannot be verified, so **stop**.
  Let a named human accept the unknown, record who and when, keep that stage marked unmeasured in
  every report, and check the ceiling against what was measured. (Without the acceptance step this
  traps a run forever: loom hit it when a session limit cut a stage off mid-run.)
- Give every stage its own timeout too. loom's is 30 minutes, which turned out too short for
  building a container image under emulation; make it configurable per stage.

**Done when** a run with `maxUsd` below its real cost stops before the stage that would cross it,
and says which ceiling and what each stage spent.

## Level 3: an event log, and the quality signal

Append one JSON line per event to `run-events.jsonl`, with a timestamp from your clock. A small, fixed
vocabulary beats free text, because you will query it:

`run.started` `run.resumed` `run.completed` · `stage.started` `stage.completed` `stage.failed`
`stage.skipped` `stage.interrupted` `stage.stale` · `gate.waiting` `gate.approved` `gate.invalidated`
· `loop.iterated` `loop.exhausted` · `policy.evaluated` · `artifact.corrected`

```json
{"at":"2026-10-25T05:49:43.218684Z","kind":"stage.started","stage":"context-engineer","sequence":1}
{"at":"2026-10-26T00:22:18.146343Z","kind":"gate.approved","gate":"confirm-design","approvalMethod":"flag"}
```

**The quality signal is `artifact.corrected`, and it matters more than cost.** At each human gate:

1. Record the digest of what the human was shown.
2. When they approve, compare it with what is on disk now.
3. If it changed, the human corrected the agent: record `artifact.corrected` with the producing
   agent, the gate, lines added and removed, and a stored diff.

Corrections per stage execution, per agent, is the closest thing a pipeline has to an accuracy
measure. An agent whose output is approved unchanged is doing its job; one that is rewritten at every
gate is costing human time that no token count shows.

**Done when** editing an artifact at a gate and approving it produces an `artifact.corrected` line
naming the agent, with the diff saved.

## Level 4: traces and a queryable store

**Traces** (OpenTelemetry):

- One root span per run, a child per stage, a grandchild per model call.
- Use the GenAI semantic-convention attributes, so standard tools read them:
  `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.response.model`,
  `gen_ai.response.finish_reasons`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`.
  Put what the convention lacks under your own prefix (loom uses `loom.usage.cost_usd`,
  `loom.usage.cache_read_tokens`, `loom.stage.id`, `loom.stage.agent`, `loom.stage.iteration`).
- Write OTLP/JSON request bodies, one per line, to `traces.jsonl` beside the run. Each line can be
  POSTed to a collector unchanged later. Export over the network only when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set.
- Pass `TRACEPARENT` to subprocesses, so a tool call can join its stage's trace. Treat it as
  best-effort: not every hop carries it.
- Tool-call spans record argument names, lengths and salted hashes, never raw values (rule 7).

**Store**: a SQLite file rebuilt from the archived run files, never the source of truth. loom's tables:
`runs`, `stages`, `events`, `corrections`, `policy_decisions`, `policy_outcomes`. Because it is a
projection of what is in git, deleting it loses nothing, and a rebuild command restores it.

**Done when** you can answer, with one query each: cost per run over time; cost and attempts per
agent; correction rate per agent; and which stages retried more than twice.

## Using it (loom)

```bash
loom state show --spec <spec>          # one run: stages, status, tokens, cost
loom factory inbox                     # per-stage spend for anything waiting or parked
loom memory runs                       # every run: state, tokens, cost, corrections
loom memory agents                     # per agent: attempts, failures, corrections, latency, cost
loom memory corrections                # which agents humans corrected most
loom memory retries --agent code-reviewer --more-than 2
loom memory ingest                     # rebuild the store from docs/features/
```

Add `--json` to any `loom memory` query for machine-readable output.

## What the numbers already showed

From loom's six shipped orders (October 2026, Claude Sonnet, small Go changes):

- **$5–$14 per order.** The most expensive was the one that changed the most files.
- **92–97% of input tokens are prompt-cache reads.** Cost is driven by how much context each stage
  re-reads, not by how much it writes. Pruning context is the biggest lever.
- **The planning agents cost more than the coding agent.** Average cost per stage: analyst $1.72,
  context-engineer $1.47, developer $1.33, code-reviewer $0.96, qa-engineer $0.76, security-reviewer
  $0.55, tech-writer $0.51. That is the opposite of what most teams would guess, and only measurement
  shows it.
- **Every failure was plumbing, not reasoning.** Malformed output, schema mismatches, a sandbox
  block, a session limit, unreported spend. The event log and the unmeasured-spend rule are what
  made those visible instead of silent.

Your numbers will differ. The point is to have them.

## What not to build first

- A dashboard. Answer the three questions with a query first; chart what you find yourself
  querying weekly.
- Cross-project aggregation. Keep metrics project-local until a team asks for a roll-up, and
  agree who can see it before building it.
- Alerts on cost. A budget that stops the run is the alert.
