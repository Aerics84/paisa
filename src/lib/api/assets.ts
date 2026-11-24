import type { Dayjs } from "dayjs";
import { parseDate, request, requestApi } from "./client";
import type { AssetBreakdown, BalanceSummary } from "./assets.gen";
import { postingsFromWire, type Posting } from "./ledger";
import type { PostingReturn as WirePostingReturn } from "./service.gen";
import type {
  AccountGain as WireAccountGain,
  AccountGainResponse as WireAccountGainResponse,
  Aggregate as WireAggregate,
  AllocationResponse as WireAllocationResponse,
  AllocationTarget as WireAllocationTarget,
  CommodityBreakdown,
  CurrencyAccountGain as WireCurrencyAccountGain,
  CurrencyNetworth as WireCurrencyNetworth,
  Gain as WireGain,
  GainResponse as WireGainResponse,
  InvestmentResponse as WireInvestmentResponse,
  InvestmentYearlyCard as WireInvestmentYearlyCard,
  Networth as WireNetworth,
  NetworthResponse as WireNetworthResponse,
  PortfolioAggregate,
  PortfolioAllocationGroups as WirePortfolioAllocation,
  RebalanceItem as WireRebalanceItem,
  RebalancePlan as WireRebalancePlan,
  SaveAllocationTargetsResponse
} from "./types.gen";
import type { AllocationTargetEntry } from "../allocation_targets";

export type { AssetBreakdown };

/** Networth point with a dayjs date instead of a wire timestamp. */
export type Networth = Omit<WireNetworth, "date"> & { date: Dayjs };

/** Wire currency timeline converted to domain networth points. */
export type CurrencyNetworth = Omit<WireCurrencyNetworth, "networthTimeline"> & {
  networthTimeline: Networth[];
};

export interface NetworthResponse {
  networthTimeline: Networth[];
  currencyNetworths: CurrencyNetworth[];
  xirr: number;
}

export type PostingReturn = Omit<WirePostingReturn, "annualized_return"> & {
  annualized_return: number | null;
};

/** Account-level gain with every date converted to dayjs. */
export type CurrencyAccountGain = Omit<WireCurrencyAccountGain, "networthTimeline" | "postings"> & {
  networthTimeline: Networth[];
  postings: Posting[];
};

/** Account-level gain with every date converted to dayjs. */
export type AccountGain = Omit<
  WireAccountGain,
  "networthTimeline" | "postings" | "posting_returns" | "currencies"
> & {
  networthTimeline: Networth[];
  postings: Posting[];
  posting_returns: PostingReturn[];
  currencies: CurrencyAccountGain[];
};

export interface AccountGainResponse {
  gain_timeline_breakdown: AccountGain;
  portfolio_allocation: PortfolioAllocation;
  asset_breakdown: AssetBreakdown;
}

/** One gain row of GET /api/gain with dates converted. */
export type Gain = Omit<WireGain, "networth" | "postings"> & {
  networth: Networth;
  postings: Posting[];
};

export interface GainResponse {
  gain_breakdown: Gain[];
}

/** Investment yearly card with financial-year bounds converted to dayjs. */
export type InvestmentYearlyCard = Omit<
  WireInvestmentYearlyCard,
  "start_date" | "end_date" | "postings"
> & {
  start_date: Dayjs;
  end_date: Dayjs;
  postings: Posting[];
};

export interface InvestmentResponse {
  assets: Posting[];
  yearly_cards: InvestmentYearlyCard[];
}

export type AllocationTargetStatus = "in_band" | "overweight" | "underweight";

/** Allocation aggregate point with a dayjs date and the computed chart percent. */
export type Aggregate = Omit<WireAggregate, "date"> & {
  date: Dayjs;
  percent: number;
};

export type AllocationTarget = Omit<WireAllocationTarget, "aggregates" | "status"> & {
  aggregates: Record<string, Aggregate>;
  status: AllocationTargetStatus;
};

export type RebalanceAction = "BUY" | "SELL" | "HOLD";

export type RebalanceMode = "cash_injection" | "full_rebalance";

export type RebalanceItem = Omit<WireRebalanceItem, "action"> & {
  action: RebalanceAction;
};

export type RebalancePlan = Omit<WireRebalancePlan, "mode" | "items"> & {
  mode: RebalanceMode;
  items: RebalanceItem[];
};

export interface Allocation {
  aggregates: Record<string, Aggregate>;
  aggregates_timeline: Record<string, Aggregate>[];
  allocation_targets: AllocationTarget[];
  rebalance?: RebalancePlan;
}

/** Portfolio allocation response. It carries no dates, so the wire shape is reused. */
export type PortfolioAllocation = WirePortfolioAllocation;

export type { CommodityBreakdown, PortfolioAggregate };

function networthFromWire(wire: WireNetworth): Networth {
  return { ...wire, date: parseDate(wire.date) };
}

function networthTimelineFromWire(wire: readonly WireNetworth[] | null | undefined): Networth[] {
  return (wire ?? []).map(networthFromWire);
}

function currencyNetworthFromWire(wire: WireCurrencyNetworth): CurrencyNetworth {
  return {
    ...wire,
    networthTimeline: networthTimelineFromWire(wire.networthTimeline)
  };
}

function postingReturnFromWire(wire: WirePostingReturn): PostingReturn {
  return {
    ...wire,
    annualized_return: wire.annualized_return ?? null
  };
}

function currencyAccountGainFromWire(wire: WireCurrencyAccountGain): CurrencyAccountGain {
  return {
    ...wire,
    networthTimeline: networthTimelineFromWire(wire.networthTimeline),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

function accountGainFromWire(wire: WireAccountGain): AccountGain {
  return {
    ...wire,
    networthTimeline: networthTimelineFromWire(wire.networthTimeline),
    postings: postingsFromWire(wire.postings ?? []),
    posting_returns: (wire.posting_returns ?? []).map(postingReturnFromWire),
    currencies: (wire.currencies ?? []).map(currencyAccountGainFromWire)
  };
}

function gainFromWire(wire: WireGain): Gain {
  return {
    ...wire,
    networth: networthFromWire(wire.networth),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

function investmentYearlyCardFromWire(wire: WireInvestmentYearlyCard): InvestmentYearlyCard {
  return {
    ...wire,
    start_date: parseDate(wire.start_date),
    end_date: parseDate(wire.end_date),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

function aggregateFromWire(wire: WireAggregate): Aggregate {
  return { ...wire, date: parseDate(wire.date), percent: 0 };
}

function aggregatesFromWire(
  wire: Record<string, WireAggregate> | null | undefined
): Record<string, Aggregate> {
  const result: Record<string, Aggregate> = {};
  for (const [account, aggregate] of Object.entries(wire ?? {})) {
    result[account] = aggregateFromWire(aggregate);
  }
  return result;
}

function allocationTargetFromWire(wire: WireAllocationTarget): AllocationTarget {
  return {
    ...wire,
    // The server constrains status to these values in computeAllocationTarget.
    status: wire.status as AllocationTargetStatus,
    aggregates: aggregatesFromWire(wire.aggregates)
  };
}

function rebalanceItemFromWire(wire: WireRebalanceItem): RebalanceItem {
  return {
    ...wire,
    // The server only emits BUY, SELL or HOLD.
    action: wire.action as RebalanceAction
  };
}

function rebalancePlanFromWire(wire: WireRebalancePlan): RebalancePlan {
  return {
    ...wire,
    // The handler accepts an empty mode and echoes it back. The request side
    // treats an empty mode as cash_injection, so mirror that default here.
    mode: wire.mode === "" ? "cash_injection" : (wire.mode as RebalanceMode),
    items: (wire.items ?? []).map(rebalanceItemFromWire)
  };
}

export function networthResponseFromWire(wire: WireNetworthResponse): NetworthResponse {
  return {
    networthTimeline: networthTimelineFromWire(wire.networthTimeline),
    currencyNetworths: (wire.currencyNetworths ?? []).map(currencyNetworthFromWire),
    xirr: wire.xirr
  };
}

export function allocationFromWire(wire: WireAllocationResponse): Allocation {
  return {
    aggregates: aggregatesFromWire(wire.aggregates),
    aggregates_timeline: (wire.aggregates_timeline ?? []).map(aggregatesFromWire),
    allocation_targets: (wire.allocation_targets ?? []).map(allocationTargetFromWire),
    rebalance: wire.rebalance ? rebalancePlanFromWire(wire.rebalance) : undefined
  };
}

export function portfolioAllocationFromWire(
  wire: WirePortfolioAllocation | null | undefined
): PortfolioAllocation {
  return {
    commodities: wire?.commodities ?? [],
    name_and_security_type: wire?.name_and_security_type ?? [],
    security_type: wire?.security_type ?? [],
    rating: wire?.rating ?? [],
    industry: wire?.industry ?? []
  };
}

export async function getNetworth(): Promise<NetworthResponse> {
  return networthResponseFromWire(await request<WireNetworthResponse>("/api/networth"));
}

export async function getInvestment(): Promise<InvestmentResponse> {
  const wire = await request<WireInvestmentResponse>("/api/investment");
  return {
    assets: postingsFromWire(wire.assets ?? []),
    yearly_cards: (wire.yearly_cards ?? []).map(investmentYearlyCardFromWire)
  };
}

export async function getGain(): Promise<GainResponse> {
  const wire = await request<WireGainResponse>("/api/gain");
  return { gain_breakdown: (wire.gain_breakdown ?? []).map(gainFromWire) };
}

export async function getAccountGain(account: string): Promise<AccountGainResponse> {
  const wire = await request<WireAccountGainResponse>("/api/gain/:account", undefined, { account });
  return {
    gain_timeline_breakdown: accountGainFromWire(wire.gain_timeline_breakdown),
    portfolio_allocation: portfolioAllocationFromWire(wire.portfolio_allocation),
    asset_breakdown: wire.asset_breakdown
  };
}

export async function getAllocation(): Promise<Allocation> {
  return allocationFromWire(await request<WireAllocationResponse>("/api/allocation"));
}

export async function rebalance(requestBody: {
  cash: number;
  mode: "cash_injection" | "full_rebalance";
}): Promise<RebalancePlan> {
  const wire = await requestApi<WireRebalancePlan>("/api/allocation/rebalance", {
    method: "POST",
    body: JSON.stringify(requestBody)
  });
  return rebalancePlanFromWire(wire);
}

export async function saveAllocationTargets(
  targets: readonly AllocationTargetEntry[]
): Promise<SaveAllocationTargetsResponse> {
  return requestApi<SaveAllocationTargetsResponse>("/api/allocation/targets", {
    method: "POST",
    body: JSON.stringify(targets)
  });
}

export async function getPortfolioAllocation(): Promise<PortfolioAllocation> {
  return portfolioAllocationFromWire(
    await request<WirePortfolioAllocation>("/api/portfolio_allocation")
  );
}

export async function getAssetBalance(): Promise<BalanceSummary> {
  const wire = await request<BalanceSummary>("/api/assets/balance");
  return { asset_breakdowns: wire.asset_breakdowns ?? {} };
}
