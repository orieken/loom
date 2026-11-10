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

## Mermaid diagrams

- Add a Mermaid diagram when relationships, interactions, branches, or state changes are easier to understand visually.
  Use a flowchart for decisions or context maps, a sequence diagram for interactions, and a state diagram for lifecycles.
- Keep simple facts and linear instructions in prose. Diagram selection is judgment-only: usefulness depends on the reader's task.
- Use fenced `mermaid` blocks. Give each diagram a descriptive title and an adjacent plain-language explanation of its main point.
  The explanation must let readers understand the point without rendering the diagram.
- Preserve canonical domain terms. Name bounded contexts and label crossings; distinguish commands from events and show event names as defined.
  Do not invent architecture, behavior, or decisions to complete a diagram. Flag unresolved details as open questions.
- Keep diagrams focused on one question. Split crowded diagrams and explain unfamiliar labels in the surrounding text.
- Review diagrams against the spec and implementation, as applicable. Check rendering when a Mermaid renderer is available;
  otherwise report that rendering was not verified. Never claim a rendering check passed without running it.

## Mechanical check

Run `loom docs check <path>...` to find candidates against this rule in a file or a directory. The
command checks sentence length, plain vocabulary, modal conflicts, an estimated passive-voice
share, and `DOMAIN_DICTIONARY.md` "Synonyms to AVOID" terms. It skips a synonym that is one
lowercase word, such as `check` or `tool`, because in prose that word is usually ordinary English. A human or agent reviewer MUST still
check active voice as a whole, noun clusters, and one-term-one-meaning: the command does not check
them. It also does not check a run of more than three consecutive nouns. Reliable detection needs
part-of-speech tagging, and neither the Go standard library nor this project's dependencies carry
one. `loom docs check` is a report, not a gate: it names a candidate, and a person or agent still
has to fix it.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
