---
type: feature
status: in-progress
created: 2026-10-21
---

# Define the pattern spec schema

## Summary
Give every repeatable workflow one typed, versioned file: a pattern spec. Mining (F3), proposals (F4) and rendering (F5) then share one shape instead of each parsing prose. Invalid specs fail in the editor and in `loom health`, and the error names the field.

## Acceptance Criteria
- [ ] Given a valid pattern spec in `.claude/patterns/`, when the user runs `loom health`, then the pattern line passes and counts the spec.
- [ ] Given a pattern spec with a missing required field, when the user runs `loom health`, then health fails and names the file and the field path.
- [ ] Given a pattern spec with an unknown field, when the user runs `loom health`, then health fails and names the file and the field path.
- [ ] Given a step without an executor, when the user runs `loom health`, then health fails and names the step id.
- [ ] Given a step with an executor other than `tool`, `skill` or `human`, when health runs, then health fails and lists the allowed values.
- [ ] Given a `human` step without a reason, when the user runs `loom health`, then health fails and lists the allowed reasons.
- [ ] Given a non-human step with a reason, when the user runs `loom health`, then health fails for that step.
- [ ] Given two steps with the same id, when the user runs `loom health`, then health fails and names the duplicate id.
- [ ] Given a step that consumes a value that no input and no earlier step produces, when health runs, then health fails and names the value.
- [ ] Given an input of type `enum` without values, when the user runs `loom health`, then health fails for that input.
- [ ] Given an input with a default that does not match its type, when the user runs `loom health`, then health fails for that input.
- [ ] Given a required input that also has a default, when the user runs `loom health`, then health fails for that input.
- [ ] Given a `verify` entry that names no existing skill or tool, when the user runs `loom health`, then health fails and names the reference.
- [ ] Given a spec whose `name` differs from its file name, when the user runs `loom health`, then health fails and shows both names.
- [ ] Given a spec with `source: mined` and no evidence, when the user runs `loom health`, then health fails for `provenance.evidence`.
- [ ] Given evidence whose last-seen date is before its first-seen date, when health runs, then health fails for `provenance.evidence`.
- [ ] Given a spec with status `candidate` in `shared/patterns/`, when health runs, then health fails and says that shared specs MUST be approved.
- [ ] Given a schema version that the loader does not know, when health runs, then health fails and names the supported versions.
- [ ] Given a local spec with the name of a shared spec, when health runs, then health warns that the local spec overrides the shared one.
- [ ] Given a project without pattern specs, when the user runs `loom health`, then the pattern line passes and reports zero specs.
- [ ] Given an invalid pattern spec open in VS Code or Cursor with the settings template applied, when the user edits it, then the editor marks the invalid field.
- [ ] Given the two example specs, when health runs, then both validate. A reviewer MUST be able to follow each one without reading the schema.
- [ ] Given a change to the Go types, when the committed JSON Schema is not regenerated, then the test suite fails.
- [ ] Given a fixture spec, when the Go validator and the JSON Schema both check it, then both MUST agree on every structural rule.

## Out of Scope
- Rendering a spec into a skill, tool stub or runbook (F5).
- Mining or proposing specs (F3, F4).
- Any UI, and any command that creates or edits a spec.
- Branching, nested or conditional steps. Version 1 is a linear list.
- References from one pattern spec to another.
- Installing `shared/patterns/` into projects. This feature only validates the specs that exist.
- Hub packaging and signing (F8).

## Hypothesis and Target Metric
- We believe one typed schema will remove per-feature parsing of pattern specs. F3, F4 and F5 MUST read and write specs only through the shared Go package, measured by the importers of that package and by zero custom YAML parsers for patterns, by the merge of F5. The date is an open question.

## Edge Cases and Failure Modes
- Empty file: health fails and says the file holds no spec.
- YAML that does not parse: health fails with the file and the line, and prints no file content.
- A file in a pattern directory without the `.pattern.yaml` suffix: health ignores it.
- A spec with zero steps: health fails. A pattern MUST have at least one step.
- An input that no step consumes: health warns. Mined specs can carry unused inputs, but a reviewer SHOULD see them.
- A very large file (over 256 KB): health fails without reading the rest of the file.
- Two specs in one directory with the same `name`: health fails and names both files.
- An unreadable pattern directory: health fails and names the directory.

## Domain Language
- **Bounded context**: `Agent Orchestration`. A pattern spec is a canonical definition in `shared/`, like an agent or a skill.
- **Pattern Spec**: A typed YAML file that describes one repeatable workflow. It holds intent, triggers, inputs, preconditions, steps, verification and provenance.
- **Executor Tag**: The mandatory value on each step that names who runs it: `tool` (deterministic Go or MCP), `skill` (model judgment) or `human` (a person). It drives rendering in F5.
- **Human Step Reason**: Why a step stays with a person: `approval`, `irreversible` or `judgment-out-of-scope`.
- **Pattern Provenance**: Where a spec came from (`hand-written` or `mined`), the evidence for a mined spec, and its lifecycle status.
- **Pattern Status**: `candidate`, `approved`, `forged` or `retired`. Only the last three are allowed in `shared/patterns/`.
- **Shared Pattern** and **Local Pattern**: A spec in `shared/patterns/` and a spec in `.claude/patterns/`. A local spec overrides a shared spec of the same name.

## Non-Functional Requirements
- Health latency: loading and validating 100 specs MUST add less than 200 ms to `loom health` on a warm cache.
- File size cap: 256 KB per spec. A larger file MUST fail without a full read.
- Determinism: the same files MUST give the same messages in the same order.
- Network: none. Validation MUST work offline.
- Complexity: every function under cyclomatic complexity 7. Changed-line coverage of at least 85%.

## Trust Boundaries
- Pattern spec files → loader: treat them as untrusted input. Later, F4 writes specs from model drafts, and F8 brings them from a hub. A spec MUST NOT run, include or fetch anything at load time.
- YAML parsing: anchors, aliases and custom tags MUST NOT expand into an unbounded structure. The loader MUST reject a file over the size cap before it parses it.
- Spec text in messages: health output names files, field paths and ids. It MUST NOT print field values that come from the spec, except the names it compares (`name` and the file name).
- Evidence: `provenance.evidence` holds counts, dates, a cluster id and repo identifiers only. It MUST NOT hold prompt text, code or file paths (`architecture-guardrails.md` #9).

## Test Approach
Go unit and integration tests. No Saturday or Sunday.
- Table-driven tests for each validation rule, with one valid and one invalid fixture per rule, under `testdata/`.
- A schema test: the committed JSON Schema matches the schema generated from the Go types (the same pattern as `cmd/gen-schemas`).
- An agreement test: every structural fixture gives the same verdict from the Go validator and from the JSON Schema.
- A health integration test: a project tree with valid, invalid and overriding specs gives the expected lines and exit code.
- A test that both example specs validate.
- A benchmark of 100 specs for the latency budget.

## Open Questions
- The target date for the hypothesis: F5 has no date yet. Assumption: measure the metric at the F5 merge.
- Tool names for `verify`: the MCP registry holds the tool names, but scripts such as `check-agent-versions-ci.sh` are not tools. Assumption: `verify` names skills and registered MCP tools only, and scripts wait for F5's tool stubs.
- Repo identifiers in evidence: a plain repo name can be sensitive once F8 shares specs. Assumption: a free string in version 1, and F8 decides whether to hash it.
- Does `loom health` validate the specs in `shared/patterns/` of an installed project, or only in this repository? Assumption: both pattern directories that exist in the project tree.

## Decisions
- Steps are a linear list in version 1. No nesting, branching or conditions.
- Input types are a closed set: `string`, `number`, `boolean`, `path`, `enum` (with `values`) and `list` (a list of strings).
- File layout is flat: `shared/patterns/<name>.pattern.yaml` and `.claude/patterns/<name>.pattern.yaml`. The kebab-case `name` MUST match the file name. A local spec overrides a shared spec of the same name, and health warns.
- A pattern spec MUST NOT reference another pattern spec in version 1.
- The Go types are the source of truth. `cmd/gen-schemas` generates `shared/schemas/pattern-spec.schema.json`, and a test fails when the committed schema drifts. The contract `shared/contracts/pattern-spec-contract.md` holds the rules a JSON Schema cannot express: step references, evidence rules and the status rule for shared specs.
- Each spec carries `schemaVersion: 1`. A schema change bumps the version and adds a row to a new `shared/schemas/CHANGELOG.md`, in the same way agents carry a version and a CHANGELOG row.
- Field names use lowerCamelCase, the same as skill frontmatter (`intentPatterns`). `triggers` uses the same `keywords` and `intentPatterns` keys as skills.
- Examples live in `shared/patterns/examples/`, so they are validated but are not promoted specs. Suggested examples: `regenerate-codemap` (mostly `tool`) and `bump-agent-version` (`skill` edit, `tool` check, `human` approval).

## Infrastructure / Deploy Notes
- New files: `shared/schemas/pattern-spec.schema.json` (generated), `shared/schemas/CHANGELOG.md`, `shared/contracts/pattern-spec-contract.md`, `shared/patterns/examples/*.pattern.yaml`, and a Go package under `internal/` for the types, the loader and the validator.
- `.vscode/settings.json.example` and `.cursor/settings.json.example` gain a `yaml.schemas` entry for `shared/patterns/**/*.pattern.yaml` and `.claude/patterns/*.pattern.yaml`.
- `loom health` gains a pattern line: the spec count, then failures and warnings.
- `DOMAIN_DICTIONARY.md` gains the terms above.
- No environment variables, no migration and no change to `episodes.db`. Standard deploy.
