# JavaScript Toolchain Maintenance

This page defines Paisa's JavaScript maintenance contract so routine dependency updates stay boring and framework migrations stay isolated.

## Runtime Baseline

- Primary JavaScript runtime: Node 24.22.1
- Primary package manager: npm 11.19
- Local version source of truth: [`.nvmrc`](../../.nvmrc)
- Repository metadata source of truth: [`package.json`](../../package.json)
- Secondary runtime: Bun 1.4.4, used only for the targeted import and regression test commands already defined in `package.json`
- Secondary runtime version source of truth: [`.bun-version`](../../.bun-version)

GitHub Actions, local development, and Nix-based workflows should converge on this baseline unless a workflow documents a deliberate exception.

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

If the Nix dependency snapshot is expected to stay in sync with `package-lock.json`, regenerate it after the lockfile changes:

```bash
npx node2nix --development --input package.json --lock package-lock.json --node-env ./flake/node-env.nix --composition ./flake/default.nix --output ./flake/node-package.nix
```

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

`Dockerfile` and `Dockerfile.demo` build on Alpine 3.24 and install JavaScript dependencies with `npm ci` from the committed `package-lock.json`.

| Base image                  | Digest (documented, not pinned)                                           |
| --------------------------- | ------------------------------------------------------------------------- |
| `node:24.22.1-alpine3.24`   | `sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53` |
| `golang:1.26.10-alpine3.24` | `sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308` |
| `alpine:3.24.3`             | `sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a` |

The Dockerfiles keep the tags. Digests are recorded for auditability only, so builds are maintainable but not bit-for-bit reproducible; a tag can point to a different image later. Pin the digests in a separate change if strict reproducibility is required.

Validation of the container images runs in CI or on a host with a Docker daemon; the local baseline host had no running daemon.
