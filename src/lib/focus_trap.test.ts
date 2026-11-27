import { describe, expect, test } from "bun:test";
import { nextFocusIndex } from "./focus_trap";

describe("focus trap", () => {
  test("wraps forward and backward", () => {
    expect(nextFocusIndex(0, 3, false)).toBe(1);
    expect(nextFocusIndex(2, 3, false)).toBe(0);
    expect(nextFocusIndex(0, 3, true)).toBe(2);
    expect(nextFocusIndex(2, 3, true)).toBe(1);
  });

  test("handles unknown current index and empty lists", () => {
    expect(nextFocusIndex(-1, 3, false)).toBe(0);
    expect(nextFocusIndex(9, 3, true)).toBe(2);
    expect(nextFocusIndex(0, 0, false)).toBe(-1);
  });
});
