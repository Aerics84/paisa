import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("Navbar sub-link guard", () => {
  test("checks selectedSubLink before reading children", () => {
    const source = readFileSync(join(__dirname, "Navbar.svelte"), "utf-8");
    expect(source).toMatch(
      /if\s*\(\s*selectedSubLink\s*&&\s*!_\s*\.isEmpty\(\s*selectedSubLink\.children\s*\)\s*\)/
    );
  });
});
