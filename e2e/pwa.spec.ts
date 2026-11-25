import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

let server: Awaited<ReturnType<typeof startServer>>;

test.beforeAll(async () => {
  server = await startServer({
    name: "pwa",
    port: 5770,
    config: { ledger_cli: "hledger" }
  });
});

test.afterAll(async () => {
  await server?.stop();
});

test("service worker caches the shell, never finance data, and serves it offline", async ({
  page,
  context
}) => {
  await page.goto(`${server.url}/`);
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute(
    "href",
    "/manifest.webmanifest"
  );
  await expect(page.getByTestId("dashboard-page")).toBeVisible();

  await page.waitForFunction(() => Boolean(navigator.serviceWorker?.controller), null, {
    timeout: 30_000
  });
  const activeState = await page.evaluate(() =>
    navigator.serviceWorker.ready.then((registration) => registration.active?.state)
  );
  expect(activeState).toBe("activated");

  const cacheNames = await page.evaluate(() => caches.keys());
  expect(cacheNames.some((name) => name.startsWith("paisa-shell-"))).toBe(true);

  const cachedUrls = async () =>
    page.evaluate(async () => {
      const urls: string[] = [];
      for (const name of await caches.keys()) {
        const cache = await caches.open(name);
        for (const request of await cache.keys()) urls.push(request.url);
      }
      return urls;
    });

  const beforeApi = await cachedUrls();
  await page.evaluate(() => fetch("/api/config").then((response) => response.json()));
  const afterApi = await cachedUrls();
  expect(afterApi.filter((url) => url.includes("/api/"))).toEqual([]);
  expect(afterApi.length).toBe(beforeApi.length);

  // A controlled online reload ensures the worker owns the page before the
  // network is taken away.
  await page.reload();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();

  await context.setOffline(true);
  await page.reload({ waitUntil: "domcontentloaded" });

  await expect(
    page.locator('[data-testid="dashboard-page"], [data-testid="error-page"]').first()
  ).toBeVisible({ timeout: 30_000 });
  const body = await page.locator("body").innerText();
  expect(body).not.toContain("Acme GmbH");
  expect(body).not.toContain("4.800");
  expect(await page.title()).toBe("Paisa");
});
