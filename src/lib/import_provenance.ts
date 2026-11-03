import type { IngestionFormat } from "./utils";

export interface IngestionColumnMapping {
  header_row: number;
  date: number;
  description: number;
  debit: number;
  credit: number;
  amount: number;
  balance: number;
  currency: number;
  dr_cr: number;
}

export interface StatementRequestOptions {
  format: IngestionFormat;
  content: string;
  delimiter: string;
  baseAccount: string;
  fileName?: string;
  pasted?: boolean;
  locale?: string;
  columnMapping?: IngestionColumnMapping;
}

export interface StatementRequest {
  format: IngestionFormat;
  content: string;
  delimiter: string;
  base_account: string;
  source_type: string;
  original_path: string;
  locale: string;
  column_mapping?: IngestionColumnMapping;
}

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter((part) => part.length > 0);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function extension(path: string): string {
  const name = basename(path);
  const index = name.lastIndexOf(".");
  return index >= 0 ? name.slice(index + 1).toLowerCase() : "";
}

/**
 * Builds the statement parse request including the provenance fields the
 * backend uses for import manifests. The SHA-256 hash is intentionally left to
 * the Go side so the browser does not need Web Crypto.
 */
export function buildStatementRequest(options: StatementRequestOptions): StatementRequest {
  const locale = options.locale || "en-IN";

  if (options.pasted) {
    const sourceType = options.format === "csv" ? "csv" : "pasted";
    return {
      format: options.format,
      content: options.content,
      delimiter: options.delimiter,
      base_account: options.baseAccount,
      source_type: sourceType,
      original_path: sourceType === "csv" ? "pasted.csv" : "pasted-statement",
      locale,
      column_mapping: options.columnMapping
    };
  }

  const name = options.fileName ? basename(options.fileName) : "";
  const ext = extension(name);

  let sourceType: string;
  if (ext === "csv" || ext === "ofx" || ext === "qfx") {
    sourceType = ext;
  } else if (options.format === "excel" || options.format === "pdf") {
    sourceType = options.format;
  } else if (options.format === "ofx" || options.format === "qfx") {
    sourceType = options.format;
  } else {
    sourceType = "csv";
  }

  return {
    format: options.format,
    content: options.content,
    delimiter: options.delimiter,
    base_account: options.baseAccount,
    source_type: sourceType,
    original_path: name || `statement.${sourceType}`,
    locale,
    column_mapping: options.columnMapping
  };
}
