---
feature: "intake-slice-check"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: intake-slice-check

## An order with no slice block validates exactly as before

Verifies: An order's frontmatter MUST accept an optional slice block (of, spec, digest, manifest, covers); an order with no slice block MUST validate exactly as it did before the field existed.

```gherkin
Scenario: An order with no slice block validates exactly as before
  Given An intake order with no slice block in its frontmatter
  When The order is checked
  Then The order validates exactly as it did before the slice field existed
```

## An order's optional slice block is accepted when present

Verifies: An order's frontmatter MUST accept an optional slice block (of, spec, digest, manifest, covers); an order with no slice block MUST validate exactly as it did before the field existed.

```gherkin
Scenario: An order's optional slice block is accepted when present
  Given An intake order whose frontmatter declares a slice block with of, spec, digest, manifest, and covers
  When The order is checked
  Then The slice block is parsed and accepted rather than rejected as an unknown field
```

## A bare covers entry covers every level its criterion has

Verifies: A spec criterion with observableVia api or ui MUST have one level to cover, and both MUST give it two (api and ui); AC1:ui covers only that level, a bare AC2 covers every level AC2 has.

```gherkin
Scenario: A bare covers entry covers every level its criterion has
  Given A spec criterion observable via both api and ui
  And A slice's covers list names that criterion with no level suffix
  When The order is checked
  Then The slice is treated as covering both the api level and the ui level of that criterion
```

## A level-suffixed covers entry covers only that level

Verifies: A spec criterion with observableVia api or ui MUST have one level to cover, and both MUST give it two (api and ui); AC1:ui covers only that level, a bare AC2 covers every level AC2 has.

```gherkin
Scenario: A level-suffixed covers entry covers only that level
  Given A spec criterion observable via both api and ui
  And A slice's covers list names that criterion with the :ui suffix
  When The order is checked
  Then The slice is treated as covering only the ui level, leaving the api level for another slice to cover
```

## An uncovered criterion level is reported by name

Verifies: Across every order sharing one slice.of, loom intake check MUST report a violation naming the criterion and level for an uncovered level, for a level two slices both cover (naming both orders), and for a covers entry naming a criterion or level the spec does not have.

```gherkin
Scenario: An uncovered criterion level is reported by name
  Given Several orders sharing one slice.of
  And A criterion level that no slice's covers list names
  When loom intake check runs over the directory
  Then The check reports a violation naming the uncovered criterion and level
```

## A criterion level covered by two slices is reported with both orders named

Verifies: Across every order sharing one slice.of, loom intake check MUST report a violation naming the criterion and level for an uncovered level, for a level two slices both cover (naming both orders), and for a covers entry naming a criterion or level the spec does not have.

```gherkin
Scenario: A criterion level covered by two slices is reported with both orders named
  Given Several orders sharing one slice.of
  And Two of those orders both list the same criterion level in their covers
  When loom intake check runs over the directory
  Then The check reports a violation naming the doubly covered level and both orders that cover it
```

## A covers entry naming an unknown criterion is reported

Verifies: Across every order sharing one slice.of, loom intake check MUST report a violation naming the criterion and level for an uncovered level, for a level two slices both cover (naming both orders), and for a covers entry naming a criterion or level the spec does not have.

```gherkin
Scenario: A covers entry naming an unknown criterion is reported
  Given An order whose covers list names a criterion id the spec does not declare
  When loom intake check runs over the directory
  Then The check reports a violation naming the unknown criterion and the spec that does not have it
```

## A covers entry naming a level its criterion does not have is reported

Verifies: Across every order sharing one slice.of, loom intake check MUST report a violation naming the criterion and level for an uncovered level, for a level two slices both cover (naming both orders), and for a covers entry naming a criterion or level the spec does not have.

```gherkin
Scenario: A covers entry naming a level its criterion does not have is reported
  Given An order whose covers list names a real criterion with a level suffix that criterion does not have
  When loom intake check runs over the directory
  Then The check reports a violation naming the criterion and the level it does not have
```

## Disagreeing slice digests are reported

Verifies: Slices of one spec MUST report a violation when their slice.digest values differ, when a digest does not match the SHA-256 of its manifest file, or when a manifest-listed file no longer matches its recorded hash, naming the file.

```gherkin
Scenario: Disagreeing slice digests are reported
  Given Two orders sharing one slice.of
  And Those orders declare different slice.digest values
  When loom intake check runs over the directory
  Then The check reports a violation naming the disagreement
```

## A slice digest that does not match its manifest is reported

Verifies: Slices of one spec MUST report a violation when their slice.digest values differ, when a digest does not match the SHA-256 of its manifest file, or when a manifest-listed file no longer matches its recorded hash, naming the file.

```gherkin
Scenario: A slice digest that does not match its manifest is reported
  Given An order's slice.digest value
  And That value does not equal the sha256 of the manifest file it names
  When loom intake check runs over the directory
  Then The check reports a violation naming the mismatch between the digest and the manifest
```

## A manifest-listed file that no longer matches its recorded hash is named

Verifies: Slices of one spec MUST report a violation when their slice.digest values differ, when a digest does not match the SHA-256 of its manifest file, or when a manifest-listed file no longer matches its recorded hash, naming the file.

```gherkin
Scenario: A manifest-listed file that no longer matches its recorded hash is named
  Given A manifest listing a file and its recorded sha256 hash
  And That file's current contents no longer match the recorded hash
  When loom intake check runs over the directory
  Then The check reports a violation naming the file that no longer matches its recorded hash
```

## A depends-on cycle among slices is reported naming every order in it

Verifies: Slices of one spec MUST report a violation for a depends-on cycle among them, naming every order in the cycle, and for a depends-on entry naming no order in the directory.

```gherkin
Scenario: A depends-on cycle among slices is reported naming every order in it
  Given Several orders sharing one slice.of
  And Their depends-on entries form a cycle among each other
  When loom intake check runs over the directory
  Then The check reports a violation naming every order on the cycle
```

## A dangling depends-on entry is reported

Verifies: Slices of one spec MUST report a violation for a depends-on cycle among them, naming every order in the cycle, and for a depends-on entry naming no order in the directory.

```gherkin
Scenario: A dangling depends-on entry is reported
  Given An order whose depends-on list names an id no order in the directory has
  When loom intake check runs over the directory
  Then The check reports a violation naming the missing order id
```

## An unreadable spec or manifest file is reported against the order naming it

Verifies: A spec or manifest file that cannot be read MUST be a violation on the order naming it, and a path that leaves the project root MUST be a violation; an unverified split must never be reported valid.

```gherkin
Scenario: An unreadable spec or manifest file is reported against the order naming it
  Given An order whose slice.spec or slice.manifest path cannot be read
  When loom intake check runs over the directory
  Then The check reports a violation against that order naming the file it could not read, and never reports the split as valid
```

## A slice path that leaves the project root is rejected without being opened

Verifies: A spec or manifest file that cannot be read MUST be a violation on the order naming it, and a path that leaves the project root MUST be a violation; an unverified split must never be reported valid.

```gherkin
Scenario: A slice path that leaves the project root is rejected without being opened
  Given An order whose slice.spec or slice.manifest path resolves outside the project root
  When loom intake check runs over the directory
  Then The check reports a violation naming the escaping path
  And The file at that path is never read
```

## A passing slice split prints one summary line per spec

Verifies: A passing split MUST print one line per spec with its slice and level counts, and the exit code contract MUST stay unchanged: 0 only when every order is valid.

```gherkin
Scenario: A passing slice split prints one summary line per spec
  Given Every order sharing one slice.of together covers every criterion level with no violation
  When loom intake check runs over the directory
  Then The check prints one line for that spec naming its slice count and level count
  And The check exits 0
```

## The existing exit-code contract is unchanged by the new check

Verifies: A passing split MUST print one line per spec with its slice and level counts, and the exit code contract MUST stay unchanged: 0 only when every order is valid.

```gherkin
Scenario: The existing exit-code contract is unchanged by the new check
  Given A directory of orders where every order is valid on its own but the cross-order slice check finds a violation
  When loom intake check runs over the directory
  Then The check exits non-zero, exactly as it already does for a single-file violation
```

## The intake template documents the slice block

Verifies: templates/claude-feature-team/intake/TEMPLATE.md and README.md MUST document the slice block, and the existing template vocabulary test MUST still pass.

```gherkin
Scenario: The intake template documents the slice block
  Given The intake TEMPLATE.md and README.md files
  When A person reads the intake template documentation
  Then They find the slice block's fields (of, spec, digest, manifest, covers) documented
  And The existing template vocabulary test still passes unmodified
```

## A realistic reagent-map-style slice split passes with no violations

Verifies: A fixture modeled on the reagent-map split MUST pass (AC1 and AC3 covered once at each level by different slices, AC14 covered bare by one slice), and each violation above MUST have a test built by breaking that fixture in one way, which fails when the check is removed.

```gherkin
Scenario: A realistic reagent-map-style slice split passes with no violations
  Given A fixture modeled on the reagent-map split: two slices covering AC1 and AC3 once at each level, and AC14 covered bare by one slice
  When loom intake check runs over the fixture
  Then The check reports no violations
  And The check prints the summary line for that spec
```

## The delivered change stays inside its complexity, coverage, and scope bounds

Verifies: Every new or changed function MUST have cyclomatic complexity below 7, every changed line MUST be covered by a test that can fail, and no file outside scope.paths MUST change except this run's own delivery record under docs/features/intake-slice-check/.

```gherkin
Scenario: The delivered change stays inside its complexity, coverage, and scope bounds
  Given The full set of files this feature changes
  When A complexity tool and a coverage tool are run over the diff
  Then Every new or changed function measures below cyclomatic complexity 7
  And Every changed line is covered by a test that can fail
  And No file outside the order's declared scope.paths has changed
```
