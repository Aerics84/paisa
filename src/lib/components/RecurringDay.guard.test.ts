import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("recurring calendar layout", () => {
  test("limits grid rows and gives each day an internal scroll area", () => {
    const page = readFileSync(
      join(__dirname, "..", "..", "routes", "(app)", "cash_flow", "recurring", "+page.svelte"),
      "utf-8"
    );
    expect(page).toContain("recurring-calendar-grid");
    expect(page).toContain("grid-auto-rows: minmax(0, 1fr)");
    expect(page).toContain("height: calc(100vh - 150px)");
    expect(page).toContain("@media (min-width: 768px)");
  });

  test("pins seven explicit columns instead of relying on Bulma grid utilities", () => {
    const page = readFileSync(
      join(__dirname, "..", "..", "routes", "(app)", "cash_flow", "recurring", "+page.svelte"),
      "utf-8"
    );
    expect(page).toContain("recurring-weekdays-grid");
    expect(page).toContain("grid-template-columns: repeat(7, minmax(0, 1fr))");
    expect(page).toContain("display: grid");
    expect(page).toContain("min-width: 768px");
    expect(page).not.toContain("grid-cols-7");
  });

  test("keeps the day header outside the scrolling schedule list", () => {
    const day = readFileSync(join(__dirname, "RecurringDay.svelte"), "utf-8");
    expect(day).toContain("recurring-day__schedules");
    expect(day.indexOf("recurring-day__schedules")).toBeGreaterThan(
      day.indexOf("has-text-centered")
    );
  });

  test("preview page uses the same structure", () => {
    const preview = readFileSync(
      join(__dirname, "..", "..", "routes", "preview", "cash_flow", "recurring", "+page.svelte"),
      "utf-8"
    );
    expect(preview).toContain("recurring-calendar-grid");
  });

  test("keeps the desktop height inside the CSS breakpoint", () => {
    const pages = [
      join(__dirname, "..", "..", "routes", "(app)", "cash_flow", "recurring", "+page.svelte"),
      join(__dirname, "..", "..", "routes", "preview", "cash_flow", "recurring", "+page.svelte")
    ];
    for (const page of pages) {
      const source = readFileSync(page, "utf-8");
      expect(source).not.toContain('isMobile() ? "" : "height: calc(100vh - 150px);"');
      expect(source).toMatch(/@media \(min-width: 768px\)[\s\S]*height: calc\(100vh - 150px\)/);
    }
  });
});
