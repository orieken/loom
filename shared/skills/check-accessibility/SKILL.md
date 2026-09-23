---
name: check-accessibility
description: Scans UI files (HTML, Vue, React) for semantic HTML and accessibility violations (e.g., onClick on div elements).
triggers:
  keywords: ["accessibility", "a11y", "semantic html", "wcag", "aria", "keyboard navigation"]
  intentPatterns: ["check frontend accessibility", "review ui for a11y", "is this html semantic"]
standalone: true   # must work without MCP/external systems
---

## When To Use
Use this skill when reviewing any changes to UI components, HTML templates, or frontend code (React, Vue, etc.).
Do NOT use when reviewing backend services, database migrations, or purely logic-based utility files.

## Context To Load First
1. `ARCHITECTURE_RULES.md`
2. `.claude/feature-workspace/<feature-name>/implementation-notes.md` (if available)

## Process
1. Identify the frontend UI files being modified.
   - What to do: List all modified `.html`, `.vue`, `.tsx`, `.jsx` files.
   - What to produce: A list of targets.
   - What to check: Ensure targets are actually frontend UI files.
2. Run the accessibility check.
   - What to do: Run `./.claude/skills/check-accessibility/check.sh [target]`
   - What to produce: The console output of the script.
   - What to check: Determine if any rules (like `onClick` on `<div>`, missing ARIA) are violated.
3. Handle violations.
   - What to do: If violations exist, instruct the developer to refactor using native semantic elements (`<button>`, `<nav>`) and correct ARIA attributes.
   - When to pause: If a significant UI redesign is required to fix the issue, pause for human approval.

## Output Format
```markdown
# Accessibility Report

## Violations
- `path/to/component.tsx`: Interactive `onClick` found on non-interactive `<div>`.
- `path/to/form.html`: `<input>` missing associated `<label>`.

## Recommended Fixes
- Replace `<div onClick={...}>` with `<button type="button" onClick={...}>`.
```

## Guardrails
- You MUST REJECT code that uses `div`-soup for interactive elements.
- Never approve PRs with new WCAG keyboard navigation violations.

## Standalone Mode
If `check.sh` is unavailable, manually inspect the file contents using `grep` or `Read` to search for `onClick` on `div`/`span` tags, missing `alt` attributes on `img` tags, and missing `aria-` labels on custom interactive components.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
