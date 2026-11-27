import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const read = (...parts: string[]) => readFileSync(join(__dirname, ...parts), "utf-8");

describe("preview dashboard settings", () => {
  test("dialog is labelled, keyboard closable and preset driven", () => {
    const source = read("PreviewDashboardSettings.svelte");
    expect(source).toContain('role="dialog"');
    expect(source).toContain('ariaModal="true"');
    expect(source).toContain('$t("dashboard.customizeTitle")');
    expect(source).toContain('$t("dashboard.resetDefault")');
    expect(source).toContain('event.key === "Escape"');
    expect(source).toContain("PREVIEW_PRESETS");
    expect(source).toContain("setWidgetVisible");
    expect(source).toContain("returnFocusTo");
    expect(source).toContain("dialogElement");
    expect(source).not.toMatch(/\bon:click=/);
  });

  test("page gates the four existing sections through the layout", () => {
    const page = read("..", "..", "routes", "preview", "+page.svelte");
    expect(page).toContain("dashboardLayout.visible.hero");
    expect(page).toContain("dashboardLayout.visible.monthlyReview");
    expect(page).toContain("dashboardLayout.visible.insights");
    expect(page).toContain("dashboardLayout.visible.support");
    expect(page).toContain("PreviewDashboardSettings");
    expect(page).toContain("saveLayout");
    expect(page).toContain('data-testid="preview-customize"');
  });
});
