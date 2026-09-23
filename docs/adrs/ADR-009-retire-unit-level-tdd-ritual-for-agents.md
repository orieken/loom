# ADR-009: Retire the Unit-Level TDD Ritual for Agents; Keep Acceptance Tests Written from the Spec

## Status

Accepted — depends on ADR-008

## Date

2026-09-22

## Deciders

Oscar Rieken

## Context

`shared/rules/testing-conventions.md` says **ALWAYS practice TDD/BDD — Red-Green-Refactor**, and the
framework carries three mechanisms for it: the `test-driven-developer` agent, the four-role
`TDDWorkflow` (`shared/workflows/tdd-workflow.md`), and `deliver-atdd`.

TDD's design benefit comes from friction between whoever writes the failing test and whoever makes it
pass — the test writer does not yet know the implementation. **An agent already knows.** It has a
solution in context before the first assertion is written, so the test documents a decision already
made rather than constraining one. The framework says this about itself: `test-driven-developer.md`
concedes that when it writes both, "that gap collapses", and `docs/patterns/testing-pyramid.md` says
standalone agent TDD "oversells the design benefit". The ritual survives as ordering with the reason
removed, and the four-role workflow spends handoffs and context re-reads to simulate friction that does
not occur. Splitting the roles across agents helps less than it appears: two agents running the same
model on the same analysis share most of what the separation was meant to withhold.

The mechanisms also contradict one another, which is what a ritual without a reason tends to produce:

- `developer.md` instructs "**Red**: write the failing test… first" and, further down, "Do NOT write
  test files. That is the QA engineer's job."
- `TDDWorkflow` gives its RED stage to `unit-tester`, whose contract is tests for *existing* code and
  which `testing-pyramid.md` says "explicitly does NOT follow the Three Laws".
- RED's audit is `tool-validator`, which audits `shared/skills/*/SKILL.md` files, not tests — while the
  workflow's retry policy says a failed audit means "rewrite tests to fix annotation violations".
- The loop's "machine-enforced" termination and "machine-verifiable" coverage gate are prose. The
  `loom` executor has no TDD or ATDD support; a model counts its own iterations.

Two parts of the practice do earn their keep with agents, for reasons other than design pressure:

1. **A test must be shown able to fail.** This is evidence that the test detects anything, and it is
   the part agents skip. ADR-008 makes it a measured property of the result instead of a step order.
2. **Acceptance tests written from the spec, not from the code.** Tests derived from an implementation
   tend to assert what the code does rather than what it should; tests derived from acceptance criteria
   catch "built the wrong thing", which nothing downstream of the code can. That independence is the
   useful part of `deliver-atdd`.

## Decision

1. **Unit tests are written by the implementer, with the code, in either order.** `developer` owns its
   unit tests. "Done" is ADR-008's definition, not a sequence of red and green steps. The contradictory
   "do not write test files" line is removed.
2. **The Three Laws stop being a rule and become a technique.** `testing-conventions.md` drops
   "ALWAYS practice TDD — Red-Green-Refactor" for unit-level agent work. `testing-pyramid.md` keeps the
   Three Laws as a documented technique a human or agent may use, and says plainly it is not required.
3. **`TDDWorkflow` is retired**, along with its flat `tdd-state.json` checkpoint (a gap recorded under
   roadmap C.2). `/orchestrate --workflow tdd` stops being offered.
4. **`test-driven-developer` is removed, not deprecated.** The distinction it drew — test-first versus
   not — no longer changes what is produced or how it is checked, and `developer` does the work.
   `deliver-atdd` Phase 3 invokes `developer`. The removal is recorded in the agent CHANGELOG and the
   release notes name `developer` as the replacement; there is no alias.
5. **Acceptance tests stay spec-derived and implementation-blind.** `qa-engineer` writes scenarios from
   the acceptance criteria before the implementation exists, as `deliver-atdd` Phases 1–2 already do.
   Where a pipeline lets the scenario author read the implementation first, that is the defect to fix;
   under `loom run` the stage's input projection is what enforces it.
6. **`unit-tester` is unchanged.** Characterization of existing code was never TDD.

## Consequences

- **Easier**: Fewer handoffs, fewer agents, and no internal contradictions about who writes a test.
  The requirement becomes something the executor can check rather than something a model is asked to
  remember doing.
- **Harder**: The framework can no longer claim to practise TDD with agents. The Zero to Agent SDET
  curriculum teaches it; the framework is the source of truth, so the curriculum is updated to teach
  ADR-008's definition of done rather than the framework bending to the curriculum. A reviewer loses the step-by-step
  trail a Red-Green-Refactor run leaves; ADR-008's numbers replace it, but they describe the result,
  not the path.
- **Changed**: About thirty files reference TDD, `test-driven-developer`, or `TDDWorkflow` — rules,
  agents, skills, blueprints, orchestration docs, generated platform configs, and the platform content
  embedded in `cmd/loom/internal/platform/content.go`. Removing `test-driven-developer` is a breaking
  change for any team invoking it by name, accepted deliberately: a deprecation release would keep a
  name that promises a discipline the framework no longer practises. It ships in a release whose notes
  say so.

## Alternatives Considered

| Option | Why rejected |
|---|---|
| Keep everything, fix only the contradictions | Repairs the symptoms and keeps paying for friction that does not exist. |
| Keep `TDDWorkflow` and make it real under `loom run` | Would make the ritual enforceable, and enforce the part that does not help. The effort belongs to ADR-008's checks. |
| Keep `test-driven-developer` as an alias of `developer` | Cheapest migration, but a name that promises a discipline the framework no longer practises. |
| Deprecate for one release, then remove | Considered and declined (2026-09-22): the release in between would ship the contradiction this ADR exists to remove. Removal is immediate, and the release notes carry the migration. |
| Drop acceptance-level separation too | Loses the one independence that does catch something — specification drift. Its cost is one extra authoring step per feature. |
| Require TDD for humans, not agents | The rule files are written for agents and humans read the same text. Stating the Three Laws as a technique covers humans who want it without mandating it. |
| Keep TDD because the Training curriculum teaches it | The curriculum follows the framework, not the reverse. |

## Fitness Function

- **ADR-008's three checks** carry the part of TDD that is kept: evidence that the tests can fail.
- **Spec-blindness of acceptance authoring** — under `loom run`, a test asserts that the scenario
  author's stage input is a projection of the analysis that contains no implementation artifact
  (roadmap L3.62). Under the markdown pipeline this is judgment-only: the executor does not mediate what
  a model reads there.
- **The retirement itself** — nothing fails today on a reference to an agent or workflow that no longer
  exists (checked 2026-09-22: neither `health-check.sh` nor `check-inventory-drift.sh` looks for one).
  Roadmap L3.61 adds that check, proved red against a planted reference, so the thirty-odd files cannot
  keep citing `test-driven-developer` after it is gone.
