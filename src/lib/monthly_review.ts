import dayjs from "dayjs";
import type { Graph } from "./utils";

export type MonthlyActivityState = "ready" | "no_recorded_activity";
export type MonthlyPerformanceState = "ready" | "not_available";

export interface MonthlyMetricPoint {
  month: string;
  value: number;
  available: boolean;
  is_month_to_date: boolean;
}

export interface MonthlyPositionPerformance {
  account: string;
  gain_loss: number;
  total_return_percent: number;
  dividend_income: number;
  net_contributions: number;
  net_withdrawals: number;
  price_as_of: string;
}

/**
 * Period-specific review response. Live net worth and checking balances are
 * intentionally not part of it.
 */
export interface MonthlyReviewResponse {
  month: string;
  activity_state: MonthlyActivityState;
  period_complete: boolean;
  net_income_after_tax: number;
  net_tax: number;
  net_expense: number;
  surplus: number;
  has_income_postings: boolean;
  has_expense_postings: boolean;
  flow_graph: Graph;
  net_income_trend: MonthlyMetricPoint[];
  investment_contributions: number;
  investment_market_movement: number;
  dividend_income: number;
  best_performers: MonthlyPositionPerformance[];
  weakest_performers: MonthlyPositionPerformance[];
  performance_state: MonthlyPerformanceState;
  performance_as_of: string;
}

/**
 * A response is only renderable while it still belongs to the selected month.
 * This prevents an older asynchronous request from overwriting a newer pick.
 */
export function isCurrentMonthlyReview(
  review: MonthlyReviewResponse | null,
  selectedMonth: string
): boolean {
  return !!review && !!selectedMonth && review.month === selectedMonth;
}

export function monthlyReviewEndpoint(selectedMonth: string): string {
  return `/api/dashboard/monthly?month=${encodeURIComponent(selectedMonth)}`;
}

export interface TrendPoint {
  month: string;
  value: number | null;
  available: boolean;
  isMonthToDate: boolean;
}

/**
 * Missing months stay `null`; they must never be plotted or labelled as 0.
 */
export function trendSeries(review: MonthlyReviewResponse | null): TrendPoint[] {
  if (!review?.net_income_trend) {
    return [];
  }

  return review.net_income_trend.map((point) => ({
    month: point.month,
    value: point.available ? point.value : null,
    available: point.available,
    isMonthToDate: point.is_month_to_date
  }));
}

export function hasRecordedActivity(review: MonthlyReviewResponse | null): boolean {
  return review?.activity_state === "ready";
}

export type SurplusState =
  "ready" | "no-activity" | "income-missing" | "expense-missing" | "no-income-or-expense";

/**
 * A surplus is only shown when the month has both recorded income and recorded
 * expenses. Otherwise the interface shows the missing side instead of turning
 * incomplete bookkeeping into a confident number.
 */
export function surplusState(review: MonthlyReviewResponse | null): SurplusState {
  if (!review || review.activity_state !== "ready") {
    return "no-activity";
  }

  if (!review.has_income_postings && review.has_expense_postings) {
    return "income-missing";
  }

  if (review.has_income_postings && !review.has_expense_postings) {
    return "expense-missing";
  }

  if (!review.has_income_postings && !review.has_expense_postings) {
    return "no-income-or-expense";
  }

  return "ready";
}

export interface PerformanceRows {
  best: MonthlyPositionPerformance[];
  weakest: MonthlyPositionPerformance[];
  asOf: string | null;
}

/**
 * Ranking rows are only returned for a ready state; a not-available state is
 * never converted into a list of zero-return positions.
 */
export function performanceRows(review: MonthlyReviewResponse | null): PerformanceRows {
  if (!review || review.performance_state !== "ready") {
    return { best: [], weakest: [], asOf: null };
  }

  return {
    best: review.best_performers ?? [],
    weakest: review.weakest_performers ?? [],
    asOf: review.performance_as_of || null
  };
}

export function positionLabel(account: string): string {
  const parts = account.split(":");
  return parts[parts.length - 1] || account;
}

/**
 * Formats both a `YYYY-MM` key and a full RFC3339 timestamp as a compact
 * month label. Checking history is serialized as a timestamp, while trend
 * points are serialized as month keys.
 */
export function formatMonthLabel(value: string): string {
  const date = value.length === 7 ? dayjs(value + "-01") : dayjs(value);
  return date.isValid() ? date.format("MMM YY") : value;
}
