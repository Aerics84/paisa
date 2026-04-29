# Paisa

[![Matrix](https://img.shields.io/matrix/paisa%3Amatrix.org?logo=matrix)](https://matrix.to/#/#paisa:matrix.org)

**Paisa** is a Personal finance manager. It builds on
top of the [ledger](https://www.ledger-cli.org/) double entry accounting tool. Checkout
[documentation](https://paisa.fyi) to get started.

## Docker Images

Ready to use Docker images are published at `ghcr.io/aerics84/paisa`.

- `ghcr.io/aerics84/paisa:latest` includes Paisa with `ledger`
- `ghcr.io/aerics84/paisa:latest-hledger` adds `hledger`
- `ghcr.io/aerics84/paisa:latest-beancount` adds `beancount`
- `ghcr.io/aerics84/paisa:latest-all` adds both `hledger` and `beancount`

Example:

```bash
docker run --rm -p 7500:7500 ghcr.io/aerics84/paisa:latest
```

For the demo image:

```bash
docker run --rm -p 7500:7500 ghcr.io/aerics84/paisa:latest-demo
```

## Development Workflow

`npm install` configures repo-local Git hooks automatically.

- `git commit` formats staged Prettier-supported files and re-stages them.
- `git push` runs the critical pre-push checks from `scripts/preflight-critical.mjs`.
- `npm run format:changed` formats locally changed files on demand.

The pre-push check validates changed-file formatting, the Svelte/TypeScript check, targeted import and regression tests, the frontend production build, and the Go build.

If a staged file also has unstaged edits, the commit hook stops instead of silently staging extra hunks. In that case, format the file first and stage it again.

# Demo

A demo of the Web UI can be found at [https://demo.paisa.fyi](https://demo.paisa.fyi)

## Status

I use it to track my personal finance. Most of my personal use cases
are covered. Feel free to open an issue if you found a bug or start a
discussion if you have a feature request. If you have any question,
you can ask on [Matrix chat](https://matrix.to/#/#paisa:matrix.org).

## License

This software is licensed under [the AGPL 3 or later license](./COPYING).
