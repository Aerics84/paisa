import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { deleteSheetBackups, getSheetFile, getSheetFiles, saveSheetFile } from "./sheets";

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

const sheetFile = {
  name: "budget.paisa",
  content: "value = 1\n",
  versions: [] as string[],
  operation: ""
};

const posting = {
  id: 1,
  transaction_id: "t1",
  date: "2026-10-31T00:00:00Z",
  payee: "Fixture Payee",
  account: "Assets:Checking",
  commodity: "INR",
  quantity: 100,
  amount: 100,
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
  market_amount: 100,
  balance: 100
};

function postedBody(index = 0) {
  return JSON.parse(String(calls[index].init?.body));
}

test("getSheetFiles maps posting dates and defaults null collections", async () => {
  mockJSON({ files: [sheetFile], postings: [posting] });

  const response = await getSheetFiles();
  expect(calls[0].url).toBe("/api/sheets/files");
  expect(response.files).toHaveLength(1);
  expect(response.files[0].name).toBe("budget.paisa");
  expect(response.files[0].type).toBe("file");
  expect(response.postings).toHaveLength(1);
  expect(dayjs.isDayjs(response.postings[0].date)).toBe(true);
  expect(response.postings[0].date.format("YYYY-MM-DD")).toBe("2026-10-31");

  mockJSON({ files: null, postings: null });
  await expect(getSheetFiles()).resolves.toEqual({ files: [], postings: [] });
});

test("getSheetFile posts the file name and returns the typed file", async () => {
  mockJSON({ file: sheetFile });

  const response = await getSheetFile("budget.paisa");
  expect(calls[0].url).toBe("/api/sheets/file");
  expect(calls[0].init?.method).toBe("POST");
  expect(postedBody()).toEqual({ name: "budget.paisa" });
  expect(response.file.name).toBe("budget.paisa");
  expect(response.file.type).toBe("file");
  expect(response.file.versions).toEqual([]);

  mockJSON({ message: "Invalid file name" });
  await expect(getSheetFile("../secret.paisa")).rejects.toThrow("Invalid file name");
});

test("deleteSheetBackups returns the refreshed file", async () => {
  mockJSON({ file: sheetFile });

  const response = await deleteSheetBackups("budget.paisa");
  expect(calls[0].url).toBe("/api/sheets/file/delete_backups");
  expect(postedBody()).toEqual({ name: "budget.paisa" });
  expect(response.file.versions).toEqual([]);
});

test("saveSheetFile distinguishes saved and failed responses", async () => {
  mockJSON({ saved: true, file: sheetFile });

  const saved = await saveSheetFile({ name: "budget.paisa", content: "value = 1\n" });
  expect(calls[0].url).toBe("/api/sheets/save");
  expect(postedBody()).toEqual({ name: "budget.paisa", content: "value = 1\n" });
  expect(saved.saved).toBe(true);
  if (saved.saved) {
    expect(saved.file.name).toBe("budget.paisa");
  }

  mockJSON({ saved: false, message: "File already exists" });
  const failed = await saveSheetFile({ name: "budget.paisa", content: "", operation: "create" });
  expect(failed.saved).toBe(false);
  expect(failed.message).toBe("File already exists");
});
