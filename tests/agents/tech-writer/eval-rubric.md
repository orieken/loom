# Eval Rubric: tech-writer / input-qa-report.md

- **StorageAdapter is documented as a new interface**: the docs-report explicitly notes the new `StorageAdapter` interface and either updates API docs or flags it as a docs task — not silently skipped.
- **New env vars appear in the right place**: `AWS_S3_BUCKET` and `AWS_REGION` are added to the environment configuration section of the README/runbook (not just noted as "needed"), with example values.
- **S3 lifecycle rule gap is surfaced**: the agent picks up the QA note that a 30-day lifecycle rule is recommended but not yet implemented and either opens a chore or includes it in the ops runbook as a manual step.
- **CHANGELOG entry is specific**: the CHANGELOG entry names the feature ("user avatar upload"), the new interface, and the new env vars — not a generic "added new feature" line.
- **ADR evaluated (even if not required)**: the agent considers whether the decision to hide S3 behind `StorageAdapter` warrants an ADR entry, and either creates one or explicitly decides it doesn't rise to the threshold with a brief reason.
- **Prose is in active voice**: more than 90% of the descriptive and procedural sentences in the docs-report, README text and CHANGELOG entry are active ("The adapter uploads the file"), not passive ("The file is uploaded"). Count the passives you find.
- **Sentence length is capped**: no instruction or step runs past 20 words and no explanation past 25 (`shared/rules/writing-style.md`). Quote the longest sentence you find.
- **Noun stacks are limited**: no run of more than 3 consecutive nouns. Quote any run you find.
- **Vocabulary is plain**: the text says "use", "before" and "about" instead of "utilize", "prior to" and "with regard to".
- **Terms stay consistent**: `StorageAdapter`, `AWS_S3_BUCKET` and `AWS_REGION` appear with the same spelling and meaning in every updated document, with no alternate name for the same thing.

## How to Grade
For each bullet, quote the specific line(s) of `actual-output.md` that satisfy it. If a bullet has no supporting quote, mark it FAIL and say what's missing.
