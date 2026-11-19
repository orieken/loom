# Phase 5 — Testing Skills

## Goal
Create the skills agents use to write and audit tests. Each skill loads golden examples before it writes anything.

## Inputs
- `templates/SKILL-TEMPLATE.md`, the blueprints, and the pattern documents from phases 3 and 4.
- The skill folder your agent reads (`.claude/skills/<name>/SKILL.md` for Claude Code). Adapt the path for other tools.

## Steps
Create one skill per bullet. Use `templates/SKILL-TEMPLATE.md` for structure. Keep each `SKILL.md` under 150 lines.

1. **`api-test`** — writes API tests in the Sunday-like pattern.
   - Context to load: the order, `blueprints/api-sunday-like.md`, `docs/patterns/api-declarative-client.md`, each exemplar with `level: api`.
   - Process: read the OpenAPI document or endpoint list; build the coverage matrix; add or extend a client; add a model and
     a factory in separate files; write one test per matrix cell; validate schemas; set timeouts; show one failing run.
   - Guardrails: no `any`, no custom retries, no live third-party mutations without the approval gate.
2. **`ui-e2e-test`** — writes UI end-to-end tests in the Saturday-like pattern.
   - Context to load: the order, `blueprints/ui-e2e-saturday-like.md`, `docs/patterns/ui-e2e-site-centric.md`, each exemplar with `level: ui-e2e`.
   - Process: write the Gherkin scenario first, naming the acceptance criterion; reuse existing pages and partials; add missing
     elements to the right page; put multi-page work in a flow; show one failing run.
   - Guardrails: no classic page object; telemetry only in adapters; no fixed sleeps.
3. **`unit-test`** — writes unit and integration tests.
   - Context to load: the order, `blueprints/unit.md`, exemplars with `level: unit`.
   - Process: for existing legacy code, write characterization tests first, then change behavior in a separate commit.
   - Guardrails: Arrange, Act, Assert; one concept per test; doubles only at interfaces.
4. **`test-audit`** — finds gaps. It reports; it never writes tests.
   - Process: list pages, flows, clients, and endpoints; list scenarios and tests that reference them; report orphans (defined but unused),
     broken references (used but undefined), uncovered matrix cells, and tests with no issue or AC annotation.
   - Output: a ranked list of gaps, each with a suggested order title.
5. **`new-blueprint`** — adds a custom blueprint.
   - Process: interview the user with these questions, one at a time: What test type? Which language? Which framework and driver?
     Which sites or services? Which auth method (name only)? Which existing test is the best example? What speed budget? Then write
     `blueprints/custom/<id>.md` from the template, add the registry entry with `"custom": true`, create a stub skill
     `<id>-test` from the template, and run the blueprint validator.
   - Guardrails: ask before writing any file; never write secret values; stop if the validator fails.
6. **`exemplar-audit`** — checks golden examples (see phase 6). Read-only: it never edits a test or the manifest.

## Files
`.claude/skills/{api-test,ui-e2e-test,unit-test,test-audit,new-blueprint,exemplar-audit}/SKILL.md`, `scripts/validate-skills.<ext>`, and its tests.

## Verification
- Each skill's frontmatter `name` equals its folder name, and `standalone: true` is present.
- Each skill has the sections: When To Use, Context To Load First, Process, Output Format, Guardrails, Time-Box. Write a `validate-skills` script that checks the frontmatter, these headings, and the 150-line limit, and run it. Phase 7 fills in each Time-Box section.
- Quote a `description` that contains a colon followed by a space; an unquoted one is invalid YAML and the skill will not load.
- Dry-run `unit-test` on a small function: it writes a test, shows it failing once, then passing. Dry-run `api-test` and `ui-e2e-test` only if the framework for that test type exists. If it does not, do not fake the run: record `deferred to phase 10`.
- Dry-run `test-audit`. It returns at least a "no exemplars yet" finding, which phase 6 resolves for the levels that can have an exemplar now.

## Budget
`max-usd: 6`, `max-attempts: 2`, `max-hours: 3`.

## Time-box
Stage timeout 30 minutes. One skill per stage. A dry-run counts toward the time-box.
