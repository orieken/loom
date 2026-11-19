---
name: kebab-case-name            # MUST equal the directory name
description: One sentence. Routing uses it, so say when to use the skill. Quote it if it contains a colon.
triggers:
  keywords: ["write api test"]
  intentPatterns: ["*api test*"]
standalone: true                 # a skill works without any other tool installed
---

# <Skill title>

## When To Use
- <Situation>

Do NOT use when:
- <Situation, and which skill to use instead>

## Context To Load First
1. The order file, if one exists.
2. The blueprint named in the order.
3. Each exemplar listed for this test type in `.claude/exemplars.yaml`. Read the test file the entry names.
4. `.claude/rules/` — all files.

## Process
1. <Step>
2. Write the test. Show that it can fail: break the behavior once, see the test fail, restore it.
3. Run the new tests and the complexity check.

## Output Format
<What the skill reports at the end: files written, tests added, coverage of changed lines, one failing run shown.>

## Guardrails
- Do not copy an exemplar's content. Copy its shape.
- Do not weaken an assertion to make a test pass.
- Stop and park the order when a budget or time-box is reached.

## Time-Box
Stop at the stage timeout (`stage-timeout-minutes` in the order). Before each stage, run the budget check; on `stop`, write a checkpoint (done, left, files touched) and park. Do not extend a limit yourself.
