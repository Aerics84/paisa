import { describe, expect, test } from "bun:test";

import {
  allocationTargetsToEntries,
  allocationTargetToEntry,
  validateAllocationTargetEntries,
  type AllocationTargetEntry
} from "./allocation_targets";
import type { AllocationTarget } from "./utils";

function target(overrides: Partial<AllocationTarget> = {}): AllocationTarget {
  return {
    name: "Equity",
    target: 60,
    current: 70,
    aggregates: {},
    market_amount: 7000,
    drift: 10,
    drift_tolerance: 5,
    status: "overweight",
    accounts: ["Assets:Equity:*"],
    commodities: [],
    ...overrides
  };
}

function entry(overrides: Partial<AllocationTargetEntry> = {}): AllocationTargetEntry {
  return {
    name: "Equity",
    target: 60,
    drift: 5,
    accounts: ["Assets:Equity:*"],
    ...overrides
  };
}

describe("allocation target editing", () => {
  test("uses drift_tolerance as the drift band instead of actual drift", () => {
    const result = allocationTargetToEntry(target({ drift: 10, drift_tolerance: 3 }));

    expect(result.drift).toBe(3);
    expect(result.drift).not.toBe(10);
  });

  test("round-trips a configured tolerance that differs from current drift", () => {
    const response = target({ drift: -10, drift_tolerance: 2.5 });
    const [result] = allocationTargetsToEntries([response]);

    expect(result.drift).toBe(2.5);
    expect(result.name).toBe("Equity");
    expect(result.accounts).toEqual(["Assets:Equity:*"]);
  });

  test("falls back to the default tolerance when the configured value is absent", () => {
    const result = allocationTargetToEntry(target({ drift: 10, drift_tolerance: 0 }));

    expect(result.drift).toBe(5);
  });

  test("rejects targets whose percentages do not sum to 100", () => {
    const error = validateAllocationTargetEntries([
      entry({ name: "Equity", target: 60 }),
      entry({ name: "Debt", target: 30, accounts: ["Assets:Debt:*"] })
    ]);

    expect(error).toContain("sum to 100");
  });

  test("accepts targets whose percentages sum to 100", () => {
    const error = validateAllocationTargetEntries([
      entry({ name: "Equity", target: 60.25 }),
      entry({ name: "Debt", target: 39.75, accounts: ["Assets:Debt:*"] })
    ]);

    expect(error).toBeNull();
  });
});
