import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { startServer } from "./helpers/server.mjs";

test.describe.configure({ mode: "serial" });

let server: Awaited<ReturnType<typeof startServer>>;

function hasHledger() {
  try {
    execFileSync("hledger", ["--version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

test.beforeAll(async () => {
  test.skip(!hasHledger(), "hledger is required to validate and sync the imported CSV");
  server = await startServer({
    name: "import",
    port: 5763,
    config: { ledger_cli: "hledger" }
  });
});

test.afterAll(async () => {
  await server?.stop();
});

test("CSV statement is staged, committed and visible in the ledger", async ({ page }) => {
  await page.goto(`${server.url}/ledger/import`);
  await expect(page.getByTestId("import-page")).toBeVisible();

  await page.getByRole("link", { name: /Statement Staging|Kontoauszüge vorbereiten/ }).click();
  await expect(
    page.getByRole("heading", { name: /Staged Transactions|Vorbereitete Buchungen/ })
  ).toBeVisible();

  const dropzone = page.locator('input[type=file][accept*=".ofx"]');
  await dropzone.setInputFiles(join(import.meta.dirname, "fixtures", "sample.csv"));

  await expect(page.getByText("Coffee Shop", { exact: true })).toBeVisible();
  await expect(page.getByText("Book Store", { exact: true })).toBeVisible();

  await page.locator("#statement-base-account").fill("Assets:Checking:ING");

  const debitAccounts = page.locator('input[list="staging-debit-accounts"]');
  await expect(debitAccounts).toHaveCount(2);
  await debitAccounts.nth(0).fill("Expenses:Food");
  await debitAccounts.nth(1).fill("Expenses:Leisure");

  const commit = page.getByRole("button", {
    name: /Commit 2 transactions|2 Buchungen übernehmen/
  });
  await expect(commit).toBeEnabled();
  await commit.click();

  await expect(
    page.getByText(
      /Successfully appended 2 transactions|2 Buchungen erfolgreich an den Ledger angehängt/
    )
  ).toBeVisible({ timeout: 20_000 });

  const journal = join(server.dir, "main.ledger");
  const content = readFileSync(journal, "utf8");
  expect(content).toContain("Coffee Shop");
  expect(content).toContain("Book Store");
  // The germany-eu journal declares `format 1.000,00 EUR`; a dot here would
  // make hledger read 12.50 as 1250.
  expect(content).toContain("-12,50 EUR");
  expect(content).toContain("-30,00 EUR");

  await page.goto(`${server.url}/ledger/posting`);
  await expect(page.getByText(/Coffee Shop/).first()).toBeVisible();
  await expect(page.getByText(/Book Store/).first()).toBeVisible();
});
