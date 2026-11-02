import type { StagedTransaction } from "./utils";

/**
 * The default inclusion policy for a freshly ingested statement: only rows that
 * the backend already classified may be committed automatically. Suggested but
 * unassigned rows and possible duplicates stay unchecked until the user acts.
 */
export function defaultIncludedTransactions(transactions: StagedTransaction[]): boolean[] {
  return transactions.map((tx) => !tx.is_duplicate && !!tx.selected_account);
}

function rowKey(tx: StagedTransaction): string {
  return [tx.date, tx.payee, tx.memo, tx.amount, tx.is_debit ? "debit" : "credit"].join("|");
}

function isManualDecision(tx: StagedTransaction): boolean {
  return tx.confidence === "manual" || tx.confidence === "accepted";
}

type ExistingRow = {
  transaction: StagedTransaction;
  included: boolean;
};

/**
 * Merge a fresh ingestion result into the rows already shown in the staging
 * table. A rule change may reclassify untouched rows, but must never throw away
 * accounts the user typed/accepted or an inclusion choice they made.
 *
 * Rows are matched by their backend index first and by
 * date + payee + memo + amount + direction as a fallback. Rows that only exist
 * in the new result use the normal default inclusion policy.
 */
export function mergeIngestedTransactions(
  existing: StagedTransaction[],
  existingIncluded: boolean[],
  incoming: StagedTransaction[]
): { transactions: StagedTransaction[]; included: boolean[] } {
  const byIndex = new Map<number, ExistingRow>();
  const byKey = new Map<string, ExistingRow>();

  existing.forEach((transaction, idx) => {
    const row = { transaction, included: !!existingIncluded[idx] };
    byIndex.set(transaction.index, row);
    if (!byKey.has(rowKey(transaction))) {
      byKey.set(rowKey(transaction), row);
    }
  });

  const matched = incoming.map((transaction) => {
    return byIndex.get(transaction.index) ?? byKey.get(rowKey(transaction)) ?? null;
  });

  const transactions = incoming.map((fresh, idx) => {
    const normalized = {
      ...fresh,
      selected_account: fresh.selected_account || ""
    };
    const previous = matched[idx];
    if (!previous) {
      return normalized;
    }

    if (isManualDecision(previous.transaction)) {
      return {
        ...normalized,
        selected_account: previous.transaction.selected_account,
        confidence: previous.transaction.confidence
      };
    }

    return normalized;
  });

  const included = incoming.map((fresh, idx) => {
    const previous = matched[idx];
    if (previous) {
      // Preserve the user's choice, but never leave an unassigned row selected:
      // the commit API rejects rows without a target account.
      if (!transactions[idx].selected_account) {
        return false;
      }
      return previous.included;
    }
    return !fresh.is_duplicate && !!fresh.selected_account;
  });

  return { transactions, included };
}
