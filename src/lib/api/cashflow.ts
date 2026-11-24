import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import {
  postingsFromWire,
  transactionsFromWire,
  type Posting,
  type Transaction,
  type WirePosting
} from "./ledger";
import type {
  AccountBudget as WireAccountBudget,
  Budget as WireBudget,
  BudgetResponse as WireBudgetResponse,
  CashFlow as WireCashFlow,
  CashFlowResponse as WireCashFlowResponse,
  ExpenseBreakdown as WireExpenseBreakdown,
  ExpenseResponse as WireExpenseResponse,
  Graph as WireGraph,
  Income as WireIncome,
  IncomeResponse as WireIncomeResponse,
  IncomeStatement as WireIncomeStatement,
  IncomeStatementResponse as WireIncomeStatementResponse,
  IncomeYearlyCard as WireIncomeYearlyCard,
  Link as WireLink,
  Node as WireNode,
  RecurringResponse as WireRecurringResponse,
  Tax as WireIncomeTax,
  TransactionSequence as WireTransactionSequence
} from "./types.gen";

/** Cashflow graph has no dates, so the wire shape can be reused. */
export type Graph = WireGraph;
export type Node = WireNode;
export type Link = WireLink;

/** Cashflow point with the month start converted to dayjs. */
export type CashFlow = Omit<WireCashFlow, "date"> & { date: Dayjs };

/** Income statement with its financial-year start converted to dayjs. */
export type IncomeStatement = Omit<WireIncomeStatement, "date"> & { date: Dayjs };

/** Monthly income timeline entry with posting dates converted. */
export type Income = Omit<WireIncome, "date" | "postings"> & {
  date: Dayjs;
  postings: Posting[];
};

/** Tax timeline entry with both financial-year bounds and postings converted. */
export type IncomeTax = Omit<WireIncomeTax, "start_date" | "end_date" | "postings"> & {
  start_date: Dayjs;
  end_date: Dayjs;
  postings: Posting[];
};

/** Yearly income card with financial-year bounds and postings converted. */
export type IncomeYearlyCard = Omit<
  WireIncomeYearlyCard,
  "start_date" | "end_date" | "postings"
> & {
  start_date: Dayjs;
  end_date: Dayjs;
  postings: Posting[];
};

export type AccountBudget = Omit<WireAccountBudget, "date" | "expenses"> & {
  date: Dayjs;
  expenses: Posting[];
  /**
   * Not part of the wire budget payload; kept for legacy callers. The backend
   * sends `forecast` as the budgeted amount.
   */
  budgeted?: number;
};

export type Budget = Omit<WireBudget, "date" | "accounts"> & {
  date: Dayjs;
  accounts: AccountBudget[];
};

export interface BudgetResponse {
  budgetsByMonth: Record<string, Budget>;
  checkingBalance: number;
  availableForBudgeting: number;
}

export interface ExpenseBreakdown {
  expenses: Record<string, Posting[]>;
  incomes: Record<string, Posting[]>;
  investments: Record<string, Posting[]>;
  taxes: Record<string, Posting[]>;
}

export interface ExpenseResponse {
  expenses: Posting[];
  month_wise: ExpenseBreakdown;
  year_wise: ExpenseBreakdown;
  graph: Record<string, Graph>;
}

export interface IncomeResponse {
  income_timeline: Income[];
  tax_timeline: IncomeTax[];
  yearly_cards: IncomeYearlyCard[];
}

export interface CashFlowResponse {
  cash_flows: CashFlow[];
}

export interface IncomeStatementResponse {
  yearly: Record<string, IncomeStatement>;
}

/**
 * Wire recurring sequence mapped to domain transactions. The scheduling fields
 * are derived by `enrichTrantionSequence` in `$lib/transaction_sequence`.
 */
export type TransactionSequenceInput = Omit<WireTransactionSequence, "transactions"> & {
  transactions: Transaction[];
};

export interface RecurringResponse {
  transaction_sequences: TransactionSequenceInput[];
}

function postingMapFromWire(
  wire: Record<string, WirePosting[] | null> | null | undefined
): Record<string, Posting[]> {
  const result: Record<string, Posting[]> = {};
  for (const [bucket, postings] of Object.entries(wire ?? {})) {
    result[bucket] = postingsFromWire(postings ?? []);
  }
  return result;
}

function expenseBreakdownFromWire(wire: WireExpenseBreakdown | null | undefined): ExpenseBreakdown {
  return {
    expenses: postingMapFromWire(wire?.expenses),
    incomes: postingMapFromWire(wire?.incomes),
    investments: postingMapFromWire(wire?.investments),
    taxes: postingMapFromWire(wire?.taxes)
  };
}

function incomeFromWire(wire: WireIncome): Income {
  return {
    ...wire,
    date: parseDate(wire.date),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

function incomeTaxFromWire(wire: WireIncomeTax): IncomeTax {
  return {
    ...wire,
    start_date: parseDate(wire.start_date),
    end_date: parseDate(wire.end_date),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

function incomeYearlyCardFromWire(wire: WireIncomeYearlyCard): IncomeYearlyCard {
  return {
    ...wire,
    start_date: parseDate(wire.start_date),
    end_date: parseDate(wire.end_date),
    postings: postingsFromWire(wire.postings ?? [])
  };
}

/**
 * Shared with the dashboard payload, which embeds the same budget objects.
 * Exported so there is a single mapping from wire budget accounts to domain.
 */
export function accountBudgetFromWire(wire: WireAccountBudget): AccountBudget {
  return {
    ...wire,
    date: parseDate(wire.date),
    expenses: postingsFromWire(wire.expenses ?? [])
  };
}

/** Shared with the dashboard payload. See `accountBudgetFromWire`. */
export function budgetFromWire(wire: WireBudget): Budget {
  return {
    ...wire,
    date: parseDate(wire.date),
    accounts: (wire.accounts ?? []).map(accountBudgetFromWire)
  };
}

/** Shared with the dashboard payload. */
export function cashFlowFromWire(wire: WireCashFlow): CashFlow {
  return { ...wire, date: parseDate(wire.date) };
}

function incomeStatementFromWire(wire: WireIncomeStatement): IncomeStatement {
  return { ...wire, date: parseDate(wire.date) };
}

/** Shared with the dashboard payload. */
export function transactionSequenceFromWire(
  wire: WireTransactionSequence
): TransactionSequenceInput {
  return { ...wire, transactions: transactionsFromWire(wire.transactions ?? []) };
}

export async function getIncome(): Promise<IncomeResponse> {
  const wire = await request<WireIncomeResponse>("/api/income");
  return {
    income_timeline: (wire.income_timeline ?? []).map(incomeFromWire),
    tax_timeline: (wire.tax_timeline ?? []).map(incomeTaxFromWire),
    yearly_cards: (wire.yearly_cards ?? []).map(incomeYearlyCardFromWire)
  };
}

export async function getExpense(): Promise<ExpenseResponse> {
  const wire = await request<WireExpenseResponse>("/api/expense");
  return {
    expenses: postingsFromWire(wire.expenses ?? []),
    month_wise: expenseBreakdownFromWire(wire.month_wise),
    year_wise: expenseBreakdownFromWire(wire.year_wise),
    graph: wire.graph ?? {}
  };
}

export async function getBudget(): Promise<BudgetResponse> {
  const wire = await request<WireBudgetResponse>("/api/budget");

  const budgetsByMonth: Record<string, Budget> = {};
  for (const [month, budget] of Object.entries(wire.budgetsByMonth ?? {})) {
    budgetsByMonth[month] = budgetFromWire(budget);
  }

  return {
    budgetsByMonth,
    checkingBalance: wire.checkingBalance,
    availableForBudgeting: wire.availableForBudgeting
  };
}

export async function getCashFlow(): Promise<CashFlowResponse> {
  const wire = await request<WireCashFlowResponse>("/api/cash_flow");
  return { cash_flows: (wire.cash_flows ?? []).map(cashFlowFromWire) };
}

export async function getIncomeStatement(): Promise<IncomeStatementResponse> {
  const wire = await request<WireIncomeStatementResponse>("/api/income_statement");

  const yearly: Record<string, IncomeStatement> = {};
  for (const [year, statement] of Object.entries(wire.yearly ?? {})) {
    yearly[year] = incomeStatementFromWire(statement);
  }

  return { yearly };
}

export async function getRecurring(): Promise<RecurringResponse> {
  const wire = await request<WireRecurringResponse>("/api/recurring");
  return {
    transaction_sequences: (wire.transaction_sequences ?? []).map(transactionSequenceFromWire)
  };
}
