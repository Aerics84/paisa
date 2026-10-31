import { describe, expect, test } from "bun:test";
import { breadcrumbItems, resolveBreadcrumbTrail, type NavigationLink } from "./navigation";

const links: NavigationLink[] = [
  {
    label: "Assets",
    href: "/assets",
    children: [
      { label: "Balance", href: "/balance" },
      { label: "Gain", href: "/gain" }
    ]
  },
  { label: "Income", href: "/income" }
];

const nestedLinks: NavigationLink[] = [
  {
    label: "Assets",
    href: "/assets",
    children: [
      {
        label: "Investments",
        href: "/investments",
        children: [{ label: "Holdings", href: "/holdings" }]
      }
    ]
  }
];

describe("breadcrumb resolution", () => {
  test("resolves a direct subpage", () => {
    const trail = resolveBreadcrumbTrail("/assets/balance", links);
    expect(trail.normalizedPath).toBe("/assets/balance");
    expect(trail.selectedLink?.label).toBe("Assets");
    expect(trail.selectedSubLink?.label).toBe("Balance");
    expect(trail.selectedSubSubLink).toBeNull();
  });

  test("resolves a dynamic subpage without inventing a sub-sub link", () => {
    const trail = resolveBreadcrumbTrail("/assets/gain/Assets:Equity:AAPL", links);
    expect(trail.selectedLink?.label).toBe("Assets");
    expect(trail.selectedSubLink?.label).toBe("Gain");
    expect(trail.selectedSubSubLink).toBeNull();
  });

  test("resolves a top-level page", () => {
    const trail = resolveBreadcrumbTrail("/income/", links);
    expect(trail.normalizedPath).toBe("/income");
    expect(trail.selectedLink?.label).toBe("Income");
    expect(trail.selectedSubLink).toBeNull();
  });

  test("returns null selections for an unknown route", () => {
    const trail = resolveBreadcrumbTrail("/unknown/deep/path", links);
    expect(trail.selectedLink).toBeNull();
    expect(trail.selectedSubLink).toBeNull();
    expect(trail.selectedSubSubLink).toBeNull();
  });

  test("strips the preview prefix", () => {
    const trail = resolveBreadcrumbTrail("/preview/assets/balance", links, "/preview");
    expect(trail.normalizedPath).toBe("/assets/balance");
    expect(trail.selectedLink?.label).toBe("Assets");
    expect(trail.selectedSubLink?.label).toBe("Balance");
  });

  test("maps the preview root to slash", () => {
    expect(resolveBreadcrumbTrail("/preview", links, "/preview").normalizedPath).toBe("/");
  });

  test("resolves a real third level", () => {
    const trail = resolveBreadcrumbTrail("/assets/investments/holdings", nestedLinks);
    expect(trail.selectedLink?.label).toBe("Assets");
    expect(trail.selectedSubLink?.label).toBe("Investments");
    expect(trail.selectedSubSubLink?.label).toBe("Holdings");
    expect(breadcrumbItems(trail).map((item) => item.label)).toEqual([
      "Assets",
      "Investments",
      "Holdings"
    ]);
  });

  test("does not invent a third level for a deeper dynamic path", () => {
    const trail = resolveBreadcrumbTrail("/assets/investments/other", nestedLinks);
    expect(trail.selectedSubSubLink).toBeNull();
    expect(breadcrumbItems(trail).map((item) => item.label)).toEqual([
      "Assets",
      "Investments",
      "other"
    ]);
  });

  test("returns no breadcrumb items for an unknown route", () => {
    expect(breadcrumbItems(resolveBreadcrumbTrail("/unknown/deep/path", links))).toEqual([]);
  });

  test("keeps the dynamic last segment for a second-level detail page", () => {
    const trail = resolveBreadcrumbTrail("/assets/gain/Assets:Equity:AAPL", links);
    expect(breadcrumbItems(trail).map((item) => item.label)).toEqual([
      "Assets",
      "Gain",
      "Assets:Equity:AAPL"
    ]);
  });
});
