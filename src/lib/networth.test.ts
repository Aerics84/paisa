import { describe, expect, test } from "bun:test";
import { hasNetworthKpiData } from "./networth";
import type { CurrencyNetworth, Networth } from "./utils";

function point(): Networth {
  return {
    date: null,
    investmentAmount: 100,
    gainAmount: 10,
    withdrawalAmount: 0,
    balanceAmount: 110,
    balanceUnits: 0,
    netInvestmentAmount: 100
  } as unknown as Networth;
}

function currency(timeline: Networth[]): CurrencyNetworth {
  return {
    name: "USD",
    symbol: "$",
    commodity: "USD",
    networthTimeline: timeline
  };
}

describe("hasNetworthKpiData", () => {
  test("hides KPI cards when the selected currency has no timeline", () => {
    expect(hasNetworthKpiData(currency([]), [])).toBe(false);
  });

  test("shows KPI cards when the selected currency has data", () => {
    const timeline = [point()];
    expect(hasNetworthKpiData(currency(timeline), timeline)).toBe(true);
  });

  test("keeps the default currency behaviour when nothing is selected", () => {
    expect(hasNetworthKpiData(undefined, [])).toBe(true);
  });
});
