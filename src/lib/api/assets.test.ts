import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import {
  allocationFromWire,
  getAccountGain,
  getInvestment,
  getNetworth,
  getPortfolioAllocation,
  rebalance
} from "./assets";
import type { Posting } from "./posting.gen";
import type { AccountGainResponse, AllocationResponse, InvestmentResponse } from "./types.gen";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function postingFixture(date: string, id = 1): Posting {
  return {
    id,
    transaction_id: "t1",
    date,
    payee: "Payee",
    account: "Assets:Broker:Fund",
    commodity: "EUR",
    quantity: 1,
    amount: 100,
    status: "",
    tag_recurring: "",
    tag_period: "",
    tax_category: "",
    tax_category_legacy: "",
    transaction_begin_line: 1,
    transaction_end_line: 2,
    file_name: "main.ledger",
    forecast: false,
    note: "",
    transaction_note: "",
    market_amount: 100,
    balance: 100
  };
}

test("getNetworth converts every timeline date to dayjs", async () => {
  globalThis.fetch = mock(
    async () =>
      new Response(
        JSON.stringify({
          networthTimeline: [
            {
              date: "2026-11-22T00:00:00Z",
              investmentAmount: 100,
              withdrawalAmount: 10,
              gainAmount: 5,
              balanceAmount: 95,
              balanceUnits: 0,
              netInvestmentAmount: 90
            }
          ],
          currencyNetworths: [
            {
              name: "US Dollar",
              symbol: "$",
              commodity: "USD",
              networthTimeline: [
                {
                  date: "2026-11-21T00:00:00Z",
                  investmentAmount: 1,
                  withdrawalAmount: 0,
                  gainAmount: 0,
                  balanceAmount: 1,
                  balanceUnits: 0,
                  netInvestmentAmount: 1
                }
              ]
            }
          ],
          xirr: 12.5
        })
      )
  ) as unknown as typeof fetch;

  const response = await getNetworth();

  expect(dayjs.isDayjs(response.networthTimeline[0].date)).toBe(true);
  expect(response.networthTimeline[0].date.format("YYYY-MM-DD")).toBe("2026-11-22");
  expect(dayjs.isDayjs(response.currencyNetworths[0].networthTimeline[0].date)).toBe(true);
  expect(response.currencyNetworths[0].networthTimeline[0].date.format("YYYY-MM-DD")).toBe(
    "2026-11-21"
  );
  expect(response.networthTimeline[0].balanceAmount).toBe(95);
  expect(response.xirr).toBe(12.5);
});

test("getPortfolioAllocation turns null arrays into empty arrays", async () => {
  globalThis.fetch = mock(
    async () =>
      new Response(
        JSON.stringify({
          commodities: null,
          name_and_security_type: null,
          security_type: null,
          rating: null,
          industry: null
        })
      )
  ) as unknown as typeof fetch;

  const allocation = await getPortfolioAllocation();

  expect(allocation.commodities).toEqual([]);
  expect(allocation.name_and_security_type).toEqual([]);
  expect(allocation.security_type).toEqual([]);
  expect(allocation.rating).toEqual([]);
  expect(allocation.industry).toEqual([]);
});

test("allocationFromWire maps nested aggregate dates and keeps target metadata", () => {
  const aggregate = {
    date: "2026-11-22T00:00:00Z",
    account: "Assets:Equity:Stock",
    market_amount: 1000
  };
  const wire: AllocationResponse = {
    aggregates: { "Assets:Equity:Stock": aggregate },
    aggregates_timeline: [{ "Assets:Equity:Stock": aggregate }],
    allocation_targets: [
      {
        name: "Equity",
        target: 100,
        current: 100,
        aggregates: { "Assets:Equity:Stock": aggregate },
        market_amount: 1000,
        drift: 0,
        drift_tolerance: 5,
        status: "in_band",
        accounts: ["Assets:Equity:*"],
        commodities: []
      }
    ],
    rebalance: {
      mode: "full_rebalance",
      cash_amount: 0,
      total_before: 1000,
      total_after: 1000,
      items: [
        {
          name: "Equity",
          target: 100,
          current: 100,
          market_amount: 1000,
          target_amount: 1000,
          action: "HOLD",
          amount: 0,
          new_percent: 100
        }
      ]
    }
  };

  const allocation = allocationFromWire(wire);

  expect(dayjs.isDayjs(allocation.aggregates["Assets:Equity:Stock"].date)).toBe(true);
  expect(dayjs.isDayjs(allocation.aggregates_timeline[0]["Assets:Equity:Stock"].date)).toBe(true);
  expect(
    dayjs.isDayjs(allocation.allocation_targets[0].aggregates["Assets:Equity:Stock"].date)
  ).toBe(true);
  // percent is chart state, not a wire field.
  expect(allocation.aggregates["Assets:Equity:Stock"].percent).toBe(0);
  expect(allocation.allocation_targets[0].status).toBe("in_band");
  expect(allocation.rebalance?.items[0].action).toBe("HOLD");
});

test("getInvestment converts financial-year bounds and posting dates", async () => {
  const investment: InvestmentResponse = {
    assets: [postingFixture("2026-11-01T00:00:00Z")],
    yearly_cards: [
      {
        start_date: "2024-04-01T00:00:00Z",
        end_date: "2025-03-31T23:59:59Z",
        postings: [postingFixture("2024-05-01T00:00:00Z", 2)],
        gross_salary_income: 1000,
        gross_other_income: 0,
        net_tax: 100,
        net_income: 900,
        net_investment: 200,
        net_expense: 600,
        savings_rate: 22.22,
        overall_surplus: 300,
        overall_savings_rate: 33.33,
        overall_savings_rate_available: true,
        period_complete: true,
        has_activity: true,
        has_income_postings: true,
        has_expense_postings: true
      }
    ]
  };
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(investment))
  ) as unknown as typeof fetch;

  const response = await getInvestment();

  expect(dayjs.isDayjs(response.assets[0].date)).toBe(true);
  expect(dayjs.isDayjs(response.yearly_cards[0].start_date)).toBe(true);
  expect(response.yearly_cards[0].start_date.format("YYYY-MM-DD")).toBe("2024-04-01");
  expect(dayjs.isDayjs(response.yearly_cards[0].end_date)).toBe(true);
  expect(response.yearly_cards[0].end_date.format("YYYY-MM-DD")).toBe("2025-03-31");
  expect(dayjs.isDayjs(response.yearly_cards[0].postings[0].date)).toBe(true);
  expect(response.yearly_cards[0].postings[0].date.format("YYYY-MM-DD")).toBe("2024-05-01");
});

test("getAccountGain converts timeline, posting and currency dates", async () => {
  const networthPoint = {
    date: "2026-11-22T00:00:00Z",
    investmentAmount: 100,
    withdrawalAmount: 0,
    gainAmount: 10,
    balanceAmount: 110,
    balanceUnits: 0,
    netInvestmentAmount: 100
  };
  const detail: AccountGainResponse = {
    gain_timeline_breakdown: {
      account: "Assets:Broker",
      networthTimeline: [networthPoint],
      xirr: 12.5,
      postings: [postingFixture("2026-11-20T00:00:00Z")],
      posting_returns: [
        {
          posting_id: 1,
          unrealized_gain: 10,
          holding_days: 400,
          annualized_return: 5
        }
      ],
      currencies: [
        {
          name: "Euro",
          symbol: "€",
          commodity: "EUR",
          networthTimeline: [networthPoint],
          xirr: 1,
          postings: [postingFixture("2026-11-19T00:00:00Z", 3)]
        }
      ]
    },
    portfolio_allocation: {
      commodities: [],
      name_and_security_type: [],
      security_type: [],
      rating: [],
      industry: []
    },
    asset_breakdown: {
      group: "Assets:Broker",
      investmentAmount: 100,
      withdrawalAmount: 0,
      marketAmount: 110,
      cashAmount: 0,
      balanceUnits: 1,
      latestPrice: 110,
      xirr: 12.5,
      gainAmount: 10,
      absoluteReturn: 0.1
    }
  };
  let requested = "";
  globalThis.fetch = mock(async (input) => {
    requested = String(input);
    return new Response(JSON.stringify(detail));
  }) as unknown as typeof fetch;

  const response = await getAccountGain("Assets:Broker");

  expect(requested).toBe("/api/gain/Assets:Broker");
  expect(dayjs.isDayjs(response.gain_timeline_breakdown.networthTimeline[0].date)).toBe(true);
  expect(dayjs.isDayjs(response.gain_timeline_breakdown.postings[0].date)).toBe(true);
  expect(
    dayjs.isDayjs(response.gain_timeline_breakdown.currencies[0].networthTimeline[0].date)
  ).toBe(true);
  expect(dayjs.isDayjs(response.gain_timeline_breakdown.currencies[0].postings[0].date)).toBe(true);
  expect(response.gain_timeline_breakdown.posting_returns[0].annualized_return).toBe(5);
});

test("rebalance treats an empty wire mode as cash_injection", async () => {
  let requestInit: RequestInit | undefined;
  globalThis.fetch = mock(async (input, init) => {
    expect(input).toBe("/api/allocation/rebalance");
    requestInit = init;
    return new Response(
      JSON.stringify({
        mode: "",
        cash_amount: 0,
        total_before: 0,
        total_after: 0,
        items: []
      })
    );
  }) as unknown as typeof fetch;

  const plan = await rebalance({ cash: 0, mode: "cash_injection" });

  expect(requestInit?.method).toBe("POST");
  expect(plan.mode).toBe("cash_injection");
});
