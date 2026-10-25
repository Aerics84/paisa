import type { BalancedPosting } from "./utils";
import Papa from "papaparse";

export function buildExportRows(balancedPostings: BalancedPosting[]): Record<string, unknown>[] {
  return balancedPostings.map((balancedPosting) => {
    return {
      TransactionID: balancedPosting.from.transaction_id,
      Date: balancedPosting.from.date.format("YYYY-MM-DD"),
      Payee: balancedPosting.from.payee,
      FromAccount: balancedPosting.from.account,
      FromQuantity: balancedPosting.from.quantity,
      FromAmount: balancedPosting.from.amount,
      FromCommodity: balancedPosting.from.commodity,
      ToAccount: balancedPosting.to.account,
      ToQuantity: balancedPosting.to.quantity,
      ToAmount: balancedPosting.to.amount,
      ToCommodity: balancedPosting.to.commodity
    };
  });
}

export function download(balancedPostings: BalancedPosting[]) {
  const rows = buildExportRows(balancedPostings);

  const csv = Papa.unparse(rows);
  const downloadLink = document.createElement("a");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  downloadLink.href = window.URL.createObjectURL(blob);
  downloadLink.download = "paisa-transactions.csv";
  downloadLink.click();
}
