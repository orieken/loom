# Contract: Exemplar Tests

**Declared in**: `.claude/exemplars.yaml` (project-local)
**Consumed by**: every agent that writes a test, `exemplar-auditor`, `scripts/health-check.sh`
**Registered as**: the `test-exemplars` source in `shared/memory-registry.json`

An **Exemplar Test** is a real, executing test in a project's own suite, marked as the one to
imitate. Agents in this framework write tests constantly and learn the house pattern from prose —
`testing-conventions.md` states rules and `docs/patterns/testing-pyramid.md` states philosophy, and
neither shows one good test. This is that.

The framework already does this for its own agents: `shared/agents/memory-auditor.md` is described
as "the pattern exemplar every new counter agent should follow." Same idea, applied to tests.

---

## What an exemplar is not

- **Not a golden file.** `DOMAIN_DICTIONARY.md` reserves that term for the structural check over
  `tests/agents/*/actual-output.md`. A golden file is a recorded baseline you compare *against*; an
  exemplar is a specimen you *copy from*. Opposite instructions.
- **Not a protected test, yet.** Marking a test as an exemplar does not currently stop an agent
  editing it. That barrier is deliberately a separate decision — see the roadmap.
- **Not the most important test.** The criterion is "what should someone copy", not "what would hurt
  most to lose". Those select different tests, and conflating them gives you neither.

## Where an exemplar lives

**In the project's own suite, in place.** Never copied into a `.claude/` directory. An exemplar the
real suite does not execute stops being evidence the moment it stops passing, and nothing notices.
The mark travels with the test; only the index lives elsewhere.

## Marked twice, and the disagreement is the check

1. **An annotation on the test**, language-native, per `testing-conventions.md`'s Test Annotation
   Convention — the same mechanism as `@issue` and `@ac`, not a new format.
2. **An entry in `.claude/exemplars.yaml`**, carrying the metadata a reader needs to choose between
   exemplars without opening all of them.

Two sources of truth is the obvious objection. The answer is that `health-check` asserts them equal
in *both* directions: every manifest entry's test carries the annotation, and every annotated test
appears in the manifest. Checked redundancy, not drift. Generating one from the other would mean
parsing six languages' test files, which is worse than the duplication.

## How to mark a test

Alongside the issue/AC annotation `testing-conventions.md` already requires, using the same native
mechanism:

| Language | Mark |
|---|---|
| TypeScript | `@exemplar` in the JSDoc block |
| Python | `@pytest.mark.exemplar` |
| Java | `@Tag("exemplar")` |
| C# | `[Trait("Exemplar", "unit")]` |
| Go | `// exemplar: <what it demonstrates>` above the function |
| Gherkin | `@exemplar` on the scenario |

## Manifest schema

```yaml
# .claude/exemplars.yaml
exemplars:
  - test: TestReportedDistinguishesUnmeasuredFromCheap  # the test's name, as the runner reports it
    file: internal/provider/claude/envelope_test.go     # repo-relative
    language: go                                        # go | typescript | python | java | csharp | kotlin | swift | rust
    level: unit                                         # unit | integration | api-contract | acceptance | e2e
    demonstrates: >                                     # why THIS one — what a reader should take from it
      Table-driven cases with descriptive names, boundary values on each input
      dimension, and one behaviour per subtest.
```

`demonstrates` is required and is the field that does the work. "A good unit test" helps nobody; an
exemplar that does not say what it is demonstrating is a file path.

## Coverage is bounded by the repository

Declare exemplars only for the `(language, level)` pairs a project actually has tests for. A Go
service is never asked for a Kotlin exemplar. Missing coverage for a pair the project *does* test is
a finding for `exemplar-auditor`, not a build failure — a project mid-adoption should not be blocked.

## Disqualifiers — mechanical, and checkable

An exemplar that meets any of these is not an exemplar, whatever the manifest says:

1. **It does not run**, or is not part of the suite. It proves nothing.
2. **It cannot fail.** Mutate the behaviour it covers; if no assertion fires, it is vacuous — and an
   exemplar that cannot fail teaches every test written after it to be equally hollow. This is
   `code-reviewer`'s Test Evidence criterion and `backfill-unit-tests` step 6, applied to the one
   test that propagates.

   Follow step 6's discipline exactly here, because a false clear on an exemplar propagates:
   **confirm the mutant landed and still builds** before believing any result, and mutate something
   the assertion depends on rather than whatever is easiest to edit.

   Three ways to be misled, and they point in opposite directions. A mutant that never applied, and
   one that applied but changed no behaviour, both read like "this test cannot fail" — the finding
   this disqualifier exists to make, so a false positive here is expensive. A mutant that broke the
   build reads like the opposite: the test command exits non-zero, which looks like the exemplar
   caught it, and the disqualifier is silently cleared. Step 6 has the full statement; do not
   restate a subset of it here.
3. **It lacks its issue/AC annotation**, which `testing-conventions.md` requires of every test. An
   exemplar violating the convention it is supposed to demonstrate is the worst case in the set.
4. **Cyclomatic complexity ≥ 7** in the test body (`analyze-complexity`). A test a reader cannot
   follow is not a pattern.
5. **Its filename breaks the `name.type.extension` convention** for its language.

## What no check can settle

Whether the exemplar is *good*. An auditor reading a test against this contract is making the same
class of judgement as `code-reviewer` reading a diff, and nothing verifies that judgement was made
honestly. The deterministic half is that an exemplar runs, can fail, carries its annotations, stays
simple, and has not silently changed. Taste stays human, and this contract does not pretend
otherwise.

## Exemplars are not gated, and that was measured rather than assumed

Marking a test as an exemplar does **not** stop an agent editing it. That looks like an oversight, so
here is the reasoning and the evidence behind it (roadmap L3.46).

The manifest addresses exemplars by **file path**, so a barrier could only fire on file changes. Over
this repository's own three exemplars:

| | |
|---|---|
| Commits touching an exemplar's file | 10 |
| …that created the exemplar | 3 |
| …that added the exemplar annotation | 1 |
| **…that changed the file without touching the exemplar** | **4** |
| **…that changed an exemplar's body after it was declared** | **0** |

A barrier would have halted four runs for edits to *neighbouring functions in the same file*, and
none for an actual exemplar change. A gate that halts wrongly every time it fires is one people learn
to approve without reading, which costs more than the protection is worth — and it would have been
protecting against something that has not yet happened once.

**What exists instead**: the manifest records a digest, `health-check` warns when it moves, and
`exemplar-auditor` reads the test and says whether it still holds. Detection and judgment, without a
barrier.

**What would reverse this**: an exemplar actually being edited in a way that degraded it, or function
-level addressing becoming cheap. The first is the signal to watch; the second was rejected in L3.40
because finding function boundaries in six languages is worse than the duplication it would remove.

*Sample: three exemplars, one repository, roughly three weeks. Small, and the honest basis for a
decision that is cheap to revisit.*

## Staleness

The manifest records each exemplar's digest. A changed digest does not mean the exemplar is wrong —
it means nobody has confirmed it is still right. `exemplar-auditor` re-reads it and says.

A stale exemplar is worse than none: it silently teaches the wrong pattern to every test written
after it drifts. It is a few-shot prompt delivered through the filesystem, and it should be reviewed
with that in mind.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
