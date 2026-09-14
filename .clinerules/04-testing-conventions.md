---
paths:
  - "**/*.spec.*"
  - "**/*.test.*"
  - "**/*.feature"
  - "**/steps/**"
---
# Testing Rules

Cross-language testing principles and the Saturday/Sunday framework conventions. Language-specific
tooling (unit test framework, fake-data/factory libraries, per-language Playwright bindings, reporting)
lives in each language's own `shared/rules/<language>-conventions.md` — this file covers what's shared
across all of them.

For the *why* behind the categories and disciplines below (FIRST principles, the Three Laws of TDD, when
role separation preserves TDD's design pressure and when it doesn't), see
`docs/patterns/testing-pyramid.md`. This file is the
enforceable "always/never" side; that one is the philosophy.

## Test Categories

Every test belongs to exactly one level. The level determines the writing agent, the framework, and the
principles that apply. Full detail in `docs/patterns/testing-pyramid.md`; this table is the enforcement
map.

| Level | Written by | Framework | Speed budget | Principles |
|---|---|---|---|---|
| Unit | `test-driven-developer` (greenfield) or `unit-tester` (backfill/characterization) | project's language convention (Vitest/pytest/JUnit/xUnit/`testing`) | fractions of a second | FIRST |
| Integration | `qa-engineer` (or `test-driven-developer` when the integration IS the feature) | same as unit | seconds, not minutes | FIRST |
| API Contract | `api-test-generator` (from OpenAPI) or `qa-engineer` (hand-written) | Sunday (`BaseApiClient` + `IHttpAdapter` + Fluent Matchers + Zod + Resilience Primitives) | seconds | Sunday's Declarative API Client Pattern |
| Acceptance | `qa-engineer` inside `deliver-atdd` or `deliver-feature` | Gherkin (Cucumber.js / Reqnroll / pytest-bdd / Cucumber-JVM per language) | tens of seconds per scenario | scenario IS the AC, business language |
| E2E / UI | `qa-engineer` following Saturday conventions | Saturday (Cucumber.js + Playwright, Site-Centric pattern) | minutes total for the suite | Saturday's Site-Centric Pattern |

## Saturday Framework (E2E / UI Testing)
ALWAYS use the Site-Centric pattern: `BaseSite`, `BasePage`, `BaseElement`, `BaseFlow`.
NEVER use traditional Page Object Model (POM).
ALWAYS use Playwright driven by Cucumber.js for UI automation.
ALWAYS include OpenTelemetry instrumentation for every BDD scenario.

## Sunday Framework (API Testing)
ALWAYS use Vitest for unit tests and Playwright for integration/E2E API tests.
ALWAYS use the custom api fixture (`api`) and fluent matchers (`toHaveStatus`, `toBeSuccessful`, `toRespondWithin`).
ALWAYS extend `BaseApiClient` for domain-specific API clients.
ALWAYS validate schemas with Zod (`validateSchema()`).
NEVER use custom retry loops — use `CircuitBreaker` or `ExponentialBackoffStrategy`.

## Test Quality
CRITICAL: Test coverage MUST be >= 85%.
CRITICAL: Cyclomatic complexity per function MUST be < 7.
ALWAYS practice TDD/BDD — Red-Green-Refactor.
NEVER write feature code without tests.
ALWAYS keep tests "moist," not fully dry — DRY the setup noise (via Flows, Factories, fixtures) but
keep the critical assertion path visible in the test itself. A test verifying search results should
show "user searched for X" prominently in the test body; how they clicked into the search input should
not. Over-DRYing by hiding the interesting user action inside a flow makes the test read like a magic
incantation and defeats the point. This is the same idea as DAMP (Descriptive And Meaningful Phrases),
a well-known counterpoint to blanket DRY in tests.

## Reporting Pipeline
Cucumber JSON summaries feed the Friday dashboard (see `shared/rules/approval-gates.md` gate #1 —
posting to Friday requires explicit human approval, same as any other external-facing action). Every
language's port of Saturday should be able to produce Cucumber-JSON-compatible output, or a bridge to
it, so results from any language funnel through the same reporting/approval pipeline rather than each
language inventing its own.

## Test Annotation Convention

Every test carries traceability back to two things: (1) the issue that motivated it, and (2) the
specific acceptance criterion (or fine-grained behavior derived from one) it verifies. Report-time
mapping (e.g., `qa-report.md`'s "acceptance criteria covered X/Y") is a snapshot — the moment a test
gets renamed or moved, that mapping evaporates. In-test annotation is durable because it lives with
the test itself.

The AC being verified may originate from a Gherkin scenario the framework wrote (via `qa-engineer`
inside `deliver-atdd`) or from an external ticket (JIRA, Linear, GitHub issue). Either way, the
annotation format is the same — the source of the AC doesn't change how it's linked to the test.

### What every test carries

- **Issue reference**: free-form string — `PROJ-123`, `ENG-456`, `#789`, or a URL. Teams pick the
  format that matches their tracker; the framework doesn't lock this to any specific one.
- **AC reference**: a short excerpt of the AC text, or a numbered reference if the ticket/spec
  enumerates them (e.g., `AC1: user can register with valid email and password`).

### Use native language mechanisms, not homegrown comments

**TypeScript (Vitest/Jest)** — JSDoc block above the test:
```typescript
/**
 * @issue PROJ-123
 * @ac AC1: user can register with valid email and password
 */
test('creates user with valid credentials', async () => { ... });
```

**Python (pytest)** — docstring; optional custom marker for filtering by issue:
```python
def test_creates_user_with_valid_credentials():
    """PROJ-123 / AC1: user can register with valid email and password"""
    ...
```

**Java (JUnit 5)** — `@Tag` for filtering + `@DisplayName` for the report:
```java
@Tag("issue:PROJ-123")
@DisplayName("PROJ-123 - AC1: user can register with valid credentials")
@Test
void createsUserWithValidCredentials() { ... }
```

**C# (xUnit)** — `[Trait(...)]` on the `[Fact]` or `[Theory]`:
```csharp
[Trait("Issue", "PROJ-123")]
[Trait("AC", "AC1: user can register with valid credentials")]
[Fact]
public void CreatesUserWithValidCredentials() { ... }
```

**Go (`testing`)** — comment above the test function:
```go
// PROJ-123 / AC1: user can register with valid credentials
func TestCreatesUserWithValidCredentials(t *testing.T) { ... }
```

**Gherkin (Cucumber / pytest-bdd / Reqnroll / Cucumber-JVM)** — tag on the scenario. The scenario name
itself IS the AC, so no separate AC annotation is needed:
```gherkin
@issue:PROJ-123
Scenario: PROJ-123 - user can register with valid credentials
  Given ...
```

### Per-level granularity

- **Unit** and **Integration**: annotate the specific AC (or fine-grained behavior derived from one).
  One AC often spawns 3-5 unit tests exercising different edge cases; each gets the same issue-ref but
  different AC-ref granularity (`AC1`, `AC1 - empty email`, `AC1 - invalid email format`).
- **API Contract**: issue-ref for the change, AC-ref for the specific contract requirement (e.g.,
  `returns 429 on 6th request per minute`).
- **Acceptance** and **E2E/UI**: the Gherkin scenario IS the AC. Tag with issue-ref; scenario name/text
  IS the AC. Nothing further needed.

### Exemplar tests

One test per `(language, level)` pair may additionally be marked as the **exemplar** — the one to
read before writing a new test of that kind. It is a real test in the suite, marked in place with a
language-native `exemplar` tag alongside its issue/AC annotation, and declared in
`.claude/exemplars.yaml`. Per-language marks, the manifest schema and the disqualifiers live in
`shared/contracts/exemplar-contract.md`. Not "golden" — `DOMAIN_DICTIONARY.md` reserves that word.

### Enforcement

Documented convention only — no CI fitness function today. Matches how Sandi Metz's class/method line
limits and boolean-parameter guidance are handled in `CLAUDE.md`: flagged for humans, not gated in CI.
Adding a "grep every test for a `PROJ-\d+`-shape reference" check is real work with real false-positive
risk (URL-format issue references, teams without a tracker prefix, tests genuinely exploring behavior
before an AC exists yet) — easy to add later if the convention catches on, expensive to walk back if
it doesn't.
