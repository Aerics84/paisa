import { afterEach, describe, expect, test } from "bun:test";
import { commodityTooltip, findCommodityInfo } from "./commodity";

const previousConfig = (globalThis as any).USER_CONFIG;

afterEach(() => {
  (globalThis as any).USER_CONFIG = previousConfig;
});

function useCommodities(commodities: unknown[]) {
  (globalThis as any).USER_CONFIG = { commodities };
}

describe("commodity tooltip", () => {
  test("resolves display name and symbol from config", () => {
    useCommodities([
      {
        name: "IE00BF0L3536",
        display_name: "AIB Group",
        price: { provider: "com-yahoo", code: "A5G.IR" }
      }
    ]);

    const info = findCommodityInfo("IE00BF0L3536");

    expect(info).toEqual({ displayName: "AIB Group", code: "A5G.IR" });

    const html = commodityTooltip("IE00BF0L3536");
    expect(html).toContain("<strong>AIB Group</strong>");
    expect(html).toContain("Symbol: A5G.IR");
    expect(html).toContain("Commodity: IE00BF0L3536");
  });

  test("returns an empty tooltip for unconfigured commodities", () => {
    useCommodities([]);

    expect(commodityTooltip("NIFTY")).toBe("");
  });

  test("returns an empty tooltip when no display name or symbol exists", () => {
    useCommodities([{ name: "NIFTY", type: "mutualfund", price: {} }]);

    expect(commodityTooltip("NIFTY")).toBe("");
  });

  test("handles a missing user config", () => {
    (globalThis as any).USER_CONFIG = undefined;

    expect(findCommodityInfo("NIFTY")).toEqual({ displayName: "", code: "" });
    expect(commodityTooltip("NIFTY")).toBe("");
  });

  test("escapes configured values", () => {
    useCommodities([
      {
        name: "A&B",
        display_name: "A&B <Fund>",
        price: { code: "<X>" }
      }
    ]);

    const html = commodityTooltip("A&B");

    expect(html).toContain("A&amp;B &lt;Fund&gt;");
    expect(html).toContain("&lt;X&gt;");
  });
});
