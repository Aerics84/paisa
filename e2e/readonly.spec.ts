import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

let server: Awaited<ReturnType<typeof startServer>>;

test.beforeAll(async () => {
  server = await startServer({
    name: "readonly",
    port: 5762,
    config: { readonly: true, ledger_cli: "hledger" }
  });
});

test.afterAll(async () => {
  await server?.stop();
});

test("readonly mode is advertised and POST /api/config is a no-op", async ({ page, request }) => {
  await page.goto(`${server.url}/more/config`);
  await expect(page.getByTestId("config-page")).toBeVisible();
  await expect(
    page
      .getByText(/^(readonly|schreibgeschützt)$/)
      .filter({ visible: true })
      .first()
  ).toBeVisible();

  const before = readFileSync(server.configPath);
  const response = await request.post(`${server.url}/api/config`, {
    data: {
      journal_path: "other.ledger",
      db_path: "other.db",
      readonly: false,
      ledger_cli: "hledger"
    }
  });

  expect(response.status()).toBe(200);
  expect(await response.json()).toEqual({ success: true });
  expect(readFileSync(server.configPath).equals(before)).toBe(true);
});
