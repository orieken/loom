# A line of small factories

Prompts for building the factories that split delivery into pieces an expert in each can review.

| Prompt | Factory (plan) | Produces | Its reviewers |
|---|---|---|---|
| [00-factory-blueprint.md](00-factory-blueprint.md) | — | The patterns every factory reuses | — |
| [01-shared-intake.md](01-shared-intake.md) | all | Intake schema v2: shared frontmatter, per-plan body, pinned inputs, named reviewers | — |
| [02-spec-factory.md](02-spec-factory.md) | `deliver-spec` | Signed criteria, Gherkin, draft downstream orders | requester, domain SME, QA lead |
| [03-test-factory-saturday.md](03-test-factory-saturday.md) | `deliver-tests-saturday` | UI automation on Saturday | app owner, test architect, QA lead |
| [04-test-factory-sunday.md](04-test-factory-sunday.md) | `deliver-tests-sunday` | API automation on Sunday | service owner, test architect, QA lead |
| [05-pilot-cesium-reagent-map.md](05-pilot-cesium-reagent-map.md) | all | The POC: one feature through every factory, one role per gate | analyst, developer, tester |
| [06-runner-images.md](06-runner-images.md) | — | The Debian and UBI9 browser-runner images, for any repo | — |
| [07-agent-metrics-and-costs.md](07-agent-metrics-and-costs.md) | — | How we measure agent cost and quality, for any team or pipeline | — |
| [05-pilot-decisions.md](05-pilot-decisions.md) | all | Choices made in the pilot, each a candidate skill, tool or rule | — |
| (exists) | `deliver-feature` | Product code | as today |

## How they connect

```
                 ┌──────────────── spec package (pinned by digest) ────────────────┐
problem ─► deliver-spec ─┬─► deliver-tests-sunday ─► API suite ─┐                   │
                         ├─► deliver-tests-saturday ─► UI suite ─┼─► judges ─► deliver-feature
                         └───────────────────────────────────────┘
```

One floor — one supervisor, ledger, inbox, intake parser. Each factory is a plan, chained by
`inputs` pinned by digest. The test suites are the hidden judge for `deliver-feature`: it builds
against the signed scenarios and never sees the automation that grades it.

## Build order

1. Close factory Phase 1 (`docs/roadmaps/factory-phases.md`): L3.74, the nightly real-provider run.
2. `05` slice 0: upgrade Saturday's framework install, repair the golden tests, confirm WebGL in `containers/Dockerfile`, no-network maps.
3. `05` wave 1 by hand, logging decisions in `05-pilot-decisions.md`; code orders on the floor.
4. `01` — intake v2, `target.yaml`, reviewers; L3.75, the inbox filtered by reviewer.
5. Build `04`, `02` and `03` from what the by-hand stages needed, in that order.
6. `05` phase 2: retarget to `testing-with-certs`.
