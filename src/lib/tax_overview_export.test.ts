import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import {
  TAX_OVERVIEW_CSV_COLUMNS,
  taxOverviewCSV,
  taxOverviewCSVRows
} from "./tax_overview_export";
import type { TaxOverview } from "./utils";

const overview: TaxOverview = {
  years: ["2025"],
  selected_year: "2025",
  summary: {
    expense_candidates: 699,
    matched_expense_posting_count: 1,
    capital_income_booked: 118.06,
    capital_income_calculated: 0,
    diagnostic_count: 1
  },
  sections: [
    {
      key: "work_expenses",
      label: "Werbungskosten",
      form: "Anlage N",
      total: 699,
      posting_count: 1,
      categories: [
        {
          key: "work_equipment",
          label: "Arbeitsmittel",
          total: 699,
          postings: [
            {
              transaction_id: "txn-1",
              date: dayjs("2025-09-09"),
              payee: "PayPal | Bürostuhl",
              account: "Expenses:Household:Goods",
              amount: 699,
              note: "",
              file_name: "main.ledger",
              transaction_begin_line: 10,
              source: "transaction",
              category: "work_equipment"
            }
          ]
        }
      ]
    }
  ],
  coverage: { accounts_with_unmatched_postings: 1, unmatched_posting_count: 2, accounts: [] },
  capital_income: {
    realized_gains: 0,
    realized_losses: 0,
    dividends: 118.06,
    interest: 0,
    taxes_booked: 0,
    sales: 0,
    accounts: []
  },
  diagnostics: []
};

describe("tax overview CSV", () => {
  test("keeps the fixed column order", () => {
    expect(TAX_OVERVIEW_CSV_COLUMNS).toEqual([
      "section",
      "category",
      "date",
      "payee",
      "account",
      "amount",
      "source",
      "note",
      "file",
      "line"
    ]);
  });

  test("starts with a UTF-8 BOM", () => {
    expect(taxOverviewCSV(overview).startsWith("\uFEFF")).toBe(true);
  });

  test("keeps raw capital income note and source", () => {
    const rows = taxOverviewCSVRows(overview);
    expect(rows[0].source).toBe("transaction");
    const dividend = rows.find((r) => r.category === "dividends");
    expect(dividend?.section).toBe("capital_income");
    expect(dividend?.note).toBe("aus dem Ledger, kein Steuererklärungswert");
    expect(dividend?.source).toBe("");
  });

  test("keeps negative amounts negative", () => {
    const negative = taxOverviewCSVRows({
      ...overview,
      sections: [
        {
          ...overview.sections[0],
          categories: [
            {
              ...overview.sections[0].categories[0],
              postings: [{ ...overview.sections[0].categories[0].postings[0], amount: -45 }]
            }
          ]
        }
      ]
    });
    expect(Number(negative[0].amount)).toBe(-45);
  });
});
