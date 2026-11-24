import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import type { BalancedPosting as GeneratedBalancedPosting } from "./accounting.gen";
import type { Posting as GeneratedPosting } from "./posting.gen";
import type { Transaction as GeneratedTransaction } from "./transaction.gen";
import type { BalanceResponse, LedgerResponse, TransactionResponse } from "./types.gen";

/**
 * Wire posting: dates are RFC3339 strings, amounts are JSON numbers.
 *
 * Exported so callers can distinguish the untouched API payload from the
 * domain posting.
 */
export type WirePosting = GeneratedPosting;

/**
 * Domain posting: `date` is a Dayjs instance, everything else is the wire
 * value.
 */
export type Posting = Omit<WirePosting, "date"> & { date: Dayjs };

/** Wire transaction: dates and postings are still raw API values. */
export type WireTransaction = GeneratedTransaction;

/** Domain transaction: transaction and posting dates are Dayjs instances. */
export type Transaction = Omit<WireTransaction, "date" | "postings"> & {
  date: Dayjs;
  postings: Posting[];
};

/** Wire balanced posting pair from GET /api/transaction/balanced. */
export type WireBalancedPosting = GeneratedBalancedPosting;

/** Domain balanced posting pair with both sides converted. */
export type BalancedPosting = Omit<WireBalancedPosting, "from" | "to"> & {
  from: Posting;
  to: Posting;
};

export function postingFromWire(wire: WirePosting): Posting {
  return { ...wire, date: parseDate(wire.date) };
}

export function postingsFromWire(wire: readonly WirePosting[]): Posting[] {
  return wire.map(postingFromWire);
}

export function transactionFromWire(wire: WireTransaction): Transaction {
  return {
    ...wire,
    date: parseDate(wire.date),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

export function transactionsFromWire(wire: readonly WireTransaction[]): Transaction[] {
  return wire.map(transactionFromWire);
}

export function balancedPostingFromWire(wire: WireBalancedPosting): BalancedPosting {
  return {
    from: postingFromWire(wire.from),
    to: postingFromWire(wire.to)
  };
}

export async function getLedger(): Promise<{ postings: Posting[] }> {
  const response = await request<LedgerResponse>("/api/ledger");
  return { postings: postingsFromWire(response.postings ?? []) };
}

export async function getTransactions(): Promise<{ transactions: Transaction[] }> {
  const response = await request<TransactionResponse>("/api/transaction");
  return { transactions: transactionsFromWire(response.transactions ?? []) };
}

export async function getBalancedPostings(): Promise<{ balancedPostings: BalancedPosting[] }> {
  const response = await request<BalanceResponse>("/api/transaction/balanced");
  return {
    balancedPostings: (response.balancedPostings ?? []).map(balancedPostingFromWire)
  };
}
