<!--
Update this file before creating a release tag. The release workflows use it as
the GitHub release body. Keep the heading and bullets in sync with the matching
CHANGELOG section.
-->

### 0.8.6.6 (2026-11-23)

Automatic background price updates and the follow-up hardening.

- Run price updates automatically Monday to Friday at 22:30 in the configured
  time zone, with catch-up on server start and when the UI opens. The schedule
  is configurable through the new `price_update` block; readonly deployments
  and installations without provider-backed commodities are unaffected.
- Keep the UI responsive during updates: the server runs one background job at
  a time, exposes its status and job ID, and the web UI only shows a short
  toast while it watches the job. "Update Prices" now performs a full refresh
  in the background.
- Fetch only a seven-day overlap for incremental runs, validate every provider
  response before writing, merge by date so partial responses never delete
  history, batch large inserts, and load up to four commodities in parallel.
- Support time-windowed fetches for Yahoo, currency, EU metal and Alpha Vantage
  providers; other providers keep their full-history fallback.
- Persist the last attempt, success, partial success and failure in a
  versioned state file, let `paisa update` share that state, and report price
  failures with a non-zero exit code.
- Keep the configuration editor usable when `price_update` is unset and make
  invalid schedule values fall back with a single warning instead of blocking
  startup.
- Embed IANA timezone data so named time zones load even when the operating
  system or container has no timezone database.
- Parse quoted commodity names with spaces and punctuation in Ledger and
  hledger price output, so funds such as
  `"AXIS SMALL CAP FUND - DIRECT PLAN"` keep their names during price sync.
- Reject Yahoo price responses whose timestamp and close arrays differ in
  length instead of panicking; cached prices and the remaining commodities
  continue to sync.
- Generate imported ledger entries with `YYYY-MM-DD` dates and quoted
  payees and consolidated descriptions, so spaces and punctuation cannot
  corrupt the journal.
- Support Beancount 3 through `bean-query`: read source locations from
  `ANY_META('filename')`/`ANY_META('lineno')`, skip generated padding
  postings, and import prices from the `#prices` table. Beancount 2 keeps
  using the `prices` table.
