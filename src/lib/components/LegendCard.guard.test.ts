import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("LegendCard layout", () => {
  test("wraps entries and allows long labels to wrap", () => {
    const source = readFileSync(join(__dirname, "LegendCard.svelte"), "utf-8");
    expect(source).toContain("flex-wrap");
    expect(source).toContain("gap-x-4");
    expect(source).toContain("gap-y-2");
    expect(source).toContain("whitespace-pre-wrap");
    expect(source).toContain("break-words");
    expect(source).not.toContain("gap-0 ");
    expect(source).not.toContain("whitespace-pre ");
  });

  test("keeps legend boxes from shrinking", () => {
    const source = readFileSync(join(__dirname, "LegendCard.svelte"), "utf-8");
    expect(source).toContain("shrink-0");
    expect(source).toContain("min-w-0");
  });
});
