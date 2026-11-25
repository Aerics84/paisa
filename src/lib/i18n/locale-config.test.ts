import { afterEach, describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import { get } from "svelte/store";
import { applyLocale, configUpdated, formatCurrency, formatPercentage } from "../utils";
import { locale, resolveLocale, setLocale, t } from "./index";

const previousConfig = (globalThis as any).USER_CONFIG;

function useConfig(config: Record<string, unknown>) {
  (globalThis as any).USER_CONFIG = {
    ...previousConfig,
    display_precision: 2,
    ...config
  };
}

afterEach(() => {
  (globalThis as any).USER_CONFIG = previousConfig;
  setLocale("en");
  dayjs.locale("en");
  // configUpdated mutates the loaded locale definitions; restore their
  // defaults so this file cannot leak a week start into other suites.
  dayjs.updateLocale("en", { weekStart: 0 });
  dayjs.updateLocale("de", { weekStart: 1 });
});

describe("locale resolution from config", () => {
  test("german regional config selects german", () => {
    expect(resolveLocale("de-DE")).toBe("de");
    expect(resolveLocale("en-IN")).toBe("en");
    expect(resolveLocale("")).toBe("en");
  });

  test("germany tax profile with english locale stays english", () => {
    const config = { regional_profile: "germany-eu", tax_regime: "germany", locale: "en" };
    expect(resolveLocale(config.locale)).toBe("en");
  });
});

describe("applying the configured locale", () => {
  test("switching locale updates the store, translations and dayjs", () => {
    useConfig({ locale: "de-DE" });
    expect(applyLocale(globalThis.USER_CONFIG.locale)).toBe("de");
    expect(get(locale)).toBe("de");
    expect(get(t)("login.submit")).toBe("Anmelden");
    expect(dayjs.locale()).toBe("de");
    expect(dayjs("2026-03-15").format("MMMM")).toBe("März");

    useConfig({ locale: "en-IN" });
    expect(applyLocale(globalThis.USER_CONFIG.locale)).toBe("en");
    expect(get(locale)).toBe("en");
    expect(get(t)("login.submit")).toBe("Sign in");
    expect(dayjs.locale()).toBe("en");
    expect(dayjs("2026-03-15").format("MMMM")).toBe("March");
  });

  test("configUpdated applies locale changes from the loaded config", () => {
    useConfig({ locale: "de-DE" });
    configUpdated();
    expect(get(locale)).toBe("de");
    expect(dayjs.locale()).toBe("de");

    useConfig({ locale: "en-IN" });
    configUpdated();
    expect(get(locale)).toBe("en");
    expect(dayjs.locale()).toBe("en");
  });

  test("germany profile with english locale keeps english dates", () => {
    useConfig({ locale: "en", regional_profile: "germany-eu", tax_regime: "germany" });
    configUpdated();
    expect(get(locale)).toBe("en");
    expect(dayjs.locale()).toBe("en");
    expect(dayjs("2026-03-15").format("MMMM")).toBe("March");
  });
});

describe("number formatting locale", () => {
  test("formats numbers and percentages with the configured regional locale", () => {
    useConfig({ locale: "de-DE" });
    expect(formatCurrency(1234.5, 2)).toBe("1.234,50");
    expect(formatPercentage(0.125, 1)).toContain("12,5");
  });

  test("keeps indian digit grouping for en-IN", () => {
    useConfig({ locale: "en-IN" });
    expect(formatCurrency(1234567.5, 2)).toBe("12,34,567.50");
  });

  test("falls back to the resolved locale when the config locale is blank", () => {
    useConfig({ locale: "" });
    expect(formatCurrency(1234.5, 2)).toBe("1,234.50");
  });
});
