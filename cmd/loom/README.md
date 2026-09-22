# loom CLI

The `loom` binary installs the framework's agents, skills, and rules across AI
coding platforms and serves the framework's MCP tools. Install it via
`brew install orieken/tap/loom` or
`go install github.com/orieken/loom/cmd/loom@latest`.

## Commands

| Command | Purpose |
|---|---|
| `loom install` (alias `loom init`) | Install framework content for detected platforms (`--target`, `--platform`, `--copy`, `--dry-run`, `--stack`, `--level` — see the maturity-level table in the root README; profiles are defined in `shared/levels.yaml`) |
| `loom health` | Verify installed configs match the canonical `shared/` source, and report the project's agentic maturity level (see below) |
| `loom tools status` / `loom tools install` | Report / install opt-in context tools |
| `loom mcp serve` | Serve the framework MCP tools over stdio |
| `loom run` | Execute the delivery pipeline for a feature spec (experimental — see below) |
| `loom uninstall` | Remove installed framework content |
| `loom update` | Update installed framework content |

## Running pipelines (experimental)

`loom run` is the executor skeleton decided by ADR-006 (loom executes
pipelines) and built by roadmap item M0.4. It runs the built-in linear
`deliver-feature` plan — the same 14-agent sequence as the markdown
pipeline — stage by stage, persisting durable state after every transition.

```bash
# Execute the pipeline for a spec (spawns `claude -p` per stage)
loom run --spec docs/features/user-auth/spec.md

# Continue an interrupted run from its checkpoint
loom run --spec docs/features/user-auth/spec.md --resume

# Deterministic dry run with canned artifacts (no LLM calls)
loom run --spec docs/features/user-auth/spec.md --provider mock
```

- **State**: `.claude/feature-workspace/<feature>/run-state.json` — schema-versioned,
  written atomically (temp file + rename), with per-stage status, timestamps, and
  the SHA-256 of each stage's artifact. A fresh run refuses to start over existing
  state (pass `--resume` or delete the file); `--resume` requires existing state.
- **Integrity on resume**: before the run loop trusts anything, every COMPLETED stage's artifact is
  re-hashed in Go and compared to the digest recorded when it finished. A changed or missing
  artifact demotes that stage to `STALE` — it re-runs — and the demotion cascades to every stage
  completed after it, whose own output came from content that no longer exists. The resume prints
  what it invalidated and why. There is no flag to skip verification (roadmap L2.12).
- **Interruption**: the first Ctrl-C cancels the in-flight stage and persists a
  clean `INTERRUPTED` checkpoint; a second Ctrl-C kills immediately. `--resume`
  skips completed stages and re-runs the interrupted one.
- **Providers**: `claude` (default) spawns the `claude` CLI headless per stage,
  building the prompt from the agent's `shared/agents/<agent>.md` definition; if
  the binary is missing the stage fails with a remediation message — there is no
  silent fallback. `mock` is for tests and dry runs.

- **Approval gates**: a gated stage does not start until a human approves its gate
  (roadmap L2.13). The built-in plan gates `developer` behind `confirm-design`,
  `qa-engineer` behind `confirm-security`, and `devops-engineer` behind
  `confirm-ship` — the same stops `deliver-feature` asks a human for. On a
  terminal you are asked `approve gate "X" for stage "Y"? [y/N]` at the barrier;
  otherwise the run persists `WAITING_APPROVAL`, prints the resume command, and
  exits with code **3** so scripts can tell "waiting on a human" from a failure:

  ```bash
  loom run --spec docs/features/user-auth/spec.md --resume --approve confirm-design
  ```

  `--approve` is only valid with `--resume`, and only for the gate the run is
  actually halted on. Nothing an agent returns can approve a gate.
- **Any edit resets the gate**: an approval binds to the SHA-256 of every artifact
  completed when it was given. Edit one of them and the approval is invalidated — the
  record is kept, naming what changed — and the run halts at that gate again until you
  approve the state as it now stands. A stage that re-runs to a byte-identical artifact
  keeps its approval; work completed *after* an approval belongs to the next gate.
  Approving a run whose own verification is about to reset that approval is refused up
  front, rather than recorded and immediately destroyed.

**What it does NOT do yet** (skeleton by design — see `docs/roadmaps/BUILD-ROADMAP.md`):
no `--from-phase` or rollback (L2.15), no retries or backoff, no parallelism
(L3.3), and no policy evaluation (L2.16).

## Recording markdown-pipeline checkpoints

`loom state` is how the `deliver-feature` skill records checkpoints without a model
computing its own integrity hashes (roadmap L2.12). The model keeps routing — conditional
skips, contract retry loops, the code-reviewer loop, none of which `loom run` can do yet —
while this binary reads each artifact and hashes it.

```bash
loom state record --spec docs/features/user-auth/spec.md --stage analyst   --artifact .claude/feature-workspace/user-auth/analysis.md
loom state verify --spec docs/features/user-auth/spec.md   # exits non-zero on any mismatch
loom state approve --spec docs/features/user-auth/spec.md --gate confirm-design
loom state show --spec docs/features/user-auth/spec.md [--json]
```

- No subcommand accepts a caller-supplied digest. A caller that could hand `loom` a hash
  could hand it a wrong one, which is the failure mode being closed.
- Stages carry a **sequence** assigned when first recorded and preserved when re-recorded, so
  a CHANGES REQUESTED loop keeps its place in the run and `verify` knows what is downstream of
  an edited artifact. The markdown pipeline has no fixed plan, so recording order is the only
  ordering available.
- `state approve` **records** a human's approval for audit; it does not enforce the gate.
  Enforcement exists only for stages run by `loom run` (see above). `state verify` does
  report an approval as INVALIDATED once a bound artifact changes, and exits non-zero —
  detection, which is strictly better than the nothing that came before it, and still not
  a barrier.
- `loom state timeline --spec <spec> [--json]` prints the run's event log (see below).
- The two pipelines refuse each other's state files: `loom run` will not resume a run recorded
  by `loom state`, and vice versa. They route differently, so resuming across them would replay
  the wrong work.

## Typed pipeline state

Seven artifacts of the built-in plan are **typed state** instead of markdown (roadmap L2.9):
`analysis`, `architecture`, `route`, `review`, `implementation-notes`, `security-report`, and
`qa-report`. Every hop between them carries fields, not prose.

- **Where**: `.claude/feature-workspace/<feature>/state/<stage>.json`. That document IS the
  stage's artifact, so digest verification and the staleness cascade cover it exactly as they
  cover any other artifact.
- **Schemas**: `shared/schemas/pipeline/*.schema.json`, generated from `internal/state/` by
  `go run ./cmd/gen-schemas`. Never hand-edit them; a test fails when they drift from the
  structs. The schema is inlined into the stage prompt, so a typed run does not depend on the
  framework being installed in the target project.
- **Markdown is a view**: `analysis.md`, `implementation-notes.md`, `qa-report.md` and the rest
  are *rendered* from state under the filenames every contract and downstream agent already
  expects — including the retrieval frontmatter. This is what keeps the typed hop invisible to
  the stages that are still untyped: `sre-engineer`, `devops-engineer` and `accessibility-engineer`
  read `implementation-notes.md` and cannot tell the difference. The view is derived and not
  digest-tracked: annotate it freely, nothing downstream reads it and nothing will demote a stage
  because you did.
- **Projections**: a consuming stage receives the fields its contract needs, not the whole
  upstream document. They are keyed by `(consuming stage, upstream kind)`, because what a stage
  reads and what it writes vary independently — the architect and the qa-engineer both read the
  analysis and get demonstrably different slices of it. A stage with several upstreams, like
  `qa-engineer` with three, gets one labelled block each, so provenance survives and same-named
  fields cannot silently collide.
- **Contract rules become invariants**: two things the contracts already asserted are now checked
  when state loads rather than grepped afterwards — a `qa-report` with a non-zero failed count,
  and a CRITICAL or HIGH security finding with no fix applied, are validation errors that fail the
  stage. A red suite cannot become completed state.
- **No LLM on the data path**: `qa-engineer` and `tech-writer` used to read a model-written summary
  of `analysis.md`. They now receive projections of it (roadmap L2.10).
- **Invalid output fails loudly**: a response that is not a single JSON object (raw, or one
  fenced block) fails the stage, as does one missing a required field or inventing a field the
  schema does not have. There is no repair or retry loop.

The eight remaining artifacts — the end-of-pipeline reports and `context-manifest` — still pass
markdown, unchanged. Nothing evaluates a condition over them yet.

## What past runs did

Every run records itself into a project-local store at `.claude/memory/episodes.db` when it ends —
completed, halted at a gate, interrupted, or failed (roadmap L3.5). Query it with `loom memory`:

```
loom memory runs                       # every recorded run: state, tokens, cost, corrections
loom memory retries --agent code-reviewer --more-than 2
loom memory corrections                # which agents a human had to correct most
loom memory agents                     # per-agent attempts, failures, corrections, latency, cost
loom memory policies                   # every policy decision, beside what the human did
loom memory ingest                     # rebuild the store from docs/features/
```

Add `--json` to any query for machine-readable output.

- **It collects nothing new.** Stage timings, loop rounds, gate halts, human corrections, token
  counts and routing reasons were already recorded per run; they just died with the feature
  workspace. This keeps them.
- **The store is a projection, not the record.** `run-state.json`, `run-events.jsonl` and the typed
  stage documents under `state/` are archived into `docs/features/<name>/` beside the artifacts they
  describe, so the history is in git. The typed documents matter beyond replay: they hold the review
  verdict, security findings, test results and changed paths a policy reads, and a run archived
  without them cannot answer those questions afterwards.
  `.claude/memory/` is gitignored, and `loom memory ingest` rebuilds it from the archive — deleting
  the database loses nothing.
- **Project-local, always.** Nothing is aggregated across projects and nothing is uploaded.
- **Recording never fails a run.** If the store cannot be written, the run says so and carries on.

## Policies: what would have been decided

`loom run` loads `.claude/policies/*.policy.yaml`, evaluates every policy watching a gate against
the run's own state, and prints what they decided (roadmap L2.16):

```
Policy at gate "confirm-design": require-human → stops for a human (matched: require-human-on-critical-findings)
  require-human-on-critical-findings  TRUE     require-human
  auto-approve-refactor               UNKNOWN  auto-approve  (unknown: diffLines, fitnessFunction.allPass)
```

**Nothing is auto-approved.** A matching `auto-approve` policy changes nothing about whether you are
asked — the run halts at every gate exactly as it did before, and the record says what *would* have
happened. That is deliberate: the first run to skip a barrier should not also be the first evidence
the evaluator decides what a human would. Honouring a decision is roadmap L2.19.

- **`UNKNOWN` is not a failure.** A policy asking about a fact the run cannot answer resolves to
  unknown, never to true, and the decision names which fact it could not see — more useful than a
  bare "no match". Since L2.20 every field the vocabulary declares resolves from run state: three
  that never could (`diffType`, `dryRunPass`, `fitnessFunction.allPass`) were removed rather than
  left in, and a policy naming one now fails to load. Unknown still happens — most often at
  `confirm-design`, which precedes the review, security and QA stages whose facts a policy asks
  about.
- **Some gates cannot be governed at all.** Shipping, migrations, contract-phase drops, external API
  mutations and deploys are always human — that list is compiled in, and a policy naming one fails
  to load rather than being ignored.
- **`--dry-run-policies`** evaluates against a finished run and prints the decisions without
  touching anything: `loom run --spec <file> --dry-run-policies`.
- **Off switch**: `policiesEnabled: false` in `.claude/delivery-policy.yaml` skips evaluation
  entirely.

Decisions are also on the timeline as `policy.evaluated` and in `run-state.json`, so they survive
the terminal scrollback — and `loom memory policies` reads them back across every recorded run,
beside the approval the human gave at the same gate:

```
FEATURE                GATE               EFFECT         AGREEMENT  POLICIES
user-auth              confirm-security   auto-approve   agreed     auto-approve-refactor=TRUE …
```

That corpus is what L2.19's stop condition is a judgement about, so it is worth knowing what it
does *not* establish. "Agreed" means a policy asked to auto-approve and a human then approved the
same gate — the human could see the decision, so it is concurrence rather than an independent
second opinion. And `honoured` stays 0 until L2.19 ships; a non-zero figure there means something
skipped a barrier, which is a defect and not progress.

## Editing an artifact at a gate

When a run halts and you disagree with what an agent produced, there are two
different things you can do to the files in the workspace, with two different
consequences. Knowing which is which matters, because one of them throws your
edit away.

**Annotate the rendered view** — `analysis.md`, `qa-report.md`, and the other
markdown files for typed stages. This is the recommended way to say "this should
have said X". The executor never reads these files back: they are rendered from
`state/<stage>.json`, so your edit cannot corrupt the run and nothing goes stale.
On approval the change is recorded as a **human correction**, attributed to the
agent that produced it, with a unified diff retained under
`.approved/<gate>/corrections/` (roadmap L4.5).

It is **advisory**. The pipeline does not adopt what you wrote, and the next time
that stage runs the view is re-rendered over your text. What survives is the
record — which is the point: it is a labelled example of an agent getting
something wrong, which is the signal the framework has always specified and never
collected.

**Editing a tracked artifact is a different act.** For a stage that still writes
markdown, and for `state/<stage>.json` on a typed one, the file IS what integrity
tracks. Changing it makes the stage STALE, so the executor **re-runs it** and the
agent's fresh output replaces what you wrote (L2.12). Any approval bound to that
artifact resets (L2.14). Your edit is still captured as a correction before the
re-run, but do not expect the text itself to survive — that path means "do this
again", not "use my version".

Neither is a way to hand-write an artifact the pipeline will use. If an agent's
output is wrong enough that you want to replace it outright, the honest move is to
fix the spec or the agent and re-run.

Corrections show up in `loom state show` and on the timeline:

```
  corrected by a human:
    analyst                  +4/-1 at confirm-design — .../corrections/analyst.diff
```

## Telemetry: what a run cost

Every run emits a **Run Trace** (roadmap L3.8): one root span for the run, a child per
stage, and a grandchild per model invocation.

- **Where, by default**: `.claude/feature-workspace/<feature>/traces.jsonl`, beside
  `run-state.json` and `run-events.jsonl`. Each line is a complete OTLP/JSON request
  body, so a saved file can be POSTed to a collector unmodified. `--otel-file <path>`
  moves it; `--no-telemetry` records nothing.
- **Over the network**: set `OTEL_EXPORTER_OTLP_ENDPOINT` and traces also go to that
  collector over OTLP/HTTP. The endpoint is opt-in and the file is not, because what
  makes a phone-home default rude is egress — a file beside the run's own state has
  none, and a trace that only exists when someone predicted they would want it cannot
  answer a question about a run that already finished.
- **Cost, without any of the above**: token counts and dollars are recorded on each
  stage in run state, so the run summary and `loom state show` report them with no
  collector configured:

```
usage: 48120 in / 9330 out tokens (31002 cache read, 2048 cache write) — $1.8730
```

  That is **Reported Usage**: the claude CLI says what it was charged and loom records
  it verbatim. A price table in this repository would be wrong within a quarter while
  sounding authoritative. Note that absent Reported Usage is not zero — a provider that
  reported nothing is a different fact from one that reported zero, and the run summary
  stays silent rather than printing a total of $0.00.

- **Tool calls**: `loom mcp serve` traces each tool call, exporting only when an OTLP
  endpoint is set — it is spawned by a host application and may outlive many runs, so
  there is no run to scope a file to. Arguments are recorded with secret-shaped names
  redacted and every value length-capped. A tool call joins its stage's trace when the
  `TRACEPARENT` environment variable survives the hop from `loom run` through the
  claude CLI, and starts a clean trace of its own when it does not — loom does not
  spawn that server, and MCP carries no trace context, so this is best-effort by
  construction.

Nothing about a run depends on any of it. With no exporter and no endpoint, tracing is
a no-op and the run behaves identically.

## Routing: which stages actually run

Immediately after the analyst — the earliest point the facts exist — the executor
computes the **Delivery Route** from the typed analysis and records it as
`route.md` plus `state/router.json` (roadmap L3.0). The run prints a one-line
summary and halts at the design gate:

```
routed 7 of 12 stages — skipped architect, performance-engineer, data-engineer,
  accessibility-engineer, devops-engineer (see .../route.md)
Halted at gate "confirm-design" before stage "developer" — approval required.
```

- **Predicates, not prose**: a context crossing or migration or new dependency or
  performance threshold summons the architect; a threshold summons performance; an
  expand/contract migration summons data; an accessibility requirement (which the
  analysis contract makes mandatory for any UI) summons accessibility; a non-empty
  DevOps task list summons devops. Each decision records its reason.
- **The route is approved with the design.** It completes before `confirm-design`,
  so the approval binds its digest: edit `route.md`'s source to force a stage back
  in and the gate resets, exactly as editing any other approved artifact does.
- **Two stages are never routed around.** `code-reviewer` and `security-reviewer`
  always run — an unnecessary review wastes an invocation, a skipped one does not
  fail so cheaply. `visual-qa-engineer` also always runs: its condition is about
  the environment, not the feature (see ADR-007).
- **A gate survives its stage being skipped.** `confirm-ship` guards
  `devops-engineer`; routing devops out still stops at the gate, because the
  checkpoint is about reaching that point in the run.
- `loom state show` lists what was routed out and why.

## The review loop

`developer → code-reviewer` is a **bounded loop** declared in plan data (roadmap
L2.17): the executor reads the reviewer's typed verdict, sends the developer back
when it asks for changes, and counts the rounds.

```
review loop: changes requested — round 2 of 3, re-running from developer
```

- **The condition is a field, not a grep.** `review-approved` reads
  `ReviewState.Verdict`. The rendered `code-review-report.md` still carries the
  bolded literal for the markdown pipeline, but nothing parses prose to decide
  whether to loop.
- **Every round is retained.** `.iterations/<stage>.<n>.<ext>`, each with its own
  digest recorded in run state — so what round two actually said is verifiable,
  not merely remembered. `loom state show` reports the round count.
- **The bound is three.** The markdown pipeline's version of this loop was
  unbounded ("repeat until APPROVED"); the executor's has a number, because a
  loop it cannot bound is one it cannot safely run.
- **Exhausting the bound halts at `confirm-unresolved-review`.** Not a failure
  and not a silent pass: a human accepts the outstanding findings or stops the
  run, and that approval binds the artifacts like any other.
- A gate *before* the loop is approved once and does not re-halt each round.

## Run event timeline

Both pipelines append to `.claude/feature-workspace/<feature>/run-events.jsonl` — one JSON
object per line, written by this binary with timestamps taken from the clock at the moment
each transition happens. Stage durations are therefore *measured* by subtracting two
timestamps, not estimated by a model after the fact.

```bash
loom state timeline --spec docs/features/user-auth/spec.md
#  0s  run.started
#  0s  stage.started        analyst
#  4s  stage.completed      analyst
#  4s  gate.waiting         developer confirm-design
# 2m1s gate.approved        confirm-design tty
# 2m1s run.resumed
```

`run.started` fires once per run and `run.resumed` once per continuation, so a run halted at
three gates records one start and three resumes — "how long did this run take" has one answer
(roadmap L3.16). Until that shipped, re-entering `Run` after each gate emitted a second
`run.started`, and every consumer of the log had to know to ignore all but the first.

The full list of recorded kinds is `shared/schemas/telemetry/run-event-types.md`, which is
**generated** from the Go enum — this README deliberately does not restate it, because the
hand-maintained copy that used to sit here had fallen nine kinds behind.

- **Append-only.** The file is never rewritten or truncated; each event is one write. A
  process killed mid-write can leave a torn final line, which readers skip rather than
  failing on.
- **Its event types are generated.** One Go enum is the source of truth; the JSON Schema and the
  table of types under `shared/schemas/telemetry/` come from it via `go run ./cmd/gen-schemas`, and
  a test fails when they drift. Adding a kind without documenting it fails the build (roadmap L3.9).
- **Not the trace.** This is the audit log — gates, digests, staleness — and it stays readable
  with no collector configured and no exporter running. The run's OpenTelemetry trace, in
  `traces.jsonl` beside it, answers a different question: how long each stage took and what it
  cost. Neither replaces `pipeline-trace.json`, whose `budgetUtilization` and iteration counts
  are still model-written estimates.

## Maturity level report

`loom health` ends with an agentic-maturity assessment driven by
`shared/levels.yaml`: it reports the highest level whose *entire* mechanical
evidence set passes, the passing evidence, and a concrete gap checklist for
the next level. Evidence is strictly mechanical — installed bundles on disk,
an MCP server that is configured in `.mcp.json` **and** actually answers
`tools/list` when spawned, a non-empty telemetry stream, policy files present.
Documentation-only bundles (`docsOnly` in `levels.yaml`) never count as
evidence, and a level whose enforcement bundles are all gated on unlanded
roadmap items is reported as not attainable yet rather than pretended into
reach.

```
Agentic maturity (shared/levels.yaml):
  Level 1 — Foundational prompts
  ✓ bundle "core-rules" installed
  ✓ bundle "agents" installed
  gaps to Level 2:
    ✗ MCP server not configured: read .mcp.json: no such file
    ✗ bundle "workflows" not fully installed (missing workflows)
```

## MCP server

`loom mcp serve` exposes the deterministic framework tools
(`analyze_complexity`, `check_accessibility`, `check_ubiquitous_language`,
`verify_dependencies`, `search_ki`, `search_docs`, `validate_artifact`)
over MCP stdio transport.
Structured JSON logs go to stderr, or to a file with `--log-file <path>` —
never stdout, which carries the MCP wire protocol. The server runs until the
client closes stdin or the process receives SIGINT.

Register it with Claude Code:

```bash
claude mcp add loom -- loom mcp serve
```

Or add it to `.mcp.json` (project) / `~/.claude/mcp.json` (global):

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

`AI_ASSISTANT_DOTFILES_PATH` is only required for `search_ki` (it points the
tool at the Knowledge Item corpus); the other five tools need no
configuration. See [shared/mcp/README.md](../../shared/mcp/README.md) for the
full tool reference and environment variables.
