import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import {
  PREVIEW_LAYOUT_STORAGE_KEY,
  PREVIEW_WIDGETS,
  defaultLayout,
  latestTransactions,
  loadLayout,
  normalizeLayout,
  presetLayout,
  saveLayout,
  setWidgetVisible
} from "./preview_dashboard";
import type { Transaction } from "./api/ledger";

const ids = PREVIEW_WIDGETS.map((widget) => widget.id);
const visibleIds = (layout: ReturnType<typeof defaultLayout>) =>
  ids.filter((id) => layout.visible[id]);

function memoryStorage() {
  const values = new Map<string, string>();
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value)
  };
}

const transaction = (id: string, date: string) =>
  ({ id, date: dayjs(date) }) as Transaction;

describe("preview dashboard layout", () => {
  test("default shows every widget", () => {
    const layout = defaultLayout();
    expect(layout.preset).toBe("overview");
    expect(visibleIds(layout)).toEqual(ids);
  });

  test("presets hide the expected sections", () => {
    expect(visibleIds(presetLayout("investor"))).toEqual(["hero", "monthlyReview", "insights"]);
    expect(visibleIds(presetLayout("cashflow"))).toEqual([
      "hero",
      "monthlyReview",
      "support",
      "recentTransactions"
    ]);
    expect(visibleIds(presetLayout("minimal"))).toEqual(["hero", "monthlyReview"]);
  });

  test("a manual toggle marks the layout custom and survives normalization", () => {
    const next = setWidgetVisible(defaultLayout(), "insights", false);
    expect(next.preset).toBe("custom");
    expect(next.visible.insights).toBe(false);
    expect(next.visible.hero).toBe(true);
    expect(normalizeLayout(next)).toEqual(next);
  });

  test("switching presets discards manual changes", () => {
    const custom = setWidgetVisible(presetLayout("investor"), "support", true);
    expect(custom.visible.support).toBe(true);
    expect(presetLayout("investor").visible.support).toBe(false);
  });

  test("normalization fills missing ids and drops unknown ones", () => {
    const next = normalizeLayout({
      preset: "investor",
      visible: { hero: false, nope: true }
    });
    expect(next.preset).toBe("investor");
    expect(next.visible.hero).toBe(false);
    expect(next.visible.support).toBe(false);
    expect(next.visible.monthlyReview).toBe(true);
    expect("nope" in next.visible).toBe(false);
  });

  test("invalid values fall back to overview", () => {
    expect(normalizeLayout(null)).toEqual(defaultLayout());
    expect(normalizeLayout({})).toEqual(defaultLayout());
    expect(normalizeLayout({ preset: "nope", visible: { hero: false } })).toEqual(defaultLayout());
  });

  test("storage round-trips, never writes on load and survives failures", () => {
    const storage = memoryStorage();
    saveLayout(presetLayout("minimal"), storage);
    expect(loadLayout(storage)).toEqual(presetLayout("minimal"));

    const writes: string[] = [];
    const readOnlyStorage: Pick<Storage, "getItem" | "setItem"> = {
      getItem: () => null,
      setItem: (key) => {
        writes.push(key);
      }
    };
    loadLayout(readOnlyStorage);
    expect(writes).toEqual([]);

    const failing = {
      getItem: () => {
        throw new DOMException("denied", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("full", "QuotaExceededError");
      }
    };
    expect(loadLayout(failing)).toEqual(defaultLayout());
    expect(() => saveLayout(defaultLayout(), failing)).not.toThrow();

    storage.setItem(PREVIEW_LAYOUT_STORAGE_KEY, "{not json");
    expect(loadLayout(storage)).toEqual(defaultLayout());
  });

  test("recent transactions are newest first, valid before invalid, and limited", () => {
    const invalid = { id: "bad", date: dayjs("not-a-date") } as Transaction;
    const result = latestTransactions(
      [
        transaction("a", "2026-11-01"),
        invalid,
        transaction("b", "2026-11-03"),
        transaction("c", "2026-11-02")
      ],
      4
    );
    expect(result.map((item) => item.id)).toEqual(["b", "c", "a", "bad"]);
    expect(latestTransactions([], 3)).toEqual([]);
  });
});
