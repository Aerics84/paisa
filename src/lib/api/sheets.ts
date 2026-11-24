import { requestApi, type RequestOptions } from "./client";
import { postingFromWire, type Posting } from "./ledger";
import type {
  SheetFileResponse,
  SheetFilesResponse,
  SheetFile as GeneratedSheetFile,
  SheetSaveResponse
} from "./types.gen";

/** Sheet file as it appears on the wire. */
export type WireSheetFile = GeneratedSheetFile;

/**
 * Domain sheet file. The API does not send the `type` discriminator used by
 * the directory tree, so the client adds it while mapping.
 */
export type SheetFile = GeneratedSheetFile & { type: "file" };

/** Payload accepted by the sheet file and save endpoints. */
export type SheetFileInput = {
  name: string;
  content: string;
  operation?: string;
};

export type SheetFilesResult = {
  files: SheetFile[];
  postings: Posting[];
};

export type SheetSaveResult =
  | { saved: true; file: SheetFile; message?: string }
  | { saved: false; message?: string; file?: undefined };

function sheetFileFromWire(wire: GeneratedSheetFile): SheetFile {
  return {
    ...wire,
    type: "file",
    versions: wire.versions ?? []
  };
}

export async function getSheetFiles(options?: RequestOptions): Promise<SheetFilesResult> {
  const response = await requestApi<SheetFilesResponse>("/api/sheets/files", options);
  return {
    files: (response.files ?? []).map(sheetFileFromWire),
    postings: (response.postings ?? []).map(postingFromWire)
  };
}

export async function getSheetFile(
  name: string,
  options?: RequestOptions
): Promise<{ file: SheetFile }> {
  const response = await requestApi<SheetFileResponse>("/api/sheets/file", {
    method: "POST",
    body: JSON.stringify({ name }),
    ...options
  });
  if (!response.file) {
    throw new Error(response.message ?? `Sheet ${name} not found`);
  }
  return { file: sheetFileFromWire(response.file) };
}

export async function deleteSheetBackups(
  name: string,
  options?: RequestOptions
): Promise<{ file: SheetFile }> {
  const response = await requestApi<SheetFileResponse>("/api/sheets/file/delete_backups", {
    method: "POST",
    body: JSON.stringify({ name }),
    ...options
  });
  if (!response.file) {
    throw new Error(response.message ?? `Sheet ${name} not found`);
  }
  return { file: sheetFileFromWire(response.file) };
}

export async function saveSheetFile(
  file: SheetFileInput,
  options?: RequestOptions
): Promise<SheetSaveResult> {
  const response = await requestApi<SheetSaveResponse>("/api/sheets/save", {
    method: "POST",
    body: JSON.stringify(file),
    ...options
  });

  if (!response.saved) {
    return { saved: false, message: response.message };
  }
  if (!response.file) {
    throw new Error("Save succeeded without a file");
  }
  return { saved: true, file: sheetFileFromWire(response.file), message: response.message };
}
