# Test Annotation Convention

Every test carries traceability back to two things: the issue that motivated it, and the specific
acceptance criterion it verifies. `shared/rules/testing-conventions.md` states that as the rule;
this file is the per-language mechanics — six languages' worth of syntax, of which any one test
needs exactly one.

Moved out of the rule file on 2026-09-22 (roadmap L3.19). It was ~1,054 tokens, over half of that
rule, loading into every stage's prompt on every run — including the twelve stages that never write
a test. The convention did not change.

---

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
