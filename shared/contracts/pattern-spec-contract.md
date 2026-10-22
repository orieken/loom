# Contract: pattern spec (`*.pattern.yaml`)

**Produced by**: humans writing a spec by hand today. Later, the proposal step (pattern mining F4) writes specs
from mined clusters, and a human approves them.
**Consumed by**: `loom health`, which validates every spec it finds; `validate-artifact` for one file; the miner
(F3) and the renderers of `loom forge` (F5), which read and write specs only through `internal/patternspec`.

A pattern spec describes one repeatable workflow: what it achieves, when to offer it, the values that vary, the
steps in order and who runs each one, how to check the result, and where the spec came from.

## Where specs live

| Directory | Holds | Status allowed |
|---|---|---|
| `shared/patterns/` | Promoted specs, shared with every project | `approved`, `forged`, `retired` |
| `shared/patterns/examples/` | Reference examples. They are validated, but are not promoted specs | `approved`, `forged`, `retired` |
| `.claude/patterns/` | Project-local specs, including candidates | any |

- The file is `<name>.pattern.yaml`, and `name` MUST match `<name>`. Other files in these directories are ignored,
  and subdirectories other than `examples/` are not scanned.
- A local spec with the name of a shared spec overrides it. `loom health` warns, because the override hides the
  shared version.

## Structure

The JSON Schema `shared/schemas/pattern-spec.schema.json` holds the structure. It is generated from the Go types
in `internal/patternspec`; change the types, then run `go run ./cmd/gen-schemas`. Every object rejects fields it
does not define.

| Field | Required | Notes |
|---|---|---|
| `schemaVersion` | yes | `1`. A spec with another version is refused before anything else is checked |
| `name` | yes | kebab-case |
| `version` | yes | `MAJOR.MINOR.PATCH` |
| `intent` | yes | One sentence |
| `owner` | yes | The team or person who answers for the pattern |
| `triggers.keywords`, `triggers.intentPatterns` | yes | Both keys MUST exist, the same as skill triggers. An empty list is allowed |
| `inputs[]` | no | `name` (lowerCamelCase), `type`, `description`; optional `required`, `values`, `default` |
| `preconditions[]` | no | What MUST be true before the first step |
| `steps[]` | yes, at least one | `id` (kebab-case), `description`, `executor`; optional `consumes`, `produces`, `reason` |
| `verify[]` | no | `kind` (`skill` or `tool`) and `name` |
| `provenance` | yes | `source` (`hand-written` or `mined`), `status`; `evidence` for a mined spec |

Input types are a closed set: `string`, `number`, `boolean`, `path`, `enum` and `list` (a list of strings).

## Rules the schema cannot express

`loom health` applies these after the structure passes, in this order:

1. **Step ids are unique.**
2. **Human steps give a reason, and only human steps do.** The reasons are `approval`, `irreversible` and
   `judgment-out-of-scope`. The executor tag drives rendering in F5, and a human step without a reason gives a
   reviewer nothing to judge.
3. **A step consumes only what exists.** Every name in `consumes` MUST be an input or a name an earlier step
   `produces`. Steps are a linear list, so "earlier" means list order.
4. **Input names are unique.**
5. **Only an enum input lists `values`, and an enum input MUST list them.**
6. **A default matches its type.** A `required` input has no default. An enum default MUST be one of its values.
7. **Verify references exist.** A `skill` is a directory under `.claude/skills/` of the project or `shared/skills/`
   of the framework. A `tool` is a tool loom's MCP server registers.
8. **A mined spec carries its evidence**, and `lastSeen` is not before `firstSeen`.
9. **WARN: an input no step consumes.** It is allowed, because a mined spec can carry one, but a reviewer SHOULD see
   it.

Two more rules depend on where the file is: the `name` matches the file name, and a `candidate` MUST NOT live in
`shared/patterns/`.

## What a spec MUST NOT hold

- **No references to other pattern specs** in version 1. Each spec stands alone.
- **No branching or nesting.** Steps are a linear list in version 1.
- **No captured content in evidence.** `provenance.evidence` holds a cluster id, a count, repository identifiers and
  two dates. It never holds prompt text, code or file paths (`architecture-guardrails.md` #9).

## Trust

A spec is untrusted input. Loading it never runs, includes or fetches anything. A file over 256 KB is refused from
its size, before it is read. Messages name the file, the field path, step ids and allowed values, and never repeat
free text from the spec.

## Validation Rule

`validate-artifact` checks one spec by running `loom health` in the project, or by applying the structure table and
the rules above by hand when the CLI is not available. `loom health` is the enforcement path, and its result is
authoritative. Version changes are recorded in `shared/schemas/CHANGELOG.md`.

An example of each executor is in `shared/patterns/examples/`.
