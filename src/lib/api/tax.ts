import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import { postingFromWire, type Posting } from "./ledger";
import type { GermanyTaxBreakdown, Tax as WireTax } from "./taxation.gen";
import type {
  CapitalGain as WireCapitalGain,
  CapitalGainsResponse,
  FYCapitalGain as WireFYCapitalGain,
  GermanyTaxAccount as WireGermanyTaxAccount,
  GermanyTaxDiagnostic as WireGermanyTaxDiagnostic,
  GermanyTaxPostingPair as WireGermanyTaxPostingPair,
  GermanyTaxResponse,
  GermanyTaxYear as WireGermanyTaxYear,
  HarvestBreakdown as WireHarvestBreakdown,
  Harvestable as WireHarvestable,
  HarvestResponse,
  PostingPair as WirePostingPair,
  ScheduleAL as WireScheduleAL,
  ScheduleALEntry as WireScheduleALEntry,
  ScheduleALResponse,
  ScheduleALSection as WireScheduleALSection,
  TaxOverview as WireTaxOverview,
  TaxOverviewCapitalIncome as WireTaxOverviewCapitalIncome,
  TaxOverviewCapitalIncomeAccount as WireTaxOverviewCapitalIncomeAccount,
  TaxOverviewCategory as WireTaxOverviewCategory,
  TaxOverviewCoverage as WireTaxOverviewCoverage,
  TaxOverviewCoverageAccount as WireTaxOverviewCoverageAccount,
  TaxOverviewDiagnostic as WireTaxOverviewDiagnostic,
  TaxOverviewIncomeTaxPayment as WireTaxOverviewIncomeTaxPayment,
  TaxOverviewIncomeTaxPayments as WireTaxOverviewIncomeTaxPayments,
  TaxOverviewPosting as WireTaxOverviewPosting,
  TaxOverviewSection as WireTaxOverviewSection,
  TaxOverviewSummary as WireTaxOverviewSummary
} from "./types.gen";

/** Tax breakdown; it has no dates, so the wire shape is reused. */
export type Tax = WireTax;

/** Harvest breakdown row with its purchase date converted to dayjs. */
export type HarvestBreakdown = Omit<WireHarvestBreakdown, "purchase_date"> & {
  purchase_date: Dayjs;
};

/** Harvestable position with the price date and every breakdown date converted. */
export type Harvestable = Omit<WireHarvestable, "current_unit_date" | "harvest_breakdown"> & {
  current_unit_date: Dayjs;
  harvest_breakdown: HarvestBreakdown[];
};

/** FIFO pair with both postings converted to ledger postings. */
export type PostingPair = Omit<WirePostingPair, "purchase" | "sell"> & {
  purchase: Posting;
  sell: Posting;
};

/** Financial-year capital gain with its posting pairs converted. */
export type FYCapitalGain = Omit<WireFYCapitalGain, "posting_pairs"> & {
  posting_pairs: PostingPair[];
};

/** Account-level capital gain with every financial year converted. */
export type CapitalGain = Omit<WireCapitalGain, "fy"> & {
  fy: Record<string, FYCapitalGain>;
};

/** Germany tax pair with both postings converted to ledger postings. */
export type GermanyTaxPostingPair = Omit<WireGermanyTaxPostingPair, "purchase" | "sell"> & {
  purchase: Posting;
  sell: Posting;
};

/** Germany tax account with its posting pairs converted. */
export type GermanyTaxAccount = Omit<WireGermanyTaxAccount, "posting_pairs"> & {
  posting_pairs: GermanyTaxPostingPair[];
};

export type GermanyTaxSummary = GermanyTaxBreakdown;
export type GermanyTaxDiagnostic = WireGermanyTaxDiagnostic;

/** Germany tax year with its accounts converted. */
export type GermanyTaxYear = Omit<WireGermanyTaxYear, "accounts" | "diagnostics"> & {
  accounts: GermanyTaxAccount[];
  diagnostics: GermanyTaxDiagnostic[];
};

/** Resolution source of a tax overview posting. */
export type TaxOverviewPostingSource = "tag" | "transaction" | "account";

/** Tax overview expense posting with its date converted. */
export type TaxOverviewPosting = Omit<WireTaxOverviewPosting, "date" | "source"> & {
  date: Dayjs;
  source: TaxOverviewPostingSource;
};

export type TaxOverviewCategory = Omit<WireTaxOverviewCategory, "postings"> & {
  postings: TaxOverviewPosting[];
};

export type TaxOverviewSection = Omit<WireTaxOverviewSection, "categories"> & {
  categories: TaxOverviewCategory[];
};

export type TaxOverviewSummary = WireTaxOverviewSummary;
export type TaxOverviewCoverage = WireTaxOverviewCoverage;
export type TaxOverviewCoverageAccount = WireTaxOverviewCoverageAccount;
export type TaxOverviewCapitalIncome = WireTaxOverviewCapitalIncome;
export type TaxOverviewCapitalIncomeAccount = WireTaxOverviewCapitalIncomeAccount;
export type TaxOverviewDiagnostic = WireTaxOverviewDiagnostic;

export type TaxOverviewIncomeTaxPayment = Omit<WireTaxOverviewIncomeTaxPayment, "date"> & {
  date: Dayjs;
};

export type TaxOverviewIncomeTaxPayments = Omit<WireTaxOverviewIncomeTaxPayments, "postings"> & {
  postings: TaxOverviewIncomeTaxPayment[];
};

/** Tax overview with every date converted and every list normalized. */
export type TaxOverview = Omit<
  WireTaxOverview,
  "sections" | "coverage" | "capital_income" | "income_tax_payments" | "diagnostics"
> & {
  sections: TaxOverviewSection[];
  coverage: TaxOverviewCoverage;
  capital_income: TaxOverviewCapitalIncome;
  income_tax_payments: TaxOverviewIncomeTaxPayments;
  diagnostics: TaxOverviewDiagnostic[];
};

/** Schedule AL section catalog entry; it has no dates. */
export type ScheduleALSection = WireScheduleALSection;
/** Schedule AL amount row; it has no dates. */
export type ScheduleALEntry = WireScheduleALEntry;

/** Schedule AL statement with its date converted. */
export type ScheduleAL = Omit<WireScheduleAL, "date" | "entries"> & {
  date: Dayjs;
  entries: ScheduleALEntry[];
};

function harvestBreakdownFromWire(wire: WireHarvestBreakdown): HarvestBreakdown {
  return { ...wire, purchase_date: parseDate(wire.purchase_date) };
}

function harvestableFromWire(wire: WireHarvestable): Harvestable {
  return {
    ...wire,
    current_unit_date: parseDate(wire.current_unit_date),
    harvest_breakdown: (wire.harvest_breakdown ?? []).map(harvestBreakdownFromWire)
  };
}

function postingPairFromWire(wire: WirePostingPair): PostingPair {
  return {
    ...wire,
    purchase: postingFromWire(wire.purchase),
    sell: postingFromWire(wire.sell)
  };
}

function fyCapitalGainFromWire(wire: WireFYCapitalGain): FYCapitalGain {
  return {
    ...wire,
    posting_pairs: (wire.posting_pairs ?? []).map(postingPairFromWire)
  };
}

function capitalGainFromWire(wire: WireCapitalGain): CapitalGain {
  return {
    ...wire,
    fy: Object.fromEntries(
      Object.entries(wire.fy ?? {}).map(([fy, gain]) => [fy, fyCapitalGainFromWire(gain)])
    )
  };
}

function germanyTaxPostingPairFromWire(wire: WireGermanyTaxPostingPair): GermanyTaxPostingPair {
  return {
    ...wire,
    purchase: postingFromWire(wire.purchase),
    sell: postingFromWire(wire.sell)
  };
}

function germanyTaxAccountFromWire(wire: WireGermanyTaxAccount): GermanyTaxAccount {
  return {
    ...wire,
    posting_pairs: (wire.posting_pairs ?? []).map(germanyTaxPostingPairFromWire)
  };
}

function germanyTaxYearFromWire(wire: WireGermanyTaxYear): GermanyTaxYear {
  return {
    ...wire,
    accounts: (wire.accounts ?? []).map(germanyTaxAccountFromWire),
    diagnostics: wire.diagnostics ?? []
  };
}

function taxOverviewPostingFromWire(wire: WireTaxOverviewPosting): TaxOverviewPosting {
  return {
    ...wire,
    date: parseDate(wire.date),
    source: wire.source as TaxOverviewPostingSource
  };
}

function taxOverviewCategoryFromWire(wire: WireTaxOverviewCategory): TaxOverviewCategory {
  return {
    ...wire,
    postings: (wire.postings ?? []).map(taxOverviewPostingFromWire)
  };
}

function taxOverviewSectionFromWire(wire: WireTaxOverviewSection): TaxOverviewSection {
  return {
    ...wire,
    categories: (wire.categories ?? []).map(taxOverviewCategoryFromWire)
  };
}

function taxOverviewIncomeTaxPaymentFromWire(
  wire: WireTaxOverviewIncomeTaxPayment
): TaxOverviewIncomeTaxPayment {
  return { ...wire, date: parseDate(wire.date) };
}

function taxOverviewFromWire(wire: WireTaxOverview): TaxOverview {
  return {
    ...wire,
    years: wire.years ?? [],
    sections: (wire.sections ?? []).map(taxOverviewSectionFromWire),
    coverage: { ...wire.coverage, accounts: wire.coverage?.accounts ?? [] },
    capital_income: { ...wire.capital_income, accounts: wire.capital_income?.accounts ?? [] },
    income_tax_payments: {
      ...wire.income_tax_payments,
      postings: (wire.income_tax_payments?.postings ?? []).map(taxOverviewIncomeTaxPaymentFromWire)
    },
    diagnostics: wire.diagnostics ?? []
  };
}

function scheduleALFromWire(wire: WireScheduleAL): ScheduleAL {
  return {
    ...wire,
    date: parseDate(wire.date),
    entries: wire.entries ?? []
  };
}

export async function getHarvest(): Promise<{ harvestables: Record<string, Harvestable> }> {
  const wire = await request<HarvestResponse>("/api/harvest");
  return {
    harvestables: Object.fromEntries(
      Object.entries(wire.harvestables ?? {}).map(([account, harvestable]) => [
        account,
        harvestableFromWire(harvestable)
      ])
    )
  };
}

export async function getCapitalGains(): Promise<{
  capital_gains: Record<string, CapitalGain>;
  totals: Record<string, FYCapitalGain>;
}> {
  const wire = await request<CapitalGainsResponse>("/api/capital_gains");
  return {
    capital_gains: Object.fromEntries(
      Object.entries(wire.capital_gains ?? {}).map(([account, gain]) => [
        account,
        capitalGainFromWire(gain)
      ])
    ),
    totals: Object.fromEntries(
      Object.entries(wire.totals ?? {}).map(([fy, gain]) => [fy, fyCapitalGainFromWire(gain)])
    )
  };
}

export async function getCapitalIncomeTax(): Promise<{
  tax_years: Record<string, GermanyTaxYear>;
}> {
  const wire = await request<GermanyTaxResponse>("/api/capital_income_tax");
  return {
    tax_years: Object.fromEntries(
      Object.entries(wire.tax_years ?? {}).map(([year, taxYear]) => [
        year,
        germanyTaxYearFromWire(taxYear)
      ])
    )
  };
}

export async function getTaxOverview(year?: string): Promise<TaxOverview> {
  const route = year ? `/api/tax_overview?year=${encodeURIComponent(year)}` : "/api/tax_overview";
  const wire = await request<WireTaxOverview>(route);
  return taxOverviewFromWire(wire);
}

export async function getScheduleAL(): Promise<{ schedule_als: Record<string, ScheduleAL> }> {
  const wire = await request<ScheduleALResponse>("/api/schedule_al");
  return {
    schedule_als: Object.fromEntries(
      Object.entries(wire.schedule_als ?? {}).map(([fy, schedule]) => [
        fy,
        scheduleALFromWire(schedule)
      ])
    )
  };
}
