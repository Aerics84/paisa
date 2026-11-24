import { requestApi, type RequestOptions } from "./client";
import type {
  EditorFileResponse,
  EditorFilesResponse,
  EditorSaveResponse,
  EditorValidateResponse,
  FileError as WireFileError,
  LedgerFile as GeneratedLedgerFile
} from "./types.gen";

/** Validation diagnostic returned by /api/editor/validate and /api/editor/save. */
export type FileError = WireFileError;

/** Ledger file as it appears on the wire. */
export type WireLedgerFile = GeneratedLedgerFile;

/**
 * Domain ledger file. The API does not send the `type` discriminator used by
 * the directory tree, so the client adds it while mapping.
 */
export type LedgerFile = WireLedgerFile & { type: "file" };

/** Payload accepted by the editor file, validate and save endpoints. */
export type EditorFileInput = {
  name: string;
  content: string;
  operation?: string;
};

export type EditorFilesResult = {
  files: LedgerFile[];
  accounts: string[];
  payees: string[];
  commodities: string[];
};

export type EditorValidateResult = {
  errors: FileError[];
  output: string;
};

export type EditorSaveResult =
  | { saved: true; errors: FileError[]; file: LedgerFile; message?: string }
  | { saved: false; errors: FileError[]; message?: string; file?: undefined };

function ledgerFileFromWire(wire: WireLedgerFile): LedgerFile {
  return {
    ...wire,
    type: "file",
    versions: wire.versions ?? []
  };
}

export async function getEditorFiles(options?: RequestOptions): Promise<EditorFilesResult> {
  const response = await requestApi<EditorFilesResponse>("/api/editor/files", options);
  return {
    files: (response.files ?? []).map(ledgerFileFromWire),
    accounts: response.accounts ?? [],
    payees: response.payees ?? [],
    commodities: response.commodities ?? []
  };
}

export async function getEditorFile(
  name: string,
  options?: RequestOptions
): Promise<{ file: LedgerFile }> {
  const response = await requestApi<EditorFileResponse>("/api/editor/file", {
    method: "POST",
    body: JSON.stringify({ name }),
    ...options
  });
  if (!response.file) {
    throw new Error(response.message ?? `File ${name} not found`);
  }
  return { file: ledgerFileFromWire(response.file) };
}

export async function deleteEditorBackups(
  name: string,
  options?: RequestOptions
): Promise<{ file: LedgerFile }> {
  const response = await requestApi<EditorFileResponse>("/api/editor/file/delete_backups", {
    method: "POST",
    body: JSON.stringify({ name }),
    ...options
  });
  if (!response.file) {
    throw new Error(response.message ?? `File ${name} not found`);
  }
  return { file: ledgerFileFromWire(response.file) };
}

export async function validateEditorFile(
  file: EditorFileInput,
  options?: RequestOptions
): Promise<EditorValidateResult> {
  const response = await requestApi<EditorValidateResponse>("/api/editor/validate", {
    method: "POST",
    body: JSON.stringify(file),
    ...options
  });
  return { errors: response.errors ?? [], output: response.output };
}

export async function saveEditorFile(
  file: EditorFileInput,
  options?: RequestOptions
): Promise<EditorSaveResult> {
  const response = await requestApi<EditorSaveResponse>("/api/editor/save", {
    method: "POST",
    body: JSON.stringify(file),
    ...options
  });
  const errors = response.errors ?? [];

  if (!response.saved) {
    return { saved: false, errors, message: response.message };
  }
  if (!response.file) {
    throw new Error("Save succeeded without a file");
  }
  return { saved: true, errors, file: ledgerFileFromWire(response.file), message: response.message };
}
