import { describe, expect, test } from "bun:test";
import { shouldDeferNavigation } from "./navigation_guard";

describe("shouldDeferNavigation", () => {
  test("dirty navigation proceeds after the user confirms leaving", () => {
    expect(shouldDeferNavigation({ dirty: true, confirmed: true })).toBe(false);
  });

  test("dirty navigation is deferred while the user chooses to stay", () => {
    expect(shouldDeferNavigation({ dirty: true, confirmed: false })).toBe(true);
  });

  test("clean navigation proceeds without asking", () => {
    expect(shouldDeferNavigation({ dirty: false })).toBe(false);
  });

  test("an already cancelled navigation stays cancelled", () => {
    expect(shouldDeferNavigation({ dirty: true, confirmed: true, cancelled: true })).toBe(true);
  });
});
