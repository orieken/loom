# The spec factory — `deliver-spec`

Turns a problem statement into a **spec package**: acceptance criteria an SME has signed, Gherkin
scenarios a QA lead has signed, and draft work orders for the build and test factories. It writes no
product code and no test automation. Read `00-factory-blueprint.md` and `01-shared-intake.md` first.

**Scope: DESIGN + plan definition + agent and contract drafts.** Implementation follows the phased
plan this prompt produces.

## Target repo

`/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files` (the plan, agents and contracts ship
with the framework). Output of a run lands in the order's `target` repo.

## Prior context (read first)

- `shared/agents/analyst.md` — step 6 runs "Event Storming Lite internally". That is the model
  talking to itself, which is what this factory replaces with an SME.
- `shared/skills/event-storm/SKILL.md`, `shared/skills/five-whys/SKILL.md` — both are interactive by
  design ("Wait. List them. Don't suggest yet."). They cannot run unattended as written.
- `shared/agents/spec-writer.md` — the readiness critique; this factory's counter agent starts there
- `shared/agents/product-owner.md`, `shared/agents/qa-engineer.md` (the blind-scenario job)
- `DOMAIN_DICTIONARY.md`, `docs/patterns/testing-pyramid.md`
- `saturday-mcp/internal/tools/check_ubiquitous_language_tool.go` in
  `/Users/oscarrieken/Projects/Rieken/saturday-monorepo` — a deterministic language check to reuse

## The honest premise

A model cannot write *real* acceptance criteria. It writes plausible ones. They become real when the
person who owns the rule says so. This factory's value is therefore:

1. it does the mechanical work (structure, traceability, Gherkin, the language check) so the SME
   spends their time on the rules, not the format;
2. it makes the SME's gap visible — every rule without a negative example, every hotspot, every
   question — instead of hiding it in fluent prose;
3. it records what the SME corrected, which is the factory's quality metric.

Design every stage to make the SME's review faster and more pointed, not to replace it.

## Techniques, and what each is for

| Technique | Question it answers | Used in stage |
|---|---|---|
| Five Whys, as goal laddering | Why does the requester want this? What outcome would make it done? | `discovery` |
| Impact Mapping (Adzic) | Who must change behaviour, and what deliverable causes that? | `discovery` |
| Event Storming (Brandolini) | What happens in the domain: events, commands, aggregates, policies, read models, hotspots | `domain-model` |
| Example Mapping (Wynne) | For each rule: which concrete examples, and which questions remain? | `example-map` |
| Specification by Example | Turn examples into criteria and scenarios that read as business language | `acceptance-criteria`, `scenarios` |

Five Whys is a root-cause tool. Used here only to climb from the requested feature to the outcome
behind it; say so in the agent prompt so it does not drift into incident analysis. **Example
Mapping is the bridge** from an event storm to acceptance criteria and is the stage most likely to
find the missing rule — do not skip it.

Each technique needs an **async variant** of its skill: the agent drafts the board, marks every
hotspot and question explicitly, and the SME corrects at the gate. Keep the interactive skills for
when a human runs a live workshop; a `mode: workshop` order runs them synchronously and the factory
only packages the result.

## Plan

```
context-engineer
  → discovery            (goal ladder + impact map + open questions)       gate: confirm-problem    — requester
  → domain-model         (event storm draft with hotspots)                 gate: confirm-domain     — domain SME
  → example-map          (rules → examples → questions)
  → [halt if any question is marked blocking]                              gate: answer-questions   — domain SME
  → acceptance-criteria  (Given/When/Then per example, traced to rule + event)
  ⟲ spec-critic          (bounded loop, 3 rounds)
  → scenarios            (declarative Gherkin, tagged by level and runner) gate: confirm-scenarios  — QA lead
  → package              (internal)                                        gate: confirm-ship       — requester
```

Typed state per stage. The schemas to design:

- `discovery`: `goal`, `outcomes[]` (measurable), `actors[]`, `impacts[]`, `questions[]`.
- `domain-model`: `events[]` (`NounPastVerb`), `commands[]` (→ event, issued by actor), `aggregates[]`,
  `policies[]` ("when E then C"), `readModels[]`, `boundedContexts[]`, `hotspots[]` (each a question).
- `example-map`: `rules[]`, each with `examples[]` (≥1 happy, ≥1 negative) and `questions[]`
  (`blocking: bool`, `assumption`).
- `acceptance-criteria`: `criteria[]`, each with `id`, `given/when/then`, `rule`, `events[]`,
  `observableVia: ui | api | both`.
- `scenarios`: `features[]` (file path, Gherkin text), each scenario tagged `@AC-<id>`,
  `@level-<api|ui>`, `@runner-<cucumber|playwright>`.

## The counter agent: `spec-critic`

Deterministic checks first, in Go, before any model judges anything:

- every rule has at least one negative example;
- every criterion traces to a rule and at least one event; every rule is covered by a criterion;
- every `Then` names an observable outcome (no "the system processes…", no internal state);
- no implementation detail in Gherkin: no selectors, URLs, HTTP verbs, field names, clicks;
- every term in a scenario is in `DOMAIN_DICTIONARY.md` or the order's Domain Language;
- `gherkin-lint` passes.

Then the model critique, using `spec-writer`'s readiness rules. Findings go back to
`acceptance-criteria`; three rounds, then `confirm-unresolved-review`.

## Routing scenarios to the test factories

Decide each scenario's level from what makes its outcome observable — not from what is easiest to
automate. Rule to encode, from `testing-pyramid.md`: if the outcome is observable through the API,
the scenario is an API test (Sunday); it is a UI test (Saturday) only when the behaviour *is* the UI.
A scenario tagged `@level-ui` must say in a comment what the API cannot show. Runner choice inside
Saturday (`cucumber` vs `playwright`) is the target repo's decision, read from its config — not
guessed per scenario.

## The package (what `publishes: [spec-package]` means)

Opened as a PR into the target repo, never merged by the factory:

- `features/<id>/*.feature` — the scenarios
- `docs/specs/<id>/` — discovery, domain model, example map, criteria, traceability matrix, and the
  SME corrections made at each gate
- `DOMAIN_DICTIONARY.md` diff for new terms
- draft orders, `status: draft`, each pinning this package by digest in `inputs`:
  `intake/<id>.md` (`plan: deliver-feature`), `intake/<id>-ui-tests.md`
  (`plan: deliver-tests-saturday`), `intake/<id>-api-tests.md` (`plan: deliver-tests-sunday`) —
  only for the levels the scenarios actually need. A human promotes each to `ready`.

## The judge

Hard, and the design must say how it is done. Minimum:

1. A **golden set** built from the existing suites in Saturday's demo app, `testing-with-certs`
   and Sunday's `examples/demo-tests`. Reverse each into a one-paragraph Request (what someone would
   have asked for), hand only that to the factory, and compare its scenarios with the golden ones:
   rule recall (did it find each rule the golden tests check?) and missing negative cases. The golden
   suites are hidden from the run.

   Caveat to resolve first: the golden suites are golden for *what they check*. Several Saturday
   step files are not golden for *how* (they call `this.page` directly; see `03`). Grade the spec
   factory on behaviour only, and let `03`'s Phase 0 fix the style.
2. The **correction rate** at `confirm-domain` and `confirm-scenarios` across real runs, from
   `artifact.corrected`. A factory whose drafts SMEs rewrite wholesale is not saving them time.
3. A **negative harness test**: a seeded spec with a rule that has no negative example must fail
   `spec-critic`.

## Exit criteria for the first phase

1. One real order, for a real feature in Saturday's demo app, runs to a PR with a spec package.
2. Every gate was answered by the reviewer named in the order, and their corrections are recorded.
3. The golden set runs nightly and reports recall; the negative harness test fails a bad spec.
4. The draft orders it emits pass `loom intake check`.
