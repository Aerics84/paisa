import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("ledger import page guards", () => {
  test("normalises a null template response", () => {
    const source = readFileSync(join(__dirname, "+page.svelte"), "utf-8");
    expect(source).toMatch(/templates\s*=\s*templates\s*(\|\||\?\?)\s*\[\s*\]/);
    expect(source).toMatch(/if\s*\(\s*templates\.length\s*>\s*0\s*\)/);
  });
});
