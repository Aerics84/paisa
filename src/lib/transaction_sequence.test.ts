import { describe, expect, test } from "bun:test";

import { nextUnpaidSchedule, sortTrantionSequence } from "./transaction_sequence";
import type { TransactionSequence } from "./utils";

function rawSequence(key: string): TransactionSequence {
  return {
    key,
    period: "",
    interval: 30,
    transactions: []
  } as TransactionSequence;
}

describe("transaction sequence helpers", () => {
  test("sorts sequences even before their schedules are enriched", () => {
    const sorted = sortTrantionSequence([rawSequence("Rent"), rawSequence("Electricity")]);

    expect(sorted.map((ts) => ts.key)).toEqual(["Rent", "Electricity"]);
    expect(nextUnpaidSchedule(sorted[0])).toBeUndefined();
  });
});
