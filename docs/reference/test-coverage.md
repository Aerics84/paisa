# Frontend Test Coverage

This page records how the Svelte frontend test suite is run in CI and before a push, and what its coverage numbers mean.

## Baseline (measured 2026-11-24)

```bash
bun test --coverage --preload ./src/happydom.ts src
```

| Metric     | Value |
| ---------- | ----- |
| Test files | 46    |
| Tests      | 306   |
| `% Funcs`  | 78.77 |
| `% Lines`  | 76.98 |
| `expect()` | 731   |

The historical 43 files / 292 tests from the measurement baseline are superseded by these numbers; the counts above are what the suite reports on the current branch.

## Commands

| Script                   | Command                                               | Purpose                          |
| ------------------------ | ----------------------------------------------------- | -------------------------------- |
| `npm run test:unit`      | `bun test --preload ./src/happydom.ts src`            | Full frontend suite              |
| `npm run test:coverage`  | `bun test --coverage --preload ./src/happydom.ts src` | Full suite with coverage (human) |
| `npm run check:coverage` | `node ./scripts/check-coverage.mjs`                   | Coverage gate (CI)               |
| `npm run test:all`       | `npm run test:unit && npm run test:regression`        | Frontend plus CLI goldens        |

## Coverage gate

`npm run check:coverage` runs `bun test --coverage --preload ./src/happydom.ts src`, parses the `All files` row (`% Funcs`, `% Lines`), prints the measured values next to the thresholds and exits `1` when a metric is below its threshold. `2` signals a configuration or parsing error; a failing test suite keeps its own non-zero exit code.

The thresholds default to 75 % for lines and functions and are a ratchet: they start below the baseline and are raised as coverage improves. Override them for a one-off check, for example `node ./scripts/check-coverage.mjs --lines 90 --functions 85`.

## Why the gate is a script and not `bunfig.toml`

Bun's own `[test] coverageThreshold` is not an option here: Bun 1.4.4 applies it **per file**, not to the aggregate. With `coverageThreshold = { lines = 0.75, functions = 0.75 }` the full suite still reports `306 pass`, `All files 76.98 %`, and exits `1` without a message, because individual files such as `src/lib/networth.ts` (4 % lines) or `src/persisted_store.ts` (0 % funcs) are below the per-file threshold. A per-file ratchet would either fail permanently or have to be set to a value that checks nothing.

`bunfig.toml` therefore deliberately sets no threshold; it carries a comment pointing at `scripts/check-coverage.mjs`, which is the authoritative aggregate gate and the only coverage command wired into CI. `npm run test:coverage` stays available for humans who just want the table.

Behaviour of the script:

| Situation                       | Exit code                            |
| ------------------------------- | ------------------------------------ |
| Coverage at or above threshold  | `0`                                  |
| Coverage below a threshold      | `1`, one line per failed metric      |
| Test suite failed               | its own exit code, coverage not read |
| Missing/invalid `All files` row | `2`                                  |
| Invalid argument                | `2`                                  |

## Bun 1.4.4 limitation: no branch coverage

Bun 1.4.4 reports only `% Funcs` and `% Lines`. Even with `--coverage-reporter=lcov` it emits no `BRF`/`BRDA` records, so branch coverage cannot be measured or enforced with the current runner. Rather than invent a threshold that the runner cannot check, this program adds targeted branch tests for authentication, configuration and migrations (Tasks 1.1–1.3, 3.1–3.2) and keeps the numeric gate on lines and functions.

## Where the suite runs

- `.github/workflows/test.yml` runs `npm run check:coverage` after `npm run check`; the gate executes the full suite, so CI does not run `test:unit` a second time.
- `.github/workflows/npm12-gate.yml` runs `npm run check:coverage` after the P-JS gate on Linux, macOS and Windows.
- `scripts/preflight-critical.mjs` (`npm run test:prepush`) runs the full frontend suite before the build and Go steps. The suite itself takes roughly 1.5 s, well inside the pre-push budget. The pre-push hook does not run the coverage gate; keep that in CI only.
