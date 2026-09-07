# Delivery Route: console-log-filtering

## Stages

| Stage | Runs | Why |
|---|---|---|
| `architect` | yes | structural work: a context crossing, a data-model change, a new dependency, a performance threshold, or an explicit flag |
| `performance-engineer` | yes | a performance requirement carries a measurable threshold |
| `data-engineer` | **no** | no data-model change to sequence |
| `developer` | yes | always runs; not skippable by routing |
| `code-reviewer` | yes | always runs; not skippable by routing |
| `accessibility-engineer` | **no** | no accessibility requirement, so the analysis describes no UI surface |
| `security-reviewer` | yes | always runs; not skippable by routing |
| `qa-engineer` | yes | always runs; not skippable by routing |
| `visual-qa-engineer` | yes | always runs; not skippable by routing |
| `sre-engineer` | yes | always runs; not skippable by routing |
| `tech-writer` | yes | always runs; not skippable by routing |
| `devops-engineer` | **no** | the analysis lists no DevOps tasks |

## Skipped

- `data-engineer` — no data-model change to sequence
- `accessibility-engineer` — no accessibility requirement, so the analysis describes no UI surface
- `devops-engineer` — the analysis lists no DevOps tasks

## How this was decided

Computed from `analysis.md` after the analyst stage, by predicates in `internal/state/` —
not by a model re-reading the analysis (roadmap L3.0).

Editing this file resets the design gate's approval: the run will halt until a human
approves the route as it then stands.
