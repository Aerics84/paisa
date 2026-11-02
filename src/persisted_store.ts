import { persisted } from "svelte-local-storage-store";
import { writable, get } from "svelte/store";

export const obscure = persisted("obscure", false);

export const cashflowExpenseDepthAllowed = writable({ min: 1, max: 1 });
export const cashflowExpenseDepth = persisted("cashflowExpenseDepth", 0);
export const cashflowIncomeDepthAllowed = writable({ min: 1, max: 1 });
export const cashflowIncomeDepth = persisted("cashflowIncomeDepth", 0);

// name of the selected networth currency, empty for the default currency
export const selectedCurrency = persisted("selectedCurrency", "");

export function setCashflowDepthAllowed(expense: number, income: number) {
  cashflowExpenseDepthAllowed.set({ min: 1, max: expense });
  if (get(cashflowExpenseDepth) == 0 || get(cashflowExpenseDepth) > expense) {
    cashflowExpenseDepth.set(expense);
  }

  cashflowIncomeDepthAllowed.set({ min: 1, max: income });
  if (get(cashflowIncomeDepth) == 0 || get(cashflowIncomeDepth) > income) {
    cashflowIncomeDepth.set(income);
  }
}
