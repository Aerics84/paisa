import { afterEach, expect, test, mock } from "bun:test";
import {
  deleteEditorBackups,
  getEditorFile,
  getEditorFiles,
  saveEditorFile,
  validateEditorFile
} from "./editor";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

type FetchCall = { url: string; init?: RequestInit };

let calls: FetchCall[] = [];

function mockJSON(body: unknown, status = 200) {
  calls = [];
  globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(body), { status });
  }) as unknown as typeof fetch;
}

const ledgerFile = {
  name: "main.ledger",
  content: "2026-11-01 Fixture\n",
  versions: ["main.ledger.backup.2026-11-20-10-00-00.000"],
  operation: ""
};

function postedBody(index = 0) {
  return JSON.parse(String(calls[index].init?.body));
}

test("getEditorFiles maps files and defaults null lists", async () => {
  mockJSON({
    files: [ledgerFile],
    accounts: ["Assets:Checking"],
    payees: ["Fixture Payee"],
    commodities: ["INR"]
  });

  const response = await getEditorFiles();
  expect(calls[0].url).toBe("/api/editor/files");
  expect(response.files).toHaveLength(1);
  expect(response.files[0].name).toBe("main.ledger");
  expect(response.files[0].type).toBe("file");
  expect(response.files[0].versions).toEqual(ledgerFile.versions);
  expect(response.accounts).toEqual(["Assets:Checking"]);
  expect(response.payees).toEqual(["Fixture Payee"]);
  expect(response.commodities).toEqual(["INR"]);

  mockJSON({ files: null, accounts: null, payees: null, commodities: null });
  await expect(getEditorFiles()).resolves.toEqual({
    files: [],
    accounts: [],
    payees: [],
    commodities: []
  });
});

test("getEditorFile posts the file name and throws on an error message", async () => {
  mockJSON({ file: { ...ledgerFile, versions: [] } });

  const response = await getEditorFile("main.ledger");
  expect(calls[0].url).toBe("/api/editor/file");
  expect(calls[0].init?.method).toBe("POST");
  expect(postedBody()).toEqual({ name: "main.ledger" });
  expect(response.file.name).toBe("main.ledger");
  expect(response.file.type).toBe("file");
  expect(response.file.versions).toEqual([]);

  mockJSON({ message: "Invalid file name" });
  await expect(getEditorFile("../secret.ledger")).rejects.toThrow("Invalid file name");
});

test("deleteEditorBackups returns the refreshed file", async () => {
  mockJSON({ file: { ...ledgerFile, versions: [] } });

  const response = await deleteEditorBackups("main.ledger");
  expect(calls[0].url).toBe("/api/editor/file/delete_backups");
  expect(postedBody()).toEqual({ name: "main.ledger" });
  expect(response.file.versions).toEqual([]);
});

test("validateEditorFile defaults null errors and keeps diagnostics", async () => {
  mockJSON({
    errors: [
      { file_name: "main.ledger", line_from: 1, line_to: 2, error: "", message: "Bad entry" }
    ],
    output: "validation output"
  });

  const response = await validateEditorFile({ name: "main.ledger", content: "bad" });
  expect(calls[0].url).toBe("/api/editor/validate");
  expect(postedBody()).toEqual({ name: "main.ledger", content: "bad" });
  expect(response.errors).toHaveLength(1);
  expect(response.errors[0].message).toBe("Bad entry");
  expect(response.output).toBe("validation output");

  mockJSON({ errors: null, output: "" });
  await expect(validateEditorFile({ name: "main.ledger", content: "" })).resolves.toEqual({
    errors: [],
    output: ""
  });
});

test("saveEditorFile distinguishes saved and failed responses", async () => {
  mockJSON({ errors: [], saved: true, file: { ...ledgerFile, versions: [] } });

  const saved = await saveEditorFile({ name: "main.ledger", content: "2026-11-01 Fixture\n" });
  expect(calls[0].url).toBe("/api/editor/save");
  expect(postedBody()).toEqual({ name: "main.ledger", content: "2026-11-01 Fixture\n" });
  expect(saved.saved).toBe(true);
  if (saved.saved) {
    expect(saved.file.name).toBe("main.ledger");
    expect(saved.file.type).toBe("file");
  }

  mockJSON({
    errors: [{ file_name: "", line_from: 1, line_to: 1, error: "", message: "Bad entry" }],
    saved: false,
    message: "Validation failed"
  });
  const failed = await saveEditorFile({ name: "main.ledger", content: "bad" });
  expect(failed.saved).toBe(false);
  expect(failed.message).toBe("Validation failed");
  expect(failed.errors).toHaveLength(1);
});

test("requestApi surfaces the {error} body on validation failures", async () => {
  mockJSON({ error: "Invalid file name" }, 400);

  await expect(getEditorFile("../secret.ledger")).rejects.toThrow("Invalid file name");
});
