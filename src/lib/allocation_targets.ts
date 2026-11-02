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
  let total = 0;
  for (const entry of entries) {
    if (!entry.name.trim()) {
      return "All asset class names must be filled.";
    }
    if (entry.target < 0 || entry.target > 100) {
      return `Target percentage for ${entry.name} must be between 0 and 100.`;
    }
    if (entry.drift < 0 || entry.drift > 100) {
      return `Drift tolerance for ${entry.name} must be between 0 and 100.`;
    }
    if (entry.accounts.length === 0) {
      return `Please assign at least one account to "${entry.name}".`;
    }
    total += Number(entry.target) || 0;
  }

  if (entries.length > 0 && Math.abs(total - 100) > 0.01) {
    return `Target percentages must sum to 100 (currently ${total}%).`;
  }

  return null;
}
