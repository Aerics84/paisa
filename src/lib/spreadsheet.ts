import Papa from "papaparse";
import * as XLSX from "xlsx";
import _ from "lodash";
import { format } from "./journal";
import { pdf2array } from "./pdf";
import * as XlsxPopulate from "xlsx-populate";

interface Result {
  data: string[][];
  rows?: Array<Record<string, any>>;
  error?: string;
}

export function parse(file: File): Promise<Result> {
  let extension = file.name.split(".").pop();
  extension = extension?.toLowerCase();
  if (extension === "csv" || extension === "txt") {
    return parseCSV(file);
  } else if (extension === "sta" || extension === "mt940") {
    return parseTextLines(file);
  } else if (extension === "xlsx" || extension === "xls") {
    return parseXLSX(file);
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
        resolve(results);
      },
      error: function (error) {
        reject(error);
      },
      delimitersToGuess: [",", "\t", "|", ";", Papa.RECORD_SEP, Papa.UNIT_SEP, "^"]
    });
  });
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
  const buffer = await readFile(file);
  try {
    const sheet = XLSX.read(buffer, { type: "binary" });
    const json = XLSX.utils.sheet_to_json<string[]>(sheet.Sheets[sheet.SheetNames[0]], {
      header: 1,
      blankrows: false,
      rawNumbers: false
    });
    return { data: json };
  } catch (e) {
    if (/password-protected/.test(e.message)) {
      const password = prompt(
        "Please enter the password to open this XLSX file. Press cancel to exit."
      );
      if (password === null) {
        return { data: [], error: "Password required." };
      }

      try {
        const workbook = await XlsxPopulate.fromDataAsync(buffer, { password });
        const sheet = workbook.sheet(0);
        if (sheet) {
          let json = sheet.usedRange().value();
          json = _.map(json, (row) => {
            return _.map(row, (cell) => {
              if (cell) {
                return cell.toString();
              }
              return "";
            });
          });

          return { data: json };
        }
      } catch (e) {
        // follow through to the error below
      }

      return { data: [], error: "Unable to parse Password protected XLSX" };
    }
    throw e;
  }
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

  if (document.getElementsByTagNameNS("*", "BkToCstmrStmt").length > 0) {
    return parseCAMTDocument(document);
  }

  return { data: [], error: "Unsupported XML document" };
}

function parseCAMTDocument(document: Document): Result {
  const entries = Array.from(document.getElementsByTagNameNS("*", "Ntry"));
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
      Array.from(entry.getElementsByTagNameNS("*", "Ustrd")).map((node) => node.textContent || ""),
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
  return parent.getElementsByTagNameNS("*", localName).item(0);
}

function firstText(parent: Element | Document, ...path: string[]) {
  let current: Element | null = parent as Element;
  for (const localName of path) {
    current = current?.getElementsByTagNameNS("*", localName).item(0);
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
