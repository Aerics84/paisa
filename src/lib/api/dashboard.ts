import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import type { AssetBreakdown } from "./assets.gen";
import type { Posting, Transaction } from "./ledger";
import { postingsFromWire, transactionsFromWire } from "./ledger";
import { budgetFromWire, cashFlowFromWire, transactionSequenceFromWire } from "./cashflow";
import type { TransactionSequenceInput } from "./cashflow";
import type {
  BudgetResponse as WireBudgetResponse,
  DashboardResponse as WireDashboardResponse,
  ExpensePaceSummary as WireExpensePace,
  Networth as WireNetworth
} from "./types.gen";
import type { Budget, CashFlow, ExpensePace, GoalSummary, Networth } from "../utils";

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
  transactionSequences: TransactionSequenceInput[];
  transactions: Transaction[];
  budget: { budgetsByMonth: Record<string, Budget> };
  goalSummaries: GoalSummary[];
}

function networthFromWire(wire: WireNetworth): Networth {
  return { ...wire, date: parseDate(wire.date) };
}

function budgetResponseFromWire(wire: WireBudgetResponse): Dashboard["budget"] {
  const budgetsByMonth: Record<string, Budget> = {};
  for (const [month, budget] of Object.entries(wire.budgetsByMonth ?? {})) {
    budgetsByMonth[month] = budgetFromWire(budget);
  }
  return { budgetsByMonth };
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
    transactionSequences: (wire.transactionSequences ?? []).map(transactionSequenceFromWire),
    transactions: transactionsFromWire(wire.transactions ?? []),
    budget: budgetResponseFromWire(wire.budget),
    // The generated goal types use plain strings where the domain type narrows
    // them to the documented literal unions.
    goalSummaries: (wire.goalSummaries ?? []) as unknown as GoalSummary[]
  };
}
