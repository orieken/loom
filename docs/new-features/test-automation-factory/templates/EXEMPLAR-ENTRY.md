# Exemplar entry

An exemplar is a real test that runs in your suite and that agents copy as a model. Mark it twice.

## 1. Annotate the test, using your language's own mechanism

| Language | Mark |
|---|---|
| TypeScript, Gherkin | `@exemplar` in the JSDoc or as a tag |
| Python | `@pytest.mark.exemplar` |
| Java | `@Tag("exemplar")` |
| C# | `[Trait("Exemplar", "<level>")]` |
| Go | `// exemplar:` comment above the test |

The mark sits on a comment or tag line directly above the test. The test also carries its issue reference and acceptance-criterion reference (Gherkin needs only the issue tag).

## 2. List it in `.claude/exemplars.yaml`

```yaml
exemplars:
  - test: "creates an order and returns 201"
    file: tests/api/orders-create.spec.ts
    language: typescript
    level: api
    blueprint: api-sunday-like
    demonstrates: "Declarative client call, status matcher, schema validation, timeout set"
    can-fail: "2026-11-17: changed the schema literal in order.model.ts; this test failed; restored"
    digest: "sha256:<optional; a changed file then raises a warning>"
```

## Disqualifiers

An exemplar is invalid when any of these is true:

- The test does not run, or is skipped.
- The test cannot fail. A mutation of the behavior it checks does not break it.
- The issue or acceptance-criterion annotation is missing.
- A function in the test has cyclomatic complexity of 7 or more.
- The file name does not follow `name.type.extension`.
