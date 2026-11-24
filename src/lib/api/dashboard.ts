import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import type { AssetBreakdown } from "./assets.gen";
import type { Posting, Transaction } from "./ledger";
import { postingsFromWire, transactionsFromWire } from "./ledger";
import type {
  AccountBudget as WireAccountBudget,
  Budget as WireBudget,
  BudgetResponse as WireBudgetResponse,
  CashFlow as WireCashFlow,
  DashboardResponse as WireDashboardResponse,
  ExpensePaceSummary as WireExpensePace,
  Networth as WireNetworth
} from "./types.gen";
import type { AccountBudget, Budget, CashFlow, ExpensePace, GoalSummary, Networth } from "../utils";

/** Wire recurring transaction sequence; transactions still carry string dates. */
export type WireTransactionSequence = WireDashboardResponse["transactionSequences"][number];

/** Recurring transaction sequence with posting/transaction dates converted. */
export type TransactionSequence = Omit<WireTransactionSequence, "transactions"> & {
  transactions: Transaction[];
};

export interface Dashboard {
  checkingBalances: { asset_breakdowns: Record<string, AssetBreakdown> };
  /**
   * `month` stays a wire string: the UI treats it as an opaque month bucket and
   * compares/sorts it as such.
   */
  checkingBalanceTimeline: WireDashboardResponse["checkingBalanceTimeline"];
  availableMonthRange: { from: Dayjs; to: Dayjs };
  networth: { networth: Networth; xirr: number };
  expenses: Record<string, Posting[]>;
  expense_pace: ExpensePace | null;
  cashFlows: CashFlow[];
  transactionSequences: TransactionSequence[];
  transactions: Transaction[];
  budget: { budgetsByMonth: Record<string, Budget> };
  goalSummaries: GoalSummary[];
}

function networthFromWire(wire: WireNetworth): Networth {
  return { ...wire, date: parseDate(wire.date) };
}

function cashFlowFromWire(wire: WireCashFlow): CashFlow {
  return { ...wire, date: parseDate(wire.date) };
}

function accountBudgetFromWire(wire: WireAccountBudget): AccountBudget {
  return {
    account: wire.account,
    date: parseDate(wire.date),
    actual: wire.actual,
    forecast: wire.forecast,
    available: wire.available,
    rollover: wire.rollover,
    expenses: postingsFromWire(wire.expenses ?? [])
  };
}

function budgetFromWire(wire: WireBudget): Budget {
  return {
    date: parseDate(wire.date),
    accounts: wire.accounts.map(accountBudgetFromWire),
    availableThisMonth: wire.availableThisMonth,
    endOfMonthBalance: wire.endOfMonthBalance,
    forecast: wire.forecast
  };
}

function budgetResponseFromWire(wire: WireBudgetResponse): Dashboard["budget"] {
  const budgetsByMonth: Record<string, Budget> = {};
  for (const [month, budget] of Object.entries(wire.budgetsByMonth ?? {})) {
    budgetsByMonth[month] = budgetFromWire(budget);
  }
  return { budgetsByMonth };
}

function sequenceFromWire(wire: WireTransactionSequence): TransactionSequence {
  return { ...wire, transactions: transactionsFromWire(wire.transactions ?? []) };
}

function expensePaceFromWire(wire: WireExpensePace | null | undefined): ExpensePace | null {
  if (!wire) {
    return null;
  }
  return {
    period: wire.period,
    projectedExpenses: wire.projectedExpenses,
    overBudget: wire.overBudget
  };
}

export async function getDashboard(): Promise<Dashboard> {
  const wire = await request<WireDashboardResponse>("/api/dashboard");

  const expenses: Record<string, Posting[]> = {};
  for (const [month, postings] of Object.entries(wire.expenses ?? {})) {
    expenses[month] = postingsFromWire(postings ?? []);
  }

  return {
    checkingBalances: {
      asset_breakdowns: wire.checkingBalances?.asset_breakdowns ?? {}
    },
    checkingBalanceTimeline: wire.checkingBalanceTimeline ?? [],
    availableMonthRange: {
      from: parseDate(wire.availableMonthRange.from),
      to: parseDate(wire.availableMonthRange.to)
    },
    networth: {
      networth: networthFromWire(wire.networth.networth),
      xirr: wire.networth.xirr
    },
    expenses,
    expense_pace: expensePaceFromWire(wire.expense_pace),
    cashFlows: (wire.cashFlows ?? []).map(cashFlowFromWire),
    transactionSequences: (wire.transactionSequences ?? []).map(sequenceFromWire),
    transactions: transactionsFromWire(wire.transactions ?? []),
    budget: budgetResponseFromWire(wire.budget),
    // The generated goal types use plain strings where the domain type narrows
    // them to the documented literal unions.
    goalSummaries: (wire.goalSummaries ?? []) as unknown as GoalSummary[]
  };
}
