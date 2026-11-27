import "../happydom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { get } from "svelte/store";
import { getColorPreference, initializeTheme, setColorPreference, themePreference } from "./theme";

class SystemTheme extends EventTarget {
  matches = false;

  setDark(dark: boolean) {
    this.matches = dark;
    this.dispatchEvent(new Event("change"));
  }
}

const storageKey = "theme-preference";
let media: SystemTheme;
let originalMatchMedia: typeof window.matchMedia;
let disposers: Array<() => void> = [];

function dispatchStorage(key: string | null) {
  window.dispatchEvent(Object.assign(new Event("storage"), { key }));
}

beforeEach(() => {
  disposers.forEach((dispose) => dispose());
  disposers = [];
  media = new SystemTheme();
  originalMatchMedia = window.matchMedia;
  window.matchMedia = ((_query: string) => media) as unknown as typeof window.matchMedia;
  localStorage.clear();
  setColorPreference("system");
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.style.colorScheme = "";
});

afterEach(() => {
  disposers.forEach((dispose) => dispose());
  disposers = [];
  window.matchMedia = originalMatchMedia;
  localStorage.clear();
});

describe("theme preferences", () => {
  test("defaults to system and follows live changes without saving a resolved color", () => {
    const resolved: string[] = [];
    disposers.push(initializeTheme((value) => resolved.push(value)));

    expect(get(themePreference)).toBe("system");
    expect(getColorPreference()).toBe("light");
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(document.documentElement.style.colorScheme).toBe("light");

    media.setDark(true);
    expect(getColorPreference()).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");

    media.setDark(false);
    expect(resolved).toEqual(["light", "dark", "light"]);
    expect(localStorage.getItem(storageKey)).toBeNull();
  });

  test("explicit modes ignore system changes and returning to system resolves immediately", () => {
    disposers.push(initializeTheme(() => undefined));

    setColorPreference("dark");
    media.setDark(false);
    expect(document.documentElement.dataset.theme).toBe("dark");

    setColorPreference("light");
    media.setDark(true);
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(localStorage.getItem(storageKey)).toBe("light");

    setColorPreference("system");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem(storageKey)).toBe("system");

    media.setDark(false);
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  test("synchronizes saved preferences and treats missing or invalid values as system", () => {
    disposers.push(initializeTheme(() => undefined));

    localStorage.setItem(storageKey, "dark");
    dispatchStorage(storageKey);
    expect(get(themePreference)).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");

    localStorage.setItem(storageKey, "invalid");
    dispatchStorage(storageKey);
    expect(get(themePreference)).toBe("system");
    expect(document.documentElement.dataset.theme).toBe("light");

    localStorage.clear();
    media.setDark(true);
    dispatchStorage(null);
    expect(get(themePreference)).toBe("system");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  test("removes subscriptions and browser listeners on cleanup", () => {
    const resolved: string[] = [];
    const dispose = initializeTheme((value) => resolved.push(value));
    dispose();

    media.setDark(true);
    setColorPreference("dark");

    expect(resolved).toEqual(["light"]);
    expect(document.documentElement.dataset.theme).toBe("light");
  });
});
