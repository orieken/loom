#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$REPO_DIR/shared"

MODE=""
TARGET_DIR=""
DRY_RUN=false
USE_COPY=false
PLATFORM_FILTER=""
SHOW_TOUR=false
SYNC_MEMORY=false
SYNC_SUBCOMMAND="pull"
SYNC_ARGS=()
FRAMEWORK_LEVEL="base"   # "base" = current framework only; "full" = AOS layers included
WITH_CONFIGS=false       # --with-configs: copy shared/configs/ into the target project
WITH_MCP=false           # --with-mcp: copy shared/mcp/ scaffold into the target project
STACK_FILTER=""          # --stack: language stacks to include in generated configs (e.g. go,typescript)

usage() {
  cat <<'EOF'
Usage: install.sh [OPTIONS]

Install the Context Engineering Framework on this machine.

Modes (pick one):
  --global              Symlink configs to ~/  (always current after a git pull)
  --project <path>      Symlink configs into a target project directory (also always current --
                         the target keeps depending on this repo's checkout staying where it is)
  --project             Symlink configs into the current directory
  --sync-memory [pull|push]
                        Sync Knowledge Items with the org knowledge-hub repo.
                        pull (default): diff and optionally apply org KIs into shared/knowledge/
                        push: promote local .claude/knowledge/ KIs to the org repo via PR
                        Requires .claude/sync-config.yaml — see ADR-003 for setup.

Framework level (optional, default --base):
  --base                Install the base framework only (current behavior, v2.x/v3.x parity).
                         No AOS runtime layers are wired. This is the default.
  --full                Install the base framework PLUS AOS runtime layers (v3.2+):
                         - Copies shared/hooks/ example hook files to .claude/hooks/ (disabled by default)
                         - Symlinks shared/rag/ and shared/orchestration/ into the project
                         - Prints AOS opt-in next steps
                         AOS layers are opt-in even in --full mode: files are present but no hook is
                         enabled unless you explicitly set enabled: true in .claude/hooks/.

Options:
  --copy                Use real copies instead of symlinks -- required on Windows/WSL (symlinks need
                         elevated permissions there), and also the way to make a --project install
                         independent of this repo's checkout (a symlinked project install breaks if
                         this repo is later moved or deleted)
  --with-configs        Copy linter fitness-function configs from shared/configs/ into the target
                         project root. Files are always copied (never symlinked) so teams can edit
                         them. Existing files are left untouched (idempotent). Only meaningful with
                         --project mode — silently skipped in --global mode.
  --with-mcp            (deprecated — use 'loom mcp serve') Copy shared/mcp/ reference source into <target>/<project-name>-mcp/
                         and run go mod tidy to produce a ready-to-build MCP server. Only
                         meaningful with --project mode — silently skipped in --global mode.
  --platform <name>     Only install for a specific platform (claude-code, cursor, windsurf, github-copilot, gemini, openai-codex, jetbrains, roo-code, cline)
  --stack <list>        Comma-separated language stacks for this project (e.g. go or go,typescript).
                        Stored in .claude/framework-install.json and auto-applied by
                        scripts/generate-configs.sh on every subsequent regeneration.
                        Stacks: go, typescript, python, csharp, java, kotlin, swift, rust, iac
  --dry-run             Show what would be installed without doing it
  --tour                Run the onboarding skill after install
  --confirm             For --sync-memory: apply changes (default is diff-only dry run)
  -h, --help            Show this help

Examples:
  ./install.sh --global
  ./install.sh --project /path/to/my-app
  ./install.sh --project --full                    # install with AOS layers
  ./install.sh --project --with-configs            # also copy linter configs
  ./install.sh --project --with-mcp               # also scaffold an MCP server
  ./install.sh --project --platform claude-code
  ./install.sh --global --dry-run
  ./install.sh --sync-memory
  ./install.sh --sync-memory pull --confirm
  ./install.sh --sync-memory push --confirm
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --global)    MODE="global"; shift ;;
    --project)
      MODE="project"
      if [[ $# -gt 1 && ! "$2" =~ ^-- ]]; then
        TARGET_DIR="$2"; shift 2
      else
        TARGET_DIR="$(pwd)"; shift
      fi
      ;;
    --sync-memory)
      SYNC_MEMORY=true
      if [[ $# -gt 1 && ( "$2" == "pull" || "$2" == "push" ) ]]; then
        SYNC_SUBCOMMAND="$2"; shift 2
      else
        shift
      fi
      ;;
    --confirm)   SYNC_ARGS+=("--confirm"); shift ;;
    --copy)      USE_COPY=true; shift ;;
    --platform)  PLATFORM_FILTER="$2"; shift 2 ;;
    --stack)     STACK_FILTER="$2"; shift 2 ;;
    --dry-run)   DRY_RUN=true; shift ;;
    --tour)      SHOW_TOUR=true; shift ;;
    --base)           FRAMEWORK_LEVEL="base"; shift ;;
    --full)           FRAMEWORK_LEVEL="full"; shift ;;
    --with-configs)   WITH_CONFIGS=true; shift ;;
    --with-mcp)       WITH_MCP=true; shift ;;
    -h|--help)        usage ;;
    *)                echo "Unknown option: $1"; usage ;;
  esac
done

# --sync-memory is a standalone operation; bypass the install mode requirement.
if $SYNC_MEMORY; then
  exec bash "$REPO_DIR/scripts/sync-memory.sh" "$SYNC_SUBCOMMAND" "${SYNC_ARGS[@]}"
fi

if [[ -z "$MODE" ]]; then
  echo "Error: specify --global or --project <path>"
  echo "Run './install.sh --help' for usage."
  exit 1
fi

AGENT_COUNT=$(find "$SHARED_DIR/agents" -name "*.md" -not -name "CHANGELOG.md" | wc -l | tr -d ' ')
SKILL_COUNT=$(find "$SHARED_DIR/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
RULE_COUNT=$(find "$SHARED_DIR/rules" -name "*.md" | wc -l | tr -d ' ')

log()  { echo "  $1"; }
ok()   { echo "  [ok] $1"; }
skip() { echo "  [skip] $1"; }
dry()  { echo "  [dry-run] $1"; }

# Returns 0 (true) if the given stack token should be included.
# When STACK_FILTER is empty every stack passes (unfiltered behavior).
stack_includes() {
  local lang="$1"
  [[ -z "$STACK_FILTER" ]] && return 0
  echo "$STACK_FILTER" | tr ',' '\n' | grep -qxF "$lang"
}

# Install a single rule file via symlink or copy.
_install_rule_file() {
  local src="$1"
  local dest="$2"
  [[ -f "$src" ]] || return 0
  if [[ -L "$dest" ]] && [[ "$(readlink "$dest")" == "$src" ]]; then
    skip "$(basename "$dest") (already linked)"
    return
  fi
  if $USE_COPY; then
    cp "$src" "$dest"
    ok "copied $(basename "$dest")"
  else
    ln -sf "$src" "$dest"
    ok "linked $(basename "$dest")"
  fi
}

# Create .claude/rules/ as a directory with selective per-file symlinks when
# --stack is set. Core rules are always included; language conventions only when
# their stack token matches STACK_FILTER. Falls back to a full directory
# symlink when STACK_FILTER is empty (original behavior, handled by the caller).
link_stack_rules() {
  local claude_dir="$1"
  local rules_dir="$claude_dir/rules"

  if $DRY_RUN; then
    dry "would create $rules_dir/ with stack-scoped rule symlinks (stack: $STACK_FILTER)"
    return
  fi

  # Convert a pre-existing full directory symlink to a real directory
  if [[ -L "$rules_dir" ]]; then
    local backup="${rules_dir}.bak.$(date +%s)"
    mv "$rules_dir" "$backup"
    log "backed up $rules_dir -> $backup (converting to selective symlinks)"
  fi

  mkdir -p "$rules_dir"

  # Core rules — always present regardless of stack
  for rule_name in \
    "approval-gates.md" \
    "architecture-guardrails.md" \
    "design-principles.md" \
    "memory-trust-boundary.md" \
    "testing-conventions.md"; do
    _install_rule_file "$SHARED_DIR/rules/$rule_name" "$rules_dir/$rule_name"
  done

  # Language-specific rules — only when their stack token is in STACK_FILTER
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
    local rule_name="${entry##*:}"
    if stack_includes "$lang"; then
      _install_rule_file "$SHARED_DIR/rules/$rule_name" "$rules_dir/$rule_name"
    fi
  done

  local linked_count
  linked_count=$(find "$rules_dir" -maxdepth 1 -name "*.md" | wc -l | tr -d ' ')
  ok "$rules_dir/ — $linked_count of $(find "$SHARED_DIR/rules" -name "*.md" | wc -l | tr -d ' ') rule files linked (stack: $STACK_FILTER)"
}

link_or_copy() {
  local src="$1"
  local dest="$2"

  if [[ -e "$dest" || -L "$dest" ]]; then
    if [[ -L "$dest" ]]; then
      local existing_target
      existing_target=$(readlink "$dest")
      if [[ "$existing_target" == "$src" ]]; then
        skip "$dest (already linked)"
        return
      fi
    elif $USE_COPY && [[ -e "$dest" ]] && diff -rq "$src" "$dest" > /dev/null 2>&1; then
      skip "$dest (already copied, content identical)"
      return
    fi
    local backup="${dest}.bak.$(date +%s)"
    if $DRY_RUN; then
      dry "would backup $dest -> $backup"
    else
      mv "$dest" "$backup"
      log "backed up $dest -> $backup"
    fi
  fi

  if $DRY_RUN; then
    if $USE_COPY; then
      dry "would copy $src -> $dest"
    else
      dry "would symlink $dest -> $src"
    fi
    return
  fi

  mkdir -p "$(dirname "$dest")"
  if $USE_COPY; then
    if [[ -d "$src" ]]; then
      cp -r "$src" "$dest"
    else
      cp "$src" "$dest"
    fi
    ok "copied $dest"
  else
    ln -s "$src" "$dest"
    ok "linked $dest -> $src"
  fi
}

detect_platforms() {
  local detected=()

  detected+=("claude-code")

  if command -v cursor &>/dev/null || [[ -d "$HOME/.cursor" ]]; then
    detected+=("cursor")
  fi

  if [[ -f "$HOME/.windsurfrules" ]] || command -v windsurf &>/dev/null; then
    detected+=("windsurf")
  fi

  if command -v gh &>/dev/null && gh extension list 2>/dev/null | grep -q copilot; then
    detected+=("github-copilot")
  elif [[ -d "$HOME/.config/github-copilot" ]]; then
    detected+=("github-copilot")
  fi

  if command -v gemini &>/dev/null || [[ -d "$HOME/.gemini" ]]; then
    detected+=("gemini")
  fi

  if [[ -d "$HOME/Library/Application Support/JetBrains" ]] || [[ -d "$HOME/.config/JetBrains" ]]; then
    detected+=("jetbrains")
  fi

  if [[ -d "$HOME/.vscode/extensions" ]] && ls "$HOME/.vscode/extensions/" 2>/dev/null | grep -qi "roo-cline\|rooveterinaryinc"; then
    detected+=("roo-code")
  fi

  if [[ -d "$HOME/.vscode/extensions" ]] && ls "$HOME/.vscode/extensions/" 2>/dev/null | grep -qi "saoudrizwan.claude-dev\|cline"; then
    detected+=("cline")
  fi

  if [[ -f "$HOME/.openai.md" ]] || [[ -f ".openai.md" ]]; then
    detected+=("openai-codex")
  fi

  echo "${detected[@]}"
}

should_install() {
  local platform="$1"
  if [[ -n "$PLATFORM_FILTER" ]]; then
    [[ "$platform" == "$PLATFORM_FILTER" ]]
  else
    return 0
  fi
}

resolve_model_tier() {
  local platform="$1"
  local overrides_file=""
  if [[ "$MODE" == "project" && -n "$TARGET_DIR" ]]; then
    overrides_file="$TARGET_DIR/.claude/model-overrides.yaml"
  else
    overrides_file="$HOME/.claude/model-overrides.yaml"
  fi

  if command -v python3 &>/dev/null; then
    python3 "$REPO_DIR/scripts/resolve-model-tier.py" \
      --platform "$platform" \
      --defaults "$SHARED_DIR/model-defaults.yaml" \
      --agents-dir "$SHARED_DIR/agents" \
      ${overrides_file:+--overrides "$overrides_file"} 2>/dev/null || true
  fi
}

install_antigravity() {
  log ""
  log "--- Gemini Antigravity (confirmed 2026-07-02 via tests/platform-verification/antigravity.md) ---"

  if [[ "$MODE" == "global" ]]; then
    # Confirmed: Antigravity's global skills root is ~/.gemini/config/skills/ — NOT ~/.agents/skills/
    # (that was an earlier, unconfirmed guess). Global rules use a single ~/.gemini/GEMINI.md file per
    # secondary-source research, not yet confirmed the way the skills path and AGENTS.md have been —
    # left unhandled here rather than generating unconfirmed content.
    if ! $DRY_RUN; then mkdir -p "$HOME/.gemini/config" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/skills" "$HOME/.gemini/config/skills"
  else
    # Confirmed: project-level AGENTS.md is read for rules (generated separately, see
    # generate_agents_md in scripts/generate-configs.sh). Project-level .agents/skills/ and
    # .agents/rules/ are the documented convention but weren't directly exercised by the 2026-07-02
    # test (no .agents/ existed at session start, so it fell back to the global skills root) — kept
    # since they match the codelab's documented project-scope convention and don't contradict anything
    # confirmed so far.
    if ! $DRY_RUN; then mkdir -p "$TARGET_DIR/.agents" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/skills" "$TARGET_DIR/.agents/skills"
    link_or_copy "$SHARED_DIR/rules" "$TARGET_DIR/.agents/rules"
  fi
}

install_jetbrains() {
  log ""
  log "--- JetBrains AI Assistant + Junie (Tier 2: Personas + Rules) ---"
  # .aiassistant/rules/ and .junie/guidelines.md are generated files — delegate to generate-configs.sh
  log "  [info] .aiassistant/rules/ and .junie/guidelines.md generated via scripts/generate-configs.sh"

  if [[ "$MODE" == "global" ]]; then
    # Global Junie guidelines
    local junie_global_dir="$HOME/.junie"
    if ! $DRY_RUN; then mkdir -p "$junie_global_dir" 2>/dev/null || true; fi
    log "  [info] global ~/.junie/AGENTS.md should mirror AGENTS.md — copy manually or re-run with --project"
  fi
  resolve_model_tier "jetbrains"
}

install_roo_code() {
  log ""
  log "--- Roo Code (Tier 2+: Custom Modes) ---"
  # .roomodes and .roo/rules/ are generated files — delegate to generate-configs.sh
  # install_generated_configs() at the end of this script handles both platforms when
  # PLATFORM_FILTER matches; no symlinks needed (Roo Code has no symlink-based install pattern).
  log "  [info] .roomodes and .roo/rules/ generated via scripts/generate-configs.sh"
  resolve_model_tier "roo-code"
}

install_cline() {
  log ""
  log "--- Cline (Tier 2: Personas + Rules) ---"
  # .clinerules/ is a generated directory — delegate to generate-configs.sh
  log "  [info] .clinerules/ generated via scripts/generate-configs.sh"
  resolve_model_tier "cline"
}

install_cursor() {
  log ""
  log "--- Cursor (native agents/skills confirmed 2026-07-06, rules still generated inline) ---"

  # Cursor reads .cursor/agents/*.md and .cursor/skills/*/SKILL.md using the same open standard
  # shared/agents/ and shared/skills/ already follow -- symlink directly instead of flattening into
  # .mdc personas (the retired Epic 11 workaround). Rules still go through generate-configs.sh
  # (install_generated_configs) since Cursor Rules can't follow file references, only agents/skills can.
  if [[ "$MODE" == "global" ]]; then
    local cursor_dir="$HOME/.cursor"
    if ! $DRY_RUN; then mkdir -p "$cursor_dir" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/agents" "$cursor_dir/agents"
    link_or_copy "$SHARED_DIR/skills" "$cursor_dir/skills"
  else
    local cursor_dir="$TARGET_DIR/.cursor"
    if ! $DRY_RUN; then mkdir -p "$cursor_dir" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/agents" "$cursor_dir/agents"
    link_or_copy "$SHARED_DIR/skills" "$cursor_dir/skills"
  fi
}

install_claude_code() {
  log ""
  log "--- Claude Code (Tier 1: Full) ---"

  if [[ "$MODE" == "global" ]]; then
    local claude_dir="$HOME/.claude"
    if ! $DRY_RUN; then mkdir -p "$claude_dir" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/agents" "$claude_dir/agents"
    link_or_copy "$SHARED_DIR/skills" "$claude_dir/skills"
    if [[ -n "$STACK_FILTER" ]]; then
      link_stack_rules "$claude_dir"
    else
      link_or_copy "$SHARED_DIR/rules" "$claude_dir/rules"
    fi
  else
    local claude_dir="$TARGET_DIR/.claude"
    if ! $DRY_RUN; then mkdir -p "$claude_dir" 2>/dev/null || true; fi
    link_or_copy "$SHARED_DIR/agents" "$claude_dir/agents"
    link_or_copy "$SHARED_DIR/skills" "$claude_dir/skills"
    if [[ -n "$STACK_FILTER" ]]; then
      link_stack_rules "$claude_dir"
    else
      link_or_copy "$SHARED_DIR/rules" "$claude_dir/rules"
    fi

    link_or_copy "$SHARED_DIR/ARCHITECTURE_RULES.md" "$TARGET_DIR/ARCHITECTURE_RULES.md"
    link_or_copy "$SHARED_DIR/DOMAIN_DICTIONARY.md" "$TARGET_DIR/DOMAIN_DICTIONARY.md"

    if [[ ! -f "$TARGET_DIR/CLAUDE.md" ]]; then
      local template_claude="$REPO_DIR/templates/claude-feature-team/CLAUDE.md"
      if [[ -f "$template_claude" ]]; then
        if $DRY_RUN; then
          dry "would copy CLAUDE.md template to $TARGET_DIR/"
        else
          cp "$template_claude" "$TARGET_DIR/CLAUDE.md"
          ok "installed CLAUDE.md template"
        fi
      fi
    else
      skip "CLAUDE.md already exists"
    fi

    install_team_template_dir features
    install_team_template_dir intake
  fi
}

# install_team_template_dir <name> copies templates/claude-feature-team/<name>/*.md
# into the project once. Each is a starting point the team edits, so an
# existing directory is never touched. intake/ holds the factory's work-order
# template (roadmap L3.64); loom install copies the same two directories.
install_team_template_dir() {
  local name="$1"
  local source_dir="$REPO_DIR/templates/claude-feature-team/$name"
  if [[ -d "$TARGET_DIR/$name" ]]; then
    skip "$name/ already exists"
    return
  fi
  [[ -d "$source_dir" ]] || return
  if $DRY_RUN; then
    dry "would copy $name/ template"
    return
  fi
  mkdir -p "$TARGET_DIR/$name"
  cp "$source_dir"/*.md "$TARGET_DIR/$name/" 2>/dev/null || true
  ok "installed $name/ template"
}

install_aos_layers() {
  log ""
  log "--- AOS Runtime Layers (--full mode) ---"

  local target_claude_dir
  if [[ "$MODE" == "global" ]]; then
    target_claude_dir="$HOME/.claude"
  else
    target_claude_dir="$TARGET_DIR/.claude"
  fi

  # Link shared/rag/ and shared/orchestration/ for reference from the project
  link_or_copy "$SHARED_DIR/rag" "$target_claude_dir/rag-interface"
  link_or_copy "$SHARED_DIR/orchestration" "$target_claude_dir/orchestration-interface"

  # Seed .claude/hooks/ with example hook files (all disabled by default)
  local hooks_dir="$target_claude_dir/hooks"
  if $DRY_RUN; then
    dry "would create $hooks_dir/ and copy shared/hooks/examples/*.yaml (all disabled by default)"
  else
    mkdir -p "$hooks_dir"
    if [[ -d "$SHARED_DIR/hooks/examples" ]]; then
      for f in "$SHARED_DIR/hooks/examples/"*.yaml; do
        [[ -f "$f" ]] || continue
        local dest="$hooks_dir/$(basename "$f")"
        if [[ ! -f "$dest" ]]; then
          cp "$f" "$dest"
          ok "seeded $dest (disabled by default)"
        else
          skip "$dest (already exists)"
        fi
      done
    fi
    # Also seed the Phase 3 hooks if they don't exist yet
    for src in \
      "$SHARED_DIR/hooks/on-retrospective-written.yaml" \
      "$SHARED_DIR/hooks/scheduled-monthly.yaml"; do
      [[ -f "$src" ]] || continue
      local dest="$hooks_dir/$(basename "$src")"
      if [[ ! -f "$dest" ]]; then
        cp "$src" "$dest"
        ok "seeded $dest (disabled by default)"
      else
        skip "$dest (already exists)"
      fi
    done
    ok "AOS hooks seeded in $hooks_dir/ — all disabled by default"
  fi
}

install_configs() {
  if [[ "$MODE" != "project" ]]; then
    skip "--with-configs is only meaningful in --project mode; skipping"
    return
  fi

  log ""
  log "--- Linter Configs (--with-configs) ---"

  local configs_src="$SHARED_DIR/configs"
  local configs_dest="$TARGET_DIR"

  if [[ ! -d "$configs_src" ]]; then
    log "  [warn] $configs_src not found — skipping config install"
    return
  fi

  for src_file in "$configs_src"/*; do
    [[ -f "$src_file" ]] || continue
    local filename
    filename="$(basename "$src_file")"
    # Skip README.md — it's framework documentation, not a project config.
    [[ "$filename" == "README.md" ]] && continue
    local dest_file="$configs_dest/$filename"
    if [[ -e "$dest_file" ]]; then
      skip "$filename (already exists — not overwritten)"
    elif $DRY_RUN; then
      dry "would copy $src_file -> $dest_file"
    else
      cp "$src_file" "$dest_file"
      ok "copied $filename -> $dest_file"
    fi
  done
}

write_install_marker() {
  local target_claude_dir
  if [[ "$MODE" == "global" ]]; then
    target_claude_dir="$HOME/.claude"
  else
    target_claude_dir="$TARGET_DIR/.claude"
  fi

  local git_tag commit_sha installed_at install_mode
  git_tag=$(cd "$REPO_DIR" && git describe --tags --abbrev=0 2>/dev/null || echo "unknown")
  commit_sha=$(cd "$REPO_DIR" && git rev-parse HEAD 2>/dev/null || echo "unknown")
  installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  install_mode=$(if $USE_COPY; then echo "copy"; else echo "symlink"; fi)

  local platforms_json
  if [[ -n "$PLATFORM_FILTER" ]]; then
    platforms_json="[\"$PLATFORM_FILTER\"]"
  elif [[ ${#DETECTED_PLATFORMS[@]} -eq 0 ]]; then
    platforms_json="[]"
  else
    platforms_json="["
    local first=true
    for p in "${DETECTED_PLATFORMS[@]}"; do
      if $first; then
        platforms_json="${platforms_json}\"$p\""
        first=false
      else
        platforms_json="${platforms_json}, \"$p\""
      fi
    done
    platforms_json="${platforms_json}]"
  fi

  mkdir -p "$target_claude_dir"
  cat > "$target_claude_dir/framework-install.json" <<EOF
{
  "source_repo": "$REPO_DIR",
  "git_tag": "$git_tag",
  "commit_sha": "$commit_sha",
  "installed_at": "$installed_at",
  "mode": "$install_mode",
  "framework_level": "$FRAMEWORK_LEVEL",
  "platforms": $platforms_json,
  "stack": "$STACK_FILTER"
}
EOF
  ok "wrote $target_claude_dir/framework-install.json (version $git_tag @ ${commit_sha:0:8})"
}

install_mcp() {
  if [[ "$MODE" != "project" ]]; then
    skip "--with-mcp is only meaningful in --project mode; skipping"
    return
  fi

  log ""
  log "--- MCP Server Scaffold (--with-mcp) ---"

  local project_name
  project_name="$(basename "$TARGET_DIR")"
  local dest="$TARGET_DIR/${project_name}-mcp"
  local mcp_src="$REPO_DIR/shared/mcp"

  if [[ -d "$dest" ]]; then
    skip "$dest already exists — not overwritten"
    return
  fi

  if $DRY_RUN; then
    dry "would copy $mcp_src -> $dest (reference source only)"
    return
  fi

  cp -r "$mcp_src" "$dest"
  ok "copied MCP reference source to $dest"
  log "  [note] --with-mcp is deprecated: the copied tree is reference source only"
  log "         (no standalone go.mod since the module merge). Use 'loom mcp serve' instead."
}

install_generated_configs() {
  local output_dir
  if [[ "$MODE" == "global" ]]; then
    output_dir="$HOME"
  else
    output_dir="$TARGET_DIR"
  fi

  local gen_args=("--output" "$output_dir")
  if $DRY_RUN; then
    gen_args+=("--dry-run")
  fi
  if [[ -n "$PLATFORM_FILTER" ]]; then
    gen_args+=("--platform" "$PLATFORM_FILTER")
  fi

  "$REPO_DIR/scripts/generate-configs.sh" "${gen_args[@]}"
}

echo ""
echo "Context Engineering Framework Installer"
echo "========================================"
echo ""
echo "Mode:     $MODE"
if [[ "$MODE" == "project" ]]; then
  echo "Target:   $TARGET_DIR"
fi
echo "Level:    $FRAMEWORK_LEVEL$(if [[ "$FRAMEWORK_LEVEL" == "base" ]]; then echo " (default — base framework only)"; else echo " (base + AOS runtime layers)"; fi)"
echo "Strategy: $(if $USE_COPY; then echo 'copy'; else echo 'symlink'; fi)"
echo "Dry run:  $DRY_RUN"
if [[ -n "$PLATFORM_FILTER" ]]; then
  echo "Platform: $PLATFORM_FILTER"
fi
echo ""

echo "Detecting installed platforms..."
DETECTED_PLATFORMS=($(detect_platforms))
echo "Found: ${DETECTED_PLATFORMS[*]}"
echo ""

if should_install "claude-code"; then
  install_claude_code
  resolve_model_tier "claude-code"
fi

if should_install "jetbrains"; then
  install_jetbrains
fi

if should_install "roo-code"; then
  install_roo_code
fi

if should_install "cline"; then
  install_cline
fi

if should_install "cursor"; then
  install_cursor
  resolve_model_tier "cursor"
fi

if should_install "gemini"; then
  install_antigravity
  resolve_model_tier "gemini_antigravity"
fi

if [[ "$FRAMEWORK_LEVEL" == "full" ]]; then
  install_aos_layers
fi

if $WITH_CONFIGS; then
  install_configs
fi

if $WITH_MCP; then
  install_mcp
fi

install_generated_configs

if ! $DRY_RUN; then
  write_install_marker
fi

echo ""
echo "========================================"
echo "Installation summary"
echo "========================================"
echo "  Agents:    $AGENT_COUNT"
echo "  Skills:    $SKILL_COUNT"
echo "  Rules:     $RULE_COUNT"
echo "  Platforms: ${#DETECTED_PLATFORMS[@]} (${DETECTED_PLATFORMS[*]})"
echo ""

if $DRY_RUN; then
  echo "This was a dry run. No files were modified."
  echo "Remove --dry-run to install for real."
else
  echo "Installation complete."
  echo ""
  echo "Next steps:"
  if [[ "$MODE" == "project" ]]; then
    echo "  1. cd $TARGET_DIR"
    echo "  2. Edit CLAUDE.md — update the stack placeholders"
    echo "  3. Write a feature: cp features/TEMPLATE.md features/my-feature.md"
    echo "  4. Launch: run 'claude' and type '/deliver-feature features/my-feature.md'"
    if [[ "$FRAMEWORK_LEVEL" == "full" ]]; then
      echo ""
      echo "AOS Runtime Layers (--full install):"
      echo "  - Hook files seeded in .claude/hooks/ — all disabled by default."
      echo "    To activate: edit a hook file and set 'enabled: true', then restart your AI tool."
      echo "  - RAG interface available at .claude/rag-interface/ — see its README for opt-in path."
      echo "  - Orchestration interface available at .claude/orchestration-interface/."
      echo "    Use /orchestrate --workflow feature-delivery instead of /deliver-feature for runtime features."
      echo "  - See docs/aos/migration-guide.md for the full AOS opt-in guide."
    fi
  else
    echo "  1. Open any project with your AI tool of choice"
    echo "  2. The framework rules and personas are active globally"
    echo "  3. For project-specific setup: ./install.sh --project /path/to/project"
    if [[ "$FRAMEWORK_LEVEL" == "full" ]]; then
      echo "  4. See docs/aos/migration-guide.md for AOS opt-in steps"
    fi
  fi
fi

if ! $DRY_RUN; then
  echo ""
  echo "========================================"
  echo "Framework health check"
  echo "========================================"
  if [[ "$MODE" == "global" ]]; then
    bash "$REPO_DIR/scripts/health-check.sh" || echo "(health-check reported issues — see above; run 'bash scripts/health-check.sh --verbose --fix' for detail and auto-repair)"
  else
    # health-check.sh validates this framework's own shared/ source tree (contracts/, knowledge/,
    # memory-registry.json, CHANGELOG.md version consistency, etc.) via $REPO_DIR resolved from its
    # own script location -- none of that gets copied into a --project target, so running it here
    # would only ever re-check this checkout, never $TARGET_DIR. Running it silently against the
    # wrong repo was worse than not running it: it looked like project validation but wasn't.
    echo "Skipped for --project installs: health-check.sh checks this framework's own shared/ source"
    echo "tree, none of which is copied into a --project target -- there's nothing there for it to"
    echo "validate. The ok/skip/fail lines logged above are the verification for $TARGET_DIR."
    echo ""
    echo "To validate the framework's own source repo instead, run:"
    echo "  bash $REPO_DIR/scripts/health-check.sh --verbose"
  fi
fi

if $SHOW_TOUR && ! $DRY_RUN; then
  echo ""
  echo "========================================"
  echo "Onboarding tour"
  echo "========================================"
  echo "This script can't invoke an AI skill itself — it's plain bash. To get the tour, open your AI"
  echo "tool in this repo (or wherever you installed to) and ask it to run the 'onboard' skill:"
  echo ""
  echo "  > /onboard"
  echo ""
  echo "or in a tool without slash commands:"
  echo ""
  echo "  Act as the onboard skill described in shared/skills/onboard/SKILL.md and give me a tour."
  echo ""
  echo "It covers: the three context layers (rules/agents/skills), how to invoke an agent, how to"
  echo "trigger a skill, how to run a full pipeline, and the approval gates that pause for confirmation."
  echo "It ends by pointing you at shared/templates/my-first-feature.md — a complete, pre-written"
  echo "feature spec you can run through /deliver-feature immediately to see the whole pipeline for real."
fi
echo ""
