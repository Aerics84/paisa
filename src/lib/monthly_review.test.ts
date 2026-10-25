import { describe, expect, test } from "bun:test";
import {
  hasRecordedActivity,
  isCurrentMonthlyReview,
  monthlyReviewEndpoint,
  performanceRows,
  surplusState,
  trendSeries,
  type MonthlyReviewResponse
} from "./monthly_review";

function monthlyReview(overrides: Partial<MonthlyReviewResponse> = {}): MonthlyReviewResponse {
  return {
    month: "2022-02",
    activity_state: "ready",
    period_complete: true,
    net_income_after_tax: 2000,
    net_tax: 100,
    net_expense: 500,
    surplus: 1500,
    has_income_postings: true,
    has_expense_postings: true,
    flow_graph: { nodes: [], links: [] },
    net_income_trend: [
      { month: "2021-12", value: 0, available: false, is_month_to_date: false },
      { month: "2022-01", value: 1800, available: true, is_month_to_date: false },
      { month: "2022-02", value: 2000, available: true, is_month_to_date: false }
    ],
    investment_contributions: 300,
    investment_market_movement: 120,
    dividend_income: 20,
    best_performers: [],
    weakest_performers: [],
    performance_state: "not_available",
    performance_as_of: "",
    ...overrides
  };
}

describe("monthly review selection", () => {
  test("accepts the review for the selected month", () => {
    expect(isCurrentMonthlyReview(monthlyReview(), "2022-02")).toBe(true);
  });

  test("rejects a stale review for another month", () => {
    expect(isCurrentMonthlyReview(monthlyReview(), "2022-03")).toBe(false);
  });

  test("rejects a missing review", () => {
    expect(isCurrentMonthlyReview(null, "2022-02")).toBe(false);
  });

  test("builds an encoded endpoint for the selected month", () => {
    expect(monthlyReviewEndpoint("2022-02")).toBe("/api/dashboard/monthly?month=2022-02");
  });
});

describe("monthly review trend", () => {
  test("keeps missing trend months unavailable instead of zero", () => {
    const series = trendSeries(monthlyReview());

    expect(series[0].value).toBeNull();
    expect(series[0].available).toBe(false);
    expect(series[1].value).toBe(1800);
    expect(series[2].value).toBe(2000);
  });

  test("marks the running month as month-to-date", () => {
    const review = monthlyReview({
      net_income_trend: [
        { month: "2021-12", value: 0, available: false, is_month_to_date: false },
        { month: "2022-01", value: 1800, available: true, is_month_to_date: false },
        { month: "2022-02", value: 500, available: true, is_month_to_date: true }
      ]
    });

    expect(trendSeries(review)[2].isMonthToDate).toBe(true);
  });

  test("returns an empty series when no review is loaded", () => {
    expect(trendSeries(null)).toEqual([]);
  });
});

describe("monthly review states", () => {
  test("reports a month without postings as no recorded activity", () => {
    const review = monthlyReview({
      activity_state: "no_recorded_activity",
      has_income_postings: false,
      has_expense_postings: false,
      net_income_after_tax: 0,
      net_expense: 0,
      surplus: 0
    });

    expect(hasRecordedActivity(review)).toBe(false);
    expect(surplusState(review)).toBe("no-activity");
  });

  test("does not claim a surplus when income is missing", () => {
    const review = monthlyReview({
      has_income_postings: false,
      net_income_after_tax: 0,
      surplus: -500
    });

    expect(surplusState(review)).toBe("income-missing");
  });

  test("does not claim a surplus when expenses are missing", () => {
    const review = monthlyReview({ has_expense_postings: false, net_expense: 0, surplus: 2000 });

    expect(surplusState(review)).toBe("expense-missing");
  });

  test("reports a real deficit as ready", () => {
    const review = monthlyReview({ net_income_after_tax: 900, net_expense: 1000, surplus: -100 });

    expect(surplusState(review)).toBe("ready");
  });

  test("never turns missing performance into zero-return rows", () => {
    const rows = performanceRows(monthlyReview());

    expect(rows.best).toEqual([]);
    expect(rows.weakest).toEqual([]);
    expect(rows.asOf).toBeNull();
  });

  test("keeps the separately identified dividend amount", () => {
    const rows = performanceRows(
      monthlyReview({
        performance_state: "ready",
        performance_as_of: "2022-02-28T00:00:00Z",
        best_performers: [
          {
            account: "Assets:Equity:VWCE",
            gain_loss: 100,
            total_return_percent: 10,
            dividend_income: 20,
            net_contributions: 0,
            net_withdrawals: 0,
            price_as_of: "2022-02-28T00:00:00Z"
          }
        ]
      })
    );

    expect(rows.best[0].dividend_income).toBe(20);
    expect(rows.asOf).toBe("2022-02-28T00:00:00Z");
  });
});
