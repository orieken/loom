# Eval Rubric: repo-doc-auditor / input-candidate.md

- **Catches the overreach**: `verdict` is `CHANGES_REQUESTED` and a **blocking** finding quotes the claim "Cryptographically
  signs every `Record` with a per-tenant key" as written. Approving the candidate is an automatic FAIL — it is the planted
  defect.
- **Says why, in terms of the facts**: the `problem` explains that the cited fields (`exports:Digest`, `exports:Record`)
  do not establish signing, keys or tenants — it points at what the sheet lacks, not just "this seems wrong".
- **Gives an actionable instruction**: the `instruction` tells the documenter something it can do from the fact sheet —
  move the claim to `openQuestions`, soften it to what `Digest` supports, or remove it. "Add more detail" or "be more
  accurate" is a FAIL.
- **No false positives**: the purpose ("Keeps a ledger of records and checks them with `Verify`.") and "Is relied on by
  `cmd/loom`" are supported by their citations and are **not** flagged as blocking. A blocking finding against either is
  a FAIL.
- **Does not do the documenter's job**: the output does not contain a rewritten document or a new purpose. It judges.
- **Does not read source it was not given**: no claim in a finding depends on the contents of a file the auditor read
  itself; nothing was recorded under `filesRead`, so there is nothing to check by reading.
- **Copied fields are exact**: `path` is `internal/ledger` and `inputHash` is the sheet's `input_hash`, character for
  character.
- **Output is JSON only**: one object, no markdown report around it.

## How to Grade
For each bullet, quote the specific field or value of `actual-output.md` that satisfies it. If a bullet has no supporting
quote, mark it FAIL and say what is missing. Approving the candidate, or blocking either sound claim, fails the whole
evaluation regardless of the other bullets.
