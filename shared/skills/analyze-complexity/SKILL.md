---
name: analyze-complexity
description: Cyclomatic complexity and function length analysis with two modes — enforcement (fails the pipeline when thresholds are exceeded) and on-demand (ranked report for ad-hoc analysis and refactoring planning). Supersedes complexity-check.
triggers:
  keywords: ["complexity", "refactor", "too long", "cyclomatic", "mccabe", "clean up", "technical debt", "refactoring"]
  intentPatterns: ["check code complexity", "enforce clean code rules", "is this function too long", "Check complexity of *", "Is * too complex?", "Complexity report on *", "What's the most complex file in *"]
standalone: true   # must work without MCP/external systems
---

## When To Use

**Enforcement mode** — use during code review, pull request checks, or after feature implementation. Runs `check.sh` against modified files and FAILs on any violation. Never approve code that exceeds the threshold.

**On-demand mode** — use when asked for a complexity overview of a file, directory, or package, or when the user mentions "technical debt" without a specific task. Produces a ranked report and refactoring roadmap; does not modify files.

Do NOT use either mode on data files, documentation, or configuration files (JSON, YAML, MD).

## Context To Load First
1. `ARCHITECTURE_RULES.md`
2. `.claude/feature-workspace/<feature-name>/implementation-notes.md` (if available, enforcement mode)
3. The target file(s) (on-demand mode)

## Modes

### Enforcement Mode

1. **Identify targets**: Modified or relevant code files (`.ts`, `.js`, `.py`, `.go`).
2. **Run check**: Execute `./.claude/skills/analyze-complexity/check.sh [target]`.
3. **Handle violations**: If exit code is non-zero, instruct the developer to refactor (Extract Method, Extract Variable, Replace Conditional with Polymorphism, etc.) until the `< 7` complexity and `< 30 LOC` limits are met. If significant refactoring alters the architecture, pause and ask for approval before applying.

### On-Demand Mode

1. **Identify language(s)** of the target.
2. **Run the appropriate tool**:
   - TypeScript: `npx eslint --rule '{"complexity": ["error", 6]}' [file]`
   - Python: `radon cc [file] -s` or `flake8 --max-complexity=6 [file]`
   - Go: `gocyclo -over 6 [file]`
   - Java: manual heuristics — recommend Checkstyle/SonarQube to user

   Note: tools take `6` as max-allowed (enforces `< 7`; a function failing at complexity 7 is the same threshold as the project-wide rule).
3. **For each violation**: name the code smell, name the Fowler refactoring operation, estimate effort (trivial / one session / needs design discussion).
4. **Rank findings** by impact: highest complexity first, then by call graph depth.
5. **Produce on-demand report** — see Output Format below.

## Output Format

### Enforcement Mode
```markdown
# Complexity Report

## Violations
- `path/to/file.ts`: Function `doComplexThing` complexity is 8 (Limit: 7)
- `path/to/file.ts`: Function `longMethod` length is 45 lines (Limit: 30)

## Recommended Refactorings
- [Named Fowler operation with justification]
```

### On-Demand Mode
Output to `.claude/feature-workspace/complexity-report.md`:

```markdown
# Complexity Report: [target]

Threshold: Cyclomatic complexity < 7 (ARCHITECTURE_RULES.md)

## Summary
- Files scanned: N
- Functions exceeding threshold: N
- Highest complexity: [function name] at [N]

## Findings (ranked by impact)

### [FunctionName] — complexity [N]
**File**: `path/to/file.ts` line X
**Smell**: [specific description]
**Refactoring**: [named Fowler operation]
**Effort**: trivial / one session / needs design discussion

## Refactoring Roadmap
1. [First because: quick win / unblocks other refactors / highest risk]

## What's Clean
[Functions within threshold — always acknowledge good work]
```

## Guardrails
- **Enforcement mode**: MUST NOT approve code that violates the complexity limit without explicit human approval. Do NOT silently ignore failures from `check.sh`.
- **On-demand mode**: Read-only. Never modify files. Never run tests. Fall back to heuristic analysis if CLI tools are unavailable — still produce a report and note it is heuristic.
- When complex refactoring would alter architecture significantly, pause and ask before applying.

## Standalone Mode
If `check.sh` is unavailable, manually count `if`, `else`, `for`, `while`, `switch` statements for cyclomatic complexity and count LOC manually. Use per-language tool list above for on-demand mode; fall back to heuristic reading if tools are not installed.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
