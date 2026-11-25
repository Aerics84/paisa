import { afterEach, expect, test } from "bun:test";
import dayjs from "dayjs";
import { mock } from "bun:test";
import {
  getCapitalGains,
  getCapitalIncomeTax,
  getHarvest,
  getScheduleAL,
  getTaxOverview
} from "./tax";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function mockJSON(body: unknown) {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(body))
  ) as unknown as typeof fetch;
}

const posting = (date: string, amount: number) => ({
  id: 1,
  transaction_id: "t1",
  date,
  payee: "Fixture Payee",
  account: "Assets:Equity:EQUITY",
  commodity: "EQUITY",
  quantity: 4,
  amount,
  status: "cleared",
  tag_recurring: "",
  tag_period: "",
  tax_category: "",
  tax_category_legacy: "",
  transaction_begin_line: 1,
  transaction_end_line: 2,
  file_name: "main.ledger",
  forecast: false,
  note: "",
  transaction_note: "",
  market_amount: amount,
  balance: amount
});

const tax = { gain: 200, taxable: 200, short_term: 0, long_term: 25, slab: 0 };

test("getHarvest converts current and purchase dates and normalizes null lists", async () => {
  mockJSON({
    harvestables: {
      "Assets:Equity:EQUITY": {
        account: "Assets:Equity:EQUITY",
        tax_category: "equity",
        total_units: 10,
        harvestable_units: 10,
        unrealized_gain: 500,
        taxable_unrealized_gain: 500,
        current_unit_price: 150,
        current_unit_date: "2026-11-22T00:00:00Z",
        harvest_breakdown: [
          {
            units: 10,
            purchase_date: "2024-01-10T00:00:00Z",
            purchase_price: 1000,
            purchase_unit_price: 100,
            current_price: 1500,
            tax
          }
        ]
      }
    }
  });

  const response = await getHarvest();
  const harvestable = response.harvestables["Assets:Equity:EQUITY"];
  expect(dayjs.isDayjs(harvestable.current_unit_date)).toBe(true);
  expect(harvestable.current_unit_date.format("YYYY-MM-DD")).toBe("2026-11-22");
  expect(dayjs.isDayjs(harvestable.harvest_breakdown[0].purchase_date)).toBe(true);
  expect(harvestable.harvest_breakdown[0].tax.long_term).toBe(25);

  mockJSON({ harvestables: null });
  await expect(getHarvest()).resolves.toEqual({ harvestables: {} });

  mockJSON({
    harvestables: {
      "Assets:Equity:EQUITY": {
        account: "Assets:Equity:EQUITY",
        tax_category: "equity",
        total_units: 0,
        harvestable_units: 0,
        unrealized_gain: 0,
        taxable_unrealized_gain: 0,
        current_unit_price: 0,
        current_unit_date: "2026-11-22T00:00:00Z",
        harvest_breakdown: null
      }
    }
  });
  const empty = await getHarvest();
  expect(empty.harvestables["Assets:Equity:EQUITY"].harvest_breakdown).toEqual([]);
});

test("getCapitalGains converts posting pair dates and keeps totals", async () => {
  mockJSON({
    capital_gains: {
      "Assets:Equity:EQUITY": {
        account: "Assets:Equity:EQUITY",
        tax_category: "equity",
        fy: {
          "2025": {
            units: 4,
            purchase_price: 400,
            sell_price: 600,
            tax,
            posting_pairs: [
              {
                purchase: posting("2024-05-01T00:00:00Z", 400),
                sell: posting("2025-06-01T00:00:00Z", -600),
                tax
              }
            ]
          }
        }
      }
    },
    totals: {
      "2025": {
        units: 4,
        purchase_price: 400,
        sell_price: 600,
        tax,
        posting_pairs: null
      }
    }
  });

  const response = await getCapitalGains();
  const gain = response.capital_gains["Assets:Equity:EQUITY"].fy["2025"];
  expect(dayjs.isDayjs(gain.posting_pairs[0].purchase.date)).toBe(true);
  expect(gain.posting_pairs[0].sell.date.format("YYYY-MM-DD")).toBe("2025-06-01");
  expect(gain.tax.long_term).toBe(25);
  expect(response.totals["2025"].posting_pairs).toEqual([]);

  mockJSON({ capital_gains: null, totals: null });
  await expect(getCapitalGains()).resolves.toEqual({ capital_gains: {}, totals: {} });
});

test("getCapitalIncomeTax converts nested posting dates and normalizes null maps", async () => {
  mockJSON({
    tax_years: {
      "2024": {
        tax_year: "2024",
        settings: {
          annual_allowance: 100,
          capital_income_tax_rate: 0.25,
          solidarity_surcharge_rate: 0.055,
          church_tax_rate: 0
        },
        summary: {
          gross_realized_gain: 120,
          realized_loss: 0,
          realized_gain: 120,
          partial_exemption_amount: 36,
          taxable_amount_before_allowance: 84,
          allowance_used: 84,
          taxable_amount: 0,
          capital_income_tax: 0,
          solidarity_surcharge: 0,
          church_tax: 0,
          total_tax: 0,
          withholding_tax_paid: 30,
          tax_credit_used: 0,
          net_tax_due: 0
        },
        accounts: [
          {
            account: "Assets:Equity:VWCE",
            commodity: "VWCE",
            commodity_type: "mutualfund",
            partial_exemption_rate: 0.3,
            units: 4,
            purchase_price: 400,
            sell_price: 520,
            realized_gain: 120,
            partial_exemption_amount: 36,
            taxable_gain: 84,
            posting_pairs: [
              {
                purchase: posting("2024-01-10T00:00:00Z", 400),
                sell: posting("2024-02-15T00:00:00Z", -520),
                realized_gain: 120,
                partial_exemption_rate: 0.3,
                partial_exemption_amount: 36,
                taxable_gain: 84
              }
            ]
          }
        ],
        diagnostics: null
      }
    }
  });

  const response = await getCapitalIncomeTax();
  const year = response.tax_years["2024"];
  expect(year.summary.net_tax_due).toBe(0);
  expect(year.accounts[0].partial_exemption_rate).toBe(0.3);
  expect(dayjs.isDayjs(year.accounts[0].posting_pairs[0].purchase.date)).toBe(true);
  expect(year.accounts[0].posting_pairs[0].sell.date.format("YYYY-MM-DD")).toBe("2024-02-15");
  expect(year.diagnostics).toEqual([]);

  mockJSON({ tax_years: null });
  await expect(getCapitalIncomeTax()).resolves.toEqual({ tax_years: {} });
});

test("getTaxOverview maps section and payment dates and normalizes null lists", async () => {
  mockJSON({
    years: null,
    selected_year: "2025",
    summary: {
      expense_candidates: 699,
      matched_expense_posting_count: 1,
      capital_income_booked: 118.06,
      capital_income_calculated: 0,
      income_tax_paid: 110,
      income_tax_posting_count: 1,
      diagnostic_count: 0
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
                transaction_id: "chair",
                date: "2025-09-09T00:00:00Z",
                payee: "Chair Shop",
                account: "Expenses:Household:Goods",
                amount: 699,
                note: "desk chair",
                file_name: "main.ledger",
                transaction_begin_line: 10,
                source: "tag",
                category: "work_equipment"
              }
            ]
          }
        ]
      }
    ],
    coverage: { accounts_with_unmatched_postings: 0, unmatched_posting_count: 0, accounts: null },
    capital_income: {
      realized_gains: 0,
      realized_losses: 0,
      dividends: 118.06,
      interest: 0,
      taxes_booked: 12,
      sales: 0,
      accounts: null
    },
    income_tax_payments: {
      total: 110,
      posting_count: 1,
      postings: [
        {
          transaction_id: "tax",
          date: "2025-06-10T00:00:00Z",
          payee: "FINANZAMT",
          account: "Expenses:Tax:IncomeTax",
          amount: 110,
          note: "",
          file_name: "main.ledger",
          transaction_begin_line: 20
        }
      ]
    },
    diagnostics: null
  });

  const response = await getTaxOverview("2025");
  const sectionPosting = response.sections[0].categories[0].postings[0];
  expect(dayjs.isDayjs(sectionPosting.date)).toBe(true);
  expect(sectionPosting.date.format("YYYY-MM-DD")).toBe("2025-09-09");
  expect(sectionPosting.source).toBe("tag");
  expect(dayjs.isDayjs(response.income_tax_payments.postings[0].date)).toBe(true);
  expect(response.years).toEqual([]);
  expect(response.coverage.accounts).toEqual([]);
  expect(response.capital_income.accounts).toEqual([]);
  expect(response.diagnostics).toEqual([]);
});

test("getScheduleAL converts the statement date and normalizes null entries", async () => {
  mockJSON({
    schedule_als: {
      "2026": {
        entries: [
          {
            section: { code: "bank", section: "B (1) (iv) (a)", details: "Financial assets" },
            amount: 5000
          }
        ],
        date: "2026-12-31T00:00:00Z"
      }
    }
  });

  const response = await getScheduleAL();
  expect(dayjs.isDayjs(response.schedule_als["2026"].date)).toBe(true);
  expect(response.schedule_als["2026"].date.format("YYYY-MM-DD")).toBe("2026-12-31");

  mockJSON({ schedule_als: null });
  await expect(getScheduleAL()).resolves.toEqual({ schedule_als: {} });

  mockJSON({ schedule_als: { "2026": { entries: null, date: "2026-12-31T00:00:00Z" } } });
  const empty = await getScheduleAL();
  expect(empty.schedule_als["2026"].entries).toEqual([]);
});
