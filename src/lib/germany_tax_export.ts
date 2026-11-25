import Papa from "papaparse";
import { get } from "svelte/store";
import { t } from "./i18n";
import type { GermanyTaxYear } from "./utils";

export function downloadGermanyTaxYear(taxYear: GermanyTaxYear) {
  const i18n = get(t);
  const rows: Array<Record<string, string | number>> = [
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.grossRealizedGain"),
      Value: taxYear.summary.gross_realized_gain
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.realizedLoss"),
      Value: taxYear.summary.realized_loss
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.netRealizedGain"),
      Value: taxYear.summary.realized_gain
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.partialExemptionAdjustment"),
      Value: taxYear.summary.partial_exemption_amount
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.taxableAmountBeforeAllowance"),
      Value: taxYear.summary.taxable_amount_before_allowance
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.allowanceUsed"),
      Value: taxYear.summary.allowance_used
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.taxableAmount"),
      Value: taxYear.summary.taxable_amount
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.capitalIncomeTax"),
      Value: taxYear.summary.capital_income_tax
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.solidaritySurcharge"),
      Value: taxYear.summary.solidarity_surcharge
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.churchTax"),
      Value: taxYear.summary.church_tax
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.grossTotalTax"),
      Value: taxYear.summary.total_tax
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.withholdingTaxPaid"),
      Value: taxYear.summary.withholding_tax_paid
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.taxCreditUsed"),
      Value: taxYear.summary.tax_credit_used
    },
    {
      Section: i18n("components.export.sectionSummary"),
      Label: i18n("components.export.netTaxDue"),
      Value: taxYear.summary.net_tax_due
    }
  ];

  taxYear.accounts.forEach((account) => {
    rows.push({
      Section: i18n("components.export.sectionAccount"),
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
      Section: i18n("components.export.sectionDiagnostic"),
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
