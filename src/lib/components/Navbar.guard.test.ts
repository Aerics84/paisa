import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("Navbar breadcrumb guard", () => {
  test("uses the shared breadcrumb resolver and item list", () => {
    const source = readFileSync(join(__dirname, "Navbar.svelte"), "utf-8");
    expect(source).toContain("resolveBreadcrumbTrail");
    expect(source).toContain("breadcrumbItems");
    expect(source).toContain("breadcrumb.length > 0");
    expect(source).toContain("afterNavigate");
  });
});
