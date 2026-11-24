import { expect, test } from "@playwright/test";

function collectBrowserErrors(page: import("@playwright/test").Page) {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  return errors;
}

test("dashboard renders with the germany-eu fixture", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
});

test("config page loads without client errors", async ({ page }) => {
  const errors = collectBrowserErrors(page);

  await page.goto("/more/config");
  await expect(page.getByTestId("config-page")).toBeVisible();
  expect(errors).toEqual([]);
});

test("import page exposes the staging flow", async ({ page }) => {
  await page.goto("/ledger/import");
  await expect(page.getByTestId("import-page")).toBeVisible();
  await expect(page.getByTestId("file-dropzone")).toBeVisible();
});

test("replacement UI components render without client errors", async ({ page }) => {
  const errors = collectBrowserErrors(page);

  await page.goto("/expense/monthly");
  await expect(page.locator(".boxed-tabs")).toBeVisible();
  await expect(page.locator(".boxed-tab.is-active")).toHaveCount(1);
  await expect(page.locator(".boxed-tab.is-active")).toHaveAttribute("aria-pressed", "true");

  await page.goto("/ledger/editor");
  await expect(page.locator(".file-tree")).toBeVisible();

  await page.goto("/more/config");
  await expect(page.getByTestId("config-page")).toBeVisible();

  expect(errors).toEqual([]);
});
