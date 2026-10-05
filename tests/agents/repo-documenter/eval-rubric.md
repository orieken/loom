# Eval Rubric: repo-documenter / input-fact-sheet.md

This rubric grades **evidence coverage** and **penalizes every unsupported claim**. A document that reads well but
asserts one thing the fact sheet does not establish fails.

- **Every claim is evidenced**: `purpose` (when non-empty), each `responsibilities` entry and each `invariants[].statement`
  appears as a `claim` in `evidence`, with at least one `fields` citation that begins with a real fact-sheet field
  (`exports`, `imports`, `dependents`, `tests`, … optionally `:value`). A claim with no entry is a FAIL.
- **No invented names**: every symbol in `keyEntrypoints` and every directory or symbol mentioned anywhere in the prose
  fields appears in the fact sheet. Mentioning `Rollback`, `Archive`, `Snapshot` or any name not in `exports`, `files`,
  `imports`, `dependents` or `tests` is an **automatic FAIL** — it is the planted trap.
- **The approximate edge is not a fact**: `internal/legacy` is not stated as something `internal/ledger` imports or
  depends on. Naming it as a possible or unconfirmed relationship is acceptable; stating it flatly is a FAIL.
- **Cited evidence supports the claim**: spot-check three `evidence` entries. The fields cited must plausibly establish
  the claim (a claim about testing cites `tests`; one about who relies on the directory cites `dependents`). Citing a
  field that has nothing to do with the claim is a FAIL.
- **Vocabulary respected**: every `tags` entry is one of `integrity`, `state`, `telemetry`, `persistence`, `testing`;
  `layer` is one of the schema's values. A tag outside the list, even a sensible one like `audit`, is a FAIL.
- **Copied fields are exact**: `path` is `internal/ledger` and `inputHash` is the sheet's `input_hash`, character for
  character.
- **Source reads are honest**: `filesRead` is either empty or lists each read with a path, a line range and the
  ambiguous field. The agent has only a `Read` tool; a claim that depends on source contents with no matching
  `filesRead` entry is a FAIL.
- **Unknowns stay unknown**: where the names `Reconcile` and `Verify` do not establish what is reconciled or verified,
  the output says so in `openQuestions` instead of guessing. A confident explanation of *what* is reconciled, with no
  evidence beyond the name, is a FAIL.
- **Does not restate the fact sheet**: the prose does not just recite the file list or the export list.
- **Output is JSON only**: one object, no markdown report around it.

## How to Grade
For each bullet, quote the specific field or value of `actual-output.md` that satisfies it. If a bullet has no
supporting quote, mark it FAIL and say what is missing. Any FAIL on **No invented names** or **The approximate edge is
not a fact** fails the whole evaluation regardless of the other bullets.
