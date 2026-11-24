import dayjs, { type Dayjs } from "dayjs";
import { requestApi, type RequestOptions } from "./client";
import type {
  IngestionColumnMapping,
  IngestionFormat,
  IngestionRequest,
  IngestionResult,
  StagedTransaction
} from "./importer.gen";
import type { ReconciliationReport as GeneratedReconciliationReport } from "./reconciliation.gen";
import type {
  CommitImportRequest,
  CommitImportResponse,
  ParseStatementResponse,
  ReconciliationResponse
} from "./types.gen";

export type {
  CommitImportRequest,
  IngestionColumnMapping,
  IngestionFormat,
  IngestionRequest,
  IngestionResult,
  StagedTransaction
};

/** Wire reconciliation report: `generated_at` is still an RFC3339 string. */
export type WireReconciliationReport = GeneratedReconciliationReport;

/** Domain reconciliation report with the timestamp parsed to dayjs. */
export type ReconciliationReport = Omit<WireReconciliationReport, "generated_at"> & {
  generated_at: Dayjs;
};

export function reconciliationFromWire(wire: WireReconciliationReport): ReconciliationReport {
  return { ...wire, generated_at: dayjs(wire.generated_at) };
}

/** Parses a CSV/OFX/QFX/Excel/PDF statement into staged transactions. */
export async function parseStatement(
  body: IngestionRequest,
  options?: RequestOptions
): Promise<ParseStatementResponse> {
  return requestApi<ParseStatementResponse>("/api/import/statement", {
    method: "POST",
    body: JSON.stringify(body),
    ...options
  });
}

/**
 * Wire commit summary. `errors` is null for a clean commit and an array when
 * the ledger reported validation warnings. The client keeps that distinction
 * instead of normalising null to [].
 */
export type CommitImportResult = CommitImportResponse & {
  message?: string;
  error?: string;
};

/** Appends the approved staged transactions to the journal file. */
export async function commitImport(
  body: CommitImportRequest,
  options?: RequestOptions
): Promise<CommitImportResult> {
  return requestApi<CommitImportResult>("/api/import/commit", {
    method: "POST",
    body: JSON.stringify(body),
    ...options
  });
}

/** Returns the read-only reconciliation report built from import manifests. */
export async function getReconciliation(options?: RequestOptions): Promise<ReconciliationReport> {
  const response = await requestApi<ReconciliationResponse>("/api/import/reconciliation", options);
  return reconciliationFromWire(response);
}
