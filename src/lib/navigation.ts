export interface NavigationLink {
  label: string;
  href: string;
  tag?: string;
  help?: string;
  hide?: boolean;
  dateRangeSelector?: boolean;
  monthPicker?: boolean;
  financialYearPicker?: boolean;
  maxDepthSelector?: boolean;
  recurringIcons?: boolean;
  disablePreload?: boolean;
  children?: NavigationLink[];
  sidebarHref?: string;
  icon?: string;
  sidebarHidden?: boolean;
}

export interface BreadcrumbResolution {
  normalizedPath: string;
  selectedLink: NavigationLink | null;
  selectedSubLink: NavigationLink | null;
  selectedSubSubLink: NavigationLink | null;
}

export interface BreadcrumbItem {
  label: string;
  tag?: string;
}

function matchesPath(path: string, base: string) {
  return path === base || path.startsWith(base + "/");
}

export function normalizeNavigationPath(pathname: string): string {
  if (!pathname) {
    return "/";
  }
  const normalized =
    pathname.length > 1 && pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;
  return normalized || "/";
}

export function resolveBreadcrumbTrail(
  pathname: string,
  links: NavigationLink[],
  prefix?: string
): BreadcrumbResolution {
  let normalizedPath = normalizeNavigationPath(pathname);

  if (prefix) {
    const normalizedPrefix = normalizeNavigationPath(prefix);
    if (normalizedPath === normalizedPrefix) {
      normalizedPath = "/";
    } else if (normalizedPath.startsWith(normalizedPrefix + "/")) {
      normalizedPath = normalizedPath.slice(normalizedPrefix.length);
    }
  }

  let selectedLink =
    links.find((link) => link.href === normalizedPath || link.sidebarHref === normalizedPath) ??
    null;

  if (!selectedLink) {
    selectedLink =
      links.find(
        (link) => (link.children?.length ?? 0) > 0 && matchesPath(normalizedPath, link.href)
      ) ?? null;
  }

  let selectedSubLink: NavigationLink | null = null;
  let selectedSubSubLink: NavigationLink | null = null;

  if (selectedLink) {
    const children = selectedLink.children ?? [];
    selectedSubLink =
      children.find((child) => selectedLink.href + child.href === normalizedPath) ??
      children.find((child) => matchesPath(normalizedPath, selectedLink.href + child.href)) ??
      null;

    if (selectedSubLink?.children?.length) {
      const base = selectedLink.href + selectedSubLink.href;
      selectedSubSubLink =
        selectedSubLink.children.find((child) => base + child.href === normalizedPath) ??
        selectedSubLink.children.find((child) => matchesPath(normalizedPath, base + child.href)) ??
        null;
    }
  }

  return {
    normalizedPath,
    selectedLink,
    selectedSubLink,
    selectedSubSubLink
  };
}

export function breadcrumbItems(resolution: BreadcrumbResolution): BreadcrumbItem[] {
  const { normalizedPath, selectedLink, selectedSubLink, selectedSubSubLink } = resolution;
  if (!selectedLink) {
    return [];
  }

  const items: BreadcrumbItem[] = [{ label: selectedLink.label, tag: selectedLink.tag }];
  if (!selectedSubLink) {
    return items;
  }

  items.push({ label: selectedSubLink.label, tag: selectedSubLink.tag });
  if (selectedSubSubLink) {
    items.push({ label: selectedSubSubLink.label });
    return items;
  }

  const combined = selectedLink.href + selectedSubLink.href;
  if (normalizedPath !== combined) {
    const segment = normalizedPath.split("/").filter(Boolean).at(-1);
    if (segment) {
      items.push({ label: decodeURIComponent(segment) });
    }
  }

  return items;
}
