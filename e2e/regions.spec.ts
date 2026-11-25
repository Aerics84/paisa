import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

let germany: Awaited<ReturnType<typeof startServer>>;
let india: Awaited<ReturnType<typeof startServer>>;

test.beforeAll(async () => {
  [germany, india] = await Promise.all([
    startServer({
      name: "regions-germany",
      port: 5764,
      config: { ledger_cli: "hledger" },
      args: ["--now", "2022-02-15"]
    }),
    startServer({
      name: "regions-india",
      port: 5765,
      fixture: "tests/fixture/inr-hledger",
      config: { ledger_cli: "hledger" },
      args: ["--now", "2022-02-15"]
    })
  ]);
});

test.afterAll(async () => {
  await Promise.all([germany?.stop(), india?.stop()]);
});

test("Germany/EU and India keep currency, locale and tax regimes separate", async ({
  page,
  request
}) => {
  for (const [label, server] of [
    ["germany", germany],
    ["india", india]
  ] as const) {
    const synced = await request.post(`${server.url}/api/sync`, { data: { journal: true } });
    expect(synced.ok(), `${label} journal sync failed`).toBe(true);
    expect((await synced.json()).success, `${label} journal sync failed`).toBe(true);
  }

  const germanyConfig = await (await request.get(`${germany.url}/api/config`)).json();
  expect(germanyConfig.config.default_currency).toBe("EUR");
  expect(germanyConfig.config.locale).toBe("de-DE");
  expect(germanyConfig.config.tax_regime).toBe("germany");

  const indiaConfig = await (await request.get(`${india.url}/api/config`)).json();
  expect(indiaConfig.config.default_currency).toBe("INR");
  expect(indiaConfig.config.locale).toBe("en-IN");
  expect(indiaConfig.config.tax_regime).toBe("india");

  await page.goto(`${germany.url}/`);
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  const germanBody = await page.locator("body").innerText();
  expect(germanBody).toContain("€");
  expect(germanBody).toMatch(/4\.800/);
  expect(germanBody).not.toContain("₹");
  await expect(page.locator('a[href="/tax/overview"]')).toHaveCount(1);
  await expect(page.locator('a[href="/tax/capital_income"]')).toHaveCount(1);
  await expect(page.locator('a[href="/tax/harvest"]')).toHaveCount(0);
  await expect(page.locator('a[href="/tax/schedule_al"]')).toHaveCount(0);

  await page.goto(`${india.url}/`);
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  const indianBody = await page.locator("body").innerText();
  expect(indianBody).toContain("₹");
  expect(indianBody).toMatch(/1,00,000/);
  expect(indianBody).not.toContain("€");
  await expect(page.locator('a[href="/tax/harvest"]')).toHaveCount(1);
  await expect(page.locator('a[href="/tax/capital_gains"]')).toHaveCount(1);
  await expect(page.locator('a[href="/tax/schedule_al"]')).toHaveCount(1);
  await expect(page.locator('a[href="/tax/overview"]')).toHaveCount(0);
});
