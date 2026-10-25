import { describe, expect, test } from "bun:test";
import { buildExportRows } from "./export";
import { parseApiDate } from "./utils";

describe("journal calendar dates", () => {
  test("parses an API instant as the journal calendar day", () => {
    const date = parseApiDate("2026-06-06T00:00:00+02:00");
    expect(date.format("YYYY-MM-DD")).toBe("2026-06-06");
    expect(date.hour()).toBe(0);
  });

  test("exports the journal calendar day, not the UTC instant", () => {
    const posting = {
      date: parseApiDate("2026-06-06T00:00:00+02:00"),
      transaction_id: "t1",
      payee: "Test",
      account: "Assets:Bank",
      quantity: 1,
      amount: 1,
      commodity: "EUR"
    };
    const rows = buildExportRows([{ from: posting, to: posting } as any]);
    expect(rows[0].Date).toBe("2026-06-06");
    expect(rows[0].Date).not.toContain("T");
  });
});
