# Blueprint: Test-Automation Factory

**Registry id**: `test-automation-factory`
**Suggested language**: TypeScript
**Supported languages**: TypeScript, Python, C#, Java, Go
**Testing levels covered**: Unit, Integration, API Contract, Acceptance, E2E/UI
**Status**: Draft — built phase by phase from `phases/`.

> **Rule of this blueprint:** the pattern is fixed and the language is soft. A team may choose any language
> and test runner. A team may not drop the layer boundaries, the interface-per-external-dependency rule,
> the budget and time-box rules, or the approval gates.

## When To Use

- You want agents to write and maintain test automation under explicit rules, budgets, and review.
- You have UI flows, APIs, or units that need tests, and you want one consistent pattern per test type.
- You want people to extend the factory with their own test types, sites, frameworks, and tools.

Do NOT use when:

- You need an application scaffold. This factory writes tests, not product code.
- You want one-off tests with no shared pattern. A plain test file is lighter.

## Layer Structure

| Layer | Responsibility | Example files |
|---|---|---|
| Domain | Abstract test building blocks and models. No framework imports. | `base-site.ts`, `base-page.ts`, `base-api-client.ts`, `order.model.ts` |
| Use-Cases | Concrete sites, flows, API clients, and factories for the system under test | `login.flow.ts`, `orders-api.client.ts`, `user.factory.ts` |
| Adapters | Browser driver, HTTP client, clock, filesystem, telemetry. Each hides behind an interface. | `playwright.adapter.ts`, `http.adapter.ts`, `otel.adapter.ts` |
| Test Runner | Test files and step definitions that bind scenarios to flows and clients | `login.spec.ts`, `orders.steps.ts` |
| Factory Tooling | Intake validator, blueprint validator, exemplar check, budget check, pre-ship script | `scripts/validate-intake.*`, `scripts/check-exemplars.*` |

Dependencies point inward only. Domain imports nothing from outer layers. Use-Cases import Domain. Adapters
implement interfaces that Domain or Use-Cases define.

## Directory Tree (TypeScript suggestion)

```
<project-root>/
├── docs/test-factory/             # this pack (kept for reference)
├── intake/                        # work orders; TEMPLATE.md and README.md live here
├── blueprints/
│   ├── ui-e2e-saturday-like.md
│   ├── api-sunday-like.md
│   ├── unit.md
│   ├── custom/                    # team-owned blueprints
│   └── registry.json
├── .claude/                       # or your agent's folder
│   ├── rules/                     # phase 1
│   ├── skills/                    # phase 5
│   └── exemplars.yaml             # phase 6
├── src/
│   ├── domain/
│   ├── sites/<site-name>/{pages,flows,partials}/
│   ├── clients/<service-name>/
│   ├── adapters/
│   └── factories/
├── features/                      # Gherkin, when the blueprint uses it
├── tests/                         # unit tests and specs
├── scripts/                       # validators and checks
├── budgets.yaml                   # phase 7
├── ARCHITECTURE_RULES.md
└── DOMAIN_DICTIONARY.md
```

## Key Abstractions (do not bypass)

- **Order** — one intake file. It names the blueprint, the target, the budget, and the acceptance criteria.
- **Blueprint** — one document per test type. The registry lists it.
- **Skill** — one prompt file per job. It loads golden examples before writing anything.
- **Exemplar** — a real, running, annotated test listed in `exemplars.yaml`.
- **Budget** — ceilings for cost, attempts, run hours, and a wall-clock deadline.
- **Gate** — a point where only a human answer lets work continue.

## Testing Pyramid Coverage

| Level | Written with skill | Pattern | Speed budget |
|---|---|---|---|
| Unit | `unit-test` | Arrange, Act, Assert | fractions of a second |
| Integration | `unit-test` with real adapters behind fakes | same | seconds |
| API contract | `api-test` | Sunday-like | seconds |
| Acceptance | `ui-e2e-test` or `api-test` with Gherkin | scenario is the acceptance criterion | tens of seconds per scenario |
| E2E/UI | `ui-e2e-test` | Saturday-like | minutes for the suite |

## Non-Negotiable Rules

- Cyclomatic complexity below 7 on every function.
- At least 85% coverage of the lines a change adds or modifies.
- Every test must be able to fail. Show the failure once.
- Every external dependency (browser, network, clock, filesystem, queue) sits behind an interface.
- File names follow `name.type.extension`.
- No secrets in code. No custom retry loops. Every network call has an explicit timeout.
- A test repair may fix the path to an assertion. It may never weaken the assertion.

## Budgets and Time-Boxes

Every order carries `budget{max-usd, max-attempts, max-hours, deadline}`. A stage has a timeout. Details are in
`phases/phase-7-budgets-and-time-boxes.md`. The `deadline` field is a mechanism this pack asks you to build.
It is not assumed to exist anywhere.

## Extending the Factory

1. Copy `templates/BLUEPRINT-TEMPLATE.md` to `blueprints/custom/<id>.md`.
2. Add an entry to `blueprints/registry.json` with `"custom": true`.
3. Run the blueprint validator. It checks required sections and the registry entry.
4. Mark at least one exemplar for the new type in `exemplars.yaml`.
5. Optional: run the `new-blueprint` skill. It interviews you and writes steps 1 to 2 for you.

## ADR-000 Seed Context

```markdown
# ADR-000: Test-automation factory

Status: Accepted
Context: <project name> needs consistent, budgeted, reviewable test automation written partly by agents.
Decision: Adopt the test-automation factory blueprint. Pattern fixed, tooling soft.
Consequences: Orders gate all agent test work. Rules in .claude/rules/ bind every agent.
Fitness functions: intake validator, blueprint validator, exemplar check, complexity check, coverage check.
```

## Downstream Skills

Order of use for one order: `test-audit` (find the gap), then `ui-e2e-test`, `api-test`, or `unit-test`
(write), then `exemplar-audit` (keep golden examples honest). `new-blueprint` runs on demand.

---
*Derived from the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
