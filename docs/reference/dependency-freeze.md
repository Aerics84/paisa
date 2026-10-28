# Dependency Freeze 2026-10

Frozen and verified on **2026-10-28** on branch `chore/dependency-refresh-2026-10`.

- **Commit:** `e9bcb6098517b27eea86a55fe5b5c35b7066d2fd`
- **Commit subject:** `ci: keep the MSYS2 PATH intact in the Windows build`
- **Freeze timestamp:** `2026-10-28T19:05:43Z`

The detail manifests and reports referenced below are local artifacts under
`.cache/dependency-refresh/` and are intentionally **not** committed. This page
is the durable summary.

## Toolchain

| Tool        | Version          |
| ----------- | ---------------- |
| Node        | 24.22.1          |
| npm         | 11.19.1          |
| Bun         | 1.4.4            |
| Go          | 1.26.10          |
| Git         | 2.54.0.windows.1 |
| Docker      | 29.8.1           |
| govulncheck | v1.9.0           |

Nix uses `nixos-26.05`; its dev shell provides Node 24.21.0, Go 1.26.10 and Bun
1.3.13, so CI Bun steps intentionally use `.bun-version` (1.4.4).

## Lockfile Hashes

From `freeze-e9bcb60…/locks.json` (SHA-256 of the manifest file:
`4e4ccd0dab52dbb135b6030d506518f947bfd9b3a4ef65d1fb790a28d0d3f0ca`):

| File                              | SHA-256                                                            |
| --------------------------------- | ------------------------------------------------------------------ |
| `flake.lock`                      | `0a3980c8227d6b3275898016d413e242b723bab2384c6154ee5672fa6313a419` |
| `go.sum`                          | `e84a62c6013e328cd295dc259aa0a152063b86cbfda8e934177c3ecaf79d3883` |
| `package-lock.json`               | `5404412e0adc2498020cd0bcf783f0f2f6022fecfe526de4bc4d3762177c22e2` |
| `tools/fontgen/package-lock.json` | `a3878214eb0dcb6b2910c3158a3c82babc2b793290f86ddb92d82f176912ffd8` |

## Artifact and Fixture Manifests

The freeze records two generated-artifact sets and the tracked fixtures:

| Manifest                                  | Files | Manifest SHA-256                                                   |
| ----------------------------------------- | ----- | ------------------------------------------------------------------ |
| Artifacts (`web/static`, `fonts`)         | 246   | `d6c3324009a2e0703ecbd9cdd5297fb7293aaeb99a2cf232aa9c68821c326644` |
| Fixtures (`tests/fixture`, SBI statement) | 159   | `df90a3a6686ff55092e23e4ef82b843aec3d210278220fc8676d4336a36c42e3` |

Representative artifact hashes:

| Artifact                       | SHA-256                                                            |
| ------------------------------ | ------------------------------------------------------------------ |
| `web/static/index.html`        | `6e699b94912e2dfa143df8cc60f59272837bb00b5808c2c4e60bb22b97f8ce89` |
| `web/static/_app/version.json` | `20596de86a488f7e824d42ee7d5a77acc2de0f8be53649d833310c47bb10cfb2` |
| `fonts/arcticons.woff2`        | `f2faf5ec467cc93570b2b54fdb7811a6db9ab22582b0b7ae37aba7229015f882` |
| `fonts/fa6-solid.woff2`        | `0d0f673608d37de87717f1306bc12174b78f0d2a5f3b9c881e05a86f4339f601` |

The SvelteKit version was pinned to `VERSION` (`version.name` in
`svelte.config.shared.js`) because SvelteKit otherwise writes a build timestamp
into `_app/version.json`. With the pin, two consecutive builds hash identically.
The font generator passes `ts: 0` to `svg2ttf` for the same reason.

## Security Summary

From `.cache/dependency-refresh/freeze-report/summary.json`:

| Target              | Critical | High | Moderate | Low | Total |
| ------------------- | -------- | ---- | -------- | --- | ----- |
| npm root            | 0        | 0    | 0        | 4   | 4     |
| npm `tools/fontgen` | 0        | 0    | 7        | 0   | 7     |

govulncheck (DB 2026-10-27, Go 1.26.10): **0 called**, 1 not-called finding.
Scanner errors: none. Gate exit code: **0**. Both npm entries are present in the
report.

The 4 low and 7 moderate findings are dev-tooling advisories without a critical
or high impact; they stay visible in the uploaded CI artifact and are reviewed
with the next routine refresh.

## Verification Matrix

| Profile                                                        | Result                                                                                                                                                 |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| P-JS (`check`, `build`, imports, investment, monthly)          | Pass                                                                                                                                                   |
| P-REG (Go build, `TZ=UTC bun test tests/regression.test.ts`)   | Pass                                                                                                                                                   |
| P-GO (`go test ./...`, `go build`)                             | Pass                                                                                                                                                   |
| P-PDF (`src/lib/pdf.test.ts`, 487-row golden)                  | Pass                                                                                                                                                   |
| P-UI (production container, 6 routes, 9 transactions)          | Pass at route level; click, date-picker and virtual-list interactions were not automated (see the ledger rulings).                                     |
| P-SEC (`security-gate.mjs`)                                    | Pass, exit 0                                                                                                                                           |
| P-CONTAINER (`Dockerfile`, `Dockerfile.demo`, `paisa version`) | Pass                                                                                                                                                   |
| P-DESKTOP (`wails build`)                                      | Pass, `Paisa.exe` built                                                                                                                                |
| P-NIX (offline builds, dev shell, mkdocs, clean source)        | Pass                                                                                                                                                   |
| P-RELEASE (`ci/*` draft-release dry run)                       | Linux App/CLI and macOS AMD64/ARM64 pass; Windows cancelled after the hosted runner stalled in `setup-node` (infrastructure). See below.               |
| P-LINT (`npm run lint`)                                        | Known Windows-only failure: Prettier CRLF diff in 231 untouched files; `eslint .` reports 0 errors (11 warnings). CI validates changed files on Linux. |

P-NIX was verified on the frozen source without a local `web/static`, with
`nix build .#paisa-web --offline`, `nix build .#default --offline`, the dev
shell (`node -v`, `go version`, `mkdocs build`) and a served
`./result/bin/paisa serve` returning HTML.

## Deliberate Pins and Follow-ups

- **beancount 2.3.6:** v3 removes `bean-report` and moves `bean-query` to
  `beanquery`; a migration needs new query-based implementations. See
  [Dependency Maintenance](dependency-maintenance.md#parser-tooling).
- **npm 12.2.1:** deferred to a default-branch follow-up because
  `workflow_dispatch` must be registered there. See
  [Dependency Maintenance](dependency-maintenance.md#npm-12-deferred).
- **TypeScript 7.0.2:** intentionally ignored for major updates until the
  surrounding toolchain accepts it.
- **73 indirect Go modules:** have newer releases; deferred until a concrete
  need. Direct modules are current.
- **NixOS 26.11:** next refresh within two weeks of release.
- **hledger 1.52.4:** pinned through a Hackage source override because nixpkgs
  (26.05 and master) still packages 1.52; issue #2254 is fixed since 1.41.
- **Windows release dry run:** re-run the `ci/windows-*` workflow after the
  hosted-runner stall; the workflow fixes are committed but no green Windows
  run exists yet.
- **PDF worker-path coverage:** the fixture golden verifies extraction; a
  browser-level upload test that exercises the production worker URL remains a
  follow-up.

## Docker Base Images

Digests recorded at freeze time (tags stay in the Dockerfiles):

| Image                       | Digest                                                                    |
| --------------------------- | ------------------------------------------------------------------------- |
| `node:24.22.1-alpine3.24`   | `sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53` |
| `golang:1.26.10-alpine3.24` | `sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308` |
| `alpine:3.24.3`             | `sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a` |

## Post-Freeze Review Fixes

A whole-branch review after the freeze produced one Critical and four Important
findings. Fix commit `bd62c3ab9a31b55fabc185d143b905d0eff95caa`:

- Treats npm audit exit codes other than 0/1, and any nonzero govulncheck exit
  in `-json` mode, as scanner errors (exit 2); govulncheck error records are
  rejected too. `scripts/security-gate.test.mjs` covers each failure path.
- Runs the CI regression test with the `.bun-version` Bun (1.4.4) inside the
  Nix shell instead of the Nix-packaged 1.3.13.
- Narrows the P-UI claim to route-level verification and corrects the phase
  status wording.

The dependency state is unchanged: the lockfile, artifact and fixture
manifests still match the freeze hashes above. The PDF worker-path test and the
Windows release dry run remain recorded follow-ups.

## Release Dry Run

The `ci/*` draft-release dry run was executed at commit `2052410` (the only
difference from the freeze commit is `.github/workflows/windows.yml`):

- **Linux App Release:** success (static ledger 3.4.1, Wails build, `.deb`).
- **Linux CLI Release:** success.
- **Mac OS AMD64 Release:** success (ledger 3.4.1 from the corrected Homebrew formula).
- **Mac OS ARM64 Release:** success.
- **Windows Release:** not green in this session. Three pre-existing workflow
  defects were fixed and committed (MSYS2 package renames, the setup-msys2
  installation path, and PowerShell `$PATH` expansion in the bash calls). The
  final run reached the MSYS2 setup step but the hosted Windows runner then
  stalled in `setup-node` for over an hour; the run was cancelled. Follow-up:
  re-run the `ci/windows-*` dry run when the runner is healthy.

The release action created one draft release with an empty tag; it was deleted
after the run. **No tags were pushed.** Logs are in
`.cache/dependency-refresh/matrix/p-release-*.log` (local).
