import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("preview flow lifecycle", () => {
  test("resets the flow renderer when the review canvas disappears", () => {
    const source = readFileSync(join(__dirname, "+page.svelte"), "utf-8");
    expect(source).toMatch(/if\s*\(!activeMonthlyReview\)\s*\{[^}]*flowRenderer\.reset\(\)/s);
  });
});
