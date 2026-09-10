# Run 4 specs

Committed here so the runs are reproducible after the clones are gone, which is the mistake
run 3 nearly made — its spec survived only because `/tmp/loom-e2e3` had not yet been reaped.

| File | Experiment | Provenance |
|---|---|---|
| `console-log-filtering.md` | A — variance and the routing saving | **Byte-identical to run 3's spec**, recovered from `/tmp/loom-e2e3/docs/features/` (md5 `b16ccb20d09e524602f9495f372e1819`). Not a reconstruction, so comparisons to run 3's $9.49 carry no spec confound. |
| `run-history-browsing.md` | B — does the narrowed routing still include? | Written for run 4 against `apps/console`, which today has `POST /api/runs` and `GET /api/runs/{runId}` and no listing endpoint. |

## Why the Experiment B spec reads the way it does

It describes a feature. It does **not** tell the analyst which fields to emit, and it must not:
B1 asks whether an analyst produces `surfaces` and a typed threshold from a natural spec. A spec
saying "declare a UI surface" would prove only that the analyst can follow an instruction.

So the facts the predicates need are present as ordinary requirements a person would write:

| What the router needs | How the spec says it |
|---|---|
| a UI surface | "The console has a page listing the runs… each row links to that run's report" |
| an accessibility requirement | "Someone using the keyboard alone can move through the list" |
| a served runtime surface | "Listing runs never blocks a run that is starting or finishing" |
| a measurable threshold | "responds in under 200ms at p95 with 10,000 runs held in the store" |
| an API change | a new `GET /api/runs` |
| pagination (guardrail #6) | "must not return everything it holds" |

## The controls matter as much as the inclusions

B2 asks that `architect`, `performance-engineer`, `sre-engineer` and `visual-qa-engineer` route
in. On its own that is passable by a router which includes everything, which is the bug L3.24
just removed. This spec therefore also carries work that must stay **out**:

- **`data-engineer` must skip** — "the store stays in memory", no schema, no migration.
- **`devops-engineer` must skip** — no CI, deployment or environment change is asked for.

A run where all six route in has not demonstrated that routing works; it has demonstrated that
routing stopped discriminating. Record both halves.
