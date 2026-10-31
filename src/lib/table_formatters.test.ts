import { describe, expect, test } from "bun:test";
import { commodityTooltipHtml, indendedAssetAccountName } from "./table_formatters";

describe("commodity tooltip", () => {
  test("shows name, symbol and ledger commodity without duplicates", () => {
    const html = commodityTooltipHtml("AIB Group", "AIB Group", "AIBRF");
    expect(html).toContain("AIB Group");
    expect(html).toContain("AIBRF");
    expect(html.match(/AIB Group/g)?.length).toBe(1);
  });

  test("escapes config values", () => {
    const html = commodityTooltipHtml("A&B", "A&B", "<X>");
    expect(html).toContain("A&amp;B");
    expect(html).toContain("&lt;X&gt;");
  });

  test("returns empty when no metadata exists", () => {
    expect(commodityTooltipHtml("", "", "")).toBe("");
  });

  test("attaches an accessible tooltip to the account link", () => {
    const cell = {
      getValue: () => "Assets:Investments:IE00BF0L3536",
      getData: () => ({
        commodity: "AIB Group",
        commodity_name: "AIB Group",
        commodity_code: "AIBRF"
      })
    } as any;
    const html = indendedAssetAccountName(cell);
    expect(html).toContain("data-tippy-content");
    expect(html).toContain('aria-label="AIB Group (AIBRF)"');
    expect(html).toContain("title=");
  });
});
