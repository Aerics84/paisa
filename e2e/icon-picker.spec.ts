import { expect, test } from "@playwright/test";

async function openFirstIconField(page: import("@playwright/test").Page) {
  await page.goto("/more/config");
  await page.waitForLoadState("networkidle");

  await page
    .getByRole("button", { name: /^Accounts$/ })
    .first()
    .click();
  await page
    .getByRole("button", { name: /Assets:Checking:ING/ })
    .first()
    .click();

  return page.getByLabel("Icon").first();
}

test("icon picker loads icons when opened and searches them", async ({ page }) => {
  const iconInput = await openFirstIconField(page);
  await iconInput.focus();
  await expect(iconInput).toBeFocused();
  await expect(page.locator(".list-item")).toHaveCount(0);

  await page.keyboard.press("ArrowDown");
  await expect(page.locator(".list-item").first()).toBeVisible();

  await iconInput.fill("mdi:home");
  await expect(page.locator(".list-item").filter({ hasText: "mdi:home" }).first()).toBeVisible();
});

test("tabbing through the config form does not load icon metadata", async ({ page }) => {
  // Strip lazy icon fonts from the configuration so the layout preload does not
  // fetch them before the keyboard interaction under test.
  await page.route("**/api/config", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    if (body?.config) {
      body.config.accounts = (body.config.accounts || []).map((account: unknown) =>
        account && typeof account === "object" ? { ...account, icon: "" } : account
      );
      body.config.goals = {};
    }
    await route.fulfill({ response, json: body });
  });

  const scriptRequests: string[] = [];
  page.on("request", (request) => {
    if (request.resourceType() === "script") {
      scriptRequests.push(request.url());
    }
  });

  const iconInput = await openFirstIconField(page);
  await iconInput.focus();
  await expect(iconInput).toBeFocused();

  const before = scriptRequests.length;
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await page.waitForTimeout(300);

  expect(scriptRequests.length).toBe(before);
});
