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
    income_tax_paid: 130,
    income_tax_posting_count: 3,
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
  income_tax_payments: {
    total: 130,
    posting_count: 3,
    postings: [
      {
        transaction_id: "p1",
        date: dayjs("2025-06-10"),
        payee: "FINANZAMT NIDDA",
        account: "Expenses:Tax:IncomeTax",
        amount: 110,
        note: "",
        file_name: "main.ledger",
        transaction_begin_line: 10
      },
      {
        transaction_id: "p2",
        date: dayjs("2025-09-10"),
        payee: "FINANZAMT NIDDA",
        account: "Expenses:Tax:IncomeTax:Advance",
        amount: 50,
        note: "",
        file_name: "main.ledger",
        transaction_begin_line: 20
      },
      {
        transaction_id: "p3",
        date: dayjs("2025-12-01"),
        payee: "FINANZAMT NIDDA",
        account: "Expenses:Tax:IncomeTax",
        amount: -30,
        note: "Erstattung",
        file_name: "main.ledger",
        transaction_begin_line: 30
      }
    ]
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

  test("escapes formula-like fields", () => {
    const csv = taxOverviewCSV({
      ...overview,
      sections: [
        {
          ...overview.sections[0],
          categories: [
            {
              ...overview.sections[0].categories[0],
              postings: [{ ...overview.sections[0].categories[0].postings[0], payee: "=SUM(1,2)" }]
            }
          ]
        }
      ]
    });
    expect(csv).toContain("'=SUM(1,2)");
  });

  test("exports every income tax payment with its refund and no duplicate section", () => {
    const rows = taxOverviewCSVRows(overview);
    const payments = rows.filter((row) => row.section === "income_tax_payments");

    expect(payments).toHaveLength(3);
    expect(payments.map((row) => Number(row.amount))).toEqual([110, 50, -30]);
    expect(payments[0].date).toBe("2025-06-10");
    expect(payments[0].payee).toBe("FINANZAMT NIDDA");
    expect(payments[2].note).toBe("Erstattung");
    expect(
      rows.filter(
        (row) => row.section !== "income_tax_payments" && row.account === "Expenses:Tax:IncomeTax"
      )
    ).toHaveLength(0);
  });
});
