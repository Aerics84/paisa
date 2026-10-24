import { afterEach, describe, expect, test } from "bun:test";
import dayjs from "dayjs";

import { renderYearlyCards } from "./investment";
import type { InvestmentYearlyCard } from "./utils";

function yearlyCard(overrides: Partial<InvestmentYearlyCard> = {}): InvestmentYearlyCard {
  return {
    start_date: dayjs("2024-04-01"),
    end_date: dayjs("2025-03-31 23:59:59"),
    postings: [],
    net_tax: 100,
    gross_salary_income: 1000,
    gross_other_income: 0,
    net_income: 900,
    net_investment: 200,
    net_expense: 600,
    savings_rate: 22.2222,
    overall_surplus: 300,
    overall_savings_rate: 33.3333,
    overall_savings_rate_available: true,
    period_complete: true,
    has_activity: true,
    ...overrides
  };
}

function render(card: InvestmentYearlyCard): string {
  document.body.innerHTML = '<div id="d3-yearly-investment-cards"></div>';
  renderYearlyCards([card]);
  return document.querySelector("#d3-yearly-investment-cards").innerHTML;
}

function useLocale(locale: string) {
  (globalThis as any).USER_CONFIG = { locale, display_precision: 2 };
}

afterEach(() => {
  delete (globalThis as any).USER_CONFIG;
});

describe("yearly investment card", () => {
  test("shows total surplus, total savings rate and investment rate", () => {
    useLocale("en-US");

    const html = render(yearlyCard());

    expect(html).toContain("Total Surplus");
    expect(html).toContain("Total Savings Rate");
    expect(html).toContain("Investment Rate");
    expect(html).toContain("33.33%");
    expect(html).toContain("22.22%");
  });

  test("shows N/A instead of a rate when net income is not positive", () => {
    useLocale("en-US");

    const html = render(
      yearlyCard({
        net_income: -100,
        overall_surplus: -700,
        overall_savings_rate: 0,
        overall_savings_rate_available: false,
        savings_rate: 0
      })
    );

    expect(html.match(/N\/A/g)).toHaveLength(2);
    expect(html).not.toContain("0.00%");
  });

  test("marks an ongoing financial year as YTD", () => {
    useLocale("en-US");

    const html = render(yearlyCard({ period_complete: false }));

    expect(html).toContain("YTD");
  });

  test("shows no recorded activity instead of a confirmed zero surplus", () => {
    useLocale("en-US");

    const html = render(
      yearlyCard({
        net_income: 0,
        net_expense: 0,
        net_investment: 0,
        net_tax: 0,
        gross_salary_income: 0,
        overall_surplus: 0,
        overall_savings_rate: 0,
        overall_savings_rate_available: false,
        savings_rate: 0,
        has_activity: false
      })
    );

    expect(html).toContain("No recorded activity");
    expect(html).not.toContain("Total Surplus");
  });

  test("explains the formula without claiming a bank balance change", () => {
    useLocale("en-US");

    const html = render(yearlyCard());

    expect(html).toContain("data-tippy-content");
    expect(html).toContain("Calculated from recorded ledger postings");
    expect(html).toContain("not the change in bank balance");
  });
});
