## Why

Paisa's current metal integration is tied to an India-specific source and does not reliably produce usable prices for European setups. In practice this leads to two failures: the provider returns India-market prices instead of EUR-denominated prices, and the current unit normalization is not robust enough for metals with different source units, which makes values such as silver materially wrong.

## What Changes

- Add a dedicated EU metal price provider that returns daily metal prices normalized to `EUR per gram`.
- Keep the existing India metal provider available, but clarify its India-only behavior and prevent it from being the implicit answer for European users.
- Support gold and silver pricing in a way that preserves Paisa's existing `quantity * unitPrice` valuation model without additional downstream unit handling.
- Document the provider behavior, supported metal codes, and currency/unit expectations for European configurations.
- Add tests that cover gold and silver normalization and verify that EUR-based metal prices flow correctly through market valuation.

## Capabilities

### New Capabilities
- `metal-pricing`: Fetch and normalize supported metal prices with explicit regional provider behavior and per-gram valuation semantics.

### Modified Capabilities
- None.

## Impact

- Affected code: `internal/scraper/metal/*`, `internal/scraper/scraper.go`, config schema/provider metadata, tests, and commodity documentation.
- Affected behavior: European metal commodities will use EUR-denominated per-gram prices instead of India-market values.
- External dependencies: Yahoo Finance metal futures for USD per troy ounce history and ECB reference exchange rates for USD to EUR normalization.
