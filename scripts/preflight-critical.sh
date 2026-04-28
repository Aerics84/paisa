#!/usr/bin/env bash
set -euo pipefail

bash ./scripts/prettier-changed.sh
bun test --preload ./src/happydom.ts src/lib/import.test.ts
go build ${GO_VERSION_LDFLAGS:-}
unset PAISA_CONFIG
TZ=UTC bun test tests/regression.test.ts
