## Context

The reported failure occurs on the ledger ingestion path when a posting quantity uses a comma as the decimal separator and has higher precision, for example `2,707819 "EWG2LD"`. The current implementation removes commas before decimal parsing, which is correct for `100,000` but incorrect for comma-decimal numbers. A similar stripping approach exists in the template helper layer used by imports.

## Goals / Non-Goals

**Goals:**
- Parse comma-decimal quantities and prices correctly, including high-precision values.
- Preserve existing support for common thousand-separated inputs such as `100,000` and `1,234.56`.
- Keep the change small and local to existing parsing helpers.

**Non-Goals:**
- Introduce full locale configuration into the parser pipeline.
- Change UI formatting behavior.
- Redesign ledger CLI output formats.

## Decisions

- Introduce a token normalization step before decimal parsing in both Go and TypeScript.
  Rationale: the bug is caused by string preprocessing, not by arithmetic or storage.
- Use a separator heuristic instead of wiring parser behavior to the UI locale.
  Rationale: journal and CLI outputs are driven by commodity formatting and may differ from the configured UI locale.
- Prefer the last separator when both `.` and `,` are present, and preserve existing plain thousand-separated inputs when only commas are present.
  Rationale: this correctly handles values such as `10.005,05` and `1,234.56` while fixing the reported high-precision case.

## Risks / Trade-offs

- [Ambiguous single-separator numbers] -> Keep conservative handling for comma-only `xxx,yyy` inputs so existing thousand-separated values do not regress.
- [Logic duplication across Go and TypeScript] -> Keep both implementations small and cover them with regression tests.
- [Locale edge cases outside the reported scope] -> Limit the change to well-defined numeric token normalization rather than broad locale plumbing.

## Migration Plan

No data migration is required. The fix changes runtime parsing only and is safe to deploy as a patch release.

## Open Questions

- None at the moment; the issue report provides a concrete reproduction and expected result.
