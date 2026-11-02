import Papa from "papaparse";
import _ from "lodash";
import { readSheet } from "read-excel-file/browser";
import { format } from "./journal";
import { pdf2array } from "./pdf";

interface Result {
  data: any[][];
  rows?: Array<Record<string, any>>;
  error?: string;
}

interface ParsedCSVResult {
  data: string[][];
  errors?: Papa.ParseError[];
}

type CSVRow = Record<string, string>;

export interface StatementTransaction {
  date: string;
  description: string;
  amount: number;
  account: string;
  expenseAccount?: string;
  raw?: Record<string, any>;
}

export interface DuplicateMatchOptions {
  dateWindowHours?: number;
  requirePayee?: boolean;
}

function normalizePayee(value: string): string {
  return _.toString(value || "")
    .trim()
    .toLowerCase()
    .replace(/\s+/g, " ");
}

export function isDuplicateStatementTransaction(
  left: Partial<StatementTransaction>,
  right: Partial<StatementTransaction>,
  options: DuplicateMatchOptions = {}
): boolean {
  const dateWindowHours = options.dateWindowHours ?? 0;
  const requirePayee = options.requirePayee ?? true;

  const sameAccount = _.toString(left.account || "") === _.toString(right.account || "");
  const sameAmount = Math.abs((left.amount ?? 0) - (right.amount ?? 0)) < 0.01;
  if (!sameAccount || !sameAmount) {
    return false;
  }

  const leftPayee = normalizePayee(_.toString(left.description || ""));
  const rightPayee = normalizePayee(_.toString(right.description || ""));
  const payeeMatch = leftPayee !== "" && leftPayee === rightPayee;

  if (requirePayee && !payeeMatch) {
    return false;
  }

  const leftDate = left.date ? new Date(left.date) : null;
  const rightDate = right.date ? new Date(right.date) : null;
  if (
    !leftDate ||
    !rightDate ||
    Number.isNaN(leftDate.getTime()) ||
    Number.isNaN(rightDate.getTime())
  ) {
    return false;
  }

  const diffHours = Math.abs(leftDate.getTime() - rightDate.getTime()) / (1000 * 60 * 60);
  return diffHours <= dateWindowHours;
}

export function deduplicateStatementTransactions(
  transactions: Array<Partial<StatementTransaction>>,
  options: DuplicateMatchOptions = {}
): StatementTransaction[] {
  const deduped: StatementTransaction[] = [];

  for (const tx of transactions) {
    const candidate = {
      date: _.toString(tx.date || ""),
      description: _.toString(tx.description || ""),
      amount: Number.isFinite(tx.amount) ? Number(tx.amount) : 0,
      account: _.toString(tx.account || ""),
      expenseAccount: tx.expenseAccount || "Expenses:Unknown"
    } as StatementTransaction;

    const existing = deduped.find((entry) =>
      isDuplicateStatementTransaction(entry, candidate, options)
    );

    if (existing) {
      existing.amount += candidate.amount;
      if (!existing.expenseAccount || existing.expenseAccount === "Expenses:Unknown") {
        existing.expenseAccount = candidate.expenseAccount;
      }
      continue;
    }

    deduped.push(candidate);
  }

  return deduped;
}

export function consolidateStatementTransactions(
  transactions: Array<Partial<StatementTransaction>>,
  options: { defaultAccount?: string; expensePrefix?: string; dateWindowHours?: number } = {}
): string {
  if (_.isEmpty(transactions)) {
    return "";
  }

  const defaultAccount = options.defaultAccount || "Assets:Unknown";
  const expensePrefix = options.expensePrefix || "Expenses";
  const dateWindowHours = options.dateWindowHours ?? 0;

  const prepared = transactions.map((tx) => ({
    date: _.toString(tx.date || "") || "1970-01-01",
    description: _.toString(tx.description || "Unknown merchant"),
    amount: Number.isFinite(tx.amount) ? Number(tx.amount) : 0,
    account: _.toString(tx.account || defaultAccount),
    expenseAccount:
      tx.expenseAccount ||
      guessExpenseAccount(_.toString(tx.description || "Unknown merchant"), expensePrefix)
  })) as StatementTransaction[];

  const deduped = deduplicateStatementTransactions(prepared, {
    dateWindowHours,
    requirePayee: true
  });
  const groups = new Map<
    string,
    { month: string; expenseAccount: string; amount: number; account: string }
  >();

  for (const tx of deduped) {
    const amount = Math.abs(tx.amount);
    const date = tx.date.includes("-") ? tx.date.slice(0, 7) : "1970-01";
    const key = `${date}|${tx.expenseAccount || "Expenses:Unknown"}`;

    const current = groups.get(key) || {
      month: date,
      expenseAccount: tx.expenseAccount || `${expensePrefix}:Unknown`,
      amount: 0,
      account: tx.account || defaultAccount
    };

    current.amount += amount;
    groups.set(key, current);
  }

  const lines: string[] = [];
  for (const group of Array.from(groups.values()).sort((a, b) => a.month.localeCompare(b.month))) {
    const monthDate = `${group.month}-01`;
    const total = Number(group.amount.toFixed(2));
    lines.push(`${monthDate} * "${group.expenseAccount}"`);
    lines.push(`  ${group.expenseAccount} ${total.toFixed(2)} INR`);
    lines.push(`  ${group.account || defaultAccount} -${total.toFixed(2)} INR`);
  }

  return lines.join("\n");
}

const EXPENSE_KEYWORDS: Array<[string, RegExp[]]> = [
  [
    "Expenses:Groceries",
    [
      /grocery|grocer|mart|supermarket|bigbasket|milk|vegetable|fruit|dairy|reliance fresh|dmart|fresh|foodmart/i
    ]
  ],
  [
    "Expenses:Dining",
    [/restaurant|cafe|coffee|zomato|swiggy|pizza|burger|bistro|food court|canteen|dining/i]
  ],
  [
    "Expenses:Transportation",
    [
      /uber|ola|metro|fuel|petrol|diesel|rail|bus|cab|taxi|parking|toll|travel|auto|transport|indian oil|bharat petrol/i
    ]
  ],
  [
    "Expenses:Shopping",
    [/amazon|flipkart|myntra|shop|shopping|marketplace|retail|ebay|store|paytm|online purchase/i]
  ],
  [
    "Expenses:Utilities",
    [
      /electricity|water|gas|wifi|internet|mobile|phone|recharge|broadband|utility|bill|bse|bescom|airtel|jio|vodafone/i
    ]
  ],
  [
    "Expenses:Health",
    [/doctor|hospital|pharmacy|clinic|apollo|health|medical|medplus|wellness|insurance|optical/i]
  ],
  ["Expenses:Travel", [/flight|hotel|booking|airline|railway|irctc|airbnb|holiday|trip|travel/i]],
  [
    "Expenses:Entertainment",
    [/netflix|spotify|movie|cinema|games|entertainment|youtube|streaming/i]
  ],
  ["Expenses:Education", [/school|college|course|tuition|exam|books|udemy|education|academy/i]],
  ["Expenses:Insurance", [/insurance|premium|policy|lic|mutual|cover|renewal/i]],
  ["Expenses:Investment", [/sip|mutual fund|nps|brokerage|stock|investment|trading|etf/i]]
];

export function guessExpenseAccount(
  description: string,
  prefix: string = "Expenses",
  rules: Array<[string, RegExp[]]> = EXPENSE_KEYWORDS
): string {
  const text = _.toString(description || "").trim();
  if (!text) {
    return `${prefix}:Unknown`;
  }

  const normalized = text.toLowerCase();
  for (const [bucket, patterns] of rules) {
    if (patterns.some((pattern) => pattern.test(normalized))) {
      return `${prefix}:${bucket.replace(/^Expenses:/, "")}`;
    }
  }

  return `${prefix}:Unknown`;
}

export function parse(file: File): Promise<Result> {
  let extension = file.name.split(".").pop();
  extension = extension?.toLowerCase();
  if (extension === "csv" || extension === "txt") {
    return parseCSV(file);
  } else if (extension === "sta" || extension === "mt940") {
    return parseTextLines(file);
  } else if (extension === "xlsx") {
    return parseXLSX(file);
  } else if (extension === "xls") {
    return parseLegacyXLS();
  } else if (extension === "pdf") {
    return parsePDF(file);
  } else if (extension === "xml") {
    return parseXML(file);
  }
  throw new Error(`Unsupported file type ${extension}`);
}

export function asRows(result: Result): Array<Record<string, any>> {
  if (result.rows) {
    return _.map(result.rows, (row, i) => {
      const dataRow = result.data[i] || [];
      return _.chain(dataRow)
        .map((cell, j) => {
          return [String.fromCharCode(65 + j), cell];
        })
        .concat([["index", i as any]])
        .concat(_.toPairs(row))
        .fromPairs()
        .value();
    });
  }

  return _.map(result.data, (row, i) => {
    return _.chain(row)
      .map((cell, j) => {
        return [String.fromCharCode(65 + j), cell];
      })
      .concat([["index", i as any]])
      .fromPairs()
      .value();
  });
}

const COLUMN_REFS = _.chain(_.range(65, 90))
  .map((i) => String.fromCharCode(i))
  .map((a) => [a, a])
  .fromPairs()
  .value();

export function render(
  rows: Array<Record<string, any>>,
  template: Handlebars.TemplateDelegate,
  options: { reverse?: boolean; trim?: boolean } = {}
) {
  const output: string[] = [];
  _.each(rows, (row) => {
    let rendered = template(_.assign({ ROW: row, SHEET: rows }, COLUMN_REFS));
    if (options.trim) {
      rendered = _.trim(rendered);
    }
    if (!_.isEmpty(rendered)) {
      output.push(rendered);
    }
  });
  if (options.reverse) {
    output.reverse();
  }

  if (options.trim) {
    return format(output.join("\n\n"));
  } else {
    return format(output.join(""));
  }
}

function parseCSV(file: File): Promise<Result> {
  return new Promise((resolve, reject) => {
    Papa.parse<string[]>(file, {
      skipEmptyLines: true,
      complete: function (results) {
        const normalized = normalizeBrokerCSV(results as ParsedCSVResult);
        if (normalized) {
          resolve(normalized);
          return;
        }
        resolve(results);
      },
      error: function (error) {
        reject(error);
      },
      delimitersToGuess: [",", "\t", "|", ";", Papa.RECORD_SEP, Papa.UNIT_SEP, "^"]
    });
  });
}

function normalizeBrokerCSV(results: ParsedCSVResult): Result | null {
  const data = results.data || [];
  if (data.length < 2) {
    return null;
  }

  const headers = data[0].map((cell) => cell.trim());
  const body = data.slice(1);

  if (matchesHeaders(headers, TRADE_REPUBLIC_TRADE_HEADERS)) {
    return normalizeTradeRows(data, body, headers, "Trade Republic");
  }
  if (matchesHeaders(headers, TRADE_REPUBLIC_CASH_HEADERS)) {
    return normalizeCashRows(data, body, headers, "Trade Republic");
  }
  if (matchesHeaders(headers, SCALABLE_TRADE_HEADERS)) {
    return normalizeTradeRows(data, body, headers, "Scalable Capital");
  }
  if (matchesHeaders(headers, SCALABLE_CASH_HEADERS)) {
    return normalizeCashRows(data, body, headers, "Scalable Capital");
  }

  return null;
}

const TRADE_REPUBLIC_TRADE_HEADERS = [
  "Datum",
  "Typ",
  "ISIN",
  "Ticker",
  "Wertpapier",
  "Anzahl",
  "Preis",
  "Währung",
  "Bruttobetrag",
  "Gebühren",
  "Steuern",
  "Abrechnungstag"
] as const;

const TRADE_REPUBLIC_CASH_HEADERS = [
  "Datum",
  "Typ",
  "Beschreibung",
  "Betrag",
  "Währung",
  "ISIN",
  "Ticker",
  "Wertpapier"
] as const;

const SCALABLE_TRADE_HEADERS = [
  "Date",
  "Transaction",
  "ISIN",
  "Symbol",
  "Security",
  "Quantity",
  "Price",
  "Currency",
  "Gross Amount",
  "Fees",
  "Taxes",
  "Settlement Date"
] as const;

const SCALABLE_CASH_HEADERS = [
  "Date",
  "Transaction",
  "Description",
  "Amount",
  "Currency",
  "ISIN",
  "Symbol",
  "Security"
] as const;

function matchesHeaders(headers: string[], expected: readonly string[]) {
  return expected.every((header, index) => headers[index] === header);
}

function normalizeTradeRows(
  data: string[][],
  body: string[][],
  headers: string[],
  broker: string
): Result {
  const rows = body.map((cells) => {
    const row = toCSVRow(headers, cells);
    const mappedType = normalizeTradeType(row["Typ"] || row["Transaction"] || "");
    const grossAmount = row["Bruttobetrag"] || row["Gross Amount"] || "";
    const fees = row["Gebühren"] || row["Fees"] || "";
    const taxes = row["Steuern"] || row["Taxes"] || "";
    const settlementDate = row["Abrechnungstag"] || row["Settlement Date"] || "";
    const symbol = row["Ticker"] || row["Symbol"] || "";
    const securityName = row["Wertpapier"] || row["Security"] || "";

    return {
      broker,
      importType: "broker-trade",
      transactionKind: mappedType,
      tradeDate: row["Datum"] || row["Date"] || "",
      settlementDate,
      isin: row["ISIN"] || "",
      symbol,
      securityName,
      quantity: normalizeQuantity(row["Anzahl"] || row["Quantity"] || ""),
      unitPrice: row["Preis"] || row["Price"] || "",
      currency: row["Währung"] || row["Currency"] || "",
      principal: grossAmount,
      feeAmount: fees,
      taxAmount: taxes,
      netCashAmount: computeNetCashAmount(mappedType, grossAmount, fees, taxes),
      description: compactJoin([mappedType ? capitalizeWord(mappedType) : "", securityName], " "),
      rawType: row["Typ"] || row["Transaction"] || ""
    };
  });

  return { data, rows };
}

function normalizeCashRows(
  data: string[][],
  body: string[][],
  headers: string[],
  broker: string
): Result {
  const rows = body.map((cells) => {
    const row = toCSVRow(headers, cells);
    const mappedType = normalizeCashType(row["Typ"] || row["Transaction"] || "");
    const symbol = row["Ticker"] || row["Symbol"] || "";
    const securityName = row["Wertpapier"] || row["Security"] || "";

    return {
      broker,
      importType: "broker-cash",
      transactionKind: mappedType,
      cashDate: row["Datum"] || row["Date"] || "",
      isin: row["ISIN"] || "",
      symbol,
      securityName,
      currency: row["Währung"] || row["Currency"] || "",
      cashAmount: row["Betrag"] || row["Amount"] || "",
      description: row["Beschreibung"] || row["Description"] || "",
      rawType: row["Typ"] || row["Transaction"] || ""
    };
  });

  return { data, rows };
}

function toCSVRow(headers: string[], cells: string[]): CSVRow {
  return _.zipObject(
    headers,
    headers.map((_, index) => (cells[index] || "").trim())
  );
}

function normalizeTradeType(value: string) {
  const normalized = value.trim().toLowerCase();
  if (["buy", "kauf"].includes(normalized)) {
    return "buy";
  }
  if (["sell", "verkauf"].includes(normalized)) {
    return "sell";
  }
  return normalized;
}

function normalizeCashType(value: string) {
  const normalized = value.trim().toLowerCase();
  if (["dividend", "dividende"].includes(normalized)) {
    return "dividend";
  }
  if (["interest", "zinsen"].includes(normalized)) {
    return "interest";
  }
  if (["deposit", "einzahlung"].includes(normalized)) {
    return "deposit";
  }
  if (["withdrawal", "auszahlung"].includes(normalized)) {
    return "withdrawal";
  }
  if (["fee", "gebühr", "gebuehr"].includes(normalized)) {
    return "fee";
  }
  if (["tax", "steuer"].includes(normalized)) {
    return "tax";
  }
  return normalized;
}

function computeNetCashAmount(
  transactionKind: string,
  principal: string,
  fees: string,
  taxes: string
) {
  if (!["buy", "sell"].includes(transactionKind)) {
    return "";
  }

  const principalValue = parseEuropeanNumber(principal);
  const feeValue = parseEuropeanNumber(fees);
  const taxValue = parseEuropeanNumber(taxes);
  if (principalValue === null || feeValue === null || taxValue === null) {
    return "";
  }

  const sign = transactionKind === "buy" ? -1 : 1;
  return String(sign * (principalValue - feeValue - taxValue));
}

function parseEuropeanNumber(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return 0;
  }

  const normalized = trimmed.replace(/\s/g, "");
  const lastComma = normalized.lastIndexOf(",");
  const lastDot = normalized.lastIndexOf(".");
  let decimalSeparator = "";
  if (lastComma > lastDot) {
    decimalSeparator = ",";
  } else if (lastDot > lastComma) {
    decimalSeparator = ".";
  }

  let normalizedDigits = normalized;
  if (decimalSeparator === ",") {
    normalizedDigits = normalizedDigits.replace(/\./g, "").replace(",", ".");
  } else if (decimalSeparator === ".") {
    normalizedDigits = normalizedDigits.replace(/,/g, "");
  } else {
    normalizedDigits = normalizedDigits.replace(/[.,]/g, "");
  }

  const parsed = Number(normalizedDigits);
  return Number.isFinite(parsed) ? parsed : null;
}

function normalizeQuantity(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  const parsed = parseEuropeanNumber(trimmed);
  if (parsed === null) {
    return trimmed;
  }

  return parsed.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 12,
    useGrouping: false
  });
}

function capitalizeWord(value: string) {
  if (!value) {
    return value;
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

async function parseTextLines(file: File): Promise<Result> {
  const text = await readText(file);
  return {
    data: text
      .split(/\r?\n/)
      .map((line) => line.trimEnd())
      .filter((line) => line.trim() !== "")
      .map((line) => [line])
  };
}

async function parseXLSX(file: File): Promise<Result> {
  try {
    return { data: (await readSheet(file)) as any[][] };
  } catch {
    return { data: [], error: "Unable to parse XLSX document" };
  }
}

function parseLegacyXLS(): Promise<Result> {
  return Promise.resolve({
    data: [],
    error: "Legacy .xls files are no longer supported. Please re-save the file as .xlsx or CSV."
  });
}

async function parsePDF(file: File): Promise<Result> {
  const buffer = await readFile(file);
  const array = await pdf2array(buffer);
  return { data: array };
}

async function parseXML(file: File): Promise<Result> {
  const xml = await readText(file);
  const parser = new DOMParser();
  const document = parser.parseFromString(xml, "application/xml");
  const parserError = document.querySelector("parsererror");
  if (parserError) {
    return { data: [], error: "Unable to parse XML document" };
  }

  if (findElements(document, "BkToCstmrStmt").length > 0) {
    return parseCAMTDocument(document);
  }

  return { data: [], error: "Unsupported XML document" };
}

function parseCAMTDocument(document: Document): Result {
  const entries = findElements(document, "Ntry");
  const rows = entries.map((entry) => {
    const bookingDate = firstChildText(entry, "BookgDt", "Dt") || firstDateTime(entry, "BookgDt");
    const valueDate = firstChildText(entry, "ValDt", "Dt") || firstDateTime(entry, "ValDt");
    const amountElement = firstElement(entry, "Amt");
    const amount = amountElement?.textContent?.trim() || "";
    const currency = amountElement?.getAttribute("Ccy") || "";
    const creditDebitIndicator = firstText(entry, "CdtDbtInd");
    const status = firstText(entry, "Sts");
    const bankTransactionCode = compactJoin([
      firstText(entry, "Cd"),
      firstText(entry, "SubFmlyCd")
    ]);
    const counterpartyName = firstText(entry, "Cdtr", "Nm") || firstText(entry, "Dbtr", "Nm") || "";
    const counterpartyIban =
      firstText(entry, "CdtrAcct", "IBAN") || firstText(entry, "DbtrAcct", "IBAN") || "";
    const remittanceInformation = compactJoin(
      findElements(entry, "Ustrd").map((node) => node.textContent || ""),
      " | "
    );
    const additionalInformation = firstText(entry, "AddtlNtryInf");
    const reference =
      firstText(entry, "AcctSvcrRef") ||
      firstText(entry, "NtryRef") ||
      firstText(entry, "EndToEndId");

    return {
      bookingDate,
      valueDate,
      amount,
      currency,
      creditDebitIndicator,
      status,
      bankTransactionCode,
      counterpartyName,
      counterpartyIban,
      remittanceInformation,
      additionalInformation,
      reference
    };
  });

  const data = rows.map((row) => [
    row.bookingDate,
    row.valueDate,
    row.amount,
    row.currency,
    row.creditDebitIndicator,
    row.counterpartyName,
    row.remittanceInformation,
    row.reference,
    row.additionalInformation
  ]);

  return { data, rows };
}

function firstElement(parent: Element | Document, localName: string) {
  return findElements(parent, localName)[0];
}

function firstText(parent: Element | Document, ...path: string[]) {
  let current: Element | null = parent as Element;
  for (const localName of path) {
    current = current ? firstElement(current, localName) || null : null;
    if (!current) {
      return "";
    }
  }
  return current.textContent?.trim() || "";
}

function firstChildText(parent: Element | Document, localName: string, childName: string) {
  const element = firstElement(parent, localName);
  if (!element) {
    return "";
  }
  return firstText(element, childName);
}

function firstDateTime(parent: Element | Document, localName: string) {
  const element = firstElement(parent, localName);
  if (!element) {
    return "";
  }
  return (firstText(element, "DtTm") || "").split("T")[0];
}

function compactJoin(values: string[], separator = " / ") {
  return values
    .map((value) => value.trim())
    .filter(Boolean)
    .join(separator);
}

function findElements(parent: Element | Document, localName: string): Element[] {
  return Array.from(parent.getElementsByTagName("*")).filter((element) => {
    const tagName = element.tagName?.split(":").pop();
    return element.localName === localName || tagName === localName;
  });
}

function readFile(file: File): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = (event) => {
      resolve(event.target.result as ArrayBuffer);
    };
    reader.onerror = (event) => {
      reject(event);
    };
    reader.readAsArrayBuffer(file);
  });
}

function readText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = (event) => {
      resolve(event.target.result as string);
    };
    reader.onerror = (event) => {
      reject(event);
    };
    reader.readAsText(file);
  });
}
