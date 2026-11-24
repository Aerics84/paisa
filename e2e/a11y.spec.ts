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
  "/more/sheets"
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

    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      // CodeMirror's internal scroll container is a third-party wrapper around
      // the focusable `.cm-content` textbox; flagging the wrapper is a false
      // positive and it cannot be configured from application code.
      .exclude(".cm-scroller")
      .analyze();
    const serious = results.violations.filter((violation) =>
      ["serious", "critical"].includes(violation.impact ?? "")
    );

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
