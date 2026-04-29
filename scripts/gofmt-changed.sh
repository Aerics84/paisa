#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

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

mapfile -t FILES < <(collect_files | awk '/\.go$/ { print }' | sort -u)

EXISTING_FILES=()
for file in "${FILES[@]}"; do
  if [[ -f "$file" ]]; then
    EXISTING_FILES+=("$file")
  fi
done

FILES=("${EXISTING_FILES[@]}")

if [[ "${#FILES[@]}" -eq 0 ]]; then
  echo "No changed Go files to check with gofmt."
  exit 0
fi

test -z "$(gofmt -l "${FILES[@]}")"
