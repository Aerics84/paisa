import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import { postingsFromWire, transactionsFromWire, type Posting, type Transaction } from "./ledger";
import type {
  CreditCardBill as WireCreditCardBill,
  CreditCardResponse as WireCreditCardResponse,
  CreditCardSummary as WireCreditCardSummary,
  CreditCardsResponse as WireCreditCardsResponse
} from "./types.gen";

/**
 * Statement with every date converted to dayjs. `paidDate` is null while the
 * bill is outstanding, matching the API response.
 */
export type CreditCardBill = Omit<
  WireCreditCardBill,
  "statementStartDate" | "statementEndDate" | "dueDate" | "paidDate" | "postings" | "transactions"
> & {
  statementStartDate: Dayjs;
  statementEndDate: Dayjs;
  dueDate: Dayjs;
  paidDate: Dayjs | null;
  postings: Posting[];
  transactions: Transaction[];
};

/** Credit card summary with the expiration date and every bill converted. */
export type CreditCardSummary = Omit<WireCreditCardSummary, "bills" | "expirationDate"> & {
  bills: CreditCardBill[];
  expirationDate: Dayjs;
};

export interface CreditCardsResponse {
  creditCards: CreditCardSummary[];
}

export interface CreditCardResponse {
  creditCard?: CreditCardSummary;
  found: boolean;
}

export function creditCardBillFromWire(wire: WireCreditCardBill): CreditCardBill {
  return {
    ...wire,
    statementStartDate: parseDate(wire.statementStartDate),
    statementEndDate: parseDate(wire.statementEndDate),
    dueDate: parseDate(wire.dueDate),
    paidDate: wire.paidDate ? parseDate(wire.paidDate) : null,
    postings: postingsFromWire(wire.postings ?? []),
    transactions: transactionsFromWire(wire.transactions ?? [])
  };
}

export function creditCardFromWire(wire: WireCreditCardSummary): CreditCardSummary {
  return {
    ...wire,
    bills: (wire.bills ?? []).map(creditCardBillFromWire),
    yearlySpends: wire.yearlySpends ?? {},
    expirationDate: parseDate(wire.expirationDate)
  };
}

export async function getCreditCards(): Promise<CreditCardsResponse> {
  const wire = await request<WireCreditCardsResponse>("/api/credit_cards");
  return { creditCards: (wire.creditCards ?? []).map(creditCardFromWire) };
}

export async function getCreditCard(account: string): Promise<CreditCardResponse> {
  const wire = await request<WireCreditCardResponse>("/api/credit_cards/:account", undefined, {
    account
  });

  const response: CreditCardResponse = { found: wire.found };
  if (wire.creditCard) {
    response.creditCard = creditCardFromWire(wire.creditCard);
  }
  return response;
}
