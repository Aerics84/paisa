import * as d3 from "d3";
import _ from "lodash";
import { lastName, type Posting } from "./utils";

export const ROOT_EXPENSE_SCOPE = "Expenses";

function scopeParts(scope: string) {
  return scope.split(":").filter(Boolean);
}

function accountParts(posting: Posting | string) {
  const account = typeof posting === "string" ? posting : posting.account;
  return account.split(":").filter(Boolean);
}

export function isInExpenseScope(posting: Posting | string, scope: string = ROOT_EXPENSE_SCOPE) {
  const account = typeof posting === "string" ? posting : posting.account;
  return account === scope || account.startsWith(scope + ":");
}

export function filterExpenseScope(
  expenses: Posting[],
  scope: string = ROOT_EXPENSE_SCOPE
): Posting[] {
  return _.filter(expenses, (expense) => isInExpenseScope(expense, scope));
}

export function childExpenseScope(scope: string, child: string) {
  return `${scope}:${child}`;
}

export function expenseBreadcrumb(scope: string) {
  const parts = scopeParts(scope);
  return parts.map((_part, index) => {
    const value = parts.slice(0, index + 1).join(":");
    return {
      label: parts[index],
      scope: value
    };
  });
}

export function expenseGroup(posting: Posting, scope: string = ROOT_EXPENSE_SCOPE) {
  const scopeDepth = scopeParts(scope).length;
  const parts = accountParts(posting);

  if (!isInExpenseScope(parts.join(":"), scope) || parts.length <= scopeDepth) {
    return null;
  }

  return parts[scopeDepth];
}

export function expenseColorKey(posting: Posting, scope: string = ROOT_EXPENSE_SCOPE) {
  return expenseGroup(posting, scope) || lastName(scope);
}

export function expenseTopLevelGroup(scope: string) {
  const parts = scopeParts(scope);
  return parts[1] || null;
}

export function expenseColorKeys(expenses: Posting[]) {
  return _.chain(expenses)
    .flatMap((expense) => accountParts(expense).slice(1))
    .uniq()
    .sort()
    .value();
}

export function pieData(expenses: Posting[], scope: string = ROOT_EXPENSE_SCOPE) {
  return d3
    .pie<{ category: string; total: number }>()
    .value((g) => g.total)
    .sort((a, b) => a.category.localeCompare(b.category))(
    _.values(byExpenseGroup(expenses, scope))
  );
}

export function byExpenseGroup(expenses: Posting[], scope: string = ROOT_EXPENSE_SCOPE) {
  return _.chain(filterExpenseScope(expenses, scope))
    .groupBy((expense) => expenseGroup(expense, scope))
    .omitBy((_ps, category) => category === "null" || category === "undefined")
    .mapValues((ps, category) => {
      return {
        category: category,
        scope: childExpenseScope(scope, category),
        postings: ps,
        total: _.sumBy(ps, (p) => p.amount)
      };
    })
    .value();
}
