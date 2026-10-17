# Eval Rubric: spec-writer / input-user-request.md

- **System preference is raised as a question**: the agent asks (or assumes and documents) whether the app should respect `prefers-color-scheme` OS/browser preference automatically, independent of the user toggle — this is a non-obvious product decision the request leaves ambiguous.
- **Persistence mechanism is specified**: the spec states WHERE the preference is stored (`localStorage`, user DB column, or cookie) and WHY — not just "remember the preference."
- **Acceptance criteria are given/when/then**: each AC is in observable, behavior-level language (e.g., "Given a user who set dark mode and refreshes the page, the app loads in dark mode") — not implementation language ("The ThemeContext stores the preference").
- **Scope is explicit**: something is explicitly put Out of Scope (e.g., per-page theme overrides, theme for email templates, admin-forced themes) — the spec writer challenged the user to narrow scope rather than leaving it open.
- **Hypothesis is asked for, not invented**: the spec has a `## Hypothesis and Target Metric` section. The request names no metric, so the agent MUST NOT make one up. It records the missing metric, amount and date as unresolved or as an Open Question, and the critique lists the gap.
- **Edge cases are separate and linked to criteria**: the spec has a `## Edge Cases and Failure Modes` section (for example, saving the preference fails). The critique checks that each edge case has a matching acceptance criterion.
- **Readiness critique is run**: the agent explicitly runs its readiness checklist (or equivalent) and either declares the spec ready or lists what's still missing before it can enter the delivery pipeline.
- **Acceptance criteria are in active voice**: every criterion names who or what acts and uses an imperative or active verb ("Display an error message", "The app loads in dark mode") — none reads "should be displayed", "is shown" or another passive form (`shared/rules/writing-style.md`).
- **Sentences stay short**: no step or criterion runs past 20 words, no descriptive sentence past 25, and no paragraph past 4 sentences. Quote the longest sentence you find.
- **Requirements use RFC 2119 keywords**: requirements are marked `MUST`, `SHOULD` or `MAY`, and a requirement is never stated with "can", "might" or "ought to".
- **Noun clusters are capped**: no run of more than 3 consecutive nouns (for example "user status authentication token"). Quote any run you find.
- **Domain terms are kept as defined**: terms such as `prefers-color-scheme`, `localStorage` or `idempotent` are not reworded into plain English.

## How to Grade
For each bullet, quote the specific line(s) of `actual-output.md` that satisfy it. If a bullet has no supporting quote, mark it FAIL and say what's missing.
