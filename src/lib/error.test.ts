import { describe, expect, test } from "bun:test";
import { formatError, isResizeObserverLoopError } from "./error";

describe("error helpers", () => {
  test("does not crash on null", () => {
    expect(formatError(null)).toBe("Unknown error");
    expect(formatError(undefined)).toBe("Unknown error");
  });
  test("recognizes ResizeObserver loop warnings", () => {
    expect(isResizeObserverLoopError("ResizeObserver loop limit exceeded")).toBe(true);
    expect(isResizeObserverLoopError("Something else")).toBe(false);
  });
});
