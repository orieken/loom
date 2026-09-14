# Exemplar Audit — 2026-09-14

## Verdict
FINDINGS — 3 exemplars audited, 2 with findings

## Per Exemplar

### creates user with valid credentials — typescript/unit
- **Mechanical**: pass. File exists, test present, `@exemplar` and both `@issue` and `@ac`
  annotations carried, four cases with no nesting.
- **Digest**: matches.
- **Demonstrates what it claims**: YES. The claim is table-driven cases with descriptive names and
  boundary values on each input dimension. The password-length dimension is pinned on both sides —
  `'password at minimum length'` at 8 and `'password one below minimum'` at 7 — which is the part
  most tables omit, and each case name states the distinction it draws rather than numbering itself.
- **Would it fail if the behavior broke**: YES. The assertion is against `result.ok`, which the
  service computes; no case asserts a value the test configured.

### rejects a request over the rate limit — typescript/api-contract
- **Mechanical**: **FAIL — vacuous.** Also missing its `@ac` annotation; it carries `@issue
  PROJ-201` and nothing naming the criterion, which `testing-conventions.md` requires of every test.
- **Digest**: matches. The file has not changed; it was like this when it was declared.
- **Demonstrates what it claims**: NO. The claim is that it asserts the contract a consumer depends
  on — status, headers and retry-after. It asserts none of the three. A reader copying this learns
  to mock the thing under test and assert that a response object exists.
- **Would it fail if the behavior broke**: **NO.** Two separate defects, either one fatal:
  `jest.spyOn(limiter, 'isOverLimit').mockReturnValue(true)` makes the limiter's own decision a
  value the test supplied, so the rate-limiting logic is never exercised — a self-fulfilling mock.
  And `expect(response).toBeDefined()` cannot fail for a call that returns an object, whatever the
  status — a vacuous assertion. Delete the limiter entirely and this test still passes.

  This is the worst finding available: an exemplar that cannot fail teaches every api-contract test
  written after it to be equally hollow, and it has been the declared pattern for three months.

### user completes checkout — typescript/e2e
- **Mechanical**: **FAIL — the file does not exist.** `e2e/checkout.feature` is absent. `git log`
  shows `e2e/checkout.feature` renamed to `e2e/purchase-flow.feature` six weeks ago, which is the
  likely explanation, but I have not read the scenario at its new path and cannot confirm the
  content still matches the claim.
- **Digest**: cannot be computed — no file.
- **Demonstrates what it claims**: UNKNOWN. Needs `e2e/purchase-flow.feature`.
- **Would it fail if the behavior broke**: UNCERTAIN — same file.

## Coverage Gaps
- **typescript / integration** — the suite tests at integration level and declares no exemplar for
  it. Unit, api-contract and e2e are all declared, so this is the only genuine gap.

## Recommended Actions
1. **Retire the rate-limit exemplar today**, before its replacement is chosen. It is actively
   teaching a pattern the framework's own `code-reviewer` would now reject, and leaving it declared
   while a better one is written costs more than having no api-contract exemplar for a week.
2. Write or promote an api-contract exemplar that asserts the response contract directly and does
   not mock the component under test. Whoever writes it should state what it demonstrates before
   writing it — the current claim was accurate about the intent and never true of the code.
3. Repoint the e2e entry at `e2e/purchase-flow.feature` after reading it, and re-record the digest.
   If the scenario changed in the rename, re-confirm the `demonstrates` claim rather than carrying
   it over.
4. Add the missing `@ac` annotation wherever the api-contract exemplar ends up.
5. Declare an integration exemplar, or record that the project has decided not to have one.

No test, manifest entry, or source file was modified. Every action above is for a human.
