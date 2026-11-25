import { beforeEach, describe, expect, test } from "bun:test";
import { setLocale } from "./i18n";
import { tooltip } from "./tooltip";

describe("tooltip", () => {
  beforeEach(() => setLocale("en"));

  test("truncates long lists but keeps the total", () => {
    const rows = Array.from({ length: 30 }, (_, i) => [`row ${i}`, `${i}`]);
    const html = tooltip(rows, { total: "123" });
    expect(html).toContain("earlier rows omitted");
    expect(html).toContain("Total");
    expect(html).toContain("123");
    expect(rows).toHaveLength(30);
  });

  test("keeps short lists intact", () => {
    const html = tooltip([["A", "1"]], { header: "Rows", total: "1" });
    expect(html).toContain("Rows");
    expect(html).toContain("Total");
    expect(html).not.toContain("omitted");
  });
});
