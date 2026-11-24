import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import { postingsFromWire, type Posting } from "./ledger";
import type {
  AssetBreakdown as WireLiabilityBreakdown,
  Interest as WireInterest,
  Overview as WireInterestOverview
} from "./liabilities.gen";
import type {
  LiabilityBalanceResponse as WireLiabilityBalanceResponse,
  LiabilityInterestResponse as WireLiabilityInterestResponse,
  LiabilityRepaymentResponse as WireLiabilityRepaymentResponse
} from "./types.gen";

/** Liability breakdown row; it has no dates, so the wire shape is reused. */
export type LiabilityBreakdown = WireLiabilityBreakdown;

/** Interest overview point with its day converted to dayjs. */
export type InterestOverview = Omit<WireInterestOverview, "date"> & { date: Dayjs };

/** Per-account interest timeline with every overview date converted. */
export type Interest = Omit<WireInterest, "overview_timeline"> & {
  overview_timeline: InterestOverview[];
};

export interface LiabilityInterestResponse {
  interest_timeline_breakdown: Interest[];
}

export interface LiabilityBalanceResponse {
  liability_breakdowns: Record<string, LiabilityBreakdown>;
}

export interface LiabilityRepaymentResponse {
  repayments: Posting[];
}

function interestOverviewFromWire(wire: WireInterestOverview): InterestOverview {
  return { ...wire, date: parseDate(wire.date) };
}

function interestFromWire(wire: WireInterest): Interest {
  return {
    ...wire,
    overview_timeline: (wire.overview_timeline ?? []).map(interestOverviewFromWire)
  };
}

export async function getLiabilityInterest(): Promise<LiabilityInterestResponse> {
  const wire = await request<WireLiabilityInterestResponse>("/api/liabilities/interest");
  return {
    interest_timeline_breakdown: (wire.interest_timeline_breakdown ?? []).map(interestFromWire)
  };
}

export async function getLiabilityBalance(): Promise<LiabilityBalanceResponse> {
  const wire = await request<WireLiabilityBalanceResponse>("/api/liabilities/balance");
  return { liability_breakdowns: wire.liability_breakdowns ?? {} };
}

export async function getLiabilityRepayment(): Promise<LiabilityRepaymentResponse> {
  const wire = await request<WireLiabilityRepaymentResponse>("/api/liabilities/repayment");
  return { repayments: postingsFromWire(wire.repayments ?? []) };
}
