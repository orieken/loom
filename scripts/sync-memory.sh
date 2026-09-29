#!/usr/bin/env bash
set -euo pipefail

# Enterprise KI sync for the Context Engineering Framework.
# See docs/adrs/ADR-003-enterprise-memory-sync.md for design rationale.
#
# Usage:
#   sync-memory.sh [pull|push] [--confirm] [--config <path>]
#
# pull  (default) — diff and optionally apply org KIs into shared/knowledge/
# push            — promote local .claude/knowledge/ KIs to the org repo via PR

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_DIR="$REPO_DIR/shared"
KI_SCHEMA="$SHARED_DIR/schemas/ki-frontmatter.schema.json"
VALIDATE_SCRIPT="$REPO_DIR/scripts/validate-frontmatter.py"

SUBCOMMAND="pull"
CONFIRM=false
CONFIG_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    pull|push) SUBCOMMAND="$1"; shift ;;
    --confirm) CONFIRM=true; shift ;;
    --config)  CONFIG_PATH="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: sync-memory.sh [pull|push] [--confirm] [--config <path>]"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ -z "$CONFIG_PATH" ]]; then
  CONFIG_PATH="$REPO_DIR/.claude/sync-config.yaml"
fi

if [[ ! -f "$CONFIG_PATH" ]]; then
  cat <<'HELP'
Error: .claude/sync-config.yaml not found.

Create it at the root of your framework checkout:

  memory_sync:
    org_repo: git@github.com:<your-org>/knowledge-hub.git
    cache_dir: ~/.claude/sync-cache
    push_pr_base: main

See docs/adrs/ADR-003-enterprise-memory-sync.md for details.
HELP
  exit 1
fi

# Simple key reader for the minimal YAML structure (no pyyaml required).
read_config_key() {
  local key="$1"
  local default="${2:-}"
  local val
  # POSIX classes, not \s: BSD sed (macOS) has no \s, so the value kept its
  # leading space and every pull tried to clone ' git@github.com:...'.
  val=$(grep -E "^[[:space:]]*${key}:" "$CONFIG_PATH" 2>/dev/null | head -1 | sed -E "s/.*${key}:[[:space:]]*//; s/[[:space:]]+$//" | tr -d '"' || true)
  echo "${val:-$default}"
}

ORG_REPO=$(read_config_key "org_repo" "")
CACHE_BASE=$(read_config_key "cache_dir" "~/.claude/sync-cache")
PUSH_BASE=$(read_config_key "push_pr_base" "main")
CACHE_BASE="${CACHE_BASE/#\~/$HOME}"

if [[ -z "$ORG_REPO" ]]; then
  echo "Error: memory_sync.org_repo not set in $CONFIG_PATH"
  exit 1
fi

# Auth: MEMORY_SYNC_TOKEN env var converts SSH URL to HTTPS for enterprises
# where SSH is disabled.
#
# SECURITY (THREAT_MODEL.md F-05): Embedding a token in a git URL exposes it in:
#   - `ps aux` output during clone/fetch (visible to other users on shared machines)
#   - git remote config cached at $CACHE_DIR/.git/config (`git remote -v` leaks it)
#   - shell history if set inline: `MEMORY_SYNC_TOKEN=ghp_xxx bash sync-memory.sh`
#
# Safer alternatives (use one if your platform supports it):
#   1. SSH key auth — leave org_repo as git@github.com:org/repo.git; no token needed.
#   2. GH_TOKEN env var — the `gh` CLI picks this up natively for GitHub operations.
#   3. git credential helper — `git config --global credential.helper store` or
#      `git config --global credential.helper osxkeychain` (macOS); the helper supplies
#      credentials without embedding them in URLs.
#
# If you must use MEMORY_SYNC_TOKEN, never set it inline in a shell command that
# appears in your history. Instead, export it from a secret manager or CI secrets store.
#
# CLONE_URL carries the token and is used for git's clone alone. ORG_REPO stays
# token-free: it is printed, and stamped into every pulled KI's sync_source —
# which, before this was split (roadmap L3.7), wrote the token into
# shared/knowledge/*.md, the files this script then tells you to commit.
CLONE_URL="$ORG_REPO"
if [[ -n "${MEMORY_SYNC_TOKEN:-}" && "$ORG_REPO" =~ ^git@github.com: ]]; then
  CLONE_URL=$(echo "$ORG_REPO" | sed "s|git@github.com:|https://$MEMORY_SYNC_TOKEN@github.com/|")
fi

# KIs the injection scan flagged, reported at the end of the pull. Held in
# a variable, not a temp file: a temp file needs an EXIT trap to clean up, and
# under bash 3.2 (macOS) an EXIT trap turns an unbound-variable crash into
# exit 0 — a pull that crashed would report success. No trap guards the pull.
FLAGGED=()

SLUG=$(echo "$ORG_REPO" | sed 's|.*[:/]||; s|\.git$||')
CACHE_DIR="$CACHE_BASE/$SLUG"
LAST_SYNC_FILE="$CACHE_DIR/.last-sync"

log()  { echo "  $1"; }
ok()   { echo "  [ok] $1"; }
info() { echo "  [info] $1"; }
warn() { echo "  [warn] $1"; }

# --- Validate a KI file against the frontmatter schema (best-effort) --------
validate_ki() {
  local ki_file="$1"
  if command -v python3 &>/dev/null && [[ -f "$VALIDATE_SCRIPT" && -f "$KI_SCHEMA" ]]; then
    python3 "$VALIDATE_SCRIPT" "$KI_SCHEMA" "$ki_file" > /dev/null 2>&1
  fi
}

# --- Injection scan (roadmap L3.7) --------------------------------------------
# Every org KI is scanned for instruction-override patterns before anything is
# written — dry run included — so a poisoned KI is flagged before it can be
# pulled. The scan is `loom ki scan`; frontmatter validation above is
# best-effort, but this is not: if no scanner can run, nothing is pulled.
#
# Scanner resolution: $LOOM_BIN when set (and then nothing else), else `loom`
# on PATH, else `go run ./cmd/loom` from this checkout. Each candidate must
# answer `ki scan --help`, so an older loom without the command is skipped
# rather than mistaken for one reporting findings.
SCANNER_CMD=()

scanner_answers() {
  "$@" ki scan --help > /dev/null 2>&1
}

resolve_scanner() {
  if [[ -n "${LOOM_BIN:-}" ]]; then
    scanner_answers "$LOOM_BIN" && SCANNER_CMD=("$LOOM_BIN")
    return 0
  fi
  if command -v loom &>/dev/null && scanner_answers loom; then
    SCANNER_CMD=(loom)
  elif [[ -f "$REPO_DIR/go.mod" ]] && command -v go &>/dev/null && (cd "$REPO_DIR" && scanner_answers go run ./cmd/loom); then
    SCANNER_CMD=(go run "$REPO_DIR/cmd/loom")
  fi
}

# scan_org_ki prints findings and returns 0 clean, 1 flagged, anything else
# when the file could not be scanned.
scan_org_ki() {
  local file="$1" output status=0
  output=$(cd "$REPO_DIR" && "${SCANNER_CMD[@]}" ki scan "$file" 2>&1) || status=$?
  printf '%s\n' "$output" | grep -v '^clean:' | sed 's/^/             /' || true
  return "$status"
}

# --- Resolve target KI directory inside a repo root -------------------------
find_ki_dir() {
  local root="$1"
  for candidate in "$root/shared/knowledge" "$root/knowledge"; do
    if [[ -d "$candidate" ]]; then
      echo "$candidate"; return
    fi
  done
  echo ""
}

# --- Update or create the org cache (clone on first run, fetch on repeat) ---
refresh_cache() {
  if [[ -d "$CACHE_DIR/.git" ]]; then
    log "Updating cached org repo ($SLUG)..."
    git -C "$CACHE_DIR" fetch origin --quiet
    git -C "$CACHE_DIR" reset --hard "origin/$PUSH_BASE" --quiet 2>/dev/null || \
      git -C "$CACHE_DIR" reset --hard origin/HEAD --quiet
    ok "cache updated"
  else
    log "Cloning org repo (first run)..."
    mkdir -p "$CACHE_BASE"
    git clone --depth=1 "$CLONE_URL" "$CACHE_DIR" --quiet
    ok "cloned to $CACHE_DIR"
  fi
}

# === PULL ===================================================================
pull_kis() {
  echo ""
  echo "=== Memory Sync: Pull ==="
  echo "Org repo: $ORG_REPO"
  echo "Cache:    $CACHE_DIR"
  echo ""

  refresh_cache

  local org_ki_dir
  org_ki_dir=$(find_ki_dir "$CACHE_DIR")
  if [[ -z "$org_ki_dir" ]]; then
    warn "No knowledge/ or shared/knowledge/ directory found in org repo."
    warn "The org repo should contain KIs under knowledge/ or shared/knowledge/."
    exit 1
  fi

  local local_ki_dir="$SHARED_DIR/knowledge"
  local project_ki_dir="$REPO_DIR/.claude/knowledge"

  # Phase 1: collision detection (org name slug vs project KI slug)
  local collisions=0
  for org_ki in "$org_ki_dir"/*.md; do
    [[ -f "$org_ki" ]] || continue
    base="$(basename "$org_ki")"
    [[ "$base" == "README.md" ]] && continue
    if [[ -f "$project_ki_dir/$base" ]]; then
      org_name=$(grep '^name:' "$org_ki" | head -1 | sed 's/name: *//' || true)
      local_name=$(grep '^name:' "$project_ki_dir/$base" | head -1 | sed 's/name: *//' || true)
      if [[ "$org_name" == "$local_name" ]]; then
        echo "  COLLISION  $base (org name '$org_name' matches .claude/knowledge/$base)"
        ((collisions++)) || true
      fi
    fi
  done

  if [[ "$collisions" -gt 0 ]]; then
    echo ""
    echo "ERROR: $collisions slug collision(s) between org KIs and .claude/knowledge/."
    echo "Rename the conflicting project or org KI before running --confirm."
    exit 1
  fi

  # Phase 2: diff
  local to_add=()
  local to_update=()

  echo "Diffing org KIs against local shared/knowledge/..."
  echo ""

  for org_ki in "$org_ki_dir"/*.md; do
    [[ -f "$org_ki" ]] || continue
    base="$(basename "$org_ki")"
    [[ "$base" == "README.md" ]] && continue

    local_ki="$local_ki_dir/$base"
    if [[ ! -f "$local_ki" ]]; then
      echo "  + ADD     $base"
      to_add+=("$base")
    elif ! diff -q "$org_ki" "$local_ki" > /dev/null 2>&1; then
      echo "  ~ UPDATE  $base"
      diff "$local_ki" "$org_ki" | grep -E '^[<>]' | head -4 | sed 's/^/             /' || true
      to_update+=("$base")
    fi
  done

  # Phase 2b: injection scan — flagged KIs are dropped from the pull.
  if [[ $(( ${#to_add[@]} + ${#to_update[@]} )) -gt 0 ]]; then
    screen_pending_kis "$org_ki_dir"
  fi

  local changes=$(( ${#to_add[@]} + ${#to_update[@]} ))

  if [[ "$changes" -eq 0 ]]; then
    report_flagged
    echo "  (no changes — shared/knowledge/ is up to date with org repo)"
    date -u "+%Y-%m-%dT%H:%M:%SZ" > "$LAST_SYNC_FILE"
    echo ""
    echo "Last-sync timestamp updated."
    return
  fi

  echo ""
  echo "$changes change(s) pending."

  if ! $CONFIRM; then
    echo ""
    echo "Dry run — no files changed. Re-run with --confirm to apply:"
    echo "  bash scripts/sync-memory.sh pull --confirm"
    report_flagged
    return
  fi

  # Phase 3: apply
  echo ""
  echo "Applying changes..."

  # The +"..." form: bash 3.2 (macOS) calls an empty array unbound under set -u.
  for base in ${to_add[@]+"${to_add[@]}"} ${to_update[@]+"${to_update[@]}"}; do
    org_ki="$org_ki_dir/$base"
    local_ki="$local_ki_dir/$base"

    if ! validate_ki "$org_ki" 2>/dev/null; then
      warn "SKIP $base — failed KI frontmatter validation (malformed org KI)"
      continue
    fi

    # Stamp sync_source + sync_pulled + sync_commit_sha if not already present.
    # sync_commit_sha records the org repo's HEAD SHA at pull time, enabling auditors
    # to trace which org-repo state introduced this KI (THREAT_MODEL.md F-04).
    local org_head_sha
    org_head_sha=$(git -C "$CACHE_DIR" rev-parse HEAD 2>/dev/null || echo "unknown")
    if ! grep -q '^sync_source:' "$org_ki"; then
      local today
      today=$(date +%Y-%m-%d)
      python3 - "$org_ki" "$local_ki" "$ORG_REPO" "$today" "$org_head_sha" <<'PYEOF'
import sys
src, dst, repo, today, sha = sys.argv[1:]
content = open(src).read()
parts = content.split('---', 2)
if len(parts) >= 3:
    parts[1] = parts[1].rstrip('\n') + (
        f'\nsync_source: {repo}\nsync_pulled: {today}\nsync_commit_sha: {sha}\n'
    )
    content = '---'.join(parts)
open(dst, 'w').write(content)
PYEOF
    else
      cp "$org_ki" "$local_ki"
    fi
    ok "applied $base"
  done

  date -u "+%Y-%m-%dT%H:%M:%SZ" > "$LAST_SYNC_FILE"

  echo ""
  echo "Sync complete. $changes KI(s) written to shared/knowledge/."
  echo "Review, then commit:"
  echo "  git add shared/knowledge/ && git commit -m 'chore(knowledge): sync KIs from org repo'"
  report_flagged
}

# screen_pending_kis scans every pending org KI and removes the flagged ones
# from to_add and to_update (both callers' locals, by bash's dynamic scope).
# A file the scanner could not scan aborts the whole pull: an unscanned KI
# must never be written.
screen_pending_kis() {
  local org_ki_dir="$1"
  resolve_scanner
  if [[ ${#SCANNER_CMD[@]} -eq 0 ]]; then
    echo ""
    echo "ERROR: no injection scanner available — refusing to pull unscanned KIs."
    echo "Install loom (with 'loom ki scan'), set LOOM_BIN, or run from a framework checkout with Go."
    exit 1
  fi
  echo ""
  echo "Scanning pending KIs for instruction-override patterns..."
  # screen_list runs in a subshell, where its exit stops only itself: the
  # status is checked here so an unscannable file really aborts the pull.
  local screened verdict kind base
  screened=$(
    screen_list add "$org_ki_dir" ${to_add[@]+"${to_add[@]}"}
    screen_list update "$org_ki_dir" ${to_update[@]+"${to_update[@]}"}
  ) || exit 1
  to_add=()
  to_update=()
  while read -r verdict kind base; do
    case "$verdict:$kind" in
      clean:add)    to_add+=("$base") ;;
      clean:update) to_update+=("$base") ;;
      flagged:*)    FLAGGED+=("$base") ;;
    esac
  done <<< "$screened"
}

# screen_list prints a verdict line per KI — "clean <kind> <name>" or
# "flagged <kind> <name>" — and exits the subshell on one it cannot scan.
screen_list() {
  local kind="$1" org_ki_dir="$2"; shift 2
  local base status
  for base in "$@"; do
    status=0
    scan_org_ki "$org_ki_dir/$base" >&2 || status=$?
    case "$status" in
      0) echo "clean $kind $base" ;;
      1) echo "  ! FLAGGED $base — not pulled" >&2; echo "flagged $kind $base" ;;
      *) echo "ERROR: could not scan $base (scanner exit $status) — refusing to pull." >&2; exit 1 ;;
    esac
  done
}

# report_flagged ends a pull that dropped flagged KIs with a non-zero exit, so
# automation notices: a flagged KI is a human's decision, not a warning.
report_flagged() {
  [[ ${#FLAGGED[@]} -gt 0 ]] || return 0
  echo ""
  echo "FLAGGED: ${#FLAGGED[@]} org KI(s) matched instruction-override patterns and were NOT pulled:"
  printf '  %s\n' "${FLAGGED[@]}"
  echo "Read each one in $CACHE_DIR. If it is safe, copy it into shared/knowledge/ by hand —"
  echo "a person deciding, never this script. See shared/rules/memory-trust-boundary.md."
  exit 1
}

# === PUSH ===================================================================
push_kis() {
  echo ""
  echo "=== Memory Sync: Push ==="
  echo "Org repo: $ORG_REPO"
  echo ""

  local project_ki_dir="$REPO_DIR/.claude/knowledge"
  if [[ ! -d "$project_ki_dir" ]]; then
    info "No .claude/knowledge/ directory — nothing to push."
    exit 0
  fi

  # Promotion heuristic: KIs created more than 30 days ago
  local thirty_days_ago
  thirty_days_ago=$(python3 -c "
import datetime
print((datetime.date.today() - datetime.timedelta(days=30)).isoformat())
" 2>/dev/null || date -v-30d +%Y-%m-%d 2>/dev/null || echo "1970-01-01")

  local candidates=()
  echo "Candidate KIs (created > 30 days ago, excluding already-synced):"

  for ki in "$project_ki_dir"/*.md; do
    [[ -f "$ki" ]] || continue
    base="$(basename "$ki")"
    [[ "$base" == "README.md" ]] && continue

    # Skip if already came from sync
    grep -q '^sync_source:' "$ki" && continue

    created=$(grep '^created:' "$ki" | head -1 | sed 's/created: *//' | tr -d '"' || true)
    if [[ -z "$created" || "$created" < "$thirty_days_ago" ]]; then
      name=$(grep '^name:' "$ki" | head -1 | sed 's/name: *//' || true)
      echo "  + $base  (name: $name, created: ${created:-unknown})"
      candidates+=("$base")
    fi
  done

  if [[ "${#candidates[@]}" -eq 0 ]]; then
    info "No eligible KIs to push (none older than 30 days, or all already synced from org)."
    exit 0
  fi

  echo ""
  if ! $CONFIRM; then
    echo "Dry run — re-run with --confirm to create a PR:"
    echo "  bash scripts/sync-memory.sh push --confirm"
    return
  fi

  local tmp_dir
  tmp_dir=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp_dir'" EXIT

  log "Cloning org repo to temp dir..."
  git clone --depth=1 "$CLONE_URL" "$tmp_dir" --quiet
  ok "cloned"

  local branch_name="ki-sync/$(basename "$REPO_DIR")-$(date +%Y%m%d)"
  git -C "$tmp_dir" checkout -b "$branch_name" --quiet

  local org_ki_target
  org_ki_target=$(find_ki_dir "$tmp_dir")
  if [[ -z "$org_ki_target" ]]; then
    mkdir -p "$tmp_dir/shared/knowledge"
    org_ki_target="$tmp_dir/shared/knowledge"
  fi

  for base in "${candidates[@]}"; do
    cp "$project_ki_dir/$base" "$org_ki_target/$base"
    ok "staged $base"
  done

  git -C "$tmp_dir" add "$org_ki_target" --quiet
  git -C "$tmp_dir" commit -m "feat(knowledge): promote KIs from $(basename "$REPO_DIR")" --quiet

  git -C "$tmp_dir" push origin "$branch_name" --quiet

  # Open PR via gh CLI when available, otherwise print instructions
  if command -v gh &>/dev/null && [[ "$ORG_REPO" =~ github.com ]]; then
    local org_slug
    org_slug=$(echo "$ORG_REPO" | sed 's|.*github.com[:/]||; s|\.git$||')
    local pr_body
    pr_body="## KI Promotion from \`$(basename "$REPO_DIR")\`

$(for base in "${candidates[@]}"; do
  name=$(grep '^name:' "$project_ki_dir/$base" | head -1 | sed 's/name: *//' || true)
  echo "- \`$base\` — $name"
done)

Promoted via \`sync-memory.sh push\` on $(date +%Y-%m-%d)."

    gh pr create \
      --repo "$org_slug" \
      --head "$branch_name" \
      --base "$PUSH_BASE" \
      --title "feat(knowledge): promote KIs from $(basename "$REPO_DIR")" \
      --body "$pr_body"
    ok "PR opened in $org_slug"
  else
    echo ""
    echo "Branch pushed. Open a PR manually:"
    echo "  Org repo: $ORG_REPO"
    echo "  Branch:   $branch_name"
    echo "  Base:     $PUSH_BASE"
  fi
}

# === Dispatch ===============================================================
case "$SUBCOMMAND" in
  pull) pull_kis ;;
  push) push_kis ;;
  *)    echo "Unknown subcommand: '$SUBCOMMAND' — use 'pull' or 'push'"; exit 1 ;;
esac
