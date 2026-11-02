import { describe, expect, it } from "bun:test";
import { resolveFinancialYear } from "./financial_year";

describe("resolveFinancialYear", () => {
  it("keeps a year that is still available", () => {
    expect(resolveFinancialYear("2024-25", ["2025-26", "2024-25"])).toBe("2024-25");
  });

  it("defaults an empty year to the latest available", () => {
    expect(resolveFinancialYear("", ["2025-26", "2024-25"])).toBe("2025-26");
  });

  it("moves a year outside the range to the latest available", () => {
    expect(resolveFinancialYear("2023-24", ["2025-26"])).toBe("2025-26");
  });

  it("returns an empty year when no years are available", () => {
    expect(resolveFinancialYear("2024-25", [])).toBe("");
  });
});
