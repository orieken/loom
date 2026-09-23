# Framework Meta-Patterns

Patterns novel to (or specific to) this framework itself, not general software patterns applied to it.
Different audience from the rest of `docs/patterns/`: these are for anyone building a similar
multi-agent orchestration system — future v3 work included — not for teams *using* this framework day
to day. Kept in a separate file so the main catalog stays scoped to general software patterns and
doesn't get diluted by mechanisms specific to this repo.

Two entries today. More may follow as v3 crystallizes; the file exists so those additions have a home
that doesn't require restructuring the general catalog.

## Pipeline / Orchestration Pattern

**Context**: When work is too large for a single agent's context window, too specialist for one agent
to do all of it well (code review + security review + accessibility review require genuinely different
lenses), and too irreversible in places to run without human checkpoints. `deliver-feature`'s
14-agent sequence is this framework's implementation of the pattern; `review-pr`, `backfill-unit-tests`,
and `deliver-atdd` are thinner variants coordinating 2-3 agents each.

**Structure**: Six mechanisms compose the pattern. Each is optional in isolation but load-bearing
together:
1. **Sequential specialist stages** — each stage owned by exactly one agent, no shared responsibility.
2. **Artifact-based handoff** — each stage writes a markdown artifact; the next stage reads it. No
   shared in-memory state between agents. This is the ONE mechanism the framework refuses to relax —
   see `docs/AGENT_REFERENCE.md`'s "subagent isolation is a hard boundary" and the KI of the same name.
3. **Structural contract validation between stages** — every contract-bound handoff runs
   `validate-artifact` against `shared/contracts/<agent>-contract.md` before the next stage can start
   (see Contract-First Agent Handoff below).
4. **Human checkpoints at irreversible steps** — `approval-gates.md` names the 8 real ones (git
   commit, DB migration contract phase, external POST, deploy, etc.); the orchestrator pauses at each.
5. **Checkpointed state for resumability** — `pipeline-state.json` records completed stages, artifact
   checksums, and contract status. `resume-pipeline` reads it to continue interrupted runs without
   restarting from scratch.
6. **Trace log for cross-run learning** — `pipeline-trace.json` records per-stage timing/status/retry
   counts, consumed by `pipeline-retrospective` and `agent-scorecard` to catch degrading agents and
   bottlenecks across many runs. Distinct from state (which is for resuming one run).

**Example**: `deliver-feature` runs 14 sequential stages: `context-engineer` → `analyst` → conditional
(`architect`, `performance-engineer`, `data-engineer`) → `developer` → `code-reviewer` → conditional
(`accessibility-engineer`, `security-reviewer`) → `qa-engineer` → `sre-engineer` → `tech-writer` →
`devops-engineer`. Each writes a named artifact; each contract-bound handoff runs `validate-artifact`.
Human checkpoints at analyst scope-confirmation, architect RFC, security-Critical resolution, and pre-
ship. State + trace files written at every checkpoint.

**When to use vs. skip**: The pattern shines for work with real specialist axes (design + review +
security + QA + docs + deploy — five to seven distinct lenses) and real irreversibility (commits,
migrations, deploys). Skip for single-axis work — a coverage backfill goes through
`backfill-unit-tests` (2 stages, one auto-chained), not the full pipeline, and a small change a single
`developer` pass can carry — with its own unit tests — needs no pipeline either.

**Trade-offs**: The ceremony has a real fixed cost — every stage's contract file, every checkpoint,
every artifact-handoff write. Worth it precisely when the work is complex enough to overflow a single
agent's context or judgment. Not worth it for anything you'd hand to a solo `developer` or
`review-pr` and get a good answer in one pass. `docs/features/context-engineering-framework/TODO.md`
Epic 24 explicitly documents which agents run in the pipeline vs. standalone, and why the two
mechanisms both exist rather than one subsuming the other.

**Related**: Contract-First Agent Handoff (the sub-pattern below governing every artifact handoff).
Gang-of-Four Chain of Responsibility is a structural cousin but simpler — CoR stages don't have
contracts, human checkpoints, or resumable state. The framework's own `deliver-feature`/`review-pr`/
`backfill-unit-tests`/`deliver-atdd` are four different scales of this pattern.

## Contract-First Agent Handoff

**Context**: When agent A produces an artifact for agent B to consume, both need to agree on the
artifact's shape without either coupling to the other's prompt. Otherwise: A's prompt drifts, A produces
a subtly different artifact, B silently receives something malformed, and downstream failure is
diagnosed as B's fault when actually A produced the wrong shape. This failure mode is documented in
`docs/features/context-engineering-framework/TODO.md` Epic 5 as the reason the contract layer was
built at all — it happened in practice before contracts existed.

**Structure**:
1. **A markdown contract file** — `shared/contracts/<agent>-contract.md` — declares the required
   sections and any content-level rules (e.g., "Acceptance Criteria section must list at least one
   criterion") the artifact must satisfy.
2. **A validator** — `validate-artifact` skill — reads the contract and checks a given artifact
   against it, returning PASS/FAIL with specific violation locations.
3. **The orchestrator invokes the validator** between every contract-bound handoff. On FAIL, the
   artifact goes back to its producing agent with the violations named; on PASS, the next stage
   starts.
4. **No agent-to-agent implicit dependency** — the contract IS the interface. Agent A's prompt can
   be edited freely as long as its output still satisfies its contract; agent B never needs to know
   A's prompt changed.

**Example**: `analyst` produces `analysis.md`, structured per `shared/contracts/analysis-contract.md`
(Summary, Acceptance Criteria, Bounded Context, Data Model Changes, API Changes, Performance SLAs,
etc.). `deliver-feature` runs `validate-artifact` before invoking `architect`. If the analysis is
missing a Bounded Context section, `validate-artifact` reports the specific gap and sends the
artifact back to `analyst` with that violation named. `architect` never runs against an incomplete
analysis.

**Why not just runtime types (Zod, Pydantic, JSON Schema)**: LLM outputs are prose, not structured
data. Zod validates types; a contract validates that a prose document contains a section titled
"Bounded Context" with non-trivial content underneath, which is a fundamentally different check.
Runtime type systems can't express that requirement; a markdown contract can. The two mechanisms
compose — Sunday's Model uses Zod for HTTP payload shape, this pattern uses markdown contracts for
LLM artifact shape.

**Trade-offs**: One contract file per pipeline agent (14 in the current setup). Contracts drift too —
a contract that goes stale as its agent's output evolves is worse than no contract, because failures
become confusing rather than absent. `check-parity.sh` and `health-check.sh` include contract-existence
checks; content-drift checks are harder to automate and rely on the same human review that catches
prompt drift itself. Worth it because the failure mode this pattern prevents (silent downstream
breakage from upstream prompt drift) is real and expensive when it happens — Epic 5 exists precisely
because it happened before the contract layer was built.

**Novel to this framework** — no direct prior art found in the multi-agent orchestration literature
as of the frameworks surveyed while building this. The closest analogs are:
- **API contracts** (OpenAPI, Pact) — for HTTP boundaries, not LLM output.
- **Interface segregation in OOP** — semantically similar (both are "define the boundary; caller and
  implementer both conform"), but not for LLM output.
- **Pipeline stage schemas in ML/data engineering** — for structured data records, not prose.

The gap this pattern fills — a lightweight, prose-shaped, LLM-parseable contract between agent stages
— didn't have an established name. Naming it here gives v3 work a canonical reference to point at
when the same mechanism reappears in different orchestration shapes.

**Related**: Pipeline / Orchestration Pattern (this is a sub-mechanism within it), `validate-artifact`
skill, `shared/contracts/` directory. The KI `subagent-isolation-is-a-hard-boundary`
(`shared/knowledge/`) is the philosophical underpinning — contracts exist because subagents can't see
each other's context, so the artifact must be self-sufficient.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*

## Preconditions That Check Themselves

**Context**: A comment states a condition the code depends on — "nothing in this package emits a
slice attribute", "measured size was 22,258 bytes", "five of the nine fields have no source". The
condition is true when written, the code moves, and nothing notices. The comment now misinforms
every reader with the authority of something that was once verified.

This happened four times in one week in this repository. In each case the prose was accurate when
written and load-bearing when it rotted: a fallback that was safe *because* nothing emitted slices,
a ceiling justified by a measurement that had drifted 5.5KB, a mutation procedure that said to
confirm the test fails but never to confirm the mutation applied.

**Structure**: a load-bearing claim names what holds it, in a fixed form:

```go
// PRECONDITION: nothing in this package emits a slice attribute.
// ENFORCED-BY: TestOnlyScalarAttributesAreEmitted
```

`ENFORCED-BY` names a Go test function, a script, or the literal `judgment-only` with a reason:

```go
// PRECONDITION: the envelope's field names match the live CLI.
// ENFORCED-BY: judgment-only — verified against one captured response (L3.15), and one
// response is not a contract.
```

`scripts/health-check.sh` fails when a `PRECONDITION` has no `ENFORCED-BY`, and when the named
enforcer does not exist. That is the whole check.

**Trade-offs**: it catches nothing nobody marked, and every case that has bitten this repository was
unmarked. It does not detect stale prose; it makes a *deliberately marked* precondition impossible
to leave unenforced, and makes an unenforceable one say so. The bet is the same one the exemplar
annotation makes — a convention that costs one line at the moment of writing, when the author still
knows whether the claim is load-bearing.

**Related**: `architecture-guardrails.md` #7 requires every structural *decision* to produce a
fitness function or be flagged judgment-only. This is the same rule applied to a *claim*, which is
the form a decision takes once it has been written down and forgotten.
