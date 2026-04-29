#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  exit 0
fi

git config core.hooksPath .githooks

if [[ -f ".githooks/pre-commit" ]]; then
  chmod +x .githooks/pre-commit
fi

if [[ -f ".githooks/pre-push" ]]; then
  chmod +x .githooks/pre-push
fi

echo "Configured git hooks path to .githooks"
