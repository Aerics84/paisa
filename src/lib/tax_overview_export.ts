import Papa from "papaparse";
import dayjs from "dayjs";
import type { TaxOverview } from "./utils";

export const TAX_OVERVIEW_CSV_COLUMNS = [
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
];

export function taxOverviewCSVRows(overview: TaxOverview): Array<Record<string, string | number>> {
  const rows: Array<Record<string, string | number>> = [];
  for (const section of overview.sections) {
    for (const category of section.categories) {
      for (const posting of category.postings) {
        rows.push({
          section: section.key,
          category: category.key,
          date: dayjs(posting.date).format("YYYY-MM-DD"),
          payee: posting.payee,
          account: posting.account,
          amount: posting.amount,
          source: posting.source,
          note: posting.note ?? "",
          file: posting.file_name,
          line: posting.transaction_begin_line
        });
      }
    }
  }
  for (const posting of overview.income_tax_payments.postings) {
    rows.push({
      section: "income_tax_payments",
      category: "income_tax",
      date: dayjs(posting.date).format("YYYY-MM-DD"),
      payee: posting.payee,
      account: posting.account,
      amount: posting.amount,
      source: "ledger",
      note: posting.note ?? "",
      file: posting.file_name,
      line: posting.transaction_begin_line
    });
  }
  const capital = overview.capital_income;
  const capitalRows: Array<[string, number]> = [
    ["realized_gains", capital.realized_gains],
    ["realized_losses", capital.realized_losses],
    ["dividends", capital.dividends],
    ["interest", capital.interest],
    ["taxes_booked", capital.taxes_booked]
  ];
  for (const [category, amount] of capitalRows) {
    rows.push({
      section: "capital_income",
      category,
      date: "",
      payee: "",
      account: "",
      amount,
      source: "",
      note: "aus dem Ledger, kein Steuererklärungswert",
      file: "",
      line: ""
    });
  }
  return rows;
}

export function taxOverviewCSV(overview: TaxOverview): string {
  return (
    "\uFEFF" +
    Papa.unparse(taxOverviewCSVRows(overview), {
      columns: TAX_OVERVIEW_CSV_COLUMNS,
      escapeFormulae: true
    })
  );
}

export function downloadTaxOverview(overview: TaxOverview) {
  const blob = new Blob([taxOverviewCSV(overview)], { type: "text/csv;charset=utf-8;" });
  const link = document.createElement("a");
  link.href = window.URL.createObjectURL(blob);
  link.download = `paisa-steueruebersicht-${overview.selected_year}.csv`;
  link.click();
}
