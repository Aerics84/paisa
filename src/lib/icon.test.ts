import { expect, test } from "bun:test";
import { iconGlyph, loadIconFont, loadIcons } from "./icon";

test("built-in fa6 icons work without dynamic loading", () => {
  expect(iconGlyph("fa6-solid:house")).not.toBe("\uFFFD");
});

test("unknown symbol falls back to replacement glyph", () => {
  expect(iconGlyph("nope:nope")).toBe("\uFFFD");
});

test("mdi glyph works after loading the font", async () => {
  await loadIconFont("mdi");
  expect(iconGlyph("mdi:home")).not.toBe("\uFFFD");
});

test("loadIcons loads every font for the picker", async () => {
  const icons = await loadIcons();
  expect(icons).toContain("fa6-solid:house");
  expect(icons).toContain("mdi:home");
  expect(icons).toContain("arcticons:100");
  expect(icons).toContain("fluent-emoji-high-contrast:1st-place-medal");
});
