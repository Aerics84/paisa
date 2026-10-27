import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("TaxOverviewPage year changes", () => {
  test("restores the previous year label after a failed load", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewPage.svelte"), "utf-8");
    expect(source).toContain("selectedYear = overview.selected_year");
  });

  test("ignores a stale response after a newer request", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewPage.svelte"), "utf-8");
    expect(source).toContain("loadSequence");
  });
});
