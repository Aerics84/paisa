import { get } from "svelte/store";
import { t } from "./i18n";
import type { AllocationTarget } from "./utils";

export interface AllocationTargetEntry {
  name: string;
  target: number;
  drift: number;
  accounts: string[];
}

export const DEFAULT_DRIFT_TOLERANCE = 5;

/**
 * Maps an allocation target API response to an editable target entry.
 *
 * `drift` in the response is the current drift (current - target), while the
 * editable drift band is `drift_tolerance`. Never use the former for editing.
 */
export function allocationTargetToEntry(target: AllocationTarget): AllocationTargetEntry {
  const tolerance = Number(target.drift_tolerance);
  return {
    name: target.name || "",
    target: Number(target.target) || 0,
    drift: Number.isFinite(tolerance) && tolerance > 0 ? tolerance : DEFAULT_DRIFT_TOLERANCE,
    accounts: [...(target.accounts || [])]
  };
}

export function allocationTargetsToEntries(targets: AllocationTarget[]): AllocationTargetEntry[] {
  return (targets || []).map(allocationTargetToEntry);
}

export function validateAllocationTargetEntries(entries: AllocationTargetEntry[]): string | null {
  const i18n = get(t);
  let total = 0;
  for (const entry of entries) {
    if (!entry.name.trim()) {
      return i18n("components.allocationTargets.validation.namesRequired");
    }
    if (entry.target < 0 || entry.target > 100) {
      return i18n("components.allocationTargets.validation.targetRange", { name: entry.name });
    }
    if (entry.drift < 0 || entry.drift > 100) {
      return i18n("components.allocationTargets.validation.driftRange", { name: entry.name });
    }
    if (entry.accounts.length === 0) {
      return i18n("components.allocationTargets.validation.accountsRequired", {
        name: entry.name
      });
    }
    total += Number(entry.target) || 0;
  }

  if (entries.length > 0 && Math.abs(total - 100) > 0.01) {
    return i18n("components.allocationTargets.validation.sum", { total });
  }

  return null;
}
