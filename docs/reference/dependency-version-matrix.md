# Dependency Version Matrix — SvelteKit 3 Wave

Recorded on 2026-10-28 from the installed packages and the npm registry. This is a maintenance record, not a copy of the upgrade plan.

## Verified peer requirements

| Package                        | Version | Peer requirements (`npm view`)                                                                                                |
| ------------------------------ | ------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `@sveltejs/kit`                | 3.1.0   | vite `^8.0.12`, svelte `^5.57.1`, typescript `^6.0.0`, `@opentelemetry/api` `^1.0.0`, `@sveltejs/vite-plugin-svelte` `^7.0.0` |
| `@sveltejs/adapter-static`     | 4.0.0   | `@sveltejs/kit` `^3.0.0-next.0`                                                                                               |
| `@sveltejs/vite-plugin-svelte` | 7.3.1   | vite `^8.0.0-beta.7 \|\| ^8.0.0`, svelte `^5.46.4`                                                                            |
| `svelte`                       | 5.57.3  | none                                                                                                                          |
| `vite`                         | 8.3.5   | optional: tsx, jiti, less, sass, yaml, stylus, terser, esbuild, sugarss, `@types/node`, sass-embedded, `@vitejs/devtools`     |
| `svelte-check`                 | 4.7.7   | svelte `^4.0.0 \|\| ^5.0.0-next.0`, typescript `^5.0.0 \|\| ^6.0.0`                                                           |

## Installed resolution

```text
@sveltejs/adapter-static@4.0.0
  @sveltejs/kit@3.1.0 deduped
@sveltejs/kit@3.1.0
  @sveltejs/vite-plugin-svelte@7.3.1 deduped
  svelte@5.57.3 deduped
  typescript@6.0.3 deduped
  vite@8.3.5 deduped
@sveltejs/vite-plugin-svelte@7.3.1
  svelte@5.57.3 deduped
  vite@8.3.5 deduped
svelte@5.57.3
svelte-check@4.7.7
  svelte@5.57.3 deduped
  typescript@6.0.3 deduped
typescript@6.0.3
vite@8.3.5
```

`npm ls` reports no invalid peers after the upgrade.

## Kit 3 migration notes

- Configuration moved from `svelte.config.js` into the `sveltekit(...)` Vite plugin (`vite.config.js`); the `kit` namespace is gone and the adapter is passed directly.
- `$lib` is kept through the documented `alias: { "$lib": "src/lib" }` compatibility option; SvelteKit marks this alias deprecated in favor of subpath imports, so a future migration can move imports to `#lib`.
- `$app/stores` was removed; `page` now comes from `$app/state` and is used without the `$` store prefix.
- `goto(url, { noScroll: true })` became `goto(url, { reset: false })`.
- `handleError` moved to `@sveltejs/kit/hooks` and now receives `{ kind, error, event }`.
- `tsconfig.json` extends `$app/tsconfig` and includes `$app/types` in `types`.
- The `paths` warning from SvelteKit 3.1.0 is emitted while the resolved `$lib` paths still work; `npm run check` is clean.
