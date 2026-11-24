import { beforeEach, expect, test } from "bun:test";
import {
  configuredIconFonts,
  iconGlyph,
  loadIconFont,
  loadIcons,
  preloadConfiguredIconFonts,
  resetIconStateForTests,
  setIconFontLoaderForTests
} from "./icon";

beforeEach(() => resetIconStateForTests());

test("built-in fa6 icons work without dynamic loading", () => {
  expect(iconGlyph("fa6-solid:house")).not.toBe("\uFFFD");
});

test("unknown symbol falls back to replacement glyph", () => {
  expect(iconGlyph("nope:nope")).toBe("\uFFFD");
});

test("mdi glyph is a placeholder before the font loads", async () => {
  expect(iconGlyph("mdi:home")).toBe("\uFFFD");
  await loadIconFont("mdi");
  expect(iconGlyph("mdi:home")).not.toBe("\uFFFD");
});

test("loadIconFont is single-flight for concurrent calls", async () => {
  let loads = 0;
  setIconFontLoaderForTests("mdi", async () => {
    loads += 1;
    await Promise.resolve();
    return { codepoints: { home: 65 } };
  });

  await Promise.all([loadIconFont("mdi"), loadIconFont("mdi")]);

  expect(loads).toBe(1);
  expect(iconGlyph("mdi:home")).toBe("A");
});

test("loadIcons loads every font for the picker", async () => {
  const icons = await loadIcons();
  expect(icons).toContain("fa6-solid:house");
  expect(icons).toContain("mdi:home");
  expect(icons).toContain("arcticons:100");
  expect(icons).toContain("fluent-emoji-high-contrast:1st-place-medal");
});

test("loadIcons rejects a partial load and retries on the next call", async () => {
  let attempts = 0;
  setIconFontLoaderForTests("mdi", async () => {
    attempts += 1;
    if (attempts === 1) {
      throw new Error("metadata chunk unavailable");
    }
    return { codepoints: { home: 65 } };
  });

  await expect(loadIcons()).rejects.toThrow("Could not load icon metadata for: mdi");

  const icons = await loadIcons();
  expect(icons).toContain("mdi:home");
  expect(attempts).toBe(2);
});

test("preloadConfiguredIconFonts dedupes lazy fonts and tolerates missing entries", async () => {
  const loaded: string[] = [];
  setIconFontLoaderForTests("mdi", async () => {
    loaded.push("mdi");
    return { codepoints: { home: 65 } };
  });
  setIconFontLoaderForTests("arcticons", async () => {
    loaded.push("arcticons");
    return { codepoints: { 100: 66 } };
  });

  const config = {
    accounts: [null, { icon: "mdi:home" }, { icon: "fa6-solid:house" }, { name: "no icon" }],
    goals: {
      retirement: [{ icon: "mdi:home" }],
      savings: [null, { icon: "arcticons:100" }]
    }
  };

  await preloadConfiguredIconFonts(config as never);

  expect(loaded.sort()).toEqual(["arcticons", "mdi"]);
});

test("preloadConfiguredIconFonts never rejects when a font fails", async () => {
  setIconFontLoaderForTests("mdi", async () => {
    throw new Error("metadata chunk unavailable");
  });

  await expect(
    preloadConfiguredIconFonts({ accounts: [{ icon: "mdi:home" }] } as never)
  ).resolves.toBeUndefined();
});

test("configuredIconFonts ignores empty configs and non-lazy fonts", () => {
  expect(configuredIconFonts(undefined)).toEqual([]);
  expect(
    configuredIconFonts({ accounts: undefined, goals: undefined } as unknown as UserConfig)
  ).toEqual([]);
  expect(
    configuredIconFonts({
      accounts: [{ icon: "fa6-solid:house" }],
      goals: {}
    } as unknown as UserConfig)
  ).toEqual([]);
});
