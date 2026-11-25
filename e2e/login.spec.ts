import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

const legacyHash = `sha256:${createHash("sha256")
  .update(createHash("sha256").update("secret").digest("hex"))
  .digest("hex")}`;

let server: Awaited<ReturnType<typeof startServer>>;

test.beforeAll(async () => {
  server = await startServer({
    name: "login",
    port: 5761,
    config: {
      ledger_cli: "hledger",
      user_accounts: [{ username: "alice", password: legacyHash }]
    }
  });
});

test.afterAll(async () => {
  await server?.stop();
});

test("legacy sha256 login is upgraded to argon2id and survives a reload", async ({ page }) => {
  await page.goto(`${server.url}/login`);
  await page.locator("#login-username").fill("alice");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: /Anmelden|Sign in/ }).click();

  await expect(page).toHaveURL(`${server.url}/`);
  await expect(page.getByTestId("dashboard-page")).toBeVisible();

  await page.reload();
  await expect(page).toHaveURL(`${server.url}/`);
  await expect(page.getByTestId("dashboard-page")).toBeVisible();

  const content = readFileSync(server.configPath, "utf8");
  expect(content).toContain("username: alice");
  expect(content).toMatch(/username: alice[\s\S]*?password: argon2id\$/);
  expect(content).not.toContain(legacyHash);
});

test("a password typed in the config form is stored as argon2id", async ({ page }) => {
  await page.goto(`${server.url}/login`);
  await page.locator("#login-username").fill("alice");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: /Anmelden|Sign in/ }).click();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();

  await page.goto(`${server.url}/more/config`);
  await expect(page.getByTestId("config-page")).toBeVisible();

  const accountsToggle = page.getByRole("button", { name: /^User Accounts/ });
  if ((await accountsToggle.getAttribute("aria-expanded")) !== "true") {
    await accountsToggle.click();
  }
  await page.getByRole("button", { name: /Add User Accounts|User Accounts hinzufügen/ }).click();

  const userAccounts = page
    .locator(
      'button[aria-label="Add User Accounts"], button[aria-label="User Accounts hinzufügen"]'
    )
    .locator(
      'xpath=ancestor::div[contains(@class,"config-header")]/following-sibling::div[contains(@class,"config-body")]'
    );
  await userAccounts.locator(".config-header button").first().click();

  await userAccounts.getByLabel("Username").first().fill("bob");
  await userAccounts.getByLabel("Password").first().fill("bob-secret-pass");
  await userAccounts.getByLabel("Password").first().press("Tab");

  const save = page.getByRole("button", { name: /^(Speichern|Save)$/ });
  await expect(save).toBeEnabled({ timeout: 15_000 });

  const saved = page.waitForResponse(
    (response) =>
      response.url() === `${server.url}/api/config` && response.request().method() === "POST"
  );
  await save.click();
  expect((await saved).ok()).toBe(true);

  const content = readFileSync(server.configPath, "utf8");
  expect(content).toMatch(/username: bob\s*\n\s*password: argon2id\$/);
  expect(content).not.toContain("bob-secret-pass");
});
