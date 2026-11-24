import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import {
  getBudget,
  getCashFlow,
  getExpense,
  getIncome,
  getIncomeStatement,
  getRecurring
} from "./cashflow";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

const posting = {
  id: 1,
  transaction_id: "t1",
  date: "2026-11-10T00:00:00Z",
  payee: "Fixture Payee",
  account: "Expenses:Food",
  commodity: "EUR",
  quantity: 10,
  amount: 10,
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
  market_amount: 10,
  balance: -10
};

const transaction = {
  id: "t1",
  date: "2026-11-10T00:00:00Z",
  payee: "Fixture Payee",
  postings: [] as (typeof posting)[],
  tag_recurring: "monthly-rent",
  tag_period: "",
  beginLine: 1,
  endLine: 2,
  fileName: "main.ledger",
  note: ""
};

function mockJSON(body: unknown) {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(body))
  ) as unknown as typeof fetch;
}

test("getCashFlow maps the point date to dayjs", async () => {
  mockJSON({
    cash_flows: [
      {
        date: "2026-11-01T00:00:00Z",
        income: 1000,
        expenses: 200,
        liabilities: 50,
        investment: 300,
        tax: 100,
        checking: 450,
        balance: 450
      }
    ]
  });

  const response = await getCashFlow();
  expect(response.cash_flows).toHaveLength(1);
  expect(dayjs.isDayjs(response.cash_flows[0].date)).toBe(true);
  expect(response.cash_flows[0].date.format("YYYY-MM-DD")).toBe("2026-11-01");
  expect(response.cash_flows[0].balance).toBe(450);
});

test("getCashFlow turns a null array into an empty list", async () => {
  mockJSON({ cash_flows: null });
  await expect(getCashFlow()).resolves.toEqual({ cash_flows: [] });
});

test("getIncome converts timeline, tax and yearly card dates", async () => {
  mockJSON({
    income_timeline: [{ date: "2026-11-01T00:00:00Z", postings: [posting] }],
    tax_timeline: [
      {
        start_date: "2026-01-01T00:00:00Z",
        end_date: "2026-12-31T23:59:59Z",
        postings: [posting]
      }
    ],
    yearly_cards: [
      {
        start_date: "2026-01-01T00:00:00Z",
        end_date: "2026-12-31T23:59:59Z",
        postings: [posting],
        gross_income: 1000,
        net_tax: 100,
        net_income: 900
      }
    ]
  });

  const response = await getIncome();
  expect(dayjs.isDayjs(response.income_timeline[0].date)).toBe(true);
  expect(dayjs.isDayjs(response.income_timeline[0].postings[0].date)).toBe(true);
  expect(dayjs.isDayjs(response.tax_timeline[0].start_date)).toBe(true);
  expect(dayjs.isDayjs(response.tax_timeline[0].end_date)).toBe(true);
  expect(dayjs.isDayjs(response.yearly_cards[0].start_date)).toBe(true);
  expect(dayjs.isDayjs(response.yearly_cards[0].end_date)).toBe(true);
  expect(dayjs.isDayjs(response.yearly_cards[0].postings[0].date)).toBe(true);
  expect(response.income_timeline[0].date.format("YYYY-MM-DD")).toBe("2026-11-01");
});

test("getIncome turns null arrays into empty lists", async () => {
  mockJSON({ income_timeline: null, tax_timeline: null, yearly_cards: null });
  await expect(getIncome()).resolves.toEqual({
    income_timeline: [],
    tax_timeline: [],
    yearly_cards: []
  });
});

test("getExpense converts every grouped posting date", async () => {
  mockJSON({
    expenses: [posting],
    month_wise: {
      expenses: { "2026-11": [posting] },
      incomes: { "2026-11": [posting] },
      investments: { "2026-11": [posting] },
      taxes: { "2026-11": [posting] }
    },
    year_wise: {
      expenses: { "2026": [posting] },
      incomes: { "2026": [posting] },
      investments: { "2026": [posting] },
      taxes: { "2026": [posting] }
    },
    graph: {}
  });

  const response = await getExpense();
  expect(dayjs.isDayjs(response.expenses[0].date)).toBe(true);
  expect(dayjs.isDayjs(response.month_wise.expenses["2026-11"][0].date)).toBe(true);
  expect(dayjs.isDayjs(response.month_wise.incomes["2026-11"][0].date)).toBe(true);
  expect(dayjs.isDayjs(response.year_wise.taxes["2026"][0].date)).toBe(true);
  expect(response.month_wise.expenses["2026-11"][0].date.format("YYYY-MM-DD")).toBe("2026-11-10");
});

test("getExpense turns null maps and arrays into empty values", async () => {
  mockJSON({
    expenses: null,
    month_wise: null,
    year_wise: null,
    graph: null
  });

  await expect(getExpense()).resolves.toEqual({
    expenses: [],
    month_wise: { expenses: {}, incomes: {}, investments: {}, taxes: {} },
    year_wise: { expenses: {}, incomes: {}, investments: {}, taxes: {} },
    graph: {}
  });
});

test("getBudget converts budget and account dates", async () => {
  mockJSON({
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
            expenses: [posting]
          }
        ],
        availableThisMonth: 90,
        endOfMonthBalance: 90,
        forecast: 100
      }
    },
    checkingBalance: 10,
    availableForBudgeting: 10
  });

  const response = await getBudget();
  const budget = response.budgetsByMonth["2026-11"];
  expect(dayjs.isDayjs(budget.date)).toBe(true);
  expect(dayjs.isDayjs(budget.accounts[0].date)).toBe(true);
  expect(dayjs.isDayjs(budget.accounts[0].expenses[0].date)).toBe(true);
  expect(budget.date.format("YYYY-MM-DD")).toBe("2026-11-01");
  expect(response.checkingBalance).toBe(10);
});

test("getBudget turns a null month map into an empty object", async () => {
  mockJSON({ budgetsByMonth: null, checkingBalance: 0, availableForBudgeting: 0 });
  await expect(getBudget()).resolves.toEqual({
    budgetsByMonth: {},
    checkingBalance: 0,
    availableForBudgeting: 0
  });
});

test("getIncomeStatement converts the statement date", async () => {
  mockJSON({
    yearly: {
      "2026": {
        startingBalance: 0,
        endingBalance: 750,
        date: "2026-01-01T00:00:00Z",
        income: { "Income:Salary": -1000 },
        interest: {},
        equity: {},
        pnl: {},
        liabilities: { "Liabilities:Loan": -50 },
        tax: { "Expenses:Tax:IncomeTax": 100 },
        expenses: { "Expenses:Food": 200 }
      }
    }
  });

  const response = await getIncomeStatement();
  const statement = response.yearly["2026"];
  expect(dayjs.isDayjs(statement.date)).toBe(true);
  expect(statement.date.format("YYYY-MM-DD")).toBe("2026-01-01");
  expect(statement.endingBalance).toBe(750);
});

test("getIncomeStatement turns a null year map into an empty object", async () => {
  mockJSON({ yearly: null });
  await expect(getIncomeStatement()).resolves.toEqual({ yearly: {} });
});

test("getRecurring converts transaction and posting dates", async () => {
  mockJSON({
    transaction_sequences: [
      {
        transactions: [{ ...transaction, postings: [posting] }],
        key: "monthly-rent",
        period: "",
        interval: 30
      }
    ]
  });

  const response = await getRecurring();
  const sequence = response.transaction_sequences[0];
  expect(sequence.key).toBe("monthly-rent");
  expect(dayjs.isDayjs(sequence.transactions[0].date)).toBe(true);
  expect(dayjs.isDayjs(sequence.transactions[0].postings[0].date)).toBe(true);
});

test("getRecurring turns a null array into an empty list", async () => {
  mockJSON({ transaction_sequences: null });
  await expect(getRecurring()).resolves.toEqual({ transaction_sequences: [] });
});
