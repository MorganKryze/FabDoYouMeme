#!/usr/bin/env bash
# scripts/env-migrate.sh
# Keep .env.{dev,preprod,prod} in sync with their .env.*.example templates.
# Auto-detects which deployments exist by probing for example files.
#
# Usage:
#   scripts/env-migrate.sh <command> [env]
#
# Commands:
#   check       Summarize drift; exit 1 if any deployment is out of sync.
#   diff        Detailed per-variable diff with reasons.
#   migrate     Interactively bootstrap missing files and append missing
#               defaults (never overwrites values you've already set).
#
# Env (optional):
#   dev | preprod | prod   Process just that deployment.
#   (omitted)              Process all three (skipping any without an example).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

DEPLOYMENTS=(dev preprod prod)

if [[ -t 1 ]]; then
  RED=$'\033[0;31m'
  YELLOW=$'\033[0;33m'
  GREEN=$'\033[0;32m'
  BLUE=$'\033[0;34m'
  BOLD=$'\033[1m'
  DIM=$'\033[2m'
  RESET=$'\033[0m'
else
  RED='' YELLOW='' GREEN='' BLUE='' BOLD='' DIM='' RESET=''
fi

DIFF_MISSING=()
DIFF_EXTRA=()

usage() {
  cat <<'EOF'
scripts/env-migrate.sh — keep your .env.* files in sync with .env.*.example

Usage:
  scripts/env-migrate.sh <command> [env]

Commands:
  check       Summarize drift; exit 1 if anything is out of sync.
  diff        Detailed per-variable diff with reasons.
  migrate     Interactively bootstrap missing files and append missing
              defaults (never overwrites values you've already set).

Env:
  dev | preprod | prod   Process just that deployment.
  (omitted)              Process all three (skips any without an example).

Typical flow after pulling an upstream update:
  make env-check          # fast "am I out of sync?" signal
  make env-diff           # see exactly what was added, removed, or renamed
  make env-migrate        # append new defaults, keep your values untouched
EOF
  exit 2
}

# Extract the sorted list of KEY names from an env file (ignores comments/blanks).
get_keys() {
  local file="$1"
  grep -E '^[A-Z_][A-Z0-9_]*=' "$file" 2>/dev/null | sed 's/=.*//' | sort -u || true
}

# Return the raw value for a key (including quotes if present), or empty.
get_value() {
  local key="$1" file="$2"
  local line
  line=$(grep -E "^${key}=" "$file" 2>/dev/null | head -n1 || true)
  echo "${line#${key}=}"
}

# Print the definition line for KEY preceded by its attached comment block.
# A blank line or a different KEY= line breaks the comment attachment, so
# comments only travel with the variable they immediately precede.
get_definition_with_comments() {
  local key="$1" file="$2"
  awk -v key="$key" '
    BEGIN { buf = "" }
    /^[[:space:]]*#/ { buf = buf $0 "\n"; next }
    /^[[:space:]]*$/ { buf = ""; next }
    $0 ~ "^" key "=" { printf "%s%s\n", buf, $0; exit }
    { buf = "" }
  ' "$file"
}

# Populate DIFF_MISSING (in source order of the example file so related
# variables stay grouped) and DIFF_EXTRA for a single deployment.
compute_diff() {
  local example="$1" live="$2"
  local live_keys example_keys example_source_keys
  live_keys=$(get_keys "$live")
  example_keys=$(get_keys "$example")
  example_source_keys=$(grep -E '^[A-Z_][A-Z0-9_]*=' "$example" 2>/dev/null \
    | sed 's/=.*//' || true)

  DIFF_MISSING=()
  DIFF_EXTRA=()

  # Walk example in source order; emit keys not found in live.
  while IFS= read -r k; do
    [[ -z "$k" ]] && continue
    if ! echo "$live_keys" | grep -qxF "$k"; then
      DIFF_MISSING+=("$k")
    fi
  done <<< "$example_source_keys"

  # Extras: in live but not in example (alphabetical is fine — no natural order).
  while IFS= read -r k; do
    [[ -z "$k" ]] && continue
    DIFF_EXTRA+=("$k")
  done < <(comm -13 <(echo "$example_keys") <(echo "$live_keys"))
}

cmd_check() {
  local env="$1"
  local example=".env.${env}.example"
  local live=".env.${env}"

  if [[ ! -f "$example" ]]; then
    printf "  %s%s%s — no example file, skipping\n" "$DIM" "$env" "$RESET"
    return 0
  fi

  if [[ ! -f "$live" ]]; then
    printf "  %s✗ %s%s — live file %s does not exist yet\n" \
      "$YELLOW" "$env" "$RESET" "$live"
    return 1
  fi

  compute_diff "$example" "$live"
  local missing=${#DIFF_MISSING[@]}
  local extra=${#DIFF_EXTRA[@]}

  if (( missing == 0 && extra == 0 )); then
    printf "  %s✓%s %s — in sync (%s)\n" "$GREEN" "$RESET" "$env" "$example"
    return 0
  fi

  local parts=()
  (( missing > 0 )) && parts+=("${RED}${missing} missing${RESET}")
  (( extra > 0 ))   && parts+=("${YELLOW}${extra} extra${RESET}")
  local joined
  joined=$(IFS=', '; printf '%s' "${parts[*]}")
  printf "  %s✗%s %s — %s\n" "$RED" "$RESET" "$env" "$joined"
  return 1
}

cmd_diff() {
  local env="$1"
  local example=".env.${env}.example"
  local live=".env.${env}"

  printf "\n%s▶ %s%s  %s%s → %s%s\n" "$BOLD" "$env" "$RESET" "$DIM" "$example" "$live" "$RESET"

  if [[ ! -f "$example" ]]; then
    printf "  (no example file, skipping)\n"
    return 0
  fi

  if [[ ! -f "$live" ]]; then
    printf "  %s%s does not exist yet.%s Run '%smake env-migrate%s' to bootstrap from the example.\n" \
      "$YELLOW" "$live" "$RESET" "$BOLD" "$RESET"
    return 1
  fi

  compute_diff "$example" "$live"

  if (( ${#DIFF_MISSING[@]} == 0 && ${#DIFF_EXTRA[@]} == 0 )); then
    printf "  %s✓ in sync%s\n" "$GREEN" "$RESET"
    return 0
  fi

  if (( ${#DIFF_MISSING[@]} > 0 )); then
    printf "\n  %s%d variable(s) missing from %s%s\n" \
      "$RED" "${#DIFF_MISSING[@]}" "$live" "$RESET"
    printf "  %s(added to the example upstream; your live file has not picked them up yet)%s\n" \
      "$DIM" "$RESET"
    for key in "${DIFF_MISSING[@]}"; do
      local val
      val=$(get_value "$key" "$example")
      printf "    %s+ %s%s=%s\n" "$GREEN" "$key" "$RESET" "$val"
    done
  fi

  if (( ${#DIFF_EXTRA[@]} > 0 )); then
    printf "\n  %s%d variable(s) in %s not present in the example%s\n" \
      "$YELLOW" "${#DIFF_EXTRA[@]}" "$live" "$RESET"
    printf "  %s(may be deprecated — removed upstream — or a custom local override)%s\n" \
      "$DIM" "$RESET"
    for key in "${DIFF_EXTRA[@]}"; do
      printf "    %s? %s%s\n" "$YELLOW" "$key" "$RESET"
    done
  fi

  printf "\n"
  return 1
}

cmd_migrate() {
  local env="$1"
  local example=".env.${env}.example"
  local live=".env.${env}"

  printf "\n%s▶ %s%s\n" "$BOLD" "$env" "$RESET"

  if [[ ! -f "$example" ]]; then
    printf "  (no example file, skipping)\n"
    return 0
  fi

  if [[ ! -f "$live" ]]; then
    printf "  %s%s does not exist.%s Create it from %s? [y/N] " \
      "$YELLOW" "$live" "$RESET" "$example"
    local answer=''
    read -r answer || true
    if [[ "$answer" =~ ^[Yy]$ ]]; then
      cp "$example" "$live"
      printf "  %s✓ created %s%s\n" "$GREEN" "$live" "$RESET"
      printf "  %sEdit it and fill in real values before running 'make %s'.%s\n" \
        "$YELLOW" "$env" "$RESET"
    else
      printf "  skipped\n"
    fi
    return 0
  fi

  compute_diff "$example" "$live"

  if (( ${#DIFF_MISSING[@]} == 0 )); then
    if (( ${#DIFF_EXTRA[@]} > 0 )); then
      printf "  %s✓ no missing vars%s (but %d unknown var(s) still present — see 'make env-diff')\n" \
        "$GREEN" "$RESET" "${#DIFF_EXTRA[@]}"
    else
      printf "  %s✓ already in sync%s\n" "$GREEN" "$RESET"
    fi
    return 0
  fi

  printf "  %d variable(s) to add. Preview:\n" "${#DIFF_MISSING[@]}"
  for key in "${DIFF_MISSING[@]}"; do
    local val
    val=$(get_value "$key" "$example")
    printf "    %s+ %s%s=%s\n" "$GREEN" "$key" "$RESET" "$val"
  done

  printf "\n  Append these (with their example comments) to %s? [y/N] " "$live"
  local answer=''
  read -r answer || true
  if [[ ! "$answer" =~ ^[Yy]$ ]]; then
    printf "  skipped\n"
    return 0
  fi

  {
    printf "\n# --- Added by scripts/env-migrate.sh on %s ---\n" "$(date '+%Y-%m-%d')"
    local block
    for i in "${!DIFF_MISSING[@]}"; do
      block=$(get_definition_with_comments "${DIFF_MISSING[$i]}" "$example")
      # Insert a blank-line separator only when this block starts with its own
      # comments (= new section). Bare KEY= blocks continue the previous section
      # flush, so a shared preamble like "# Legal / privacy policy" stays
      # grouped with all the variables that share it.
      if (( i > 0 )) && [[ "$block" == "#"* ]]; then
        printf "\n"
      fi
      printf "%s\n" "$block"
    done
  } >> "$live"

  printf "  %s✓ appended %d variable(s) to %s%s\n" \
    "$GREEN" "${#DIFF_MISSING[@]}" "$live" "$RESET"
  printf "  %sReview the new block and edit any values that need customization.%s\n" \
    "$YELLOW" "$RESET"
}

main() {
  local cmd="${1:-}"
  local env="${2:-}"

  case "$cmd" in
    check|diff|migrate) ;;
    ""|-h|--help|help) usage ;;
    *) printf "unknown command: %s\n\n" "$cmd" >&2; usage ;;
  esac

  local targets=()
  if [[ -n "$env" ]]; then
    case "$env" in
      dev|preprod|prod) targets=("$env") ;;
      *) printf "unknown env: %s (expected: dev, preprod, prod)\n" "$env" >&2; exit 2 ;;
    esac
  else
    targets=("${DEPLOYMENTS[@]}")
  fi

  printf "\n%s%sEnv migrator — mode: %s%s\n" "$BOLD" "$BLUE" "$cmd" "$RESET"

  local failed=0
  for t in "${targets[@]}"; do
    if ! "cmd_${cmd}" "$t"; then
      failed=1
    fi
  done

  printf "\n"
  if [[ "$cmd" == "check" ]] && (( failed )); then
    printf "%s✗ drift detected — run 'make env-diff' for details or 'make env-migrate' to fix%s\n\n" \
      "$RED" "$RESET"
    exit 1
  fi
}

main "$@"
