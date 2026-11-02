import { describe, expect, test } from "bun:test";
import { sanitizeSubtitle } from "./level_item";

describe("sanitizeSubtitle", () => {
  test("keeps bold and line-break markup", () => {
    expect(sanitizeSubtitle("25× Yearly Expenses<br/>Projected <b>01 Jan 2030</b>")).toBe(
      "25× Yearly Expenses<br>Projected <b>01 Jan 2030</b>"
    );
  });

  test("escapes all other markup", () => {
    expect(sanitizeSubtitle('<script>alert("x")</script>')).toBe(
      '&lt;script&gt;alert("x")&lt;/script&gt;'
    );
  });
});
