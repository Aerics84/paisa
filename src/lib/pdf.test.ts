import { describe, expect, test } from "bun:test";
import fs from "node:fs";
import * as pdfjs from "pdfjs-dist/legacy/build/pdf.mjs";
import { pdf2array } from "./pdf";

const file = "fixture/import/SBI Credit Card Statement/monthly.pdf";
const goldenFile = "fixture/import/SBI Credit Card Statement/monthly.rows.json";
const bytes = new Uint8Array(fs.readFileSync(file));
const data = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
const load = async () => pdfjs;

describe("pdf fixture", () => {
  test("extracts the SBI statement unchanged", async () => {
    const doc = await pdfjs.getDocument({ data: data.slice(0) }).promise;
    expect(doc.numPages).toBe(7);
    const firstPage = await doc.getPage(1);
    const text = await firstPage.getTextContent();
    expect(text.items.length).toBe(448);

    const rows = await pdf2array(data.slice(0), load);

    // Golden darf nur einmalig aus der alten pdfjs-Version erzeugt werden.
    if (process.env.UPDATE_PDF_GOLDEN === "1" && !fs.existsSync(goldenFile)) {
      fs.writeFileSync(goldenFile, JSON.stringify(rows, null, 2) + "\n");
    }

    const golden = JSON.parse(fs.readFileSync(goldenFile, "utf8"));
    expect(rows).toEqual(golden);
  });

  test("rejects malformed PDF data", async () => {
    await expect(pdf2array(new Uint8Array([1, 2, 3]).buffer, load)).rejects.toThrow();
  });
});
