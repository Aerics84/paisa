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
    has_income_postings: true,
    has_expense_postings: true,
    ...overrides
  };
}

function render(card: InvestmentYearlyCard): string {
  document.body.innerHTML = '<div id="d3-yearly-investment-cards"></div>';
  renderYearlyCards([card]);
  return document.querySelector("#d3-yearly-investment-cards").innerHTML;
}

function naCells(): number {
  return Array.from(document.querySelectorAll("#d3-yearly-investment-cards td")).filter(
    (td) => td.textContent.trim() === "N/A"
  ).length;
}

function cell(label: string): HTMLElement {
  const row = Array.from(document.querySelectorAll("#d3-yearly-investment-cards tr")).find((tr) =>
    tr.querySelector("td")?.textContent.trim().startsWith(label)
  );
  return row.querySelectorAll("td")[1] as HTMLElement;
}

function hasColorClass(label: string, colorClass: string): boolean {
  return cell(label).classList.contains(colorClass);
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

    for (const netIncome of [0, -100]) {
      const html = render(
        yearlyCard({
          net_income: netIncome,
          overall_surplus: netIncome - 600,
          overall_savings_rate: 0,
          overall_savings_rate_available: false,
          savings_rate: 0
        })
      );

      expect(naCells()).toBe(2);
      expect(html).not.toContain("0.00%");
    }
  });

  test("shows N/A for the surplus when no expense was recorded", () => {
    useLocale("en-US");

    const html = render(
      yearlyCard({
        net_expense: 0,
        overall_surplus: 900,
        overall_savings_rate: 100,
        overall_savings_rate_available: true,
        savings_rate: 22.2222,
        has_expense_postings: false
      })
    );

    expect(naCells()).toBe(2);
    expect(html).not.toContain("100.00%");
    expect(html).toContain("22.22%");
  });

  test("shows N/A for the surplus when no income was recorded", () => {
    useLocale("en-US");

    const html = render(
      yearlyCard({
        net_income: 0,
        gross_salary_income: 0,
        overall_surplus: -600,
        overall_savings_rate: 0,
        overall_savings_rate_available: false,
        savings_rate: 0,
        has_income_postings: false
      })
    );

    expect(naCells()).toBe(3);
    expect(html).not.toContain("\u2212600.00");
  });

  test("keeps a recorded deficit negative and a zero investment rate visible", () => {
    useLocale("en-US");

    const deficit = render(yearlyCard({ overall_surplus: -200, overall_savings_rate: -22.2222 }));
    expect(deficit).toContain("\u2212200.00");
    expect(deficit).toContain("\u221222.22%");

    const noInvestment = render(yearlyCard({ net_investment: 0, savings_rate: 0 }));
    expect(noInvestment).toContain("0.00%");
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
    expect(html).toContain("20% of net income");
  });
});

describe("yearly investment card colors", () => {
  test("colors a positive year with an above-target savings rate green", () => {
    useLocale("en-US");

    render(yearlyCard({ overall_savings_rate: 33.3333, savings_rate: 22.2222 }));

    expect(hasColorClass("Total Surplus", "has-text-success")).toBe(true);
    expect(hasColorClass("Total Savings Rate", "has-text-success")).toBe(true);
  });

  test("colors a below-target but positive savings rate amber", () => {
    useLocale("en-US");

    render(yearlyCard({ overall_savings_rate: 15.21 }));

    expect(hasColorClass("Total Surplus", "has-text-success")).toBe(true);
    expect(hasColorClass("Total Savings Rate", "has-text-warning-dark")).toBe(true);
    expect(hasColorClass("Total Savings Rate", "has-text-success")).toBe(false);
  });

  test("colors a deficit year red", () => {
    useLocale("en-US");

    render(yearlyCard({ overall_surplus: -200, overall_savings_rate: -22.2222 }));

    expect(hasColorClass("Total Surplus", "has-text-danger")).toBe(true);
    expect(hasColorClass("Total Savings Rate", "has-text-danger")).toBe(true);
  });

  test("keeps a zero surplus neutral", () => {
    useLocale("en-US");

    render(yearlyCard({ overall_surplus: 0, overall_savings_rate: 0 }));

    for (const color of ["has-text-success", "has-text-danger"]) {
      expect(hasColorClass("Total Surplus", color)).toBe(false);
    }
    expect(hasColorClass("Total Savings Rate", "has-text-warning-dark")).toBe(true);
  });

  test("keeps unavailable values and the investment rate neutral", () => {
    useLocale("en-US");

    render(
      yearlyCard({
        net_income: 0,
        overall_surplus: 0,
        overall_savings_rate: 0,
        overall_savings_rate_available: false,
        savings_rate: 0,
        has_income_postings: false
      })
    );

    for (const label of ["Total Surplus", "Total Savings Rate", "Investment Rate"]) {
      for (const color of ["has-text-success", "has-text-danger", "has-text-warning-dark"]) {
        expect(hasColorClass(label, color)).toBe(false);
      }
    }
  });

  test("marks negative amounts as negative", () => {
    useLocale("en-US");

    render(yearlyCard({ net_income: -100, net_investment: -500 }));

    expect(hasColorClass("Net Income", "has-text-danger")).toBe(true);
    expect(hasColorClass("Investment", "has-text-danger")).toBe(true);
    expect(hasColorClass("Gross Salary Income", "has-text-danger")).toBe(false);
  });
});
