# Delivery Route: console-log-filtering

## Stages

| Stage | Runs | Why |
|---|---|---|
| `architect` | yes | structural work: the analyst raised an architectural flag |
| `performance-engineer` | **no** | no performance requirement carries a threshold with a number and a unit |
| `data-engineer` | **no** | no data-model change to sequence |
| `developer` | yes | always runs; not skippable by routing |
| `code-reviewer` | yes | always runs; not skippable by routing |
| `accessibility-engineer` | **no** | the analysis declares no UI surface |
| `security-reviewer` | yes | always runs; not skippable by routing |
| `qa-engineer` | yes | always runs; not skippable by routing |
| `visual-qa-engineer` | **no** | the analysis declares no UI surface, so there is nothing to look at |
| `sre-engineer` | **no** | the analysis declares no served runtime surface, so no availability or latency SLI applies |
| `tech-writer` | yes | always runs; not skippable by routing |
| `devops-engineer` | **no** | the analysis lists no DevOps tasks |

## Skipped

- `performance-engineer` — no performance requirement carries a threshold with a number and a unit
- `data-engineer` — no data-model change to sequence
- `accessibility-engineer` — the analysis declares no UI surface
- `visual-qa-engineer` — the analysis declares no UI surface, so there is nothing to look at
- `sre-engineer` — the analysis declares no served runtime surface, so no availability or latency SLI applies
- `devops-engineer` — the analysis lists no DevOps tasks

## How this was decided

Computed from `analysis.md` after the analyst stage, by predicates in `internal/state/` —
not by a model re-reading the analysis (roadmap L3.0).

Editing this file resets the design gate's approval: the run will halt until a human
approves the route as it then stands.
