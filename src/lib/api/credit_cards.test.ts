import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { getCreditCard, getCreditCards } from "./credit_cards";

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
  date: "2026-11-10T00:00:00Z",
  payee: "Fixture Payee",
  account: "Liabilities:CreditCard:Visa",
  commodity: "EUR",
  quantity: -500,
  amount: -500,
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
  market_amount: -500,
  balance: -500
};

const bill = {
  statementStartDate: "2026-10-21T00:00:00Z",
  statementEndDate: "2026-11-20T00:00:00Z",
  dueDate: "2026-12-05T00:00:00Z",
  paidDate: null as string | null,
  credits: 100,
  debits: 500,
  DebitsRunningBalance: 500,
  openingBalance: 0,
  closingBalance: 400,
  postings: [posting],
  transactions: [
    {
      id: "t1",
      date: "2026-11-10T00:00:00Z",
      payee: "Fixture Payee",
      postings: [posting],
      tag_recurring: "",
      tag_period: "",
      beginLine: 1,
      endLine: 2,
      fileName: "main.ledger",
      note: ""
    }
  ]
};

const creditCard = {
  account: "Liabilities:CreditCard:Visa",
  network: "Visa",
  number: "4111",
  balance: 400,
  bills: [bill],
  creditLimit: 100000,
  yearlySpends: { "2025 - 26": { "2026-11": 500 } },
  expirationDate: "2030-12-31T00:00:00Z"
};

test("getCreditCards converts the card and nested bill dates", async () => {
  mockJSON({ creditCards: [creditCard] });

  const response = await getCreditCards();
  expect(response.creditCards).toHaveLength(1);

  const card = response.creditCards[0];
  expect(card.account).toBe("Liabilities:CreditCard:Visa");
  expect(dayjs.isDayjs(card.expirationDate)).toBe(true);
  expect(card.expirationDate.format("YYYY-MM-DD")).toBe("2030-12-31");
  expect(card.yearlySpends).toEqual({ "2025 - 26": { "2026-11": 500 } });

  expect(card.bills).toHaveLength(1);
  const statement = card.bills[0];
  expect(statement.statementStartDate.format("YYYY-MM-DD")).toBe("2026-10-21");
  expect(statement.statementEndDate.format("YYYY-MM-DD")).toBe("2026-11-20");
  expect(statement.dueDate.format("YYYY-MM-DD")).toBe("2026-12-05");
  expect(statement.paidDate).toBeNull();
  expect(statement.DebitsRunningBalance).toBe(500);
});

test("getCreditCards turns null lists and maps into empty values", async () => {
  mockJSON({ creditCards: null });
  await expect(getCreditCards()).resolves.toEqual({ creditCards: [] });

  mockJSON({ creditCards: [{ ...creditCard, bills: null, yearlySpends: null }] });
  const response = await getCreditCards();
  expect(response.creditCards[0].bills).toEqual([]);
  expect(response.creditCards[0].yearlySpends).toEqual({});
});

test("getCreditCard converts postings and transactions", async () => {
  mockJSON({ creditCard, found: true });

  const response = await getCreditCard("Liabilities:CreditCard:Visa");
  expect(response.found).toBe(true);
  expect(response.creditCard).toBeDefined();

  const statement = response.creditCard.bills[0];
  expect(dayjs.isDayjs(statement.postings[0].date)).toBe(true);
  expect(statement.postings[0].date.format("YYYY-MM-DD")).toBe("2026-11-10");
  expect(dayjs.isDayjs(statement.transactions[0].date)).toBe(true);
  expect(dayjs.isDayjs(statement.transactions[0].postings[0].date)).toBe(true);
});

test("getCreditCard converts a paid bill date", async () => {
  mockJSON({
    creditCard: { ...creditCard, bills: [{ ...bill, paidDate: "2026-12-01T00:00:00Z" }] },
    found: true
  });

  const response = await getCreditCard("Liabilities:CreditCard:Visa");
  const paidDate = response.creditCard.bills[0].paidDate;
  expect(dayjs.isDayjs(paidDate)).toBe(true);
  expect(paidDate.format("YYYY-MM-DD")).toBe("2026-12-01");
});

test("getCreditCard leaves a missing card absent", async () => {
  mockJSON({ found: false });

  const response = await getCreditCard("Liabilities:CreditCard:Missing");
  expect(response.found).toBe(false);
  expect(response.creditCard).toBeUndefined();
});
