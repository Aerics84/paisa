import type { Transaction } from "./api/ledger";
import type { MessageKey } from "./i18n";

export type PreviewWidgetId =
  | "hero"
  | "monthlyReview"
  | "insights"
  | "support"
  | "recentTransactions";

export type PreviewPresetId = "overview" | "cashflow" | "investor" | "minimal";

export interface PreviewWidgetMeta {
  id: PreviewWidgetId;
  titleKey: MessageKey;
  descriptionKey: MessageKey;
}

export interface PreviewDashboardLayout {
  preset: PreviewPresetId | "custom";
  visible: Record<PreviewWidgetId, boolean>;
}

export const PREVIEW_WIDGETS: PreviewWidgetMeta[] = [
  {
    id: "hero",
    titleKey: "dashboard.widget.hero",
    descriptionKey: "dashboard.widget.hero.description"
  },
  {
    id: "monthlyReview",
    titleKey: "dashboard.widget.monthlyReview",
    descriptionKey: "dashboard.widget.monthlyReview.description"
  },
  {
    id: "insights",
    titleKey: "dashboard.widget.insights",
    descriptionKey: "dashboard.widget.insights.description"
  },
  {
    id: "support",
    titleKey: "dashboard.widget.support",
    descriptionKey: "dashboard.widget.support.description"
  },
  {
    id: "recentTransactions",
    titleKey: "dashboard.widget.recentTransactions",
    descriptionKey: "dashboard.widget.recentTransactions.description"
  }
];

const PREVIEW_WIDGET_IDS = PREVIEW_WIDGETS.map((widget) => widget.id);

const PRESET_VISIBILITY: Record<PreviewPresetId, PreviewWidgetId[]> = {
  overview: ["hero", "monthlyReview", "insights", "support", "recentTransactions"],
  cashflow: ["hero", "monthlyReview", "support", "recentTransactions"],
  investor: ["hero", "monthlyReview", "insights"],
  minimal: ["hero", "monthlyReview"]
};

export const PREVIEW_PRESETS: {
  id: PreviewPresetId;
  labelKey: MessageKey;
  descriptionKey: MessageKey;
}[] = [
  {
    id: "overview",
    labelKey: "dashboard.preset.overview",
    descriptionKey: "dashboard.preset.overview.description"
  },
  {
    id: "cashflow",
    labelKey: "dashboard.preset.cashflow",
    descriptionKey: "dashboard.preset.cashflow.description"
  },
  {
    id: "investor",
    labelKey: "dashboard.preset.investor",
    descriptionKey: "dashboard.preset.investor.description"
  },
  {
    id: "minimal",
    labelKey: "dashboard.preset.minimal",
    descriptionKey: "dashboard.preset.minimal.description"
  }
];

export const PREVIEW_LAYOUT_STORAGE_KEY = "preview-dashboard-layout-v1";

function isPresetId(value: unknown): value is PreviewPresetId {
  return (
    typeof value === "string" && Object.prototype.hasOwnProperty.call(PRESET_VISIBILITY, value)
  );
}

function visibleRecord(visible: PreviewWidgetId[]): Record<PreviewWidgetId, boolean> {
  const record = Object.fromEntries(
    PREVIEW_WIDGET_IDS.map((id) => [id, visible.includes(id)])
  ) as Record<PreviewWidgetId, boolean>;
  return record;
}

export function presetLayout(id: PreviewPresetId): PreviewDashboardLayout {
  return { preset: id, visible: visibleRecord(PRESET_VISIBILITY[id]) };
}

export function defaultLayout(): PreviewDashboardLayout {
  return presetLayout("overview");
}

export function setWidgetVisible(
  layout: PreviewDashboardLayout,
  id: PreviewWidgetId,
  visible: boolean
): PreviewDashboardLayout {
  return {
    preset: "custom",
    visible: { ...layout.visible, [id]: visible }
  };
}

export function normalizeLayout(value: unknown): PreviewDashboardLayout {
  if (!value || typeof value !== "object") {
    return defaultLayout();
  }

  const candidate = value as { preset?: unknown; visible?: unknown };
  const preset = candidate.preset;

  if (preset !== "custom" && !isPresetId(preset)) {
    return defaultLayout();
  }

  const base =
    preset === "custom" ? presetLayout("overview") : presetLayout(preset as PreviewPresetId);
  const visible = { ...base.visible };

  if (candidate.visible && typeof candidate.visible === "object") {
    const input = candidate.visible as Record<string, unknown>;
    for (const id of PREVIEW_WIDGET_IDS) {
      if (typeof input[id] === "boolean") {
        visible[id] = input[id] as boolean;
      }
    }
  }

  return { preset, visible };
}

function resolveStorage(): Storage | undefined {
  try {
    if (typeof globalThis === "undefined") {
      return undefined;
    }
    return (globalThis as unknown as { localStorage?: Storage }).localStorage;
  } catch {
    return undefined;
  }
}

export function loadLayout(storage?: Pick<Storage, "getItem">): PreviewDashboardLayout {
  const target = storage ?? resolveStorage();
  if (!target) {
    return defaultLayout();
  }

  try {
    const raw = target.getItem(PREVIEW_LAYOUT_STORAGE_KEY);
    if (!raw) {
      return defaultLayout();
    }
    return normalizeLayout(JSON.parse(raw));
  } catch {
    return defaultLayout();
  }
}

export function saveLayout(
  layout: PreviewDashboardLayout,
  storage?: Pick<Storage, "setItem">
): void {
  const target = storage ?? resolveStorage();
  if (!target) {
    return;
  }

  try {
    target.setItem(PREVIEW_LAYOUT_STORAGE_KEY, JSON.stringify(layout));
  } catch {
    // Storage can be unavailable (private mode, quota, security policy).
  }
}

export function latestTransactions(transactions: Transaction[], limit = 6): Transaction[] {
  return [...transactions]
    .sort((first, second) => {
      const firstValid = dayjsIsValid(first?.date);
      const secondValid = dayjsIsValid(second?.date);

      if (firstValid && secondValid) {
        const difference = second.date.valueOf() - first.date.valueOf();
        if (difference !== 0) {
          return difference;
        }
        return compareIds(first.id, second.id);
      }

      if (firstValid) return -1;
      if (secondValid) return 1;
      return compareIds(first.id, second.id);
    })
    .slice(0, limit);
}

function dayjsIsValid(value: Transaction["date"] | undefined): boolean {
  return typeof value?.isValid === "function" && value.isValid();
}

function compareIds(first: string, second: string): number {
  return first < second ? -1 : first > second ? 1 : 0;
}
