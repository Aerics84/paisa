import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import { cashFlowNetValues } from "./cash_flow";
import type { CashFlow } from "./utils";

function cashFlow(overrides: Partial<CashFlow> = {}): CashFlow {
  return {
    date: dayjs("2026-01-01"),
    income: 5000,
    liabilities: 0,
    expenses: 2000,
    investment: 1000,
    tax: 500,
    checking: 1500,
    balance: 1500,
    ...overrides
  };
}

describe("cash flow net values", () => {
  test("subtracts tax once and separates saving from spending", () => {
    expect(cashFlowNetValues(cashFlow())).toEqual({
      netAfterTax: 2500,
      netAfterSaving: 1500
    });
  });

  test("treats a negative tax value as a refund", () => {
    expect(cashFlowNetValues(cashFlow({ tax: -300 }))).toEqual({
      netAfterTax: 3300,
      netAfterSaving: 2300
    });
  });

  test("treats an investment withdrawal as incoming cash", () => {
    expect(cashFlowNetValues(cashFlow({ investment: -400 }))).toEqual({
      netAfterTax: 2500,
      netAfterSaving: 2900
    });
  });

  test("returns zero for a balanced month", () => {
    expect(cashFlowNetValues(cashFlow({ income: 0, expenses: 0, tax: 0, investment: 0 }))).toEqual({
      netAfterTax: 0,
      netAfterSaving: 0
    });
  });
});
