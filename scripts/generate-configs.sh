#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_DIR="$REPO_DIR/shared"

OUTPUT_DIR=""
DRY_RUN=false
PLATFORM_FILTER=""
STACK_FILTER=""

usage() {
  cat <<'EOF'
Usage: generate-configs.sh [OPTIONS]

Generate platform-specific config files from the canonical shared/ layer.

Options:
  --output <dir>    Write generated files to <dir> (default: repo root)
  --platform <name> Only generate for a specific platform
  --stack <list>    Comma-separated stacks to include language rules for
                    (e.g. go,typescript). Core rules — approval-gates,
                    architecture-guardrails, design-principles, testing-conventions,
                    and memory-trust-boundary — are always included. Omitting this
                    flag includes all language convention rules (original behavior).
  --dry-run         Show what would be generated without writing
  -h, --help        Show this help

Platforms: claude-code, cursor, windsurf, github-copilot, gemini, openai-codex, jetbrains, roo-code, cline
Stacks:    go, typescript, python, csharp, java, kotlin, swift, rust, iac
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)    OUTPUT_DIR="$2"; shift 2 ;;
    --platform)  PLATFORM_FILTER="$2"; shift 2 ;;
    --stack)     STACK_FILTER="$2"; shift 2 ;;
    --dry-run)   DRY_RUN=true; shift ;;
    -h|--help)   usage ;;
    *)           echo "Unknown option: $1"; usage ;;
  esac
done

OUTPUT_DIR="${OUTPUT_DIR:-$REPO_DIR}"

# Stack auto-detection: --stack flag > $FRAMEWORK_STACK env var > framework-install.json
if [[ -z "$STACK_FILTER" ]]; then
  if [[ -n "${FRAMEWORK_STACK:-}" ]]; then
    STACK_FILTER="$FRAMEWORK_STACK"
  else
    _marker="$OUTPUT_DIR/.claude/framework-install.json"
    if [[ -f "$_marker" ]]; then
      _stack=$(grep '"stack"' "$_marker" | head -1 | sed 's/.*"stack": *"//; s/".*//' || true)
      [[ -n "$_stack" ]] && STACK_FILTER="$_stack"
    fi
    unset _marker _stack
  fi
fi

ok()   { echo "  [ok] $1"; }
dry()  { echo "  [dry-run] would generate $1"; }
skip() { echo "  [skip] $1"; }

should_generate() {
  [[ -z "$PLATFORM_FILTER" || "$1" == "$PLATFORM_FILTER" ]]
}

# Returns 0 (true) if the given language stack should have its convention rules
# included. When STACK_FILTER is empty every stack passes (original behavior).
stack_includes() {
  local lang="$1"
  [[ -z "$STACK_FILTER" ]] && return 0
  echo "$STACK_FILTER" | tr ',' '\n' | grep -qxF "$lang"
}

write_file() {
  local dest="$1"
  local content="$2"

  if $DRY_RUN; then
    dry "$dest"
    return
  fi

  mkdir -p "$(dirname "$dest")"
  echo "$content" > "$dest"
  ok "$dest"
}

collect_rules() {
  local result=""

  if [[ -z "$STACK_FILTER" ]]; then
    # No filter — include every rule file (original behavior)
    for rule_file in "$SHARED_DIR/rules/"*.md; do
      result+=$'\n'"$(cat "$rule_file")"$'\n'
    done
  else
    # Stack-scoped: core rules always, language conventions only when matched
    for rule_file in \
      "$SHARED_DIR/rules/approval-gates.md" \
      "$SHARED_DIR/rules/architecture-guardrails.md" \
      "$SHARED_DIR/rules/design-principles.md" \
      "$SHARED_DIR/rules/memory-trust-boundary.md" \
      "$SHARED_DIR/rules/test-repair-contract.md" \
      "$SHARED_DIR/rules/testing-conventions.md"; do
      [[ -f "$rule_file" ]] || continue
      result+=$'\n'"$(cat "$rule_file")"$'\n'
    done
    # Language-specific rules — one entry per stack token
    for entry in \
      "go:go-conventions.md" \
      "typescript:typescript-conventions.md" \
      "python:python-conventions.md" \
      "csharp:csharp-conventions.md" \
      "java:java-conventions.md" \
      "kotlin:kotlin-conventions.md" \
      "swift:swift-conventions.md" \
      "rust:rust-conventions.md" \
      "iac:iac-conventions.md"; do
      local lang="${entry%%:*}"
      local rule_file="$SHARED_DIR/rules/${entry##*:}"
      if stack_includes "$lang" && [[ -f "$rule_file" ]]; then
        result+=$'\n'"$(cat "$rule_file")"$'\n'
      fi
    done
  fi

  echo "$result"
}

collect_agent_roster() {
  local result=""
  result+=$'\n'"# Persona Roster"$'\n'
  result+=$'\n'"The following specialized personas are available. Invoke them by name when you need domain-specific expertise. Note: on this platform these are personas — context frames with no tool access or autonomous pipeline participation, per \`DOMAIN_DICTIONARY.md\`. Full multi-step agent orchestration is only available on Tier 1 (Claude Code)."$'\n'

  for agent_file in "$SHARED_DIR/agents/"*.md; do
    local agent_name agent_desc agent_status agent_successor
    agent_name=$(grep '^name:' "$agent_file" | head -1 | sed 's/name: *//' || true)
    agent_desc=$(grep '^description:' "$agent_file" | head -1 | sed 's/description: *//' || true)
    agent_status=$(grep '^status:' "$agent_file" | head -1 | sed 's/status: *//' || true)
    agent_successor=$(grep '^superseded_by:' "$agent_file" | head -1 | sed 's/superseded_by: *//' || true)
    if [[ -n "$agent_name" ]]; then
      if [[ "$agent_status" == "deprecated" && -n "$agent_successor" ]]; then
        result+=$'\n'"- **$agent_name** *(deprecated — use $agent_successor)*: $agent_desc"
      elif [[ "$agent_status" == "deprecated" ]]; then
        result+=$'\n'"- **$agent_name** *(deprecated)*: $agent_desc"
      else
        result+=$'\n'"- **$agent_name**: $agent_desc"
      fi
    fi
  done
  echo "$result"
}

extract_rule_content() {
  local file="$1"
  cat "$file"
}

testing_rules_body() {
  extract_rule_content "$SHARED_DIR/rules/testing-conventions.md"
}

go_backend_rules_body() {
  extract_rule_content "$SHARED_DIR/rules/go-conventions.md"
}

typescript_conventions_body() {
  extract_rule_content "$SHARED_DIR/rules/typescript-conventions.md"
}

python_conventions_body() {
  extract_rule_content "$SHARED_DIR/rules/python-conventions.md"
}

csharp_conventions_body() {
  extract_rule_content "$SHARED_DIR/rules/csharp-conventions.md"
}

java_conventions_body() {
  extract_rule_content "$SHARED_DIR/rules/java-conventions.md"
}

vue_frontend_rules_body() {
  echo "# Vue 3 + Tailwind Frontend Conventions

ALWAYS use Vue 3 Composition API with \`<script setup>\`.
NEVER use Options API in new components.
ALWAYS use Tailwind CSS utility classes — no custom CSS unless absolutely necessary.
ALWAYS extract reusable UI into composables (\`use*.ts\`) or components.
NEVER put business logic in components — extract to composables or services.
ALWAYS use TypeScript with strict mode.
NEVER use \`any\` types — use \`unknown\` with Zod validation at boundaries.
ALWAYS co-locate component tests alongside components.
CRITICAL: Components MUST be < 100 lines. Extract when larger."
}

# Broad glob covering common source file extensions across this framework's supported stacks
# (TypeScript, Go, Python, Java) — used to Auto Attach rules that should apply whenever code is
# being edited, without paying their token cost on every single request (Cursor's own guidance:
# combined alwaysApply rules should stay under ~2,000 tokens; architecture.mdc alone exceeds that).
CODE_FILE_GLOBS='["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx", "**/*.vue", "**/*.go", "**/*.py", "**/*.java"]'

generate_mdc() {
  local dest="$1"
  local description="$2"
  local always_apply="$3"
  local globs="$4"
  local body="$5"

  local frontmatter="---"$'\n'
  frontmatter+="description: \"$description\""$'\n'
  frontmatter+="alwaysApply: $always_apply"$'\n'
  if [[ -n "$globs" ]]; then
    frontmatter+="globs: $globs"$'\n'
  fi
  frontmatter+="---"$'\n'

  write_file "$dest" "${frontmatter}${body}"
}

generate_cursor() {
  echo ""
  echo "--- Cursor (Tier 1-equivalent for agents/skills, Tier 2 for rules) ---"

  local rules_dir="$OUTPUT_DIR/.cursor/rules"

  # Only approval-gates and agent-roster stay alwaysApply: true — both are cheap (a few KB) and
  # safety/awareness-critical regardless of what the user is doing (chatting, planning, or coding).
  # architecture.mdc and design-principles.mdc are large (architecture.mdc alone inlines the full
  # ARCHITECTURE_RULES.md) and code-editing-specific, so they're Auto Attached via CODE_FILE_GLOBS
  # instead — loaded when actually touching source, not on every single request. Cursor's own
  # guidance: combined alwaysApply rules should stay under ~2,000 tokens; the previous all-four-
  # always-apply setup was 2-3x over that.
  generate_mdc "$rules_dir/architecture.mdc" \
    "Architecture guardrails — Clean Architecture, SOLID, no hardcoded secrets, expand/contract migrations" \
    "false" "$CODE_FILE_GLOBS" \
    "$(extract_rule_content "$SHARED_DIR/rules/architecture-guardrails.md")

$(extract_rule_content "$SHARED_DIR/ARCHITECTURE_RULES.md")"

  generate_mdc "$rules_dir/design-principles.mdc" \
    "Design principles — Kent Beck simple design, Fowler refactoring, Sandi Metz limits, Boy Scout Rule" \
    "false" "$CODE_FILE_GLOBS" \
    "$(extract_rule_content "$SHARED_DIR/rules/design-principles.md")"

  generate_mdc "$rules_dir/approval-gates.mdc" \
    "Approval gates — NEVER skip human checkpoints for commits, deploys, migrations, external API calls" \
    "true" "" \
    "$(extract_rule_content "$SHARED_DIR/rules/approval-gates.md")"

  generate_mdc "$rules_dir/agent-roster.mdc" \
    "Persona roster — ALWAYS check this list before beginning specialized work" \
    "true" "" \
    "$(collect_agent_roster)"

  generate_mdc "$rules_dir/testing.mdc" \
    "Testing framework rules — Saturday (E2E) and Sunday (API) conventions" \
    "false" '["**/*.spec.*", "**/*.test.*", "**/*.feature", "**/steps/**"]' \
    "$(testing_rules_body)"

  generate_mdc "$rules_dir/test-repair-contract.mdc" \
    "Test repair contract — what an agent MAY and MAY NOT change when repairing a failing test" \
    "false" '["**/*.spec.*", "**/*.test.*", "**/*.feature", "**/steps/**", "**/test_*.py", "**/*_test.go"]' \
    "$(extract_rule_content "$SHARED_DIR/rules/test-repair-contract.md")"

  generate_mdc "$rules_dir/go-backend.mdc" \
    "Go backend conventions — Clean Architecture, error handling, migrations" \
    "false" '["**/*.go", "**/go.mod", "**/go.sum"]' \
    "$(go_backend_rules_body)"

  generate_mdc "$rules_dir/vue-frontend.mdc" \
    "Vue 3 + Tailwind conventions — Composition API, strict typing, component limits" \
    "false" '["**/*.vue", "**/*.tsx", "**/*.jsx", "**/components/**"]' \
    "$(vue_frontend_rules_body)"

  generate_mdc "$rules_dir/typescript-conventions.mdc" \
    "TypeScript conventions — package tooling, faker/factory libraries (non-Vue-component files)" \
    "false" '["**/*.ts"]' \
    "$(typescript_conventions_body)"

  generate_mdc "$rules_dir/python-conventions.mdc" \
    "Python conventions — uv/ruff/pytest tooling, faker/factory libraries" \
    "false" '["**/*.py"]' \
    "$(python_conventions_body)"

  generate_mdc "$rules_dir/csharp-conventions.mdc" \
    "C# conventions — .NET tooling, xUnit/Reqnroll, faker/factory libraries" \
    "false" '["**/*.cs"]' \
    "$(csharp_conventions_body)"

  generate_mdc "$rules_dir/java-conventions.mdc" \
    "Java conventions — build tooling, JUnit/Mockito, faker/factory libraries" \
    "false" '["**/*.java"]' \
    "$(java_conventions_body)"

  # Agents and skills are NOT generated here. Cursor natively reads .cursor/agents/*.md and
  # .cursor/skills/*/SKILL.md using the same open standard shared/agents/ and shared/skills/ already
  # follow (confirmed against cursor.com/docs/subagents and cursor.com/docs/skills, 2026-07-06) --
  # install.sh symlinks those directories directly, the same way it does for Claude Code, instead of
  # this script flattening each agent into a standalone .mdc persona file (the old workaround, retired
  # now that real subagent/skill loading exists).
}

collect_craftsmanship_section() {
  local result=""
  result+=$'\n'"## Craftsmanship Rules"$'\n'
  result+='You must **strictly adhere** to the patterns defined in `ARCHITECTURE_RULES.md` (Clean Architecture, DDD, GoF patterns, and micro-rules).'$'\n'
  result+='- **TDD/BDD First**: Drive design through testing. Feature code is incomplete without tests. Practice Red-Green-Refactor.'$'\n'
  result+='- **Kent Beck (Simple Design)**: 1) Passes tests, 2) Reveals intention, 3) No duplication, 4) Fewest elements.'$'\n'
  result+='- **Martin Fowler (Refactoring)**: Use named refactoring operations (Extract Function, Inline Variable, etc.) instead of vague cleanups.'$'\n'
  result+='- **Architectural Constraints & Fitness Functions**: Enforce cyclomatic complexity `< 7` and functions `< 30` LOC.'$'\n'
  result+='- **The Boy Scout Rule**: Always leave the code cleaner than you found it.'$'\n'
  result+=$'\n'
  result+="## Tech Stack"$'\n'
  result+="- **Backend / MCP**: Go"$'\n'
  result+="- **Frontend**: Vue 3 + Tailwind CSS"$'\n'
  result+="- **Test Automation (Saturday Framework)**: TypeScript, Playwright, Cucumber.js, k6"$'\n'
  result+="- **API Testing (Sunday Framework)**: Vitest, Playwright, Zod, CircuitBreaker"$'\n'
  echo "$result"
}

generate_cursorrules() {
  local dest="$OUTPUT_DIR/.cursorrules"
  local content=""

  content+="# Context Engineering Framework — All Rules"$'\n'
  content+=$'\n'
  content+="$(collect_rules)"
  content+=$'\n'
  content+="$(collect_craftsmanship_section)"
  content+=$'\n'
  content+="$(collect_agent_roster)"

  write_file "$dest" "$content"
}

generate_windsurfrules() {
  local dest="$OUTPUT_DIR/.windsurfrules"
  local content=""

  content+="# Context Engineering Framework — All Rules"$'\n'
  content+=$'\n'
  content+="$(collect_rules)"
  content+=$'\n'
  content+="$(collect_craftsmanship_section)"
  content+=$'\n'
  content+="$(collect_agent_roster)"

  write_file "$dest" "$content"
}

generate_tier3() {
  local platform_name="$1"
  local dest_path="$2"
  local header="$3"

  local content=""
  content+="$header"$'\n'
  content+=$'\n'
  content+="## AI Feature Team & Global Rules"$'\n'
  content+="You are part of the Saturday Multi-Agent Feature Team. Before beginning any complex task, architectural decision, or feature delivery, you MUST adhere to the rules below."$'\n'
  content+=$'\n'
  content+="$(collect_rules)"
  content+=$'\n'
  content+="## Craftsmanship Rules"$'\n'
  content+='You must **strictly adhere** to the patterns defined in `ARCHITECTURE_RULES.md` (Clean Architecture, DDD, GoF patterns, and micro-rules).'$'\n'
  content+='- **TDD/BDD First**: Drive design through testing. Feature code is incomplete without tests. Practice Red-Green-Refactor.'$'\n'
  content+='- **Kent Beck (Simple Design)**: 1) Passes tests, 2) Reveals intention, 3) No duplication, 4) Fewest elements.'$'\n'
  content+='- **Martin Fowler (Refactoring)**: Use named refactoring operations (Extract Function, Inline Variable, etc.) instead of vague cleanups.'$'\n'
  content+='- **Architectural Constraints & Fitness Functions**: Enforce cyclomatic complexity `< 7` and functions `< 30` LOC.'$'\n'
  content+='- **The Boy Scout Rule**: Always leave the code cleaner than you found it.'$'\n'
  content+=$'\n'
  content+="## Tech Stack"$'\n'
  content+="- **Backend / MCP**: Go"$'\n'
  content+="- **Frontend**: Vue 3 + Tailwind CSS"$'\n'
  content+="- **Test Automation**: TypeScript, Playwright, Cucumber.js, k6"$'\n'
  content+=$'\n'
  content+="$(collect_agent_roster)"

  write_file "$dest_path" "$content"
}

generate_instructions_md() {
  local dest="$1"
  local apply_to="$2"
  local body="$3"

  local frontmatter="---"$'\n'
  frontmatter+="applyTo: \"$apply_to\""$'\n'
  frontmatter+="---"$'\n'

  write_file "$dest" "${frontmatter}${body}"
}

generate_copilot_scoped_instructions() {
  local dest_dir="$OUTPUT_DIR/.github/instructions"

  # Path-scoped instructions coexist with and combine with copilot-instructions.md (the Tier 3
  # style file generated separately) — per GitHub's docs, both apply when a file matches. Mirrors
  # Cursor's per-concern .mdc files (testing, go-backend, vue-frontend, plus the four
  # <language>-conventions.mdc files); applyTo uses comma-separated glob patterns in a single quoted
  # string, not an array like Cursor's `globs` field.
  generate_instructions_md "$dest_dir/testing.instructions.md" \
    '**/*.spec.*,**/*.test.*,**/*.feature' \
    "$(testing_rules_body)"

  generate_instructions_md "$dest_dir/go-backend.instructions.md" \
    '**/*.go,**/go.mod,**/go.sum' \
    "$(go_backend_rules_body)"

  generate_instructions_md "$dest_dir/vue-frontend.instructions.md" \
    '**/*.vue,**/*.tsx,**/*.jsx' \
    "$(vue_frontend_rules_body)"

  generate_instructions_md "$dest_dir/typescript-conventions.instructions.md" \
    '**/*.ts' \
    "$(typescript_conventions_body)"

  generate_instructions_md "$dest_dir/python-conventions.instructions.md" \
    '**/*.py' \
    "$(python_conventions_body)"

  generate_instructions_md "$dest_dir/csharp-conventions.instructions.md" \
    '**/*.cs' \
    "$(csharp_conventions_body)"

  generate_instructions_md "$dest_dir/java-conventions.instructions.md" \
    '**/*.java' \
    "$(java_conventions_body)"
}

generate_agents_md() {
  local dest_path="$OUTPUT_DIR/AGENTS.md"

  local content=""
  content+="# AGENTS.md"$'\n'
  content+=$'\n'
  content+="Cross-tool agent instructions, following the https://agents.md convention. Confirmed 2026-07-02:"$'\n'
  content+="Gemini Antigravity reads this as its project-level rules (injected as \`<RULE[AGENTS.md]>\` in its"$'\n'
  content+="system prompt) — see the \`gemini\` entry in \`shared/platform-registry.json\` for how this was verified."$'\n'
  content+=$'\n'
  content+="## AI Feature Team & Global Rules"$'\n'
  content+="You are part of the Saturday Multi-Agent Feature Team. Before beginning any complex task, architectural decision, or feature delivery, you MUST adhere to the rules below."$'\n'
  content+=$'\n'
  content+="$(collect_rules)"
  content+=$'\n'
  content+="$(collect_craftsmanship_section)"
  content+=$'\n'
  content+="$(collect_agent_roster)"

  write_file "$dest_path" "$content"
}

generate_jetbrains() {
  echo ""
  echo "--- JetBrains AI Assistant + Junie (Tier 2: Personas + Rules) ---"

  local rules_dir="$OUTPUT_DIR/.aiassistant/rules"
  local junie_dir="$OUTPUT_DIR/.junie"

  if $DRY_RUN; then
    dry ".aiassistant/rules/*.md (10 project rules)"
    dry ".junie/guidelines.md (Junie legacy path)"
    return
  fi

  mkdir -p "$rules_dir" "$junie_dir"

  # Always-active rules — prepend IDE mode hint as HTML comment (no frontmatter standard in JetBrains)
  local always_hint="<!-- JetBrains AI Assistant Project Rule | Recommended: Always
     Configure: Settings > AI Assistant > Project Rules > set to 'Always' -->"$'\n\n'

  write_file "$rules_dir/00-approval-gates.md" \
    "${always_hint}$(extract_rule_content "$SHARED_DIR/rules/approval-gates.md")"

  write_file "$rules_dir/01-design-principles.md" \
    "${always_hint}$(extract_rule_content "$SHARED_DIR/rules/design-principles.md")"

  write_file "$rules_dir/02-architecture-guardrails.md" \
    "${always_hint}$(extract_rule_content "$SHARED_DIR/rules/architecture-guardrails.md")"

  write_file "$rules_dir/03-agent-roster.md" \
    "${always_hint}$(collect_agent_roster)"

  # File-pattern scoped rules
  local pattern_hint_testing="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.spec.*,*.test.*,*.feature
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/04-testing-conventions.md" \
    "${pattern_hint_testing}$(testing_rules_body)"

  local pattern_hint_go="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.go
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/05-go-conventions.md" \
    "${pattern_hint_go}$(go_backend_rules_body)"

  local pattern_hint_ts="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.ts
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/06-typescript-conventions.md" \
    "${pattern_hint_ts}$(typescript_conventions_body)"

  local pattern_hint_py="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.py
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/07-python-conventions.md" \
    "${pattern_hint_py}$(python_conventions_body)"

  local pattern_hint_cs="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.cs
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/08-csharp-conventions.md" \
    "${pattern_hint_cs}$(csharp_conventions_body)"

  local pattern_hint_java="<!-- JetBrains AI Assistant Project Rule | Recommended: By file patterns: *.java
     Configure: Settings > AI Assistant > Project Rules > set pattern -->"$'\n\n'
  write_file "$rules_dir/09-java-conventions.md" \
    "${pattern_hint_java}$(java_conventions_body)"

  # .junie/guidelines.md — Junie's legacy-but-reliable path; same content as AGENTS.md
  local junie_content=""
  junie_content+="# Junie Guidelines (Saturday Framework)"$'\n'
  junie_content+=$'\n'
  junie_content+="Project guidelines for the JetBrains Junie agent. Junie reads this file before falling back to"$'\n'
  junie_content+="AGENTS.md at the repository root — same content is kept in sync by the framework installer."$'\n'
  junie_content+=$'\n'
  junie_content+="## AI Feature Team & Global Rules"$'\n'
  junie_content+="You are part of the Saturday Multi-Agent Feature Team. Before beginning any complex task, architectural decision, or feature delivery, you MUST adhere to the rules below."$'\n'
  junie_content+=$'\n'
  junie_content+="$(collect_rules)"
  junie_content+=$'\n'
  junie_content+="$(collect_craftsmanship_section)"
  junie_content+=$'\n'
  junie_content+="$(collect_agent_roster)"

  write_file "$junie_dir/guidelines.md" "$junie_content"
}

generate_roo_code() {
  echo ""
  echo "--- Roo Code (Tier 2+: Custom Modes) ---"

  local roomodes_dest="$OUTPUT_DIR/.roomodes"
  local roo_rules_dir="$OUTPUT_DIR/.roo/rules"

  if $DRY_RUN; then
    dry ".roomodes (${actual_agent_count:-?} custom modes)"
    dry ".roo/rules/*.md (framework global rules)"
    return
  fi

  # Generate .roomodes YAML — each shared agent becomes a custom mode
  mkdir -p "$(dirname "$roomodes_dest")"
  python3 "$REPO_DIR/scripts/generate-roomodes.py" "$SHARED_DIR/agents" > "$roomodes_dest"
  ok "$roomodes_dest"

  # Copy global framework rules into .roo/rules/ — applies to ALL modes
  mkdir -p "$roo_rules_dir"
  for rule_file in "$SHARED_DIR/rules/"*.md; do
    local rule_name
    rule_name=$(basename "$rule_file")
    cp "$rule_file" "$roo_rules_dir/$rule_name"
    ok "$roo_rules_dir/$rule_name"
  done
}

generate_cline() {
  echo ""
  echo "--- Cline (Tier 2: Personas + Rules) ---"

  local clinerules_dir="$OUTPUT_DIR/.clinerules"

  if $DRY_RUN; then
    dry ".clinerules/*.md (framework rules + agent roster)"
    return
  fi

  mkdir -p "$clinerules_dir"

  # Always-active rules (no paths restriction)
  write_file "$clinerules_dir/00-approval-gates.md" \
    "$(extract_rule_content "$SHARED_DIR/rules/approval-gates.md")"

  write_file "$clinerules_dir/01-design-principles.md" \
    "$(extract_rule_content "$SHARED_DIR/rules/design-principles.md")"

  write_file "$clinerules_dir/02-architecture-guardrails.md" \
    "$(extract_rule_content "$SHARED_DIR/rules/architecture-guardrails.md")"

  # Agent roster (always-active, so all modes are surfaced)
  write_file "$clinerules_dir/03-agent-roster.md" \
    "$(collect_agent_roster)"

  # Path-scoped rules — only loaded when editing matching files
  local testing_header
  testing_header="---"$'\n'"paths:"$'\n'"  - \"**/*.spec.*\""$'\n'"  - \"**/*.test.*\""$'\n'"  - \"**/*.feature\""$'\n'"  - \"**/steps/**\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/04-testing-conventions.md" \
    "${testing_header}$(testing_rules_body)"

  local go_header
  go_header="---"$'\n'"paths:"$'\n'"  - \"**/*.go\""$'\n'"  - \"**/go.mod\""$'\n'"  - \"**/go.sum\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/05-go-conventions.md" \
    "${go_header}$(go_backend_rules_body)"

  local ts_header
  ts_header="---"$'\n'"paths:"$'\n'"  - \"**/*.ts\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/06-typescript-conventions.md" \
    "${ts_header}$(typescript_conventions_body)"

  local py_header
  py_header="---"$'\n'"paths:"$'\n'"  - \"**/*.py\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/07-python-conventions.md" \
    "${py_header}$(python_conventions_body)"

  local cs_header
  cs_header="---"$'\n'"paths:"$'\n'"  - \"**/*.cs\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/08-csharp-conventions.md" \
    "${cs_header}$(csharp_conventions_body)"

  local java_header
  java_header="---"$'\n'"paths:"$'\n'"  - \"**/*.java\""$'\n'"---"$'\n'
  write_file "$clinerules_dir/09-java-conventions.md" \
    "${java_header}$(java_conventions_body)"
}

echo ""
echo "Context Engineering Framework — Config Generator"
echo "================================================="
echo ""
echo "Source:  $SHARED_DIR"
echo "Output:  $OUTPUT_DIR"
echo "Dry run: $DRY_RUN"
if [[ -n "$PLATFORM_FILTER" ]]; then
  echo "Platform: $PLATFORM_FILTER"
fi
if [[ -n "$STACK_FILTER" ]]; then
  echo "Stack:   $STACK_FILTER (language rules filtered — core rules always included)"
fi
echo ""

GENERATED=0
actual_agent_count=$(find "$SHARED_DIR/agents" -maxdepth 1 -name "*.md" ! -name "CHANGELOG.md" | wc -l | tr -d ' ')

if should_generate "jetbrains"; then
  generate_jetbrains
  ((GENERATED += 11)) || true
fi

if should_generate "roo-code"; then
  generate_roo_code
  rule_count=$(find "$SHARED_DIR/rules" -maxdepth 1 -name "*.md" | wc -l | tr -d ' ')
  ((GENERATED += 1 + rule_count)) || true
fi

if should_generate "cline"; then
  generate_cline
  ((GENERATED += 10)) || true
fi

if should_generate "cursor"; then
  generate_cursor
  generate_cursorrules
  agent_persona_count=$(find "$SHARED_DIR/agents" -name "*.md" -not -name "CHANGELOG.md" | wc -l | tr -d ' ')
  ((GENERATED += 8 + agent_persona_count))
fi

if should_generate "windsurf"; then
  generate_windsurfrules
  ((GENERATED++)) || true
fi

if should_generate "github-copilot"; then
  echo ""
  echo "--- GitHub Copilot (Tier 2: Personas + Rules) ---"
  generate_tier3 "github-copilot" \
    "$OUTPUT_DIR/.github/copilot-instructions.md" \
    "# Copilot Instructions (Saturday Framework)"
  generate_copilot_scoped_instructions
  ((GENERATED += 4))
fi

if should_generate "gemini"; then
  echo ""
  echo "--- Gemini / Antigravity (confirmed 2026-07-02: reads AGENTS.md; .gemini/antigravity/instructions.md is NOT read) ---"
  generate_agents_md
  ((GENERATED += 1))
fi

if should_generate "openai-codex"; then
  echo ""
  echo "--- OpenAI / Codex (Tier 3: System Prompt) ---"
  generate_tier3 "openai-codex" \
    "$OUTPUT_DIR/.openai.md" \
    "# OpenAI / Codex Instructions (Saturday Framework)"
  ((GENERATED++)) || true
fi

echo ""
echo "================================================="
echo "Generated $GENERATED config file(s)"
if $DRY_RUN; then
  echo "This was a dry run. No files were written."
fi
echo ""
