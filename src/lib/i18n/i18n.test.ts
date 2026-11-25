import { describe, expect, test } from "bun:test";
import { get } from "svelte/store";
import { de } from "./messages/de";
import { en } from "./messages/en";
import { locale, resolveLocale, setLocale, t } from "./index";

describe("catalog parity", () => {
  test("every english key exists in german", () => {
    expect(Object.keys(de).sort()).toEqual(Object.keys(en).sort());
  });

  test("every german key exists in english", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(de).sort());
  });
});

describe("interpolation", () => {
  test("substitutes named values", () => {
    setLocale("de");
    expect(get(t)("greeting", { name: "Ada" })).toBe("Hallo, Ada");
  });

  test("substitutes numeric values as text", () => {
    setLocale("en");
    expect(get(t)("greeting", { name: 42 })).toBe("Hello, 42");
  });

  test("leaves unknown placeholders untouched", () => {
    setLocale("en");
    expect(get(t)("greeting", {})).toBe("Hello, {name}");
    expect(get(t)("greeting")).toBe("Hello, {name}");
  });
});

describe("key lookup", () => {
  test("unknown key falls back to the key", () => {
    setLocale("en");
    expect(get(t)("does.not.exist" as never)).toBe("does.not.exist");
  });

  test("returns the localized string for the active locale", () => {
    setLocale("en");
    expect(get(t)("login.submit")).toBe("Sign in");
    setLocale("de");
    expect(get(t)("login.submit")).toBe("Anmelden");
  });
});

describe("locale resolution", () => {
  test("maps de-DE to de", () => {
    expect(setLocale("de-DE")).toBe("de");
  });

  test("maps en-IN to en", () => {
    expect(setLocale("en-IN")).toBe("en");
  });

  test("normalizes regional variants case-insensitively", () => {
    expect(resolveLocale("DE-de")).toBe("de");
    expect(resolveLocale("de-AT")).toBe("de");
    expect(resolveLocale("en-GB")).toBe("en");
    expect(resolveLocale("fr-FR")).toBe("en");
    expect(resolveLocale("")).toBe("en");
  });

  test("derives only from config.locale, never regional profile or tax regime", () => {
    expect(resolveLocale("germany")).toBe("en");
    expect(resolveLocale("india")).toBe("en");
  });

  test("updates the locale store", () => {
    setLocale("de");
    expect(get(locale)).toBe("de");
    setLocale("en");
    expect(get(locale)).toBe("en");
  });

  test("an active subscription re-renders on locale switch", () => {
    setLocale("en");
    const seen: string[] = [];
    const unsubscribe = t.subscribe((translate) => seen.push(translate("login.submit")));
    setLocale("de");
    unsubscribe();
    expect(seen).toEqual(["Sign in", "Anmelden"]);
  });
});

describe("pluralization", () => {
  test("uses Intl.PluralRules for german one/other", () => {
    setLocale("de");
    expect(get(t)("bookings", { count: 1 })).toBe("1 Buchung");
    expect(get(t)("bookings", { count: 2 })).toBe("2 Buchungen");
    expect(get(t)("bookings", { count: 0 })).toBe("0 Buchungen");
  });

  test("uses Intl.PluralRules for english one/other", () => {
    setLocale("en");
    expect(get(t)("bookings", { count: 1 })).toBe("1 booking");
    expect(get(t)("bookings", { count: 2 })).toBe("2 bookings");
  });

  test("non-plural keys still resolve when count is present", () => {
    setLocale("en");
    // No greeting_one / greeting_other exists; lookup must fall back to the base key.
    expect(get(t)("greeting", { count: 1, name: "Ada" })).toBe("Hello, Ada");
  });

  test("keeps the count value in the interpolation", () => {
    setLocale("de");
    expect(get(t)("bookings", { count: 21 })).toBe("21 Buchungen");
  });
});
