## 1. Provider Contract And Source Selection

- [x] 1.1 Finalize the EU metal data source and document its currency, unit, historical coverage, and licensing assumptions in the change artifacts.
- [x] 1.2 Define the new EU provider code, label, description, and supported metal codes for the first iteration.
- [x] 1.3 Confirm whether the first iteration supports only `gold-999` and `silver-999` or also derived lower-purity gold codes.

## 2. Provider Implementation

- [x] 2.1 Add a new EU metal provider under `internal/scraper/metal` or a sibling scraper package with daily price fetching logic.
- [x] 2.2 Normalize provider output to `EUR per gram` before persisting `price.Price` rows.
- [x] 2.3 Register the new provider in `internal/scraper/scraper.go` and expose its metadata through the existing provider APIs.
- [x] 2.4 Keep the existing India provider separate and update its metadata so its India-specific behavior is explicit.

## 3. Configuration And Documentation

- [x] 3.1 Extend config schema/provider options so the new EU provider is selectable in commodity configuration.
- [x] 3.2 Update commodity documentation to distinguish the India provider from the new EU provider.
- [x] 3.3 Document the normalized valuation contract for metal prices as `currency per gram`.

## 4. Verification

- [x] 4.1 Add tests for gold normalization to stored `EUR per gram` values.
- [x] 4.2 Add tests for silver normalization to stored `EUR per gram` values, covering the unit mismatch that currently produces wrong values.
- [x] 4.3 Add or update an integration-style test to verify that downstream valuation still works via `quantity * unitPrice` without metal-specific logic.
