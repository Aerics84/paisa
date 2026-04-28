#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

collect_changed_files() {
  if [[ -n "${PRETTIER_BASE_SHA:-}" ]] &&
    [[ "${PRETTIER_BASE_SHA}" != "0000000000000000000000000000000000000000" ]] &&
    git cat-file -e "${PRETTIER_BASE_SHA}^{commit}" 2>/dev/null; then
    git diff --name-only --diff-filter=ACMRTUXB "${PRETTIER_BASE_SHA}...HEAD" -- src
    return
  fi

  if git rev-parse --verify HEAD >/dev/null 2>&1; then
    git diff --name-only --diff-filter=ACMRTUXB HEAD -- src
    git diff --cached --name-only --diff-filter=ACMRTUXB -- src
    return
  fi

  find src -type f
}

mapfile -t files < <(collect_changed_files | awk 'NF' | sort -u)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "No changed files in src to check with Prettier."
  exit 0
fi

./node_modules/.bin/prettier --check "${files[@]}"
