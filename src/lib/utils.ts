import dayjs from "dayjs";
import "dayjs/locale/de";
import updateLocale from "dayjs/plugin/updateLocale";
dayjs.extend(updateLocale);
import _ from "lodash";
import * as d3 from "d3";
import type { JSONSchema7 } from "json-schema";
import { get } from "svelte/store";
import { obscure, readonly } from "../persisted_store";
import chroma from "chroma-js";
import { iconGlyph, preloadConfiguredIconFonts } from "./icon";
import { resolveLocale, setLocale, type Locale } from "./i18n";
import { parseDate, rawRequest, reviveDates, type RequestOptions } from "./api/client";
import type { BalancedPosting, Posting, Transaction } from "./api/ledger";
import type {
  AccountGain,
  Aggregate,
  AllocationTarget,
  AssetBreakdown,
  CommodityBreakdown,
  CurrencyAccountGain,
  CurrencyNetworth,
  Gain,
  InvestmentYearlyCard,
  Networth,
  PortfolioAggregate,
  PortfolioAllocation,
  PostingReturn,
  RebalanceItem,
  RebalancePlan
} from "./api/assets";
import type {
  AccountBudget,
  Budget,
  CashFlow,
  Graph,
  Income,
  IncomeStatement,
  IncomeTax,
  IncomeYearlyCard,
  Link,
  Node,
  TransactionSequenceInput
} from "./api/cashflow";

export type { BalancedPosting, Posting, Transaction };
export type {
  AccountBudget,
  Budget,
  CashFlow,
  Graph,
  Income,
  IncomeStatement,
  IncomeTax,
  IncomeYearlyCard,
  Link,
  Node,
  TransactionSequenceInput
};
export type {
  AccountGain,
  Aggregate,
  AllocationTarget,
  AssetBreakdown,
  CommodityBreakdown,
  CurrencyAccountGain,
  CurrencyNetworth,
  Gain,
  InvestmentYearlyCard,
  Networth,
  PortfolioAggregate,
  PortfolioAllocation,
  PostingReturn,
  RebalanceItem,
  RebalancePlan
};
export type { Interest, InterestOverview, LiabilityBreakdown } from "./api/liabilities";
export type { CreditCardBill, CreditCardSummary } from "./api/credit_cards";
export type {
  CapitalGain,
  FYCapitalGain,
  GermanyTaxAccount,
  GermanyTaxDiagnostic,
  GermanyTaxPostingPair,
  GermanyTaxSummary,
  GermanyTaxYear,
  HarvestBreakdown,
  Harvestable,
  PostingPair,
  ScheduleAL,
  ScheduleALEntry,
  ScheduleALSection,
  Tax,
  TaxOverview,
  TaxOverviewCapitalIncome,
  TaxOverviewCapitalIncomeAccount,
  TaxOverviewCategory,
  TaxOverviewCoverage,
  TaxOverviewCoverageAccount,
  TaxOverviewDiagnostic,
  TaxOverviewIncomeTaxPayment,
  TaxOverviewIncomeTaxPayments,
  TaxOverviewPosting,
  TaxOverviewPostingSource,
  TaxOverviewSection,
  TaxOverviewSummary
} from "./api/tax";
export type {
  GoalContributionMonth,
  GoalIntelligenceData,
  GoalSummary,
  RetirementGoalProgress,
  SavingsGoalProgress
} from "./api/goals";
import type { LedgerFile } from "./api/editor";
import type { SheetFile } from "./api/sheets";
export type { EditorFileInput, FileError as LedgerFileError, LedgerFile } from "./api/editor";
export type { SheetFile } from "./api/sheets";

const DEFAULT_USER_CONFIG: UserConfig = {
  regional_profile: "germany-eu",
  tax_regime: "germany",
  germany_tax: {
    annual_allowance: 1000,
    capital_income_tax_rate: 0.25,
    solidarity_surcharge_rate: 0.055,
    church_tax_rate: 0
  },
  default_currency: "EUR",
  readonly: false,
  locale: "de-DE",
  time_zone: "Europe/Berlin",
  journal_path: "",
  display_precision: 2,
  db_path: "",
  financial_year_starting_month: 1,
  amount_alignment_column: 52,
  week_starting_day: 1,
  goals: {},
  accounts: []
};

function getUserConfig(): UserConfig {
  if (typeof USER_CONFIG !== "undefined" && USER_CONFIG) {
    return USER_CONFIG;
  }

  return DEFAULT_USER_CONFIG;
}

export interface AutoCompleteItem {
  label: string;
  id: string;
  [key: string]: unknown;
}

export interface AutoCompleteField {
  id: string;
  label: string;
  help: string;
  inputType: string;
}

export interface PriceProvider {
  code: string;
  fields: AutoCompleteField[];
  label: string;
  description: string;
}

export interface TransactionSchedule {
  actual: dayjs.Dayjs;
  scheduled: dayjs.Dayjs;
  transaction: Transaction;
  key: string;
  amount: number;
}

export type TransactionSequence = TransactionSequenceInput & {
  // computed
  schedules: TransactionSchedule[];
  pastSchedules: TransactionSchedule[];
  futureSchedules: TransactionSchedule[];
  schedulesByMonth: Record<string, TransactionSchedule[]>;
};

export interface Price {
  id: string;
  date: dayjs.Dayjs;
  commodity_type: string;
  commodity_id: string;
  commodity_name: string;
  value: number;
}

export interface AccountTfIdf {
  tf_idf: Record<string, Record<string, number>>;
  index: {
    docs: Record<string, Record<string, number>>;
    tokens: Record<string, Record<string, number>>;
  };
}

export interface Issue {
  level: string;
  summary: string;
  description: string;
  details: string;
}

export interface Point {
  date: dayjs.Dayjs;
  value: number;
}

export interface Forecast {
  date: dayjs.Dayjs;
  value: number;
  error: number;
}

export interface ExpensePace {
  period: string;
  projectedExpenses: number;
  overBudget?: number | null;
}

export interface Legend {
  shape: "line" | "square" | "texture";
  color: string;
  label: string;
  texture?: any;
  onClick?: (legend: Legend) => void;

  selected?: boolean;
}

export interface Directory {
  type: "directory";
  name: string;
  children: Array<Directory | LedgerFile | SheetFile>;
}

export function buildDirectoryTree<T extends LedgerFile | SheetFile>(files: T[]) {
  const root: Directory = {
    type: "directory",
    name: "",
    children: []
  };

  for (const file of _.sortBy(files, (f) => f.name)) {
    const parts = file.name.split("/");
    let current = root;
    for (const part of _.dropRight(parts, 1)) {
      let found = current.children.find((c) => c.name === part);
      if (!found) {
        found = {
          type: "directory",
          name: part,
          children: []
        };
        current.children.push(found);
      }
      current = found as Directory;
    }
    current.children.push(file);
  }

  return root.children;
}

export interface SheetFileError {
  line_from: number;
  line_to: number;
  error: string;
  message: string;
}

export interface Log {
  time: dayjs.Dayjs;
  level: string;
  msg: string;
}

/** Mirrors internal/pricesync.Status. */
export interface PriceUpdateStatus {
  enabled: boolean;
  managed: boolean;
  running: boolean;
  job_id: number;
  last_job_id: number;
  last_job_success: boolean;
  last_job_partial: boolean;
  stale: boolean;
  should_update: boolean;
  last_attempt_at: string | null;
  last_success_at: string | null;
  last_partial_success_at: string | null;
  last_error: string;
  last_warning: string;
  failed_commodities: string[];
  next_run_at: string | null;
}

/** Mirrors the /api/sync response (both foreground and background variants). */
export interface SyncResponse {
  success: boolean;
  started?: boolean;
  status?: PriceUpdateStatus;
  message?: string;
  partial?: boolean;
  failed_commodities?: string[];
  warning?: string;
}

export interface SheetLineResult {
  line: number;
  result: string;
  error: boolean;
  underline?: boolean;
  bold?: boolean;
  align?: "left" | "right";
}

export function parseApiDate(value: string) {
  return parseDate(value);
}

export function ajax(
  route: "/api/config"
): Promise<{ config: UserConfig; schema: JSONSchema7; now: dayjs.Dayjs; accounts: string[] }>;
export function ajax(route: "/api/diagnosis"): Promise<{ issues: Issue[] }>;
export function ajax(route: "/api/logs"): Promise<{ logs: Log[] }>;
export function ajax(route: "/api/price"): Promise<{ prices: Record<string, Price[]> }>;

export function ajax(route: "/api/account/tf_idf"): Promise<AccountTfIdf>;

export function ajax(
  route: "/api/price/delete",
  options?: RequestOptions
): Promise<{ success: boolean; message: string }>;

export function ajax(
  route: "/api/sync/status",
  options?: RequestOptions
): Promise<PriceUpdateStatus>;
export function ajax(route: "/api/sync", options?: RequestOptions): Promise<SyncResponse>;
export function ajax(
  route: "/api/price/providers",
  options?: RequestOptions
): Promise<{ providers: PriceProvider[] }>;

export function ajax(
  route: "/api/price/providers/delete/:provider",
  options?: RequestOptions,
  params?: Record<string, string>
): Promise<{ success?: boolean; message?: string; error?: string }>;

export function ajax(
  route: "/api/price/autocomplete",
  options?: RequestOptions
): Promise<{ completions: AutoCompleteItem[] }>;
export function ajax(route: "/api/init", options?: RequestOptions): Promise<any>;

export function ajax(
  route: "/api/config",
  options?: RequestOptions
): Promise<{ success: boolean; error?: string }>;

export function ajax(route: "/api/version"): Promise<{ version: string }>;

export function ajax(route: "/api/ping"): Promise<{ success: boolean; error?: string }>;

export function ajax<T = any>(
  route: string,
  options?: RequestOptions,
  params?: Record<string, string>
): Promise<T>;

export async function ajax(
  route: string,
  options?: RequestOptions,
  params?: Record<string, string>
) {
  const { body } = await rawRequest(route, options, params);
  return reviveDates(JSON.parse(body));
}

export async function login(username: string, password: string) {
  return await ajax<{ success: boolean; username?: string; error?: string }>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password })
  });
}

export async function authStatus(): Promise<{
  auth_enabled: boolean;
  authenticated: boolean;
  username: string;
  locale: string;
}> {
  return await ajax("/api/auth/status");
}

export async function logout() {
  try {
    await fetch("/api/auth/logout", {
      method: "POST",
      credentials: "same-origin"
    });
  } catch {
    // Clearing the session is best effort: a network failure must not trap the
    // user in the logged-in view. The cookie is HttpOnly, so the client cannot
    // remove it directly.
  }
}

export async function hashPassword(password: string): Promise<string> {
  const response = await ajax<{ hash?: unknown; error?: string }>("/api/auth/hash", {
    method: "POST",
    body: JSON.stringify({ password })
  });

  const hash = response?.hash;
  if (typeof hash !== "string" || !hash.startsWith("argon2id$")) {
    const message =
      typeof response?.error === "string" && response.error
        ? response.error
        : "Could not hash password";
    throw new Error(message);
  }

  return hash;
}

function normalize(value: number) {
  if (get(obscure)) {
    value = 0;
  }

  // minus 0
  if (1 / value === -Infinity) {
    value = 0;
  }

  if (!Number.isFinite(value)) {
    value = 0;
  }

  return value;
}

/**
 * Apply the UI locale derived from `config.locale` to the message store and
 * dayjs, returning the active locale.
 *
 * Only `config.locale` is consulted: a Germany tax profile may still use an
 * English UI, so regional profile and tax regime never pick the language.
 */
export function applyLocale(configLocale: string): Locale {
  const active = setLocale(configLocale);
  dayjs.locale(active);
  return active;
}

/**
 * Locale for `Intl` number/currency/percentage formatting.
 *
 * The configured BCP-47 tag is used as-is so regional conventions such as
 * Indian digit grouping (`en-IN`) survive. Only a missing or blank locale falls
 * back to the resolved UI locale, which also keeps `Intl` from throwing on an
 * empty language tag.
 */
function formattingLocale(config: UserConfig): string {
  const configured = typeof config.locale === "string" ? config.locale.trim() : "";
  return configured || resolveLocale(config.locale);
}

export function configUpdated() {
  const config = getUserConfig();
  readonly.set(!!config.readonly);
  const activeLocale = applyLocale(config.locale);
  if (config.week_starting_day != null) {
    dayjs.updateLocale(activeLocale, {
      weekStart: config.week_starting_day
    });
  }
  // A config change can introduce a lazy icon font; load it so the existing
  // refresh/remount path renders the glyph instead of the replacement glyph.
  void preloadConfiguredIconFonts(config);
}

export function supportsTaxFeatures(config: UserConfig = getUserConfig()) {
  return config.tax_regime === "india" || config.tax_regime === "germany";
}

export function supportsIndiaTaxFeatures(config: UserConfig = getUserConfig()) {
  return config.tax_regime === "india";
}

export function supportsGermanyTaxFeatures(config: UserConfig = getUserConfig()) {
  return config.tax_regime === "germany";
}

export function supportsScheduleAL(config: UserConfig = getUserConfig()) {
  return config.tax_regime === "india";
}

export function supportsPortfolioAnalysis(config: UserConfig = getUserConfig()) {
  return config.regional_profile === "india";
}

export function setNow(value: dayjs.Dayjs) {
  if (value) {
    globalThis.__now = value;
  }
}

export function now(): dayjs.Dayjs {
  if (globalThis.__now) {
    return globalThis.__now;
  }
  return dayjs();
}

function unicodeMinus(value: string) {
  return value.replace(/^-/, "\u2212");
}

export function formatCurrency(value: number, precision: number = null, locale: string = null) {
  const config = getUserConfig();
  value = normalize(value);

  if (precision == null) {
    precision = config.display_precision;
  }

  return unicodeMinus(
    value.toLocaleString(locale ?? formattingLocale(config), {
      minimumFractionDigits: precision,
      maximumFractionDigits: precision
    })
  );
}

export function formatCurrencyCrude(value: number) {
  return formatCurrencyCrudeWithPrecision(value, -1);
}

export function formatCurrencyCrudeWithLocale(value: number, locale: string = null) {
  return formatCurrencyCrudeWithPrecision(value, -1, locale);
}

export function formatCurrencyCrudeWithPrecision(
  value: number,
  precision: number,
  locale: string = null
) {
  const config = getUserConfig();
  value = normalize(value);

  const options: Intl.NumberFormatOptions = {
    notation: "compact"
  };

  if (precision < 0) {
    options.maximumFractionDigits = 2;
  } else {
    options.maximumFractionDigits = precision;
    options.minimumFractionDigits = precision;
  }

  return unicodeMinus(value.toLocaleString(locale ?? formattingLocale(config), options));
}

export function formatCurrencyWithSymbol(value: number, precision: number = null) {
  const config = getUserConfig();
  value = normalize(value);

  if (precision == null) {
    precision = config.display_precision;
  }

  if (!config.default_currency) {
    return formatCurrency(value, precision);
  }

  try {
    return unicodeMinus(
      value.toLocaleString(formattingLocale(config), {
        style: "currency",
        currency: config.default_currency,
        minimumFractionDigits: precision,
        maximumFractionDigits: precision
      })
    );
  } catch {
    return formatCurrency(value, precision);
  }
}

export function formatCurrencyCrudeWithSymbol(value: number, precision: number = -1) {
  const config = getUserConfig();
  value = normalize(value);

  if (!config.default_currency) {
    return formatCurrencyCrudeWithPrecision(value, precision);
  }

  const options: Intl.NumberFormatOptions = {
    style: "currency",
    currency: config.default_currency,
    notation: "compact"
  };

  if (precision < 0) {
    options.maximumFractionDigits = 2;
  } else {
    options.maximumFractionDigits = precision;
    options.minimumFractionDigits = precision;
  }

  try {
    return unicodeMinus(value.toLocaleString(formattingLocale(config), options));
  } catch {
    return formatCurrencyCrudeWithPrecision(value, precision);
  }
}

export function formatFloat(value: number, precision = 2) {
  const config = getUserConfig();
  value = normalize(value);

  return unicodeMinus(
    value.toLocaleString(formattingLocale(config), {
      minimumFractionDigits: precision,
      maximumFractionDigits: precision
    })
  );
}

export function formatFloatUptoPrecision(value: number, precision = 2) {
  const config = getUserConfig();
  value = normalize(value);

  return unicodeMinus(
    value.toLocaleString(formattingLocale(config), {
      maximumFractionDigits: precision
    })
  );
}

export function formatPercentage(value: number, precision = 0) {
  const config = getUserConfig();
  value = normalize(value);

  return unicodeMinus(
    value.toLocaleString(formattingLocale(config), {
      style: "percent",
      minimumFractionDigits: precision
    })
  );
}

export function formatFixedWidthFloat(value: number, width: number, precision = 2) {
  const config = getUserConfig();
  value = normalize(value);

  const formatted = unicodeMinus(
    value.toLocaleString(formattingLocale(config), {
      minimumFractionDigits: precision,
      maximumFractionDigits: precision
    })
  );

  if (formatted.length < width) {
    return formatted.padStart(width, " ");
  }
  return formatted;
}

export function forEachMonth(
  start: dayjs.Dayjs,
  end: dayjs.Dayjs,
  cb: (current: dayjs.Dayjs) => any
) {
  let current = start.startOf("month");
  while (current.isSameOrBefore(end, "month")) {
    cb(current);
    current = current.add(1, "month");
  }
}

export function forEachYear(
  start: dayjs.Dayjs,
  end: dayjs.Dayjs,
  cb: (current: dayjs.Dayjs) => any
) {
  let current = start;
  while (current.isSameOrBefore(end, "year")) {
    cb(current);
    current = current.add(1, "year");
  }
}

export function forEachFinancialYear(
  start: dayjs.Dayjs,
  end: dayjs.Dayjs,
  cb?: (current: dayjs.Dayjs) => any
) {
  let current = beginningOfFinancialYear(start);
  const years: dayjs.Dayjs[] = [];
  while (current.isSameOrBefore(end, "month")) {
    if (cb) {
      cb(current);
    }
    years.push(current);
    current = current.add(1, "year");
  }
  return years;
}

function beginningOfFinancialYear(date: dayjs.Dayjs) {
  const config = getUserConfig();
  date = date.startOf("month");
  if (date.month() + 1 < config.financial_year_starting_month) {
    return date
      .add(-1, "year")
      .add(config.financial_year_starting_month - date.month() - 1, "month");
  } else {
    return date.add(-(date.month() + 1 - config.financial_year_starting_month), "month");
  }
}

export function firstName(account: string) {
  return _.first(account.split(":"));
}

export function lastName(account: string) {
  return _.last(account.split(":"));
}

export function secondName(account: string) {
  return account.split(":")[1];
}

export function firstNames(account: string, n: number) {
  return _.take(account.split(":"), n).join(":");
}

export function restName(account: string) {
  return _.drop(account.split(":")).join(":");
}

export function parentName(account: string) {
  return _.dropRight(account.split(":"), 1).join(":");
}

export function depth(account: string) {
  return account.split(":").length;
}

export function skipTicks<Domain>(
  minWidth: number,
  scale: d3.AxisScale<Domain>,
  cb: (data: d3.AxisDomain, index: number) => string,
  points?: number
) {
  const range = scale.range();
  const width = Math.abs(range[1] - range[0]);
  const s = scale as any;
  points = points || (s.ticks ? s.ticks().length : s.domain().length);
  return function (data: d3.AxisDomain, index: number) {
    let skip = Math.round((minWidth * points) / width);
    skip = Math.max(1, skip);

    return index % skip === 0 ? cb(data, index) : null;
  };
}

export function rainbowScale(keys: string[]) {
  const x = d3
    .scaleLinear()
    .domain([0, _.size(keys) - 1])
    .range([0, 0.9]);
  return d3.scaleOrdinal(_.map(keys, (_value, i) => d3.interpolateRainbow(x(i)))).domain(keys);
}

export function darkenOrLighten(backgroundColor: string, intensity = 2) {
  const color = d3.rgb(backgroundColor);
  // http://www.w3.org/TR/AERT#color-contrast
  const brightness = (color.r * 299 + color.g * 587 + color.b) / 1000;
  if (brightness > 125) {
    return chroma(backgroundColor).darken(intensity).hex();
  }
  return chroma(backgroundColor).brighten(intensity).hex();
}

export { tooltip } from "./tooltip";

export function isMobile() {
  return window.innerWidth < 769;
}

// Width available to children: clientWidth includes the element's own padding,
// so charts that size themselves to it overflow by that padding.
export function contentWidth(element: HTMLElement) {
  const style = getComputedStyle(element);
  return (
    element.clientWidth -
    (parseFloat(style.paddingLeft) || 0) -
    (parseFloat(style.paddingRight) || 0)
  );
}

export function rem(value: number) {
  if (isMobile()) {
    return value * 0.857;
  } else {
    return value;
  }
}

export function financialYear(date: dayjs.Dayjs) {
  const config = getUserConfig();
  if (config.financial_year_starting_month == 1) {
    return date.year().toString();
  }

  if (date.month() < config.financial_year_starting_month - 1) {
    return `${date.year() - 1} - ${(date.year() % 100).toLocaleString("en-US", {
      minimumIntegerDigits: 2
    })}`;
  } else {
    return `${date.year()} - ${((date.year() + 1) % 100).toLocaleString("en-US", {
      minimumIntegerDigits: 2
    })}`;
  }
}

export function helpUrl(section: string) {
  return `https://paisa.fyi/reference/${section}`;
}

export function postingUrl(posting: Posting) {
  return `/ledger/editor/${encodeURIComponent(posting.file_name)}#${
    posting.transaction_begin_line
  }`;
}

const storageKey = "theme-preference";

export function getColorPreference() {
  if (localStorage.getItem(storageKey)) {
    return localStorage.getItem(storageKey);
  } else {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
}

export function darkLightColor(dark: string, light: string) {
  return getColorPreference() == "dark" ? dark : light;
}

export function setColorPreference(theme: string) {
  localStorage.setItem(storageKey, theme);
}

export function isZero(n: number) {
  return n < 0.0001 && n > -0.0001;
}

export function monthDays(month: string) {
  const monthStart = dayjs(month, "YYYY-MM");
  const monthEnd = monthStart.endOf("month");
  const weekStart = monthStart.startOf("week");
  const weekEnd = monthEnd.endOf("week");

  const days: dayjs.Dayjs[] = [];
  let d = weekStart;
  while (d.isSameOrBefore(weekEnd)) {
    days.push(d);
    d = d.add(1, "day");
  }
  return { days, monthStart, monthEnd };
}

export function prefixMinutesSeconds(cronExpression: string) {
  return cronExpression
    .split("|")
    .map((cron) => "0 0 " + cron)
    .join("|");
}

export function svgTruncate(width: number) {
  return function () {
    const self = d3.select(this);
    let textLength = self.node().getComputedTextLength(),
      text = self.text();
    while (textLength > width && text.length > 0) {
      text = text.slice(0, -1);
      self.text(text + "...");
      textLength = self.node().getComputedTextLength();
    }
  };
}

export function sumPostings(postings: Posting[]) {
  return postings.reduce(
    (sum, p) => (p.account.startsWith("Income:CapitalGains") ? sum + -p.amount : sum + p.amount),
    0
  );
}

export function transactionTotal(transaction: Transaction) {
  return _.sumBy(transaction.postings, (t) => _.max([0, t.amount]));
}

export function formatTextAsHtml(text: string) {
  return `<p>${_.trim(text).replaceAll("\n", "<br />")}</p>`;
}

export function groupSumBy(postings: Posting[], groupBy: _.ValueIteratee<Posting>) {
  return _.chain(postings)
    .groupBy(groupBy)
    .mapValues((ps) => _.sumBy(ps, (p) => p.amount))
    .value();
}

export function asTransaction(p: Posting): Transaction {
  return {
    id: p.transaction_id,
    date: p.date,
    payee: p.payee,
    beginLine: p.transaction_begin_line,
    endLine: p.transaction_end_line,
    fileName: p.file_name,
    note: p.transaction_note,
    tag_recurring: p.tag_recurring,
    tag_period: p.tag_period,
    postings: [p]
  };
}

export function svgUrl(identifier: string) {
  return `url(${new URL("#" + identifier, window.location.toString())})`;
}

export function dueDateIcon(dueDate: dayjs.Dayjs, clearedDate: dayjs.Dayjs) {
  let icon = "fa-circle-check";
  let glyph = iconGlyph("fa6-solid:circle-check");
  let color: string;
  let svgColor: string;

  if (!clearedDate) {
    if (dueDate.isBefore(now(), "day")) {
      color = "has-text-danger";
      icon = "fa-exclamation-triangle";
      glyph = iconGlyph("fa6-solid:triangle-exclamation");
      svgColor = "svg-text-danger";
    } else {
      color = "has-text-grey";
      svgColor = "svg-text-grey";
    }
  } else {
    if (clearedDate.isSameOrBefore(dueDate, "day")) {
      color = "has-text-success";
      svgColor = "svg-text-success";
    } else {
      color = "has-text-warning-dark";
      svgColor = "svg-text-warning-dark";
    }
  }

  return { icon, color, svgColor, glyph };
}

export function buildTree<I>(items: I[], accountAccessor: (item: I) => string): I[] {
  const result: I[] = [];

  const sorted = _.sortBy(items, accountAccessor);

  for (const item of sorted) {
    const account = accountAccessor(item);
    const parts = account.split(":");
    let current = result;
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      let found: any = current.find((c) => accountAccessor(c).split(":")[i] === part);
      if (!found) {
        found = { ...item };
        current.push(found);
      }

      if (i !== parts.length - 1) {
        found._children = found._children || [];
        current = found._children;
      }
    }
  }

  return result;
}
