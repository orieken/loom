# Writing Style: Simplified Technical English (ASD-STE100)

Applies to agents that write prose other agents or people act on: `spec-writer`, `analyst`, `tech-writer`. Not part
of the core rules: only those agents read it, so the other stages do not pay for it (`shared/levels.yaml`
`writing-rules` bundle).

## Rules

- **Sentence length.** Keep procedural steps under 20 words and descriptive sentences under 25 words. Keep
  paragraphs to 3–4 sentences at most.
- **Active voice and imperative verbs.** Use active voice for all acceptance criteria and procedural prose. Write
  "Display an error message", not "An error message should be displayed". In documentation, keep more than 90% of
  descriptive and procedural prose in active voice.
- **Noun clusters.** Allow no more than 3 consecutive nouns. Write "Token for user authentication status", not "User
  status authentication token".
- **Unambiguous modals.** Use the RFC 2119 keywords `MUST`, `SHOULD` and `MAY` for requirements, and only for
  requirements. Do not state a requirement with "can", "might" or "ought to".
- **Plain vocabulary.** Prefer "use" over "utilize", "before" over "prior to", "about" over "with regard to".
- **One term, one meaning.** Use each term the same way in every document and reference. Do not vary a term for
  style.

## Exception: domain vocabulary

Do not force the STE dictionary on established Ubiquitous Language from `DOMAIN_DICTIONARY.md` or on native software
engineering terms (for example `payload`, `idempotent`, `middleware`). Use those terms exactly as defined, every time.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
