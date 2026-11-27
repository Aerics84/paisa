import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const read = (...parts: string[]) => readFileSync(join(__dirname, ...parts), "utf-8");
const app = readFileSync(join(__dirname, "..", "..", "app.scss"), "utf-8");

describe("preview responsive shell", () => {
  test("navbar exposes an accessible drawer with focus management", () => {
    const navbar = read("PreviewNavbar.svelte");
    expect(navbar).toContain("sidebarOpen = $bindable(false)");
    expect(navbar).toContain("aria-expanded={sidebarOpen}");
    expect(navbar).toContain('aria-controls="preview-navigation"');
    expect(navbar).toContain("previousFocus");
    expect(navbar).toContain("nextFocusIndex");
    expect(navbar).toContain("paisa-shell-sidebar--open");
    expect(navbar).toContain("paisa-shell-backdrop");
    expect(navbar).toContain('event.key === "Escape"');
    expect(navbar).not.toMatch(/\bon:click=/);
  });

  test("layout makes the background inert and locks scrolling", () => {
    const layout = read("..", "..", "routes", "preview", "+layout.svelte");
    expect(layout).toContain("bind:sidebarOpen");
    expect(layout).toContain("inert={sidebarOpen}");
    expect(layout).toContain("style.overflow");
  });

  test("mobile styles slide the sidebar off canvas", () => {
    expect(app).toContain("translateX(-100%)");
    expect(app).toContain("visibility: hidden");
    expect(app).toContain(".paisa-shell-sidebar--open");
    expect(app).toContain(".paisa-shell-backdrop");
    expect(app).toContain("prefers-reduced-motion: reduce");
  });
});
