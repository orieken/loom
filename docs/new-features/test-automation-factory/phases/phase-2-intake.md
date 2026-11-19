# Phase 2 — Intake

## Goal
Give the factory a front door: a work-order template, a README that explains it, and a validator that rejects bad orders.

## Inputs
- `templates/INTAKE-TEMPLATE.md` in this pack.
- The language the project uses for scripts (Node, Python, or shell), chosen by a human in phase 0. Do not mix languages.
- A YAML library, because frontmatter parsing must be strict and no language here parses YAML in its standard library
  (Node: `yaml`; Python: `PyYAML` or `ruamel.yaml`). Add it as a dev dependency, and tell the human which one you added.
- The test runner the human chose in phase 0, with a coverage report. If it needs a glob instead of a bare directory to find tests, use the glob.

## Steps
1. Copy `templates/INTAKE-TEMPLATE.md` to `intake/TEMPLATE.md`.
2. Write `intake/README.md`. Cover: the lifecycle (`draft`, `ready`, `hold`), who writes which field, the trust
   field, autonomy, and the budget block. State that a runner never edits an order.
3. Write `scripts/validate-intake.<ext>` as a small program that follows the validator behavior in `CONTRACTS.md` (paths, output, exit codes) and is run as `validate:intake`. It MUST:
   - parse the frontmatter strictly, reject unknown keys at the top level and inside `target`, `budget`, and `scope`,
     and reject duplicate keys;
   - require `intake`, `id`, `type`, `test-type`, `blueprint`, `status`, `priority`, `created`, `budget`;
   - require the `id` to equal the file name without `.md`;
   - require every `budget` field except `deadline`; require `max-usd` above 0;
   - check `deadline`, when present, is ISO 8601 with an offset and lies in the future;
   - check `blueprint` exists in `blueprints/registry.json` once that file exists, and that the order's `test-type` equals the entry's
     `testType`. The file shape is `{ "schema": 1, "entries": [ { "id": "...", "testType": "..." } ] }`. Before phase 3,
     print a warning and skip the check;
   - check every `depends-on` id names an existing `intake/<id>.md`;
   - check `target.environment` is `local` or `staging`;
   - check `target.auth` never holds a secret value: allow only `none`, `test-account`, or `token-env-var:NAME`;
   - for `trust: external`, scan the body for instruction-override text such as "ignore previous", "disregard your rules",
     "you are now", "bypass the gate". Report each hit with a line number. Exit non-zero;
   - skip `TEMPLATE.md` and `README.md`;
   - keep every function under complexity 7: put the key vocabulary in a data module and write one small function per rule;
   - exit 0 when every order is valid and exit 1 otherwise.
4. Write unit tests for the validator. Cover each rule above with one passing and one failing case. Follow the
   rules from phase 1 (complexity below 7, interface for the filesystem).
5. Write one real sample order, `intake/sample-orders-api-tests.md`, with `status: draft`. Run the validator on it.

## Files
`intake/TEMPLATE.md`, `intake/README.md`, the helper modules the validator needs (a vocabulary of allowed keys, a file-system port), any dev dependency you added to run scripts (tell the human), `intake/sample-orders-api-tests.md`, `scripts/validate-intake.<ext>`,
`tests/scripts/validate-intake.<test-ext>`.

## Verification
- The validator exits 0 on the sample and exits 1 on a copy with an unknown key.
- A copy with `trust: external` and the text "ignore previous instructions" fails with a line number.
- A copy with `deadline` in the past fails.
- The validator tests pass. Changed-line coverage is at least 85%. Break one rule in the validator, see a test fail, restore it.
- The template and the validator agree: every key in the template is accepted by the validator.

## Budget
`max-usd: 4`, `max-attempts: 2`, `max-hours: 2`.

## Time-box
Stage timeout 30 minutes. Writing the validator and its tests is the longest step. If it is not green after
two stages (60 minutes of stage time), stop and report where it stands.
