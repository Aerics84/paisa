import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

async function seriousViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  return results.violations
    .filter((violation) => ["serious", "critical"].includes(violation.impact ?? ""))
    .map(
      (violation) =>
        `${violation.id}: ${violation.nodes.map((node) => node.target.join(" ")).join(", ")}`
    );
}

const seededBaseURL = `http://127.0.0.1:${process.env.PAISA_E2E_SEEDED_PORT || "5759"}`;

test("preview dashboard presets persist", async ({ page }) => {
  await page.goto("/preview");
  await expect(page.getByTestId("preview-section-support")).toBeVisible();

  await page.getByTestId("preview-customize").click();
  await page.getByTestId("preview-preset-investor").click();
  await page.getByTestId("preview-settings-done").click();

  await expect(page.getByTestId("preview-section-support")).toHaveCount(0);
  await expect(page.getByTestId("preview-section-insights")).toBeVisible();

  await page.reload();
  await expect(page.getByTestId("preview-section-support")).toHaveCount(0);

  await page.getByTestId("preview-customize").click();
  await page.getByTestId("preview-settings-reset").click();
  await page.getByTestId("preview-settings-done").click();
  await expect(page.getByTestId("preview-section-support")).toBeVisible();
});

test("manual toggles survive reload and do not leak across presets", async ({ page }) => {
  await page.goto("/preview");
  await page.getByTestId("preview-customize").click();
  await page.getByTestId("preview-widget-insights").uncheck();
  await page.getByTestId("preview-settings-done").click();
  await expect(page.getByTestId("preview-section-insights")).toHaveCount(0);

  await page.reload();
  await expect(page.getByTestId("preview-section-insights")).toHaveCount(0);

  await page.getByTestId("preview-customize").click();
  await page.getByTestId("preview-preset-overview").click();
  await page.getByTestId("preview-settings-done").click();
  await expect(page.getByTestId("preview-section-insights")).toBeVisible();
});

test("corrupted layout storage falls back to overview", async ({ page }) => {
  await page.goto("/preview");
  await page.evaluate(() => localStorage.setItem("preview-dashboard-layout-v1", "{not json"));
  await page.reload();
  await expect(page.getByTestId("preview-section-hero")).toBeVisible();
  await expect(page.getByTestId("preview-section-support")).toBeVisible();
});

test.describe("recent transactions", () => {
  test.use({ baseURL: seededBaseURL });

  test("render with preview links", async ({ page }) => {
    await page.goto("/preview");
    const section = page.getByTestId("preview-section-recent-transactions");
    await expect(section).toBeVisible();
    await expect(section.getByRole("link", { name: /View all|Alle anzeigen/ })).toHaveAttribute(
      "href",
      "/preview/ledger/transaction"
    );
  });
});

test.describe("mobile drawer", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("traps focus, locks scroll and restores focus", async ({ page }) => {
    await page.goto("/preview");
    const toggle = page.getByTestId("preview-nav-toggle");
    await expect(toggle).toBeVisible();
    await toggle.click();

    const nav = page.locator("#preview-navigation");
    await expect(nav).toHaveClass(/paisa-shell-sidebar--open/);
    await expect(page.locator(".paisa-shell-content")).toHaveAttribute("inert", "");

    expect(await page.evaluate(() => document.body.style.overflow)).toBe("hidden");
    expect(
      await page.evaluate(() => document.activeElement?.closest("#preview-navigation") !== null)
    ).toBe(true);

    await page.keyboard.press("Escape");
    await expect(nav).not.toHaveClass(/paisa-shell-sidebar--open/);
    expect(await page.evaluate(() => document.activeElement?.getAttribute("data-testid"))).toBe(
      "preview-nav-toggle"
    );
    expect(await page.evaluate(() => document.body.style.overflow)).toBe("");
  });
});

test("dark theme passes serious a11y checks", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("theme-preference", "dark"));
  await page.goto("/preview");
  await page.waitForLoadState("networkidle");
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  expect(await seriousViolations(page)).toEqual([]);
});
