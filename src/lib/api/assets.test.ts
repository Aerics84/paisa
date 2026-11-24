import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { allocationFromWire, getNetworth, getPortfolioAllocation } from "./assets";
import type { AllocationResponse } from "./types.gen";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

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
