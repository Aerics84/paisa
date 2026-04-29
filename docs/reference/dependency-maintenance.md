# JavaScript Toolchain Maintenance

This page defines Paisa's JavaScript maintenance contract so routine dependency updates stay boring and framework migrations stay isolated.

## Runtime Baseline

- Primary JavaScript runtime: Node 24
- Primary package manager: npm 11
- Local version source of truth: [`.nvmrc`](../../.nvmrc)
- Repository metadata source of truth: [`package.json`](../../package.json)
- Secondary runtime: Bun 1.3.13, used only for the targeted import and regression test commands already defined in `package.json`
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
