import { describe, expect, test } from "bun:test";
import { preferredSource } from "./price_source";

describe("preferredSource", () => {
  test("uses the newest price when it already has a provider source", () => {
    const history = [
      { commodity_type: "stock", commodity_id: "AAPL" },
      { commodity_type: "unknown", commodity_id: "ledger" }
    ];

    expect(preferredSource(history)).toEqual(history[0]);
  });

  test("falls back to a provider entry when the newest price comes from the ledger", () => {
    const history = [
      { commodity_type: "unknown", commodity_id: "ledger" },
      { commodity_type: "stock", commodity_id: "AAPL" }
    ];

    expect(preferredSource(history)).toEqual(history[1]);
  });

  test("keeps the newest entry when no provider source exists", () => {
    const history = [{ commodity_type: "unknown", commodity_id: "ledger" }];

    expect(preferredSource(history)).toEqual(history[0]);
  });

  test("handles an empty history", () => {
    expect(preferredSource([])).toBeUndefined();
  });
});
