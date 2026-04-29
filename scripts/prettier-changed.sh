#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MODE="check"
STAGED_ONLY="false"
RESTAGE="false"
FAIL_ON_PARTIAL_STAGING="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --write)
      MODE="write"
      ;;
    --staged)
      STAGED_ONLY="true"
      ;;
    --restage)
      RESTAGE="true"
      ;;
    --fail-on-partial-staging)
      FAIL_ON_PARTIAL_STAGING="true"
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
  shift
done

git_diff_names() {
  git diff --name-only --diff-filter=ACMRTUXB "$@" || true
}

has_commit() {
  local commitish="${1:-}"
  [[ -n "$commitish" ]] &&
    [[ "$commitish" != "0000000000000000000000000000000000000000" ]] &&
    git cat-file -e "${commitish}^{commit}" 2>/dev/null
}

collect_files() {
  if [[ "$STAGED_ONLY" == "true" ]]; then
    git_diff_names --cached
    return
  fi

  if has_commit "${PRETTIER_BASE_SHA:-}"; then
    git_diff_names "${PRETTIER_BASE_SHA}...HEAD"
    return
  fi

  if git rev-parse --verify HEAD >/dev/null 2>&1; then
    git_diff_names HEAD
    git_diff_names --cached
    return
  fi

  git ls-files
}

mapfile -t FILES < <(collect_files | awk "NF" | sort -u)

EXISTING_FILES=()
for file in "${FILES[@]}"; do
  if [[ -e "$file" ]]; then
    EXISTING_FILES+=("$file")
  fi
done

FILES=("${EXISTING_FILES[@]}")

if [[ "$FAIL_ON_PARTIAL_STAGING" == "true" && "$STAGED_ONLY" == "true" && "${#FILES[@]}" -gt 0 ]]; then
  mapfile -t UNSTAGED_FILES < <(git_diff_names | awk "NF" | sort -u)
  PARTIAL_FILES=()

  for file in "${FILES[@]}"; do
    if printf "%s\n" "${UNSTAGED_FILES[@]:-}" | grep -Fxq "$file"; then
      PARTIAL_FILES+=("$file")
    fi
  done

  if [[ "${#PARTIAL_FILES[@]}" -gt 0 ]]; then
    echo "Prettier auto-fix skipped because these staged files also have unstaged changes:" >&2
    printf -- "- %s\n" "${PARTIAL_FILES[@]}" >&2
    echo "Stage the full file or run \`npm run format:changed\` before committing." >&2
    exit 1
  fi
fi

if [[ "${#FILES[@]}" -eq 0 ]]; then
  if [[ "$STAGED_ONLY" == "true" ]]; then
    echo "No staged files to check with Prettier."
  else
    echo "No changed files to check with Prettier."
  fi
  exit 0
fi

./node_modules/.bin/prettier --plugin-search-dir . "--${MODE}" --ignore-unknown "${FILES[@]}"

if [[ "$RESTAGE" == "true" ]]; then
  git add -- "${FILES[@]}"
fi
