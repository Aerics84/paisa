import { describe, expect, test } from "bun:test";
import { buildStatementRequest } from "./import_provenance";

describe("buildStatementRequest", () => {
  test("sends file provenance and locale for an uploaded CSV", () => {
    const request = buildStatementRequest({
      format: "csv",
      content: "Date,Description\n2026-01-01,Coffee",
      delimiter: ",",
      baseAccount: "Assets:Checking",
      fileName: "C:\\statements\\checking.CSV",
      locale: "de-DE"
    });

    expect(request).toEqual({
      format: "csv",
      content: "Date,Description\n2026-01-01,Coffee",
      delimiter: ",",
      base_account: "Assets:Checking",
      source_type: "csv",
      original_path: "checking.CSV",
      locale: "de-DE"
    });
  });

  test("derives the source type from the OFX and QFX extensions", () => {
    expect(
      buildStatementRequest({
        format: "ofx",
        content: "<OFX></OFX>",
        delimiter: ",",
        baseAccount: "",
        fileName: "october.ofx"
      }).source_type
    ).toBe("ofx");

    expect(
      buildStatementRequest({
        format: "qfx",
        content: "<OFX></OFX>",
        delimiter: ",",
        baseAccount: "",
        fileName: "october.qfx"
      }).source_type
    ).toBe("qfx");
  });

  test("uses a stable provenance for pasted statements", () => {
    const request = buildStatementRequest({
      format: "csv",
      content: "Date,Description\n2026-01-01,Coffee",
      delimiter: ",",
      baseAccount: "",
      pasted: true
    });

    expect(request.source_type).toBe("csv");
    expect(request.original_path).toBe("pasted.csv");
    expect(request.locale).toBe("en-IN");
  });

  test("falls back to en-IN when no locale is configured", () => {
    const request = buildStatementRequest({
      format: "csv",
      content: "",
      delimiter: ",",
      baseAccount: "",
      fileName: "checking.csv"
    });

    expect(request.locale).toBe("en-IN");
  });
});
