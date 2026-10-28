# CHANGELOG

Releases from 0.8 onwards are fork releases. The 0.7.x sections below
document the upstream releases this fork builds on; the short-lived
0.7.5–0.7.6.1 fork bumps are folded into 0.8.

### Unreleased

- Refresh the toolchain baseline: Node 24.22.1, npm 11.19.1, Bun 1.4.4,
  Go 1.26.10, Alpine 3.24.
- Upgrade the frontend stack: Svelte 5.57, SvelteKit 3.1, Vite 8.3,
  TypeScript 6.0, ESLint 10, Prettier 3.9, Font Awesome 7, Tabulator 6.7,
  and the numeric/color/format libraries.
- Upgrade the Go stack (Gin 1.12, GORM 1.31, Wails 2.17, SQLite) and
  close the `golang.org/x/net` and `x/crypto` security findings.
- Upgrade hledger to 1.52.4 and add a chained-price regression test;
  keep beancount 2.3.6 because v3 removes `bean-report` and moves
  `bean-query` to the separate `beanquery` package.
- Move the Nix web build to `buildNpmPackage`, update the flake to
  `nixos-26.05`, and remove node2nix.
- Add a machine-readable dependency audit gate (npm + govulncheck) to CI
  and weekly Dependabot updates; the gate blocks confirmed critical/high
  findings. npm 12 is deferred to a default-branch follow-up PR.
- Freeze the refreshed dependency set and record the verified commit in
  `docs/reference/dependency-freeze.md`.

### 0.8.5.2 (2026-10-28)

- Move the tax pages from `/more/tax` to `/tax`, so they are reachable
  directly from the top-level navigation.
- Remove the help question marks from the breadcrumb.
- Correct the fork release history to `0.8.1`, `0.8.2`, `0.8.4` and
  `0.8.5.1`.
- Hide the India-only portfolio analysis page and the mutual-fund
  portfolio sync action outside the `india` regional profile.

### 0.8.5.1 (2026-10-27)

- Fix accessibility warnings in the navbar and on the demo button.
- Handle relative `source` names reported by hledger without aborting the
  parse.
- Rename accounts in CRLF ledgers from the editor.
- Polish the Germany tax overview: show amounts in diagnostics, restore
  the year label after a failed load, and ignore stale responses.
- Collapse all tax overview sections on open, hide categories without
  postings, align table columns across categories, and fix the source
  label in the dark theme.
- List the valid `tax_category` keys in the legacy-tag and
  unknown-category diagnostics.

### 0.8.5 (2026-10-27)

- Add a Germany [tax overview](https://github.com/Aerics84/paisa/blob/master/docs/reference/tax/deductions.md)
  page that groups deductible-expense candidates by tax return section,
  reports coverage gaps, and shows raw Anlage KAP values.
- Add a `tax_category` posting tag for ledger, hledger and beancount,
  plus configurable account rules for tax deductions.
- Add CSV export of the tax overview and surface sales without matching
  purchase lots.
- Fix saving import templates terminating the server when
  `salary_income_accounts` is unset; save errors are now returned to the
  UI instead of killing the process.
- Make regression snapshots deterministic with stable query ordering and
  timezone-aware month boundaries.
- Match transfer account globs against subaccounts, and keep WAL and
  `busy_timeout` settings when the connection pool grows.
- Use a theme-aware warning color for the savings rate.
- Polish the preview dashboard layout, currency formatting and month
  selector styling.

### 0.8.4 (2026-10-25)

- Add total and invested savings rates to the yearly investment cards,
  with a 20 % guideline highlight and documented formulas.
- Exclude internal asset transfers from the savings rate, and interest
  from the retirement goal's investment basis.
- Classify payroll income and allow configuring the salary income
  accounts used by the savings rate and monthly review.
- Add a preview dashboard with a selected-month financial review: net
  income trend, top/flop positions, checking account history and Sankey
  flows.
- Index postings and prices, and compute cash flow from a single posting
  query.
- Remove untrusted HTML rendering and rate-limit requests per client IP.
- Enable SQLite WAL, prune old editor backups and make the price cache
  concurrency-safe.
- Authenticate Yahoo requests with a cookie and crumb, and normalize
  GBp/GBX prices to GBP.
- Keep journal dates in the configured timezone across the API, CSV
  export and running balance.
- Read hledger `acost` for cost basis, and fix mixed positive and
  negative account globs.
- Handle empty import template lists and preserve decimal precision when
  importing.
- Fix Svelte 5 runtime errors, restore the classic dashboard tile layout,
  and repair dark-theme card backgrounds and chart sizing.
- Merge upstream v0.7.8, including the path traversal fix.
- Bump Go to 1.26, build separate Intel and Apple Silicon macOS releases,
  and restore the CI, Docker and docs workflows.
- Upgrade the bundled ledger to 3.4.1.

### 0.8.2 (2026-04-29)

- Modernize the frontend stack: Svelte 5, Bulma 1, Vite 8 and Bun 1.3,
  with an npm 11 lockfile.
- Rework the dashboard overview layout on Bulma 1.
- Add repo-local formatting hooks and check gofmt only on changed files.
- Normalize imported broker quantities.
- Fix import formatting on Windows.
- Fix Docker npm preparation and clean npm installs in CI.
- Harden regression tests when the ledger binary is missing.

### 0.8.1 (2026-04-28)

- Add built-in import templates for Trade Republic and Scalable Capital
  trade and cash CSV exports.
- Add CSV export for the Germany capital income report.
- Extend the Germany capital income report with partial exemption,
  withholding tax credits and review diagnostics.
- Fix CAMT parsing and Europe import template rendering.
- Add a pre-push test preflight.

### 0.8 (2026-04-28)

- Add the Germany tax regime with a capital income report: annual
  allowance, capital income tax, solidarity surcharge and optional church
  tax.
- Add Germany/EU starter configuration and sample journals to
  `paisa init`.
- Add import templates for CAMT.053 and MT940 bank statements, plus
  generic Germany/EU broker trade and cash exports.
- Add a Yahoo-based European metal price provider
  (`com-yahoo-eu-metal`) with EUR per gram prices.
- Add expense subcategory drilldown and separate monthly expense scopes.
- Improve retirement goal forecasting.
- Centralize version management in `VERSION`, including the About page
  and the version reported by Docker images.
- Support Indian and decimal-comma digit grouping in amount parsing.
- Trim Yahoo stock history to the available FX range.
- Publish Docker images at `ghcr.io/aerics84/paisa`.
- Build with Node 24 in CI and Nix.

### 0.7.8 (2026-10-21)

- Fix path traversal allowing file reads and backup deletion outside the ledger and sheet directories ([#420](https://github.com/ananthakumaran/paisa/issues/420)).
- Treat filenames literally when finding and deleting backups.

### 0.7.7 (2026-10-18)

- Allow configuring the server bind host.
- Ignore null Yahoo prices instead of treating them as zero.
- Show relative file paths and line ranges in Ledger sync errors.
- Add separate Intel and Apple Silicon macOS builds.

### 0.7.6 (2026-09-06)

- Include transaction id in the export
- Update Node.js and Go versions and remove Bun.
- Fix the ledger formula and update fonts.

### 0.7.5 (2025-12-02)

- Build fixes

### 0.7.4 (2025-02-23)

- Update price data domain
- Fix NixOS build

### 0.7.3 (2025-02-23)

- Fix yahoo price fetcher
- Build fixes

### 0.7.1 (2024-10-20)

- Fix remote code execution [vulnerability](https://github.com/ananthakumaran/paisa/issues/294)

### 0.7.0 (2024-08-26)

- Add [docker image variant](https://github.com/ananthakumaran/paisa/pull/274) for hledger and beancount
- Bug fixes

### 0.6.6 (2024-02-10)

- Improve tables (make it sortable)
- Show tabulated value on allocation page
- Show invested value on goals page
- Bug fixes

### 0.6.5 (2024-02-02)

- Add Liabilities > [Credit Card](https://paisa.fyi/reference/credit-cards) page
- Support password protected XLSX file
- Allow user to configure timezone
- Bug fixes

### 0.6.4 (2024-01-22)

- Add checking accounts balance to dashboard
- Improve template management UI
- Improve spinner and page transition
- Bug fixes

### 0.6.3 (2024-01-13)

- Introduce [Sheets](https://paisa.fyi/reference/sheets/): A notepad calculator with access to your ledger
- Remove flat option from cashflow > yearly page
- Dockerimage now installs paisa to /usr/bin
- Improve legends rendering on all pages
- Allow user to cancel pdf password prompt
- Add new warning for missing assets accounts from allocation target
- Support hledger's balance assertion
- Bug fixes

### 0.6.2 (2023-12-23)

- New logo
- Allow goals to be reordered
- Show goals on the dashboard page
- Bug fixes

### 0.6.1 (2023-12-16)

- Add new price provider: [Alpha Vantage](https://paisa.fyi/reference/commodities/#alpha-vantage)
- Make first day of the week configurable
- Support ledger strict mode
- Add user login support, go to `User Accounts` section in configuration page to enable it
- Show notes associated with a transaction/posting
- Bug fixes

### 0.6.0 (2023-12-09)

- Add individual account balance on goals page
- Add [keyboard shortcuts](https://paisa.fyi/reference/editor/) to format/save file on editor page
- Add ability to search posting/transaction by note
- Add option to reverse the order of generated transactions on import page
- Add option to clear price cache
- Bug fixes

### 0.5.9 (2023-11-26)

- Improve postings page
- Add income statement page (Cash Flow > Income Statement)
- Bug fixes

### 0.5.8 (2023-11-18)

- Add ability to specify rate, target date or monthly contribution to
  [savings goal](https://paisa.fyi/reference/goals/savings/)
- Improve price page
- Bug fixes

### 0.5.7 (2023-11-11)

- Add [goals](https://paisa.fyi/reference/goals)
- Remove retirement page (available under goals)
- Bug fixes

#### Breaking Changes :rotating_light:

Retirement page has been moved under goals. If you have used
retirement, you need to setup a new [retirement goal](https://paisa.fyi/reference/goals)

### 0.5.6 (2023-11-04)

- Add support for Income:CapitalGains
- Add option to control display precision
- Add new price provider for gold and silver (IBJA India)
- Add option to disable budget rollover
- Bug fixes

### 0.5.5 (2023-10-07)

- Support account icon customization
- Add beancount ledger client support

### 0.5.4 (2023-10-07)

- Add calendar view to recurring page
- Support [recurring period](https://paisa.fyi/reference/recurring/#period) configuration
- Support European number format
- Bug fixes

### 0.5.3 (2023-09-30)

- Add Docker Image
- Add Linux Application (deb package)
- Move import templates to configuration file
- Bug fixes

#### Breaking Changes :rotating_light:

User's custom import templates used to be stored in Database, which is
a bad idea in hindsight. It's being moved to the configuration
file. With this change, all the data in paisa.db would be transient
and can be deleted and re created from the journal and configuration
files without any data loss.

If you have custom template, take a backup before you upgrade and add
it again via new version. If you have already upgraded, you can still
get the data directly from the db file using the following query
`sqlite3 paisa.db "select * from templates";`

### 0.5.2 (2023-09-22)

- Add Desktop app
- Support password protected PDF on import page
- Bug fixes

#### Breaking Changes :rotating_light:

- The structure of price code configuration has been updated to make
  it easier to add more price provider in the future. In addition to
  the code, the provider name also has to be added. Refer the
  [config](https://paisa.fyi/reference/config/) documentation for more details

```diff
     type: mutualfund
-    code: 122639
+    price:
+      provider: in-mfapi
+      code: 122639
     harvest: 365
```

### 0.5.0 (2023-09-16)

- Add config page
- Embed ledger binary inside paisa
- Bug fixes

### 0.4.9 (2023-09-09)

- Add [search query](https://paisa.fyi/reference/bulk-edit/#search) support in transaction page
- Spends at child accounts level would be included in the budget of
  parent account.
- Fix the windows build, which was broken by the recent changes to
  ledger import
- Bug fixes

### 0.4.8 (2023-09-01)

- Add budget
- Add hierarchial cash flow
- Switch from float64 to decimal
- Bug fixes

### 0.4.7 (2023-08-19)

- Add dark mode
- Add bulk transaction editor
