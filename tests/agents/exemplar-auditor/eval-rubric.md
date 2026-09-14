# Eval Rubric: exemplar-auditor / input-exemplar-set.md

- **The rate-limit exemplar is disqualified as vacuous**: the only assertion is `expect(response).toBeDefined()`, and the test mocks `isOverLimit` to return true and then asserts nothing about the rejection. The auditor must answer NO to "would it fail if the behavior broke" and name it — self-fulfilling mock, vacuous assertion, or both. This is the single most important finding in the fixture.
- **It is also flagged against its own `demonstrates` claim**: the claim says it asserts status, headers and retry-after; the test asserts none of them. A verdict of PARTIAL or NO on "demonstrates what it claims", with the specific gap named.
- **The missing e2e file is reported as a mechanical disqualifier**, not as a judgement call — `e2e/checkout.feature` does not exist. Naming the rename to `purchase-flow.feature` as the likely cause is good; asserting it as fact without evidence is not required.
- **The rate-limit exemplar's missing `@ac` annotation is noticed**: it carries `@issue` but no `@ac`, which `testing-conventions.md` requires of every test and which an exemplar violating the convention it demonstrates makes worse.
- **The user-service exemplar passes**: table-driven, descriptive case names, boundary values on the password-length dimension (minimum and one below), both annotations present. It should not be flagged.
- **The integration coverage gap is reported**: the project tests at integration level and declares no exemplar for it. Unit, api-contract and e2e are declared, so integration is the only genuine gap — the auditor must not invent gaps for languages or levels the project does not test.
- **Nothing is modified**: the output recommends actions for a human and never claims to have fixed, removed, or rewritten a test or the manifest.

## How to Grade
For each bullet, quote the specific line(s) of `actual-output.md` that satisfy it. If a bullet has no supporting quote, mark it FAIL and say what's missing.
