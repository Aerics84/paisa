import { describe, expect, test } from "bun:test";
import chroma from "chroma-js";
import dayjs from "dayjs";
import { EXPENSE_CATEGORY_PALETTE, expenseCategoryColorScheme } from "./colors";
import { colorScale, resolveExpenseColor } from "./expense/monthly";
import { ROOT_EXPENSE_SCOPE } from "./expense";
import type { Posting } from "./utils";

function posting(account: string, amount = 10): Posting {
  return {
    id: account,
    transaction_id: account,
    date: dayjs("2026-01-05"),
    payee: account,
    account,
    commodity: "EUR",
    quantity: amount,
    amount,
    status: "",
    tag_recurring: "",
    transaction_begin_line: 1,
    transaction_end_line: 1,
    file_name: "main.ledger",
    note: "",
    transaction_note: "",
    market_amount: amount,
    balance: amount
  };
}

describe("expense category colors", () => {
  const knownCategories = [
    "Food",
    "Housing",
    "Transport",
    "Dining",
    "Utilities",
    "Shopping",
    "Health",
    "Entertainment",
    "Savings",
    "Other",
    "Insurance",
    "Travel",
    "Education",
    "Gifts",
    "Fees",
    "Subscriptions"
  ];

  test("first 16 known top-level categories are perceptually distinct", () => {
    const z = expenseCategoryColorScheme(knownCategories);
    const colors = knownCategories.map((category) => z(category));
    expect(new Set(colors).size).toBe(16);
    for (let i = 0; i < colors.length; i++) {
      for (let j = i + 1; j < colors.length; j++) {
        expect(chroma.deltaE(colors[i], colors[j])).toBeGreaterThanOrEqual(8);
      }
    }
    expect(EXPENSE_CATEGORY_PALETTE.length).toBeGreaterThanOrEqual(16);
  });

  test("assignment is independent of input order", () => {
    const forward = expenseCategoryColorScheme(["Food", "Car", "Health"]);
    const reversed = expenseCategoryColorScheme(["Health", "Food", "Car"]);
    for (const category of ["Food", "Car", "Health"]) {
      expect(forward(category)).toBe(reversed(category));
    }
  });

  test("root colors stay stable when another top-level category appears", () => {
    const before = colorScale([posting("Expenses:Food")]);
    const after = colorScale([posting("Expenses:Food"), posting("Expenses:Car")]);
    expect(before("Expenses:Food")).toBe(after("Expenses:Food"));
  });

  test("child colors stay stable when a sibling category appears", () => {
    const before = colorScale([posting("Expenses:Food:Other")]);
    const after = colorScale([posting("Expenses:Food:Groceries"), posting("Expenses:Food:Other")]);
    expect(before("Expenses:Food:Other")).toBe(after("Expenses:Food:Other"));
  });

  test("the same leaf name under different parents gets different colors", () => {
    const z = colorScale([posting("Expenses:Food:Other"), posting("Expenses:Transport:Other")]);
    expect(z("Expenses:Food:Other")).not.toBe(z("Expenses:Transport:Other"));
  });

  test("children keep the exact hue of their top-level family", () => {
    const z = colorScale([posting("Expenses:Food"), posting("Expenses:Food:Other")]);
    const rootHue = chroma(z("Expenses:Food")).get("hsl.h");
    const childHue = chroma(z("Expenses:Food:Other")).get("hsl.h");
    expect(Math.abs(rootHue - childHue)).toBeLessThanOrEqual(1);
  });

  test("more than 16 top-level categories stay readable", () => {
    const postings = Array.from({ length: 20 }, (_, i) => posting(`Expenses:Category${i}`));
    const z = colorScale(postings);
    for (let i = 0; i < 20; i++) {
      const lightness = chroma(z(`${ROOT_EXPENSE_SCOPE}:Category${i}`)).get("hsl.l");
      expect(lightness).toBeGreaterThanOrEqual(0.28);
      expect(lightness).toBeLessThanOrEqual(0.78);
    }
  });

  test("resolveExpenseColor works with hierarchical and flat scales", () => {
    const hierarchical = colorScale([posting("Expenses:Food")]);
    const flat = expenseCategoryColorScheme(["Food"]);
    expect(resolveExpenseColor(hierarchical, "Expenses:Food")).toBe(hierarchical("Expenses:Food"));
    expect(resolveExpenseColor(flat, "Expenses:Food")).toBe(flat("Food"));
  });
});
