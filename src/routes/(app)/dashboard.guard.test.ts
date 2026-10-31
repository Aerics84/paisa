import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("classic dashboard currency", () => {
  test("uses symbol formatting for net worth, net investment and gain/loss", () => {
    const source = readFileSync(join(__dirname, "+page.svelte"), "utf-8");
    expect(source).toContain("formatCurrencyWithSymbol(networth.balanceAmount)");
    expect(source).toContain("formatCurrencyWithSymbol(networth.netInvestmentAmount)");
    expect(source).toContain("formatCurrencyWithSymbol(networth.gainAmount)");
  });
});
