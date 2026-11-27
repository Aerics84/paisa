import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const src = (name: string) => readFileSync(join(__dirname, "..", "..", name), "utf-8");

const parseVar = (source: string, name: string) => {
  const match = source.match(new RegExp(`--${name}:\\s*(#[0-9a-fA-F]{6})`));
  if (!match) throw new Error(`missing --${name}`);
  return match[1];
};

const channel = (value: number) =>
  value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;

const luminance = (hex: string) => {
  const [r, g, b] = [0, 2, 4].map((offset) =>
    channel(parseInt(hex.slice(1 + offset, 3 + offset), 16) / 255)
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};

const contrast = (a: string, b: string) => {
  const first = luminance(a);
  const second = luminance(b);
  const [high, low] = first > second ? [first, second] : [second, first];
  return (high + 0.05) / (low + 0.05);
};

describe("preview theme contrast", () => {
  test("dashboard text tokens are defined per theme, not globally", () => {
    const app = src("app.scss");
    expect(app).not.toContain("--dashboard-text-strong:");
    expect(app).not.toContain("--dashboard-warning-strong:");
    expect(app).not.toContain("#f8fbff");
    expect(app).not.toContain("#deebfa");
    expect(src("light.scss")).toContain("--dashboard-text-strong: #0f172a");
    expect(src("dark.scss")).toContain("--dashboard-text-strong: #f8fbff");
  });

  test("dashboard text tokens meet AA on both light surfaces", () => {
    const light = src("light.scss");
    const surfaces = [parseVar(light, "app-bg"), parseVar(light, "app-canvas")];
    for (const name of [
      "dashboard-text-strong",
      "dashboard-text-bright",
      "dashboard-text-soft",
      "dashboard-text-muted",
      "dashboard-text-subtle"
    ]) {
      const color = parseVar(light, name);
      for (const surface of surfaces) {
        expect(contrast(color, surface)).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  test("preview scss and shell use only dashboard text tokens", () => {
    const preview = src("paisa-dashboard-overview.scss");
    expect(preview).not.toContain("var(--app-text-subtle)");
    expect(preview).not.toContain("var(--app-text-muted)");
    expect(preview).toContain(".axis text");
    expect(preview).toContain("var(--dashboard-text-muted)");
    expect(preview).toContain(".paisa-app-frame");

    const navbar = readFileSync(join(__dirname, "PreviewNavbar.svelte"), "utf-8");
    expect(navbar).not.toContain("var(--app-text-subtle)");
    expect(navbar).not.toContain("var(--app-text-muted)");
  });

  test("topbar text follows the theme", () => {
    const app = src("app.scss");
    expect(app).toMatch(/\.paisa-topbar__title\s*\{[^}]*color:\s*var\(--app-text\)/s);
    expect(app).toMatch(/\.paisa-topbar__subtitle\s*\{[^}]*color:\s*var\(--app-text-muted\)/s);
  });
});
