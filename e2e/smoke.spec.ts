import { expect, test } from "@playwright/test";

test("dashboard renders with the germany-eu fixture", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
});

test("config page loads without client errors", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));

  await page.goto("/more/config");
  await expect(page.getByTestId("config-page")).toBeVisible();
  expect(errors).toEqual([]);
});

test("import page exposes the staging flow", async ({ page }) => {
  await page.goto("/ledger/import");
  await expect(page.getByTestId("import-page")).toBeVisible();
  await expect(page.getByTestId("file-dropzone")).toBeVisible();
});
