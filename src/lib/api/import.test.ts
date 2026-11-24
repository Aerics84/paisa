import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function fetchReturning(body: unknown, status = 200) {
  return mock(
    async () => new Response(JSON.stringify(body), { status })
  ) as unknown as typeof fetch;
}

test("parseStatement posts the ingestion request and returns staged rows", async () => {
  let captured: { input: RequestInfo | URL; init?: RequestInit } | undefined;
  globalThis.fetch = mock(async (input, init) => {
    captured = { input, init };
    return new Response(
      JSON.stringify({
        total_parsed: 1,
        auto_categorized: 1,
        possible_duplicates: 0,
        already_imported: false,
        import_id: "abc",
        transactions: [
          {
            index: 0,
            date: "2026-11-01",
            payee: "Coffee Shop",
            memo: "",
            amount: 4.5,
            currency: "INR",
            is_debit: true,
            selected_account: "Expenses:Coffee",
            flag_for_review: false,
            is_duplicate: false,
            duplicate_score: 0
          }
        ]
      })
    );
  }) as unknown as typeof fetch;

  const { parseStatement } = await import("./import");
  const result = await parseStatement(
    { format: "csv", content: "Date,Description\n2026-11-01,Coffee Shop" },
    { background: true }
  );

  expect(captured?.input).toBe("/api/import/statement");
  expect(captured?.init?.method).toBe("POST");
  expect(JSON.parse(String(captured?.init?.body))).toEqual({
    format: "csv",
    content: "Date,Description\n2026-11-01,Coffee Shop"
  });
  expect(result.total_parsed).toBe(1);
  expect(result.transactions[0].date).toBe("2026-11-01");
  expect(result.transactions[0].amount).toBe(4.5);
});

test("parseStatement surfaces the server error message instead of raw JSON", async () => {
  globalThis.fetch = fetchReturning(
    { error: "could not detect valid transaction table header" },
    400
  );

  const { parseStatement } = await import("./import");
  await expect(
    parseStatement({ format: "csv", content: "" }, { background: true })
  ).rejects.toThrow("could not detect valid transaction table header");
});

test("commitImport posts staged rows and returns the commit summary", async () => {
  let captured: RequestInit | undefined;
  globalThis.fetch = mock(async (_input, init) => {
    captured = init;
    return new Response(
      JSON.stringify({
        success: true,
        committed_count: 2,
        target_file: "/ledger/main.ledger",
        manifest_committed: true,
        errors: []
      })
    );
  }) as unknown as typeof fetch;

  const { commitImport } = await import("./import");
  const response = await commitImport(
    {
      base_account: "Assets:Checking:Fixture",
      commodity: "INR",
      group_by_date: true,
      raw_content: "",
      transactions: [
        {
          date: "2026-11-01",
          payee: "Coffee Shop",
          memo: "",
          amount: 4.5,
          currency: "INR",
          is_debit: true,
          selected_account: "Expenses:Coffee",
          tags: []
        }
      ]
    },
    { background: true }
  );

  expect(captured?.method).toBe("POST");
  expect(JSON.parse(String(captured?.body))).toMatchObject({
    base_account: "Assets:Checking:Fixture",
    group_by_date: true,
    transactions: [{ selected_account: "Expenses:Coffee" }]
  });
  expect(response.committed_count).toBe(2);
  expect(response.target_file).toBe("/ledger/main.ledger");
});

test("commitImport keeps a null error list from a clean commit", async () => {
  globalThis.fetch = fetchReturning({
    success: true,
    committed_count: 1,
    target_file: "/ledger/main.ledger",
    manifest_committed: false,
    errors: null
  });

  const { commitImport } = await import("./import");
  const response = await commitImport(
    {
      base_account: "Assets:Checking:Fixture",
      commodity: "INR",
      group_by_date: false,
      raw_content: "",
      transactions: []
    },
    { background: true }
  );

  expect(response.errors).toBeNull();
});

test("getReconciliation maps generated_at to dayjs and keeps the rest of the wire shape", async () => {
  globalThis.fetch = fetchReturning({
    generated_at: "2026-11-22T10:11:12Z",
    status: "ok",
    bank_balances: [],
    duplicate_holdings: [],
    portfolio_totals: [],
    tax_year_completeness: [],
    issues: []
  });

  const { getReconciliation } = await import("./import");
  const report = await getReconciliation();

  expect(dayjs.isDayjs(report.generated_at)).toBe(true);
  expect(report.generated_at.toISOString()).toBe("2026-11-22T10:11:12.000Z");
  expect(report.status).toBe("ok");
  expect(report.bank_balances).toEqual([]);
});
