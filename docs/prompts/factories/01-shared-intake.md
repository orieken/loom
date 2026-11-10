# One intake contract for every factory

Every factory on the line — `deliver-feature`, `deliver-spec`, `deliver-tests-saturday`,
`deliver-tests-sunday` — takes work orders in the same shape, so one parser, one `loom intake check`,
one supervisor and one inbox serve all of them. Read `00-factory-blueprint.md` first.

**Scope: design the schema change, then implement it in `internal/intake/` behind a version bump.**

## Target repo

`/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`

## Prior context (read first)

- `templates/claude-feature-team/intake/TEMPLATE.md` and `README.md` — schema version 1
- `internal/intake/` — `order.go`, `parse.go`, `validate.go`, `vocabulary.go`, `check.go`
- `internal/planfile/` — how a plan file is discovered and described
- ADR-010 — claim snapshots, committed orders, `hold` vs `stop`

## The decision this prompt makes

"Same template" can mean two things. Only one of them works.

- **The frontmatter is identical across factories.** It is the work order: who asked, priority,
  budget, finish line, trust, scope. None of that depends on what is being built.
- **The body is not.** Version 1's body is a feature spec (Trust Boundaries, NFRs, Infrastructure
  Notes). A test order needs a target environment, a test-data strategy and the spec it automates. A
  spec order needs the problem, the people affected and the SMEs who will review it. Forcing one body
  makes every factory fill in sections that mean nothing to it, and empty sections are where an
  unattended run guesses.

So: **one frontmatter schema, a common body core, and per-plan body sections declared by the plan.**

## Schema version 2 — frontmatter

Everything in version 1 stays, with the same rules. Additions:

```yaml
intake: 2
type: feature            # feature | bug | spike | chore | spec | tests
plan: deliver-spec       # the factory; the plan declares which `type`s it accepts
target:                  # where the output lands, when it is not this repo
  repo: ""               # empty = this repo. Otherwise a git remote the floor can clone
  path: ""               # subdirectory inside the target, e.g. apps/ye-olde-magic-shop
inputs:                  # upstream artifacts this order consumes, pinned by digest
  - order: checkout-guest-purchase       # an order id in the ledger, which must be `shipped`
    artifact: spec-package               # a named artifact that order's plan publishes
    digest: sha256:...                   # filled by a human or `loom intake pin`, never by the supervisor
reviewers:               # who answers each gate, by gate name. Required for every gate the plan declares
  confirm-domain: "@payments-sme"
  confirm-scenarios: "@qa-lead"
```

Rules to add to `loom intake check`:

- `plan` must accept the order's `type` (the plan file declares `accepts: [spec]`, say).
- Every gate the plan declares has an entry in `reviewers`. An unnamed reviewer is how "SMEs review
  their chunk" turns into "Oscar reviews everything". A value is a handle (`@payments-sme`) or a
  role (`role:tester`) resolved through the team's `.claude/factory/team.yaml`. Teams do not all
  have an analyst, a developer and a tester, so the roles are whatever that file declares, one person
  may hold several, and an unheld role falls back to its default reviewer (roadmap L3.75).
- Every `inputs` entry names a shipped order and an artifact its plan publishes. A `digest` that does
  not match the shipped artifact fails the check — this is the cross-order version of I-1. When an
  upstream spec is re-delivered, every downstream order pinned to the old digest shows up in the inbox
  as "input changed".
- `depends-on` is implied by `inputs`; listing the same id in both is allowed, listing an input's
  order in neither is impossible.

## Targets — where the work lands is configuration, not part of the order

An order names a target; it does not describe how to build, start or test it. That lives with the
target, owned by the people who own that codebase:

```yaml
# <target repo>/.claude/factory/target.yaml
name: ye-olde-magic-shop
repo: git@github.com:orieken/saturday-monorepo.git
path: apps/ye-olde-magic-shop
stack:
  ui: { framework: saturday, runner: playwright }     # cucumber | playwright
  api: { framework: sunday, runner: playwright, openapi: [] }
environment:
  up: "docker compose -f containers/compose.yaml up -d --wait"
  down: "docker compose -f containers/compose.yaml down -v"
  baseUrl: http://localhost:8000
  egress: none                                         # tests may not reach the internet
exemplars: GOLDEN.md                                   # the files context-engineer pins as the pattern
testability:                                           # how to ask for a seam this target lacks
  plan: deliver-feature
  reviewers: { confirm-design: "@app-owner" }
preShipChecks:
  - { name: typecheck, command: pnpm -r typecheck }
  - { name: ui-suite, command: pnpm --filter @orieken/ye-olde-magic-shop test:e2e }
```

The order's `target.repo`/`target.path` resolve to one of these files. `loom intake check` fails an
order whose target has no `target.yaml`, or whose plan needs a stack the target does not declare
(a `deliver-tests-sunday` order against a target with no `api` stack).

**Where specs live: in the target, next to the code they describe** — `features/` for scenarios,
`docs/specs/<id>/` for the rest. Not in the framework repo. A spec is the product's living
documentation: its reviewers are that product's SMEs, its tests import it, and it changes when the
product changes. The framework repo holds plans, agents and contracts only.

**Where the floor runs: in the target repo**, for the first phase. `intake/` and the ledger live
there; every order's output lands in the same repo, so there is no cross-repo PR yet. Cross-repo
(tests in a separate repo from the app, which is common at work) is a second-phase feature: the
floor clones `target.repo` into the run's worktree and opens the PR there.

## Body — common core (every plan)

```markdown
# [Title — active verb + noun]
## Request          — quoted as received; untrusted
## Summary          — 1–2 sentences, non-technical
## Out of Scope
## Domain Language
## Open Questions   — each with "Assumption if unresolved: …"
```

## Body — per-plan sections

The plan file declares them, and `loom intake check` validates that each is present and non-empty
(or carries the plan's declared "none" sentinel):

```yaml
# .claude/plans/deliver-spec.yaml (excerpt)
name: deliver-spec
accepts: [spec]
publishes: [spec-package]
body:
  required: [Problem, People Affected, Known Rules, Evidence, Constraints]
  optional: [Prior Art]
```

| Plan | Required sections |
|---|---|
| `deliver-feature` | Acceptance Criteria, Non-Functional Requirements, Trust Boundaries, Test Approach, Infrastructure / Deploy Notes (version 1's body, unchanged) |
| `deliver-spec` | Problem, People Affected, Known Rules, Evidence, Constraints |
| `deliver-tests-saturday` | System Under Test, Environment, Test Data, Selectors, Runner (cucumber \| playwright) |
| `deliver-tests-sunday` | Service Under Test, API Description (OpenAPI path or URL), Environment, Auth, Test Data |

## Migration

- Version 1 orders keep parsing. Version 1 implies `plan: deliver-feature` body rules.
- `loom intake migrate <file>` rewrites `intake: 1` to `2`, adding empty `inputs`, `target` and a
  `reviewers` block with the plan's gates and `requested-by` as each value.
- The template becomes one file per plan: `intake/TEMPLATE.<plan>.md`, generated from the plan's
  `body` declaration so the template and the validator cannot drift (the I-4 lesson).

## Exit criteria

1. A version 1 order and a version 2 order for each of the four plans pass `loom intake check`.
2. An order missing a plan-required section, a reviewer for a declared gate, or with a stale input
   digest fails, naming the file, field and reason.
3. Re-shipping an upstream order puts every pinned downstream order in the inbox as "input changed".
4. Templates are generated from plan files; a test fails if a template and its plan disagree.
