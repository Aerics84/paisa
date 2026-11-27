import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const read = (...parts: string[]) => readFileSync(join(__dirname, ...parts), "utf-8");
const scss = readFileSync(join(__dirname, "..", "..", "paisa-dashboard-overview.scss"), "utf-8");

describe("preview recent transactions", () => {
  test("renders transaction cards in a responsive grid", () => {
    const source = read("PreviewRecentTransactions.svelte");
    expect(source).toContain("TransactionCard");
    expect(source).toContain("paisa-recent-grid");
    expect(source).toContain("/preview/ledger/transaction");
    expect(source).toContain('$t("dashboard.recentTransactions")');
    expect(source).toContain('$t("dashboard.viewAllTransactions")');
    expect(scss).toContain(".paisa-recent-grid");
    expect(scss).toContain("minmax(min(100%");
  });

  test("card links keep the preview prefix", () => {
    const card = read("TransactionCard.svelte");
    expect(card).toContain('linkPrefix = ""');
    expect(card).toContain("linkPrefix + postingUrl(posting)");
  });

  test("page loads, gates and reveals the recent transactions", () => {
    const page = read("..", "..", "routes", "preview", "+page.svelte");
    expect(page).toContain("latestTransactions(dashboard.transactions");
    expect(page).toContain("dashboardLayout.visible.recentTransactions");
    expect(page).toContain("transactions.length > 0");
    expect(page).toContain("PreviewRecentTransactions");
  });
});
