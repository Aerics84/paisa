import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";

import {
  byExpenseGroup,
  expenseBreadcrumb,
  expenseColorKey,
  filterExpenseScope,
  hasExpenseChildGroups,
  normalizeExpenseDetailScope,
  ROOT_EXPENSE_SCOPE
} from "./expense";
import type { Posting } from "./utils";

function posting(account: string, amount: number): Posting {
  return {
    id: `${account}-${amount}`,
    date: dayjs("2026-02-10"),
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

describe("expense drilldown helpers", () => {
  const postings = [
    posting("Expenses:Food:Restaurants", 40),
    posting("Expenses:Food:Groceries", 25),
    posting("Expenses:Car:Fuel", 55),
    posting("Expenses:Insurance", 90)
  ];

  test("filters postings by root and nested scopes", () => {
    expect(filterExpenseScope(postings, ROOT_EXPENSE_SCOPE)).toHaveLength(4);
    expect(filterExpenseScope(postings, "Expenses:Food")).toHaveLength(2);
    expect(filterExpenseScope(postings, "Expenses:Food:Restaurants")).toHaveLength(1);
  });

  test("groups root scope by top-level category", () => {
    const groups = byExpenseGroup(postings, ROOT_EXPENSE_SCOPE);

    expect(Object.keys(groups).sort()).toEqual(["Car", "Food", "Insurance"]);
    expect(groups.Food.total).toBe(65);
    expect(groups.Food.scope).toBe("Expenses:Food");
  });

  test("groups nested scope by the next direct child segment", () => {
    const groups = byExpenseGroup(
      [
        posting("Expenses:Food:Restaurants:Lunch", 20),
        posting("Expenses:Food:Restaurants:Dinner", 30),
        posting("Expenses:Food:Groceries", 15)
      ],
      "Expenses:Food"
    );

    expect(Object.keys(groups).sort()).toEqual(["Groceries", "Restaurants"]);
    expect(groups.Restaurants.total).toBe(50);
    expect(groups.Restaurants.scope).toBe("Expenses:Food:Restaurants");
  });

  test("returns no child groups for leaf scopes", () => {
    expect(
      Object.keys(byExpenseGroup([posting("Expenses:Insurance", 90)], "Expenses:Insurance"))
    ).toEqual([]);
    expect(
      Object.keys(
        byExpenseGroup([posting("Expenses:Food:Restaurants", 40)], "Expenses:Food:Restaurants")
      )
    ).toEqual([]);
  });

  test("normalizes detail scope to root or one top-level category", () => {
    expect(normalizeExpenseDetailScope(ROOT_EXPENSE_SCOPE)).toBe(ROOT_EXPENSE_SCOPE);
    expect(normalizeExpenseDetailScope("Expenses:Food")).toBe("Expenses:Food");
    expect(normalizeExpenseDetailScope("Expenses:Food:Restaurants")).toBe("Expenses:Food");
    expect(normalizeExpenseDetailScope("Assets:Cash")).toBe(ROOT_EXPENSE_SCOPE);
  });

  test("detects whether a category has deeper child groups", () => {
    expect(hasExpenseChildGroups(postings, "Expenses:Food")).toBe(true);
    expect(hasExpenseChildGroups(postings, "Expenses:Insurance")).toBe(false);
    expect(hasExpenseChildGroups([posting("Expenses:Food", 20)], "Expenses:Food")).toBe(false);
  });

  test("builds breadcrumb paths and stable color keys for nested scopes", () => {
    expect(expenseBreadcrumb("Expenses:Food:Restaurants")).toEqual([
      { label: "Expenses", scope: "Expenses" },
      { label: "Food", scope: "Expenses:Food" },
      { label: "Restaurants", scope: "Expenses:Food:Restaurants" }
    ]);

    expect(expenseColorKey(posting("Expenses:Food:Restaurants:Lunch", 12), "Expenses:Food")).toBe(
      "Restaurants"
    );
    expect(
      expenseColorKey(posting("Expenses:Food:Restaurants", 12), "Expenses:Food:Restaurants")
    ).toBe("Restaurants");
  });
});
