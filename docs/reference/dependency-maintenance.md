# JavaScript Toolchain Maintenance

This page defines Paisa's JavaScript maintenance contract so routine dependency updates stay boring and framework migrations stay isolated.

## Runtime Baseline

- Primary JavaScript runtime: Node 24.22.1
- Primary package manager: npm 12.2.1
- Local version source of truth: [`.nvmrc`](../../.nvmrc)
- Repository metadata source of truth: [`package.json`](../../package.json)
- Secondary runtime: Bun 1.4.4, used only for the targeted import and regression test commands already defined in `package.json`
- Secondary runtime version source of truth: [`.bun-version`](../../.bun-version)

GitHub Actions, local development, and Nix-based workflows should converge on this baseline unless a workflow documents a deliberate exception.

## Refresh Status (2026-10-28)

- Phases 0–6 are complete; the audit gate and the freeze report are in place.
- hledger is on 1.52.4 (Hackage source override; issue #2254 was fixed in 1.41).
- beancount stays on 2.3.6: v3 removes `bean-report` and moves `bean-query` to a separate package (see [Parser Tooling](#parser-tooling)).
- npm 12.2.1 was adopted after the refresh merged; the default-branch gate passed on 2026-10-28 (see [npm 12](#npm-12)).
- Nix uses `nixos-26.05`, `buildNpmPackage` for the web assets, and no longer uses node2nix.

## Freeze Snapshot (2026-10-28)

Validated end state before the freeze commit:

| Component                                             | Version                                |
| ----------------------------------------------------- | -------------------------------------- |
| Node / npm                                            | 24.22.1 / 11.19.1                      |
| Bun / bun-types                                       | 1.4.4 / 1.4.4                          |
| Go                                                    | 1.26.10                                |
| hledger                                               | 1.52.4 (Hackage override)              |
| beancount                                             | 2.3.6 (v3 deferred)                    |
| Svelte / SvelteKit / Vite                             | 5.57.3 / 3.1.0 / 8.3.5                 |
| TypeScript / ESLint / Prettier                        | 6.0.3 / 10.13.0 / 3.9.10               |
| Tabulator / bignumber / chroma / formatjs / financial | 6.7.1 / 11.1.5 / 3.2.0 / 9.4.3 / 0.2.4 |

`npm outdated` reports only `typescript 6.0.3 → 7.0.2`, deliberately deferred because typescript-eslint, SvelteKit and svelte-check do not accept TypeScript 7 yet. `go list -m -u all` reports 73 indirect module updates; all direct modules are current and the indirect list is documented above.

This snapshot is historical: npm moved to 12.2.1 on 2026-10-29 after the default-branch gate passed. All other rows remain current.

Security findings at the freeze: root npm 4 low, `tools/fontgen` npm 7 moderate, 0 high/critical, govulncheck 0 called and 1 not-called. No scanner errors. The follow-ups are beancount 3 (bean-report replacement), the indirect Go module set, and the NixOS 26.11 refresh within two weeks of its release.

## Dependency Audit Gate

CI runs `node scripts/security-gate.mjs --report .cache/security` after the test steps and uploads the report for 90 days. `make audit` is the local equivalent.

Exit codes:

- `0`: no confirmed finding.
- `1`: a confirmed security finding (npm critical/high or Go called findings).
- `2`: scanner or target error (invalid JSON, missing `metadata`, unavailable tool, unreadable lockfile). Exit 2 is **not** a clean security result.

The report must contain npm entries for both the repository root and `tools/fontgen`. Moderate/low findings are reported in `summary.json`, evaluated in the freeze report, and do not fail the gate.

## Dependabot

Dependabot checks npm (root and `tools/fontgen`), gomod, GitHub Actions and Docker weekly, with five open PRs per ecosystem. TypeScript major updates are ignored deliberately because the surrounding toolchain (typescript-eslint, SvelteKit, svelte-check) does not accept TypeScript 7 yet; security updates stay enabled. The audit gate intentionally blocks PRs with confirmed critical/high findings.

## npm 12

npm 12.2.1 is the supported package manager since 2026-10-29. The refresh itself stayed on npm 11.19 because `workflow_dispatch` workflows must be registered on the default branch, so the gate was built afterwards, run green, and only then were `packageManager`, `engines`, workflows and Dockerfiles switched.

The historical local pre-check on 2026-10-28 used `npx -y npm@12.2.1` (`--version`, `ci`, `run check`, `run build`): all exited 0 and `git diff --exit-code package-lock.json` stayed clean. Full `npm run lint` remains red on the Windows checkout because of the pre-existing CRLF Prettier diff across 241 files; the changed-file gate used by CI is unaffected. This pre-check was informative only, not a binding gate.

The upgrade sequence was:

1. `workflow_dispatch` workflows must be registered on the default branch. An `npm12-gate.yml` created only on a temporary branch is not a reliable trigger and can fail with `gh workflow run --ref`.
2. Add `npm12-gate.yml` with `workflow_dispatch` on the default branch.
3. Matrix: `ubuntu-latest`, `windows-latest`, `macos-latest`.
4. Acceptance: `npm -v` reports `12.2.1` on all three platforms; `package-lock.json` stays unchanged; P-JS, P-REG and P-PDF are green; all three matrix jobs are found and report `success`.
5. Only then switch `packageManager`, workflows and Dockerfiles, and inventory every npm call site.

The gate itself is `.github/workflows/npm12-gate.yml`. It is manual (`workflow_dispatch`) and must be started from the default branch:

```bash
gh workflow run npm12-gate.yml --ref master
gh run list --workflow npm12-gate.yml --limit 1
```

Each matrix job installs npm 12.2.1 over the runner's bundled npm, runs `npm ci`, asserts that `package-lock.json` is unchanged, and then runs P-JS (`check`, `build`, imports, investment, monthly), P-PDF (`src/lib/pdf.test.ts`) and P-REG (`go build`, `TZ=UTC bun test tests/regression.test.ts`). A green run on all three platforms is the evidence required by step 4; the run URL is recorded here once it passes.

The gate passed on 2026-10-28 on `master` at `6caa0ab`: [run 40782020978](https://github.com/Aerics84/paisa/actions/runs/40782020978) reported `success` for `gate (ubuntu-latest)`, `gate (windows-latest)` and `gate (macos-latest)`. Each job printed `12.2.1` before and after `npm ci`, the lockfile stayed unchanged, P-PDF passed 2/2 and P-REG passed 6/6. Linux ran P-REG in the Nix dev shell (ledger 3.4.1, hledger 1.52.4, beancount 2.3.6); macOS used the same versions from Homebrew and pip. Windows used the regression test's built-in tolerance for missing ledger CLIs.

"Lockfile unchanged" proves only the lockfile state, **not** identical platform-specific install trees. Acceptance stays limited to "lockfile unchanged + tests green on all platforms".

Adoption (2026-10-29): `packageManager` and `engines.npm` pin `12.2.1`, `Test` and the four release workflows install npm 12.2.1 before `npm ci`, and `Dockerfile` / `Dockerfile.demo` do the same before building the web assets. `tools/fontgen` shares that npm because it is driven through `npm --prefix`. The Nix `buildNpmPackage` derivation is the documented exception: it keeps the npm bundled with `pkgs.nodejs_24`, reads the same lockfile and produces the same asset tree. Re-run `npm12-gate.yml` after any change to the npm baseline or to `package-lock.json`.

## Routine Refreshes

These changes are safe to batch together when they stay within the current major version:

- npm patch and minor updates
- GitHub Action patch and minor updates
- lockfile refreshes
- editor and lint tooling refreshes that do not cross a major version boundary

Expected validation for a routine refresh:

```bash
npm run build
npm run check
npm run lint
npm run test:imports
npm run test:regression
```

For local change safety, `git push` also runs the critical subset automatically via [`scripts/preflight-critical.mjs`](../../scripts/preflight-critical.mjs): changed-file formatting, `npm run check`, import and regression tests, `npm run build`, and a Go build.

The web assets and the Go binary are built from the flake. When `package-lock.json` or `go.sum` changes, refresh the flake inputs and let Nix report the new hashes:

```bash
nix flake update nixpkgs
nix build .#paisa-web
nix build .#default
```

`buildNpmPackage` reads the root `package-lock.json`; node2nix is no longer used. A Nix hash mismatch reports the expected `npmDepsHash`/`vendorHash`, which is then written into `flake.nix` and the build is repeated. There is no separate generated dependency snapshot to keep in sync.

The flake pins `nixpkgs` to `nixos-26.05`. That stable branch is supported until 2026-12-31; the next refresh targets `nixos-26.11` within two weeks of its release (owner: repository maintainer).

## Major Upgrade Waves

Major upgrades must not be merged as one large maintenance batch. Paisa keeps them in separate waves so regressions are attributable and rollback boundaries stay narrow.

### Wave 1: Svelte / SvelteKit / Vite

Scope:

- `svelte`
- `@sveltejs/kit`
- `@sveltejs/vite-plugin-svelte`
- `vite`
- closely coupled checking/build tooling such as `svelte-check`

Why it is isolated:

- This wave can change compiler behavior, routing/build output, warnings, and plugin compatibility.
- Failures often look like application regressions even when the source logic is unchanged.

Minimum validation:

- full build and type-check
- authenticated app smoke test
- page load and route navigation smoke test

### Wave 2: Tailwind / DaisyUI / Bulma

Scope:

- `tailwindcss`
- `daisyui`
- `bulma`
- `@cityssm/bulma-sticky-table`
- any style helpers or adapters tightly coupled to those libraries

Why it is isolated:

- This wave can create broad visual regressions without changing runtime logic.
- Paisa currently mixes Tailwind- and Bulma-based styling, so CSS changes need focused review.

Minimum validation:

- visual smoke test of the main application shell
- dashboard, ledger, and import page spot checks
- regression review for layout, spacing, theming, and sticky-table behavior

### Wave 3: Special-Case Integration Libraries

Scope:

- `pdfjs-dist`
- other integration-heavy packages that change worker loading, parsing behavior, or browser/platform compatibility

Why it is isolated:

- These packages often need code changes in addition to a version bump.
- `pdfjs-dist` in particular can change worker paths and API contracts.

Minimum validation:

- import flow smoke tests for supported PDF sources
- worker loading verification in the browser build
- targeted regression tests for affected parsing helpers

## Stale Direct Dependencies

Paisa removed these direct dependencies because they are not part of the active repository contract:

- `@sveltejs/adapter-auto`: unused because the repository config uses `@sveltejs/adapter-static`
- `svelte-language-server`: editor tooling that should come from the editor environment, not the application dependency graph
- `esbuild`: provided transitively by Vite in this repository and not imported directly by project code

If one of them becomes necessary again, reintroduce it with a documented reason.

## Parser Tooling

### Beancount (deferred)

The 2026-10-28 spike installed `beancount==3.2.3` (current PyPI release) in an isolated venv:

- `bean-check` works and reports the fixture cleanly.
- `bean-query` is **no longer shipped** by beancount; it moved to the separate `beanquery` package (0.2.0).
- `bean-report` was **removed** in beancount 3.

Paisa depends on both removed entry points: `Beancount.Parse` calls `bean-query`, while `Beancount.ValidateFile` and `Beancount.Prices` call `bean-report` (`bal` and `pricesdb`). Even with `beanquery` installed, `bean-report` has no v3 replacement, so `pricesdb` and the balance text would need new query-based implementations.

The parser output also changes. On `tests/fixture/inr-beancount/main.beancount`, `bean-query "SELECT account, sum(position) GROUP BY account"` returns `22000.0` / `-109.76562772 USD` under 2.3.6 but `22000` / `-110 USD` under 3.2.3 — a valuation/formatting difference that needs a domain review, not a mechanical port. `bean-report ... pricesdb` output is unavailable in v3, so the existing prices comparison cannot be reproduced.

Decision: this branch stays on **beancount 2.3.6** (CI and both Docker variants). A follow-up spike must replace `bean-report` with bean-query queries, add `beanquery` to CI and the images, and review the valuation delta before switching.

## Font Generator

`tools/fontgen/package-lock.json` is intentionally separate from the root lockfile: `tools/fontgen` is an independent package directory whose dependencies resolve on their own. It defines **no** separate Node runtime; the generator uses the repository baseline Node 24.22.1, which satisfies `svgicons2svgfont`'s `>=24.14.0` engine requirement. A dedicated runtime would only be acceptable if it were declared here and used by CI.

`tools/fontgen/generate-font.js` passes `ts: 0` to `svg2ttf`. Without it, svg2ttf stamps the TTF `head` table with the current time and every regenerated `woff2` differs; with the fixed timestamp two consecutive runs produce identical font hashes.

## CI Runners

`ledger-parsers.yml` runs on `ubuntu-24.04`. The release workflows (`linux-app`, `linux-cli`) stay on `ubuntu-22.04` because Wails needs `libwebkit2gtk-4.0-dev`; moving those runners is a separate migration.

## Go Module Pins

As of 2026-10-28 all **direct** Go modules are at their upgraded versions; `go list -m -u` reports no direct module with a newer release. The security floors are `golang.org/x/net >= v0.62.0`, `golang.org/x/crypto >= v0.58.0`, `github.com/gin-gonic/gin >= v1.12.0`, `gorm.io/gorm >= v1.31.2` and `github.com/wailsapp/wails/v2 >= v2.17.0`.

Indirect modules are **not** mass-updated (`go get -u ./...` is an approved-exception path, not the default). At the end of the Go refresh, 73 indirect modules still had newer releases. Representative examples:

- `github.com/go-redis/redis/v8` v8.4.2 → v8.11.5
- `github.com/go-git/go-git/v5` v5.19.2 → v5.19.3
- `github.com/alecthomas/chroma/v2` v2.14.0 → v2.27.0
- `github.com/charmbracelet/glamour` v0.8.0 → v1.0.0 (major)
- `github.com/charmbracelet/lipgloss` v0.12.1 → v1.1.0 (major)
- `github.com/jaypipes/ghw` v0.21.3 → v0.26.0
- `github.com/itchyny/gojq` v0.12.13 → v0.12.19

The full local audit list is kept at `.cache/dependency-refresh/go-outdated-phase2.txt` (not committed). These updates stay deferred until a concrete security or bugfix need justifies the wider module-graph change.

## Container Base Images

`Dockerfile` and `Dockerfile.demo` build on Alpine 3.24, install npm 12.2.1 globally and then install JavaScript dependencies with `npm ci` from the committed `package-lock.json`.

| Base image                  | Digest (documented, not pinned)                                           |
| --------------------------- | ------------------------------------------------------------------------- |
| `node:24.22.1-alpine3.24`   | `sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53` |
| `golang:1.26.10-alpine3.24` | `sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308` |
| `alpine:3.24.3`             | `sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a` |

The Dockerfiles keep the tags. Digests are recorded for auditability only, so builds are maintainable but not bit-for-bit reproducible; a tag can point to a different image later. Pin the digests in a separate change if strict reproducibility is required.

Validation of the container images runs in CI or on a host with a Docker daemon; the local baseline host had no running daemon.
