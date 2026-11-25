import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

function hasLedger() {
  try {
    execFileSync("ledger", ["--version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const needsLedger = "a real ledger CLI is required to sync the generated journal";
const profileButton = /Deutschland\/EU|Germany\/EU/;
const emptyButton = /Leer starten|Start empty/;
const submitButton = /Einrichtung starten|Start setup/;

test("a fresh installation redirects to onboarding and offers both profiles", async ({ page }) => {
  const server = await startServer({ name: "onboarding-ui", port: 5766, firstRun: true });
  try {
    const response = await page.request.get(`${server.url}/api/onboarding`);
    expect(response.status()).toBe(200);
    const status = await response.json();
    expect(status.needed).toBe(true);
    expect(status.profiles.map((profile: { id: string }) => profile.id)).toEqual([
      "germany-eu",
      "india"
    ]);

    await page.goto(`${server.url}/`);
    await expect(page).toHaveURL(`${server.url}/onboarding`);
    await expect(
      page.getByRole("heading", { name: /Willkommen bei Paisa|Welcome to Paisa/ })
    ).toBeVisible();
    await expect(page.getByRole("button", { name: profileButton })).toBeVisible();
    await expect(page.getByRole("button", { name: /Indien|India/ })).toBeVisible();
    await expect(page.getByRole("button", { name: emptyButton })).toBeVisible();
    await expect(page.getByRole("button", { name: /Demo-Daten|Demo data/ })).toBeVisible();
  } finally {
    await server.stop();
  }
});

test("Germany/EU empty setup writes the config and rejects a second submit", async ({ page }) => {
  test.skip(!hasLedger(), needsLedger);

  const server = await startServer({ name: "onboarding-empty", port: 5767, firstRun: true });
  try {
    await page.goto(`${server.url}/`);
    await expect(page).toHaveURL(`${server.url}/onboarding`);

    await page.getByRole("button", { name: profileButton }).click();
    await page.getByRole("button", { name: emptyButton }).click();
    await page.getByRole("button", { name: submitButton }).click();

    await expect(page).toHaveURL(`${server.url}/`, { timeout: 60_000 });
    await expect(page.getByTestId("dashboard-page")).toBeVisible();

    const content = readFileSync(server.configPath, "utf8");
    expect(content).toContain("regional_profile: germany-eu");
    expect(content).toContain("default_currency: EUR");

    const second = await page.request.post(`${server.url}/api/onboarding`, {
      data: { profile: "germany-eu", mode: "empty" }
    });
    expect(second.status()).toBe(409);
  } finally {
    await server.stop();
  }
});

test("concurrent setup attempts yield exactly one success and one conflict", async ({
  request
}) => {
  test.skip(!hasLedger(), needsLedger);

  const server = await startServer({ name: "onboarding-race", port: 5768, firstRun: true });
  try {
    const responses = await Promise.all([
      request.post(`${server.url}/api/onboarding`, {
        data: { profile: "germany-eu", mode: "empty" }
      }),
      request.post(`${server.url}/api/onboarding`, {
        data: { profile: "germany-eu", mode: "empty" }
      })
    ]);

    const statuses = responses.map((response) => response.status()).sort();
    expect(statuses).toEqual([200, 409]);

    const config = await (await request.get(`${server.url}/api/config`)).json();
    expect(config.config.regional_profile).toBe("germany-eu");
    expect(config.config.default_currency).toBe("EUR");
  } finally {
    await server.stop();
  }
});

test("demo setup seeds postings", async ({ request }) => {
  test.skip(!hasLedger(), needsLedger);

  const server = await startServer({ name: "onboarding-demo", port: 5769, firstRun: true });
  try {
    const response = await request.post(`${server.url}/api/onboarding`, {
      data: { profile: "germany-eu", mode: "demo" },
      timeout: 120_000
    });
    expect(response.status()).toBe(200);
    expect((await response.json()).success).toBe(true);

    const ledger = await (await request.get(`${server.url}/api/ledger`)).json();
    expect(Array.isArray(ledger.postings)).toBe(true);
    expect(ledger.postings.length).toBeGreaterThan(0);
  } finally {
    await server.stop();
  }
});
