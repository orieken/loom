# Testing Pyramid

Every test in this framework belongs to exactly one level. Which level a test belongs to determines its
principles, speed budget, ownership, and the mechanism by which it earns and keeps trust. This doc names
the levels and their principles once; the framework-specific mechanics for the top two levels live in
`saturday-framework-patterns.md` (E2E/UI) and `sunday-framework-patterns.md` (API Contract) — referenced
here, not restated.

The classic pyramid shape holds: most tests should be unit tests (fastest, cheapest, hundreds per
codebase), progressively fewer at each level up, smallest count at E2E (slowest, brittlest, most
expensive per test). If a codebase has 500 tests and 400 of them drive browsers, the pyramid is
inverted — flag that as a real problem, not a stylistic preference.

## Unit Tests

**Context**: The base of the pyramid. Fast, isolated verifications of a single unit of behavior — one
class, one function, one small chunk of logic — with every external collaborator either mocked or
substituted with a test double.

**Principles (FIRST — Uncle Bob)**:
- **Fast** — fractions of a second per test, or developers won't run them (and if they don't run them,
  the tests aren't buying anything)
- **Independent** — any order, no shared mutable state between tests
- **Repeatable** — deterministic across environments (local, CI, prod-like)
- **Self-Validating** — pass/fail boolean, no log reading, no manual comparison
- **Timely** — written just before (TDD) or immediately after (backfill/characterization) the
  production code they cover — never "we'll add tests later"

**Written via**: `developer`, with the code it changes (ADR-009), or `unit-tester` (backfill /
characterization of existing code).

**Framework**: whatever the project's language convention specifies — Vitest for TypeScript, pytest for
Python, JUnit 5 for Java, xUnit for C#, standard library `testing` for Go. See
`shared/rules/<language>-conventions.md` for each.

## Integration Tests

**Context**: The middle layer. Verify component boundaries — a small handful of collaborators talking
to each other. Still fast enough to run frequently, but scope is wider than a single unit.

**Principles**: FIRST still applies — the discipline doesn't change, only the scope. If a test is slow
because it crosses too many boundaries or hits a real network/DB, that's a signal to either split it
into narrower units or push it out to an E2E test — not a signal to relax FIRST.

**Written via**: `qa-engineer` typically, or `developer` when the integration itself IS the
feature (e.g., a new adapter wiring two services).

## API Contract Tests

**Context**: Sunday's territory. Verify request/response shape, status behaviors, and resilience under
failure — the *contract* between the API and its consumers, not the API's internal implementation.

**Principles**: Every criterion in the Declarative API Client Pattern — `BaseApiClient` for
domain-scoped clients, `IHttpAdapter` for HTTP mechanics, Fluent Matchers for declarative assertions,
Schema Validation for shape, Resilience Primitives for failure behaviors. See
`sunday-framework-patterns.md` — not restated here.

**Written via**: `api-test-generator` (auto-generated from an OpenAPI spec) or `qa-engineer`
(hand-written for domains without a spec).

## Acceptance Tests

**Context**: Business-language expressions of what the system does for a user. The unit under test is a
whole user-visible behavior, expressed once in Gherkin, executed against the real integrated system.

**Principles**:
- Given/When/Then, written in business language a non-engineer can read
- **Each scenario IS a specific acceptance criterion** — the traceability lives in the scenario text
  itself, not a separate mapping doc that will go stale
- Written FIRST in an ATDD workflow (before step definitions, before implementation) — see
  `deliver-atdd/SKILL.md`
- Slower than unit tests; worth it for intent-clarity. Should be a small fraction of the total test
  count
- The AC being verified may originate from a Gherkin scenario the framework itself wrote (via
  `qa-engineer` inside `deliver-atdd`) or from an external ticket (JIRA, Linear, GitHub issue) — either
  way, the scenario becomes the executable specification for that AC, and the source of the AC doesn't
  change how it's captured

**Written via**: `qa-engineer` (scenario + step definitions) inside `deliver-atdd` or `deliver-feature`.

## E2E / UI Tests

**Context**: Saturday's territory. Whole user journeys through the real UI — the top of the pyramid,
smallest count, highest cost, most brittle by nature.

**Principles**: Every criterion in the Site-Centric Pattern — `BaseSite`, `BasePage`, `BaseElement`,
`BaseFlow`, `SiteManager`, `TabManager`. Explicitly NOT the traditional Page Object Model. See
`saturday-framework-patterns.md` — not restated here.

**Written via**: `qa-engineer` following Saturday conventions, driven by Cucumber.js + Playwright.

---

## The Three Laws of TDD (Uncle Bob)

Not a test level — a *discipline* for arriving at well-designed unit tests (and, applied at the
acceptance level, is ATDD). Cited here so agents that practice it can point at one canonical statement:

1. You may not write production code until you have first written a unit test that fails.
2. You may not write more of a unit test than is sufficient to fail. Not compiling counts as failing.
3. You may not write more production code than is sufficient to pass the currently failing test.

### Why the framework does not require it of agents (ADR-009)

XP TDD's design benefit comes from **epistemic role separation and friction** — the person writing the
failing test doesn't yet know how the implementer will solve it. **An agent already knows.** It has a
solution in context before the first `expect(...)` is written, so the test documents a decision already
made rather than constraining one. Splitting the roles across two agents helps less than it looks: they
share a model and an analysis, and so most of what the separation was meant to withhold.

So the Three Laws are a **technique** here, not a rule. What *is* required is the result, measured on the
change (ADR-008): changed lines at least 85% covered, no test without a path to a failure, and mutants on
changed lines killed. Design pressure comes from mechanisms that do not depend on who wrote the test —
cyclomatic complexity < 7 in CI, Sandi Metz's limits, SOLID, the `code-reviewer` pass.

One independence is kept, for a different reason: **acceptance tests are written from the spec, before
the implementation exists** (`deliver-atdd`). That catches "built the wrong thing", which tests derived
from the finished code cannot. It is specification drift being caught, not TDD design pressure.

### `unit-tester` explicitly does NOT follow the Three Laws

The `unit-tester` agent writes tests for code that already exists and shouldn't change. TDD is
impossible in that context — the code came first. Its discipline is Michael Feathers' *characterization*,
not Uncle Bob's TDD. Its tests still satisfy FIRST as *properties*, but they don't come from the Three
Laws process.

---

## Related

- `saturday-framework-patterns.md` — the full Site-Centric Pattern for E2E/UI (referenced here, not
  restated)
- `sunday-framework-patterns.md` — the full Declarative API Client Pattern for API Contract (referenced
  here, not restated)
- `deliver-atdd/SKILL.md` — the acceptance-first delivery workflow where the Three Laws' role
  separation is preserved
- `shared/rules/testing-conventions.md` — the enforcement-side "always/never" statements and the test
  annotation convention (issue-ref + AC-ref, per-language mechanisms) that correspond to the philosophy
  in this doc
