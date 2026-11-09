import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import { shouldShowNoDues } from "./due_date";

describe("shouldShowNoDues", () => {
  test("hides a rounded bill that has no meaningful amount due", () => {
    expect(shouldShowNoDues(null, 0.4)).toBe(true);
  });

  test("shows a bill with a full currency unit due", () => {
    expect(shouldShowNoDues(null, 1)).toBe(false);
  });

  test("shows a paid bill even when its remainder is below one currency unit", () => {
    expect(shouldShowNoDues(dayjs("2022-02-10"), 0.4)).toBe(false);
  });
});
