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

  test("starts with every section collapsed", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewPage.svelte"), "utf-8");
    expect(source).toContain("data.sections.map((s) => [s.key, false])");
  });
});

describe("TaxOverviewSection rendering", () => {
  test("skips categories without postings", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewSection.svelte"), "utf-8");
    expect(source).toContain("category.postings.length > 0");
  });

  test("uses a fixed table layout so columns align across categories", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewSection.svelte"), "utf-8");
    expect(source).toContain("table-layout: fixed");
  });

  test("does not use the light tag that turns white in the dark theme", () => {
    const source = readFileSync(join(__dirname, "TaxOverviewSection.svelte"), "utf-8");
    expect(source).not.toContain("tag is-light");
  });
});
