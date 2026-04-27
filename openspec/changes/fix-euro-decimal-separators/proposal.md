## Why

Paisa formats the UI with the configured locale, but its parsing paths are not consistently locale-aware. This breaks comma-decimal quantities such as `2,707819`, which can be interpreted as `2707819` and inflate portfolio values by several orders of magnitude.

## What Changes

- Normalize parsed numeric tokens so comma-decimal values are interpreted as decimals instead of having commas stripped blindly.
- Apply the normalization in the ledger parsing path that reads quantities and prices from CLI output.
- Apply the same normalization in import/template helpers so generated ledger entries handle European decimals consistently.
- Add regression coverage for high-precision comma-decimal quantities and for existing thousand-separated inputs.

## Capabilities

### New Capabilities
- `locale-aware-amount-parsing`: Parse localized numeric tokens with comma decimals and high precision without corrupting quantities or prices.

### Modified Capabilities

## Impact

- Affected code: `internal/ledger/ledger.go`, `internal/ledger/ledger_test.go`, `src/lib/template_helpers.ts`, `src/lib/import.test.ts`
- Affected behavior: localized quantity and amount parsing in ledger ingestion and import template helpers
- No new external dependencies or API changes
