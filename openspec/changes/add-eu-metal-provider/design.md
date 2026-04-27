## Context

Paisa stores commodity prices as a single numeric unit price and later derives valuations by multiplying `quantity * unitPrice`. This contract is simple and already used across market valuation, allocation, diagnosis, and reporting flows. The current metal provider violates the assumptions needed for European use because it is tied to an India-market source, its currency semantics are implicit, and its unit normalization is not explicit enough for metals that may arrive in different source units.

The existing stock providers already demonstrate an important pattern: provider-specific normalization happens at ingestion time, not during later valuation. This change should follow the same approach so downstream consumers remain unchanged.

## Goals / Non-Goals

**Goals:**
- Introduce a metal pricing contract that is explicit about source region, source currency, and normalized storage unit.
- Add a dedicated EU-facing metal provider that persists supported metal prices as `EUR per gram`.
- Preserve the existing downstream valuation model so market/reporting code does not need metal-specific logic.
- Make metal provider behavior testable for both gold and silver normalization.

**Non-Goals:**
- Generalize Paisa into a full multi-currency commodity pricing engine.
- Add support for every metal, purity, or regional benchmark in the first iteration.
- Redesign the `price.Price` persistence model.
- Change the journal-side quantity semantics for existing metal holdings.

## Decisions

### 1. Normalize metal prices at provider ingestion time

The new provider will convert source data into `EUR per gram` before creating `price.Price` rows.

Rationale:
- This matches the existing ingestion pattern used by Yahoo and Alpha Vantage, where provider-specific conversion happens before persistence.
- It preserves current consumers such as `GetUnitPrice`, `GetPrice`, and `GetMarketPrice`.
- It avoids spreading unit/currency branching across reporting and accounting code.

Alternatives considered:
- Add unit metadata to `price.Price` and convert later. Rejected because it is a larger cross-cutting change with no immediate need.
- Add metal-specific logic inside market valuation. Rejected because it would make a single provider quirk leak into general accounting code.

### 2. Keep the India provider and add a separate EU provider

The existing `com-purifiedbytes-metal` provider should remain available for India-oriented users, while a new EU provider is added for European setups.

Rationale:
- It preserves backward compatibility for users who intentionally rely on India prices.
- It avoids changing current behavior for existing Indian ledgers.
- It makes region-specific behavior explicit in configuration and documentation.

Alternatives considered:
- Replace the existing provider in place. Rejected because it would silently change semantics for existing users.
- Add automatic region switching behind the same provider code. Rejected because implicit provider behavior is harder to reason about and test.

### 3. Limit the first EU provider to supported codes with deterministic per-gram semantics

The first implementation supports only `gold-999` and `silver-999`. European prices are sourced from Yahoo Finance metal futures in USD per troy ounce and converted to EUR per gram using ECB reference exchange rates.

Rationale:
- The observed bug is severe for silver because unit assumptions are wrong today.
- Starting with a small, verified capability reduces the risk of publishing plausible but incorrect prices.
- It creates a clean path for later purity expansion once source and conversion rules are explicit.

Alternatives considered:
- Support all current purity codes on day one via linear purity scaling. Rejected for the first iteration because it risks encoding domain assumptions before the base source is trusted.
- Require a dedicated commercial metals API key. Rejected for the first iteration because it adds onboarding friction when the existing Yahoo + ECB combination is sufficient.

### 4. Treat provider metadata and documentation as part of the contract

Provider labels, descriptions, auto-complete options, and commodity docs must state region, supported metals, default currency behavior, and normalized unit.

Rationale:
- The current ambiguity is partly caused by UI/docs implying a generic metal provider when the implementation is India-specific.
- Clear metadata reduces misconfiguration risk for EU users.

## Risks / Trade-offs

- [EU source lacks free historical access or stable terms] -> Mitigation: choose a provider with documented historical endpoints before implementation; if needed, gate with API key configuration.
- [Gold and silver use different source units] -> Mitigation: encode unit normalization explicitly per source/provider and add tests with fixture data for both metals.
- [Provider returns EUR spot values but not per-gram values] -> Mitigation: convert from the documented source unit, typically troy ounce, using a fixed tested conversion constant.
- [Backward compatibility confusion between India and EU providers] -> Mitigation: keep separate provider codes and update descriptions/docs to make region intent explicit.
- [Additional purities are demanded immediately] -> Mitigation: document first-iteration support and leave purity expansion as a follow-up change.

## Migration Plan

1. Add the new `com-yahoo-eu-metal` provider alongside the existing India provider.
2. Extend provider registration, config schema options, and UI metadata.
3. Update documentation to distinguish India and EU provider semantics.
4. Add tests for normalized EUR-per-gram pricing and end-to-end valuation.
5. Existing India configurations continue unchanged; EU users opt in by selecting the new provider code.

Rollback:
- Remove the new provider registration and schema entry.
- Existing users of the India provider are unaffected because their code path is unchanged.

## Open Questions

- Should lower-purity EU gold codes be derived from `gold-999` in a follow-up change, or should they require their own verified source data?
