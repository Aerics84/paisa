import Papa from "papaparse";
import type { GermanyTaxYear } from "./utils";

export function downloadGermanyTaxYear(taxYear: GermanyTaxYear) {
  const rows: Array<Record<string, string | number>> = [
    { Section: "Summary", Label: "Gross Realized Gain", Value: taxYear.summary.gross_realized_gain },
    { Section: "Summary", Label: "Realized Loss", Value: taxYear.summary.realized_loss },
    { Section: "Summary", Label: "Net Realized Gain", Value: taxYear.summary.realized_gain },
    {
      Section: "Summary",
      Label: "Partial Exemption Adjustment",
      Value: taxYear.summary.partial_exemption_amount
    },
    {
      Section: "Summary",
      Label: "Taxable Amount Before Allowance",
      Value: taxYear.summary.taxable_amount_before_allowance
    },
    { Section: "Summary", Label: "Allowance Used", Value: taxYear.summary.allowance_used },
    { Section: "Summary", Label: "Taxable Amount", Value: taxYear.summary.taxable_amount },
    { Section: "Summary", Label: "Capital Income Tax", Value: taxYear.summary.capital_income_tax },
    {
      Section: "Summary",
      Label: "Solidarity Surcharge",
      Value: taxYear.summary.solidarity_surcharge
    },
    { Section: "Summary", Label: "Church Tax", Value: taxYear.summary.church_tax },
    { Section: "Summary", Label: "Gross Total Tax", Value: taxYear.summary.total_tax },
    { Section: "Summary", Label: "Withholding Tax Paid", Value: taxYear.summary.withholding_tax_paid },
    { Section: "Summary", Label: "Tax Credit Used", Value: taxYear.summary.tax_credit_used },
    { Section: "Summary", Label: "Net Tax Due", Value: taxYear.summary.net_tax_due }
  ];

  taxYear.accounts.forEach((account) => {
    rows.push({
      Section: "Account",
      Account: account.account,
      Commodity: account.commodity,
      RealizedGain: account.realized_gain,
      PartialExemptionRate: account.partial_exemption_rate ?? "",
      PartialExemptionAmount: account.partial_exemption_amount,
      TaxableGain: account.taxable_gain,
      SoldUnits: account.units,
      PurchasePrice: account.purchase_price,
      SellPrice: account.sell_price
    });
  });

  taxYear.diagnostics.forEach((diagnostic) => {
    rows.push({
      Section: "Diagnostic",
      Level: diagnostic.level,
      Summary: diagnostic.summary,
      Details: diagnostic.details
    });
  });

  const csv = Papa.unparse(rows);
  const downloadLink = document.createElement("a");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  downloadLink.href = window.URL.createObjectURL(blob);
  downloadLink.download = `paisa-germany-tax-${taxYear.tax_year}.csv`;
  downloadLink.click();
}
