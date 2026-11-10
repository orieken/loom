---
feature: "doc-style-check"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: ["doc-style-check"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Acceptance Scenarios: doc-style-check

## Checking a file with a finding prints it and exits successfully by default

Verifies: Given `loom docs check <path>...`, it reports each finding as `file:line: rule: excerpt` and exits 0 by default.

```gherkin
Scenario: Checking a file with a finding prints it and exits successfully by default
  Given A Markdown file with one style finding
  When `loom docs check` runs against that file without `--strict`
  Then The finding is printed in the form `file:line: rule: excerpt`
  And The command exits with status 0
```

## Checking a file with a finding under --strict fails

Verifies: Given `--strict`, `loom docs check` exits 1 when there is a finding.

```gherkin
Scenario: Checking a file with a finding under --strict fails
  Given A Markdown file with one style finding
  When `loom docs check <file> --strict` runs
  Then The command exits with status 1
```

## Checking a clean file under --strict succeeds

Verifies: Given `--strict`, `loom docs check` exits 0 when there are zero findings.

```gherkin
Scenario: Checking a clean file under --strict succeeds
  Given A Markdown file with zero style findings
  When `loom docs check <file> --strict` runs
  Then The command exits with status 0
```

## A fenced code block containing a flagged word is never reported

Verifies: The check ignores fenced and inline code and does not count those words toward any rule.

```gherkin
Scenario: A fenced code block containing a flagged word is never reported
  Given A Markdown file with a fenced code block containing the word `utilize`
  When `loom docs check` runs against that file
  Then No plain-vocabulary finding is reported for the word inside the fenced code block
```

## A long heading is never reported for sentence length

Verifies: The check ignores headings and does not count them toward the sentence-length rule.

```gherkin
Scenario: A long heading is never reported for sentence length
  Given A Markdown file with a heading of 30 words
  When `loom docs check` runs against that file
  Then No sentence-length finding is reported for the heading
```

## A link's URL is ignored but its visible text is scanned

Verifies: The check ignores a link's target but still scans the visible link text as ordinary prose.

```gherkin
Scenario: A link's URL is ignored but its visible text is scanned
  Given A Markdown file containing the link `[utilize this](https://example.com/utilize)`
  When `loom docs check` runs against that file
  Then No finding is reported for the word `utilize` appearing inside the URL
  And A plain-vocabulary finding is reported for `utilize` in the visible link text
```

## A 26-word descriptive sentence is reported as too long

Verifies: Given prose, the check reports a descriptive sentence over 25 words.

```gherkin
Scenario: A 26-word descriptive sentence is reported as too long
  Given A Markdown paragraph containing a 26-word sentence
  When `loom docs check` runs against that file
  Then A sentence-length finding is reported for that sentence
```

## A 21-word numbered-list step is reported as too long

Verifies: Given prose, the check reports a numbered-list step over 20 words.

```gherkin
Scenario: A 21-word numbered-list step is reported as too long
  Given A Markdown file with a numbered-list item of 21 words
  When `loom docs check` runs against that file
  Then A sentence-length finding is reported for that numbered-list item
```

## A bulleted list item is judged against the descriptive-sentence threshold

Verifies: Given prose, a bulleted (unordered) list item is judged against the 25-word descriptive threshold, not the 20-word list-step threshold.

```gherkin
Scenario: A bulleted list item is judged against the descriptive-sentence threshold
  Given A Markdown file with a 21-word bulleted (unordered) list item
  When `loom docs check` runs against that file
  Then No sentence-length finding is reported, because 21 words is under the 25-word descriptive threshold that applies to a bulleted item
```

## The consecutive-noun rule is documented as not mechanically enforced

Verifies: The check reports a run of more than three consecutive nouns only if a deterministic check can do it without a new dependency; this feature decides it cannot, and records that decision in its own output/help text and in shared/rules/writing-style.md.

```gherkin
Scenario: The consecutive-noun rule is documented as not mechanically enforced
  Given The feature as shipped
  When A reader inspects `loom docs check`'s help text and `shared/rules/writing-style.md`
  Then Both state that automatic detection of a run of more than three consecutive nouns is not implemented
  And Both name the reason: reliable detection needs part-of-speech tagging not available in the Go standard library or go.mod without adding a new dependency
```

## 'prior to' is reported with its plain-vocabulary replacement

Verifies: Given prose, the check reports words the rule replaces (`utilize`, `prior to`, `with regard to`), naming the preferred plain-vocabulary replacement.

```gherkin
Scenario: 'prior to' is reported with its plain-vocabulary replacement
  Given A Markdown file containing the phrase "prior to shipping"
  When `loom docs check` runs against that file
  Then A plain-vocabulary finding is reported naming "before" as the replacement for "prior to"
```

## 'utilize' is reported with its plain-vocabulary replacement

Verifies: Given prose, the check reports words the rule replaces (`utilize`, `prior to`, `with regard to`), naming the preferred plain-vocabulary replacement.

```gherkin
Scenario: 'utilize' is reported with its plain-vocabulary replacement
  Given A Markdown file containing the phrase "utilize the API"
  When `loom docs check` runs against that file
  Then A plain-vocabulary finding is reported naming "use" as the replacement for "utilize"
```

## A sentence mixing an RFC-2119 keyword with a weaker modal is reported

Verifies: Given prose, the check reports `can`, `might` or `ought to` in a sentence that also contains `MUST`, `SHOULD` or `MAY`.

```gherkin
Scenario: A sentence mixing an RFC-2119 keyword with a weaker modal is reported
  Given A Markdown file containing the sentence "The service MUST retry, but it might also log the failure."
  When `loom docs check` runs against that file
  Then A modal-conflict finding is reported for that sentence
```

## A sentence with only a weak modal and no RFC-2119 keyword is not reported

Verifies: A sentence with only `can`, `might` or `ought to` and no RFC-2119 keyword is not reported.

```gherkin
Scenario: A sentence with only a weak modal and no RFC-2119 keyword is not reported
  Given A Markdown file containing a sentence with the word "might" and no occurrence of MUST, SHOULD or MAY
  When `loom docs check` runs against that file
  Then No modal-conflict finding is reported
```

## A file over the 10% passive threshold reports a file-level estimate

Verifies: Given prose, the check reports each file's share of likely passive sentences when that share is over 10%, labelled as an estimate.

```gherkin
Scenario: A file over the 10% passive threshold reports a file-level estimate
  Given A Markdown file where 3 of 20 prose sentences (15%) match the 'to be' + past-participle pattern
  When `loom docs check` runs against that file
  Then One file-level finding labelled "estimate: ~15% passive" is reported
```

## A file at or below the passive threshold reports no passive-voice finding

Verifies: A file at or below the 10% passive threshold reports nothing.

```gherkin
Scenario: A file at or below the passive threshold reports no passive-voice finding
  Given A Markdown file whose share of likely passive sentences is at or below 10%
  When `loom docs check` runs against that file
  Then No passive-voice finding is reported
```

## A listed synonym to avoid is reported naming its canonical term

Verifies: Given a project with a DOMAIN_DICTIONARY.md, the check reports a term from any table's 'Synonyms to AVOID' column used in prose, naming the canonical term to use.

```gherkin
Scenario: A listed synonym to avoid is reported naming its canonical term
  Given A project with a DOMAIN_DICTIONARY.md listing `bot` as a synonym to avoid for 'Agent'
  And A Markdown file using the word "bot" in prose
  When `loom docs check` runs against that file
  Then A finding is reported naming "Agent" as the canonical term to use instead of "bot"
```

## A skip-listed ordinary-English synonym is never reported

Verifies: A listed word that is also ordinary English counts only in its exact listed surface form, and the check has a tested, explicit way to skip a listed word entirely.

```gherkin
Scenario: A skip-listed ordinary-English synonym is never reported
  Given A project DOMAIN_DICTIONARY.md listing the ordinary-English word `data` as a synonym to avoid, but also on an explicit, tested skip-list
  And A Markdown file using the word "data" in prose
  When `loom docs check` runs against that file
  Then No finding is reported for that entry, even though "data" is a listed synonym
```

## A docstyle-ignore comment suppresses the matching finding and is counted

Verifies: Given `<!-- docstyle-ignore: <rule> -->` on the line before a finding, the finding is skipped, and the report counts how many findings were skipped this way.

```gherkin
Scenario: A docstyle-ignore comment suppresses the matching finding and is counted
  Given A Markdown file with a 30-word sentence
  And The line immediately before that sentence contains `<!-- docstyle-ignore: sentence-length -->`
  When `loom docs check` runs against that file
  Then No sentence-length finding is produced for that sentence
  And The report's skipped-findings count increases by one
```

## A docstyle-ignore comment naming the wrong rule does not suppress the finding

Verifies: A docstyle-ignore comment naming a different rule than the actual finding does not suppress it.

```gherkin
Scenario: A docstyle-ignore comment naming the wrong rule does not suppress the finding
  Given A Markdown file with a finding for one rule
  And The line immediately before it contains `<!-- docstyle-ignore: <a different rule> -->`
  When `loom docs check` runs against that file
  Then The finding is still reported
  And The report's skipped-findings count does not increase for it
```

## writing-style.md documents the mechanical check and its limits

Verifies: Given `shared/rules/writing-style.md`, it names the `loom docs check` command and states plainly which of its rules the command checks mechanically and which rules remain for a human or agent reviewer.

```gherkin
Scenario: writing-style.md documents the mechanical check and its limits
  Given The feature as shipped
  When A reader opens `shared/rules/writing-style.md`
  Then It names the `loom docs check` command
  And It states plainly which rules the command checks mechanically and which rules remain for a human or agent reviewer
```

## Every rule and every ignored region has trigger and non-trigger test coverage

Verifies: Given tests, each rule has a fixture that triggers it and a fixture that does not, and each ignored region has a test showing a rule violation inside it is not reported.

```gherkin
Scenario: Every rule and every ignored region has trigger and non-trigger test coverage
  Given The completed `internal/docstyle` test suite
  When The test suite runs
  Then Each rule (sentence-length, plain-vocabulary, modal-conflict, passive-voice, ubiquitous-language) has a passing fixture that triggers it and a passing fixture that does not
  And Each ignored-region type (frontmatter, fenced code, inline code, table, heading, link target, HTML comment) has a passing fixture proving a violation inside it is not reported
```

## Checking a directory recurses into nested Markdown files

Verifies: Given a path argument, the command also works against a directory, recursing into nested Markdown files, and reports a clear error for a path that does not exist.

```gherkin
Scenario: Checking a directory recurses into nested Markdown files
  Given A directory containing nested subdirectories with .md files, one of which has a known finding
  When `loom docs check <directory>` runs
  Then The finding from the nested file is reported
```

## A missing path reports a clear error instead of a finding

Verifies: A path argument that does not exist on disk is a usage error, not a finding, and is unaffected by --strict.

```gherkin
Scenario: A missing path reports a clear error instead of a finding
  Given A path argument that does not exist on disk
  When `loom docs check <missing-path>` runs
  Then The command returns a clear error naming the missing path, not a style finding
```

## The change stays within its complexity, coverage and scope budget

Verifies: Given the change is complete, every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, and no file outside `scope.paths` changes except this run's own delivery record under `docs/features/doc-style-check/`.

```gherkin
Scenario: The change stays within its complexity, coverage and scope budget
  Given The completed implementation and its tests
  When CI runs the complexity check and the test suite with coverage
  Then Every new or changed function has cyclomatic complexity below 7
  And Every changed line is covered by a test capable of failing
  And No file outside `internal/docstyle/**`, `cmd/loom/cmd/docs*.go`, `shared/rules/writing-style.md`, and `docs/features/doc-style-check/**` has changed
```
