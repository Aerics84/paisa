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

| Script                  | Command                                               | Purpose                   |
| ----------------------- | ----------------------------------------------------- | ------------------------- |
| `npm run test:unit`     | `bun test --preload ./src/happydom.ts src`            | Full frontend suite       |
| `npm run test:coverage` | `bun test --coverage --preload ./src/happydom.ts src` | Full suite with coverage  |
| `npm run test:all`      | `npm run test:unit && npm run test:regression`        | Frontend plus CLI goldens |

`npm run test:coverage` reads [`bunfig.toml`](../../bunfig.toml), which sets

```toml
[test]
coverageThreshold = { lines = 0.75, functions = 0.75 }
```

The threshold is a ratchet: it currently sits below the baseline and is raised as coverage improves. Line and function coverage are the only metrics covered by the threshold. See [Threshold enforcement status](#threshold-enforcement-status-2026-11-24) for how far Bun 1.4.4 actually applies it.

## Bun 1.4.4 limitation: no branch coverage

Bun 1.4.4 reports only `% Funcs` and `% Lines`. Even with `--coverage-reporter=lcov` it emits no `BRF`/`BRDA` records, so branch coverage cannot be measured or enforced with the current runner. Rather than invent a threshold that the runner cannot check, this program adds targeted branch tests for authentication, configuration and migrations (Tasks 1.1–1.3, 3.1–3.2) and keeps the numeric gate on lines and functions.

## Threshold enforcement status (2026-11-24)

On Bun 1.4.4 the `coverageThreshold` key is parsed and validated — an invalid value aborts the run with `Invalid Bunfig` — but the runner does not fail the process when the reported coverage is below the threshold. Verified by temporarily setting `coverageThreshold = { lines = 1.0, functions = 1.0 }` against a file at 98 % lines: the table still reported the shortfall and `bun test` exited 0.

Treat the aggregate `All files` row as the authoritative number to check on every change, and keep reviewing the `test:coverage` output in CI until a Bun release enforces the threshold. If enforcement stays unavailable, the follow-up is an aggregate check that parses the `All files` row instead of relying on `bunfig.toml`.

## Where the suite runs

- `.github/workflows/test.yml` runs `npm run test:unit` and `npm run test:coverage` after `npm run check`.
- `.github/workflows/npm12-gate.yml` runs the same two steps after the P-JS gate on Linux, macOS and Windows.
- `scripts/preflight-critical.mjs` (`npm run test:prepush`) runs the full frontend suite before the build and Go steps. The suite itself takes roughly 1.5 s, well inside the pre-push budget.
