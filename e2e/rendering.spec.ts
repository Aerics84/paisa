import { expect, test } from "@playwright/test";

function collectBrowserErrors(page: import("@playwright/test").Page) {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  return errors;
}

test("recurring radial progress renders on the dashboard", async ({ page }) => {
  const errors = collectBrowserErrors(page);

  await page.goto("/");
  await page.waitForLoadState("networkidle");

  const ring = page.locator('.radial-progress[role="progressbar"]');
  await expect(ring).toBeVisible();
  const value = await ring.evaluate((el) =>
    getComputedStyle(el).getPropertyValue("--value").trim()
  );
  expect(value).not.toBe("");
  expect(Number(value)).toBeGreaterThanOrEqual(0);
  await expect(ring).toHaveAttribute("aria-valuenow", /\d+(\.\d+)?/);
  await expect(ring).toHaveAttribute("aria-valuetext", /due|fällig/);

  expect(errors).toEqual([]);
});

test("depth range renders in the navbar dropdown", async ({ page }) => {
  const errors = collectBrowserErrors(page);

  await page.goto("/cash_flow/yearly");
  await page.waitForLoadState("networkidle");

  const trigger = page.getByRole("button", { name: "Tiefe konfigurieren" });
  await expect(trigger).toBeVisible();
  await page.locator(".dropdown.is-hoverable").filter({ has: trigger }).hover();
  await expect(page.locator(".range").first()).toBeVisible();

  expect(errors).toEqual([]);
});

test("file tree chevron rotates between closed and open", async ({ page }) => {
  await page.goto("/ledger/editor");
  await page.waitForLoadState("networkidle");

  const folder = page.locator(".file-tree summary", { hasText: "archive" });
  const details = page.locator(".file-tree details").filter({ hasText: "archive" }).first();
  await expect(folder).toBeVisible();

  const closed = await folder.evaluate((el) => getComputedStyle(el, "::after").rotate);
  await folder.click();
  await expect(details).toHaveAttribute("open", "");
  await expect
    .poll(() => folder.evaluate((el) => getComputedStyle(el, "::after").rotate))
    .toBe("45deg");
  const open = await folder.evaluate((el) => getComputedStyle(el, "::after").rotate);

  expect(closed).toBe("-135deg");
  expect(open).toBe("45deg");
});
