import { describe, expect, test } from "bun:test";
import { format } from "./journal";
import fs from "fs";

function readFixture(name: string) {
  return fs.readFileSync(`fixture/${name}`).toString();
}

describe("journal", () => {
  test("format", () => {
    expect(format(readFixture("unformatted.ledger"))).toBe(readFixture("formatted.ledger"));
  });

  test("falls back when amount_alignment_column is missing or zero", () => {
    const previous = (globalThis as any).USER_CONFIG;
    (globalThis as any).USER_CONFIG = { amount_alignment_column: 0 };
    try {
      expect(() => format(readFixture("unformatted.ledger"))).not.toThrow();
    } finally {
      (globalThis as any).USER_CONFIG = previous;
    }
  });
});
