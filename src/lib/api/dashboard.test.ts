import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { getDashboard } from "./dashboard";
import type { DashboardResponse } from "./types.gen";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function dashboardFixture(): DashboardResponse {
  return {
    checkingBalances: { asset_breakdowns: {} },
    checkingBalanceTimeline: [
      { month: "2026-11-01T00:00:00Z", account: "Assets:Checking:Bank", balance: 10 }
    ],
    availableMonthRange: {
      from: "2026-01-01T00:00:00Z",
      to: "2026-11-01T00:00:00Z"
    },
    networth: {
      networth: {
        date: "2026-11-22T00:00:00Z",
        investmentAmount: 1,
        withdrawalAmount: 2,
        gainAmount: 3,
        balanceAmount: 4,
        balanceUnits: 5,
        netInvestmentAmount: 6
      },
      xirr: 7
    },
    expenses: {
      "2026-11": [
        {
          id: 1,
          transaction_id: "t1",
          date: "2026-11-22T00:00:00Z",
          payee: "Payee",
          account: "Expenses:Food",
          commodity: "INR",
          quantity: 10,
          amount: 10,
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
          market_amount: 10,
          balance: -10
        }
      ]
    },
    expense_pace: { period: "2026-11", projectedExpenses: 100 },
    cashFlows: [
      {
        date: "2026-11-01T00:00:00Z",
        income: 1,
        expenses: 2,
        liabilities: 0,
        investment: 3,
        tax: 0,
        checking: 4,
        balance: 5
      }
    ],
    transactionSequences: [
      {
        transactions: [
          {
            id: "t1",
            date: "2026-11-22T00:00:00Z",
            payee: "Payee",
            postings: [],
            tag_recurring: "",
            tag_period: "",
            beginLine: 1,
            endLine: 2,
            fileName: "main.ledger",
            note: ""
          }
        ],
        key: "t1",
        period: "",
        interval: 30
      }
    ],
    transactions: [],
    budget: {
      budgetsByMonth: {
        "2026-11": {
          date: "2026-11-01T00:00:00Z",
          accounts: [
            {
              account: "Expenses:Food",
              forecast: 100,
              actual: 10,
              rollover: 0,
              available: 90,
              date: "2026-11-01T00:00:00Z",
              expenses: []
            }
          ],
          availableThisMonth: 90,
          endOfMonthBalance: 90,
          forecast: 100
        }
      },
      checkingBalance: 10,
      availableForBudgeting: 10
    },
    goalSummaries: []
  };
}

test("getDashboard converts dates and keeps month bucket keys as strings", async () => {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(dashboardFixture()))
  ) as unknown as typeof fetch;

  const dashboard = await getDashboard();

  expect(dayjs.isDayjs(dashboard.availableMonthRange.from)).toBe(true);
  expect(dashboard.availableMonthRange.from.format("YYYY-MM-DD")).toBe("2026-01-01");
  expect(dayjs.isDayjs(dashboard.networth.networth.date)).toBe(true);
  expect(dayjs.isDayjs(dashboard.expenses["2026-11"][0].date)).toBe(true);
  expect(dayjs.isDayjs(dashboard.cashFlows[0].date)).toBe(true);
  expect(dayjs.isDayjs(dashboard.transactionSequences[0].transactions[0].date)).toBe(true);
  expect(dayjs.isDayjs(dashboard.budget.budgetsByMonth["2026-11"].date)).toBe(true);
  expect(dayjs.isDayjs(dashboard.budget.budgetsByMonth["2026-11"].accounts[0].date)).toBe(true);
  expect(dashboard.checkingBalanceTimeline[0].month).toBe("2026-11-01T00:00:00Z");
  expect(dashboard.expense_pace?.projectedExpenses).toBe(100);
});

test("getDashboard turns null arrays and null expense pace into safe values", async () => {
  const fixture = dashboardFixture();
  globalThis.fetch = mock(
    async () =>
      new Response(
        JSON.stringify({
          ...fixture,
          checkingBalanceTimeline: null,
          cashFlows: null,
          transactionSequences: null,
          transactions: null,
          expenses: null,
          goalSummaries: null,
          expense_pace: null
        })
      )
  ) as unknown as typeof fetch;

  const dashboard = await getDashboard();
  expect(dashboard.checkingBalanceTimeline).toEqual([]);
  expect(dashboard.cashFlows).toEqual([]);
  expect(dashboard.transactionSequences).toEqual([]);
  expect(dashboard.transactions).toEqual([]);
  expect(dashboard.expenses).toEqual({});
  expect(dashboard.goalSummaries).toEqual([]);
  expect(dashboard.expense_pace).toBeNull();
});
