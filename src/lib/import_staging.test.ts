import { describe, expect, test } from "bun:test";
import { defaultIncludedTransactions, mergeIngestedTransactions } from "./import_staging";
import type { StagedTransaction } from "./utils";

function staged(overrides: Partial<StagedTransaction> = {}): StagedTransaction {
  return {
    index: 0,
    date: "2026-01-01",
    payee: "Coffee Shop",
    memo: "",
    amount: 4.5,
    is_debit: true,
    selected_account: "",
    ...overrides
  };
}

describe("defaultIncludedTransactions", () => {
  test("includes only assigned, non-duplicate rows", () => {
    const included = defaultIncludedTransactions([
      staged({ index: 0, selected_account: "Expenses:Coffee" }),
      staged({ index: 1, selected_account: "" }),
      staged({ index: 2, selected_account: "Expenses:Coffee", is_duplicate: true }),
      staged({ index: 3, selected_account: "Expenses:Coffee", is_duplicate: false })
    ]);

    expect(included).toEqual([true, false, false, true]);
  });

  test("keeps suggested-only rows unchecked", () => {
    const included = defaultIncludedTransactions([
      staged({ suggested_account: "Expenses:Coffee", confidence: "ai_suggested" })
    ]);

    expect(included).toEqual([false]);
  });
});

describe("mergeIngestedTransactions", () => {
  test("preserves manually typed accounts and their inclusion choice", () => {
    const existing = [
      staged({ index: 0, selected_account: "Expenses:Manual", confidence: "manual" })
    ];
    const incoming = [
      staged({
        index: 0,
        selected_account: "Expenses:Rule",
        confidence: "rule",
        matched_rule_name: "Coffee Rule"
      })
    ];

    const result = mergeIngestedTransactions(existing, [false], incoming);

    expect(result.transactions[0].selected_account).toBe("Expenses:Manual");
    expect(result.transactions[0].confidence).toBe("manual");
    expect(result.transactions[0].matched_rule_name).toBe("Coffee Rule");
    expect(result.included).toEqual([false]);
  });

  test("preserves accepted suggestions", () => {
    const existing = [
      staged({ index: 0, selected_account: "Expenses:Accepted", confidence: "accepted" })
    ];
    const incoming = [staged({ index: 0, selected_account: "Expenses:Rule", confidence: "rule" })];

    const result = mergeIngestedTransactions(existing, [true], incoming);

    expect(result.transactions[0].selected_account).toBe("Expenses:Accepted");
    expect(result.transactions[0].confidence).toBe("accepted");
    expect(result.included).toEqual([true]);
  });

  test("applies a new rule to untouched rows without changing their inclusion", () => {
    const existing = [
      staged({ index: 0, selected_account: "Expenses:Old", confidence: "rule" }),
      staged({ index: 1, selected_account: "", confidence: undefined })
    ];
    const incoming = [
      staged({
        index: 0,
        selected_account: "Expenses:New",
        confidence: "rule",
        matched_rule_name: "New Rule"
      }),
      staged({
        index: 1,
        selected_account: "Expenses:Coffee",
        confidence: "rule",
        matched_rule_name: "Coffee Rule"
      })
    ];

    const result = mergeIngestedTransactions(existing, [true, false], incoming);

    expect(result.transactions[0].selected_account).toBe("Expenses:New");
    expect(result.transactions[0].confidence).toBe("rule");
    expect(result.transactions[1].selected_account).toBe("Expenses:Coffee");
    expect(result.included).toEqual([true, false]);
  });

  test("clears an automatically assigned account when a rule no longer matches", () => {
    const existing = [staged({ index: 0, selected_account: "Expenses:Old", confidence: "rule" })];
    const incoming = [staged({ index: 0, selected_account: "", confidence: undefined })];

    const result = mergeIngestedTransactions(existing, [true], incoming);

    expect(result.transactions[0].selected_account).toBe("");
    expect(result.included).toEqual([false]);
  });

  test("matches fallback rows by date + payee + memo + amount when indexes shift", () => {
    const existing = [
      staged({
        index: 9,
        payee: "Starbucks",
        date: "2026-02-02",
        amount: 7.25,
        selected_account: "Expenses:Coffee",
        confidence: "manual"
      })
    ];
    const incoming = [
      staged({
        index: 0,
        payee: "Starbucks",
        date: "2026-02-02",
        amount: 7.25,
        selected_account: "Expenses:Rule",
        confidence: "rule"
      })
    ];

    const result = mergeIngestedTransactions(existing, [false], incoming);

    expect(result.transactions[0].selected_account).toBe("Expenses:Coffee");
    expect(result.included).toEqual([false]);
  });

  test("uses the default policy for newly appearing rows", () => {
    const existing = [staged({ index: 0, payee: "Coffee", selected_account: "Expenses:Coffee" })];
    const incoming = [
      staged({ index: 0, payee: "Coffee", selected_account: "Expenses:Coffee" }),
      staged({ index: 1, payee: "Groceries", selected_account: "Expenses:Groceries" }),
      staged({ index: 2, payee: "Unknown", selected_account: "" }),
      staged({ index: 3, payee: "Duplicate", selected_account: "Expenses:Dup", is_duplicate: true })
    ];

    const result = mergeIngestedTransactions(existing, [false], incoming);

    expect(result.included).toEqual([false, true, false, false]);
  });
});
