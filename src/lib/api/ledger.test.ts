import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { postingsFromWire, transactionsFromWire } from "./ledger";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

test("getLedger converts wire dates to dayjs", async () => {
  globalThis.fetch = mock(
    async () =>
      new Response(JSON.stringify({ postings: [{ id: 1, date: "2026-11-22T00:00:00Z" }] }))
  ) as unknown as typeof fetch;

  const { getLedger } = await import("./ledger");
  const { postings } = await getLedger();
  expect(dayjs.isDayjs(postings[0].date)).toBe(true);
  expect(postings[0].date.format("YYYY-MM-DD")).toBe("2026-11-22");
});

test("postingsFromWire keeps every non-date field untouched", () => {
  const wire = {
    id: 7,
    transaction_id: "t1",
    date: "2026-11-22T00:00:00Z",
    payee: "Payee",
    account: "Assets:Checking",
    commodity: "INR",
    quantity: 2,
    amount: 20,
    status: "cleared",
    tag_recurring: "",
    tag_period: "",
    tax_category: "",
    tax_category_legacy: "",
    transaction_begin_line: 3,
    transaction_end_line: 5,
    file_name: "main.ledger",
    forecast: false,
    note: "",
    transaction_note: "",
    market_amount: 20,
    balance: 20
  };

  const posting = postingsFromWire([wire])[0];
  expect(posting.date.format("YYYY-MM-DD")).toBe("2026-11-22");
  expect(posting).toMatchObject({ ...wire, date: posting.date });
  expect(posting.id).toBe(7);
  expect(posting.amount).toBe(20);
});

test("transactionsFromWire converts transaction and posting dates only", () => {
  const wire = {
    id: "t1",
    date: "2026-11-22T00:00:00Z",
    payee: "Payee",
    postings: [
      {
        id: 7,
        transaction_id: "t1",
        date: "2026-11-22T00:00:00Z",
        payee: "Payee",
        account: "Assets:Checking",
        commodity: "INR",
        quantity: 2,
        amount: 20,
        status: "cleared",
        tag_recurring: "",
        tag_period: "",
        tax_category: "",
        tax_category_legacy: "",
        transaction_begin_line: 3,
        transaction_end_line: 5,
        file_name: "main.ledger",
        forecast: false,
        note: "",
        transaction_note: "",
        market_amount: 20,
        balance: 20
      }
    ],
    tag_recurring: "",
    tag_period: "",
    beginLine: 3,
    endLine: 5,
    fileName: "main.ledger",
    note: ""
  };

  const [transaction] = transactionsFromWire([wire]);
  expect(dayjs.isDayjs(transaction.date)).toBe(true);
  expect(dayjs.isDayjs(transaction.postings[0].date)).toBe(true);
  expect(transaction.fileName).toBe("main.ledger");
  expect(transaction.postings[0].amount).toBe(20);
});
