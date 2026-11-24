import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { getLiabilityBalance, getLiabilityInterest, getLiabilityRepayment } from "./liabilities";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function mockJSON(body: unknown) {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(body))
  ) as unknown as typeof fetch;
}

const posting = {
  id: 1,
  transaction_id: "t1",
  date: "2026-11-20T00:00:00Z",
  payee: "Fixture Payee",
  account: "Liabilities:Loan",
  commodity: "EUR",
  quantity: -200,
  amount: -200,
  status: "cleared",
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
  market_amount: -200,
  balance: -200
};

test("getLiabilityInterest converts the nested overview timeline dates", async () => {
  mockJSON({
    interest_timeline_breakdown: [
      {
        account: "Liabilities:Loan",
        apr: 0.12,
        overview_timeline: [
          {
            date: "2026-11-20T00:00:00Z",
            drawn_amount: 1000,
            repaid_amount: 200,
            interest_amount: 10
          },
          {
            date: "2026-11-21T00:00:00Z",
            drawn_amount: 1000,
            repaid_amount: 210,
            interest_amount: 10
          }
        ]
      }
    ]
  });

  const response = await getLiabilityInterest();
  expect(response.interest_timeline_breakdown).toHaveLength(1);

  const interest = response.interest_timeline_breakdown[0];
  expect(interest.account).toBe("Liabilities:Loan");
  expect(interest.apr).toBe(0.12);
  expect(interest.overview_timeline).toHaveLength(2);
  expect(dayjs.isDayjs(interest.overview_timeline[0].date)).toBe(true);
  expect(interest.overview_timeline[0].date.format("YYYY-MM-DD")).toBe("2026-11-20");
  expect(interest.overview_timeline[1].date.format("YYYY-MM-DD")).toBe("2026-11-21");
  expect(interest.overview_timeline[1].repaid_amount).toBe(210);
});

test("getLiabilityInterest turns null arrays into empty lists", async () => {
  mockJSON({ interest_timeline_breakdown: null });
  await expect(getLiabilityInterest()).resolves.toEqual({ interest_timeline_breakdown: [] });

  mockJSON({
    interest_timeline_breakdown: [{ account: "Liabilities:Loan", apr: 0, overview_timeline: null }]
  });
  const response = await getLiabilityInterest();
  expect(response.interest_timeline_breakdown[0].overview_timeline).toEqual([]);
});

test("getLiabilityBalance keeps the group-keyed map and defaults null to an object", async () => {
  mockJSON({
    liability_breakdowns: {
      "Liabilities:Loan": {
        group: "Liabilities:Loan",
        drawn_amount: 1000,
        repaid_amount: 200,
        interest_amount: 10,
        balance_amount: 800,
        apr: 0.12
      }
    }
  });

  const response = await getLiabilityBalance();
  expect(Object.keys(response.liability_breakdowns)).toEqual(["Liabilities:Loan"]);
  expect(response.liability_breakdowns["Liabilities:Loan"].balance_amount).toBe(800);

  mockJSON({ liability_breakdowns: null });
  await expect(getLiabilityBalance()).resolves.toEqual({ liability_breakdowns: {} });
});

test("getLiabilityRepayment converts posting dates only", async () => {
  mockJSON({ repayments: [posting] });

  const response = await getLiabilityRepayment();
  expect(response.repayments).toHaveLength(1);
  expect(dayjs.isDayjs(response.repayments[0].date)).toBe(true);
  expect(response.repayments[0].date.format("YYYY-MM-DD")).toBe("2026-11-20");
  expect(response.repayments[0].amount).toBe(-200);

  mockJSON({ repayments: null });
  await expect(getLiabilityRepayment()).resolves.toEqual({ repayments: [] });
});
