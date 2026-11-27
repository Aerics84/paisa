import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const A11Y_ROUTES = [
  "/",
  "/login",
  "/cash_flow/income_statement",
  "/cash_flow/monthly",
  "/cash_flow/yearly",
  "/cash_flow/recurring",
  "/expense/monthly",
  "/expense/yearly",
  "/expense/budget",
  "/assets/allocation",
  "/assets/analysis",
  "/assets/balance",
  "/assets/gain",
  "/assets/investment",
  "/assets/networth",
  "/liabilities/balance",
  "/liabilities/credit_cards",
  "/liabilities/interest",
  "/liabilities/repayment",
  "/tax/overview",
  "/tax/capital_income",
  "/ledger/import",
  "/ledger/posting",
  "/ledger/transaction",
  "/ledger/price",
  "/ledger/editor",
  "/more/config",
  "/more/goals",
  "/more/sheets",
  "/preview",
  "/preview/cash_flow/monthly"
];

for (const path of A11Y_ROUTES) {
  test(`no serious a11y violations on ${path}`, async ({ page }) => {
    await page.goto(path);
    // Wait for async data and webfonts so layout-dependent rules (contrast,
    // scrollable regions) are evaluated against the final page.
    await page.waitForLoadState("networkidle");
    await page.evaluate(async () => {
      await document.fonts.ready;
    });

    // Under SPA fallback every URL returns 200, so only the rendered error page
    // proves the route still exists.
    await expect(page.getByTestId("error-page")).toHaveCount(0);

    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
    const serious = results.violations
      .filter((violation) => ["serious", "critical"].includes(violation.impact ?? ""))
      .map((violation) => ({
        ...violation,
        // CodeMirror's third-party scroll wrapper is a known false positive:
        // the focusable `.cm-content` textbox lives inside it. Drop only this
        // rule's reports that target the wrapper; every other rule still
        // audits the whole editor subtree.
        nodes:
          violation.id === "scrollable-region-focusable"
            ? violation.nodes.filter(
                (node) => !node.target.some((target) => String(target).includes(".cm-scroller"))
              )
            : violation.nodes
      }))
      .filter((violation) => violation.nodes.length > 0);

    expect(
      serious,
      serious
        .map(
          (violation) =>
            `${violation.id} (${violation.impact}): ${violation.help} — ${violation.nodes
              .map((node) => node.target.join(" "))
              .join(", ")}`
        )
        .join("\n")
    ).toEqual([]);
  });
}
