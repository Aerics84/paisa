import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { extname, join } from "node:path";

const root = "web/static";
const MB = 1024 * 1024;

// daisyUI is already tree-shaken by Tailwind, so removing it saves little.
// Most of the remaining CSS is Bulma compiled once for the light theme and
// again inside `html[data-theme="dark"]`. The CSS budget leaves just enough
// headroom for normal style changes; re-adding the tree-shaken daisyUI
// components (~0.09 MB) would exceed it. Shrinking the bundle further needs a
// separate theming refactor to CSS custom properties.
const BUDGETS = [
  { name: "CSS", match: (ext) => ext === ".css", limit: 1.9 * MB },
  { name: "JavaScript", match: (ext) => ext === ".js", limit: 5.0 * MB },
  { name: "Workers", match: (ext) => ext === ".mjs", limit: 1.5 * MB },
  {
    name: "Fonts",
    match: (ext) => [".woff", ".woff2", ".ttf", ".otf"].includes(ext),
    limit: 6.5 * MB
  }
];

if (!existsSync(root)) {
  console.error(`Asset directory "${root}" is missing. Run "npm run build" first.`);
  process.exit(1);
}

const totals = new Map();
const cssFiles = [];

function walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(path);
    } else {
      const ext = extname(entry.name).toLowerCase() || "(none)";
      totals.set(ext, (totals.get(ext) ?? 0) + statSync(path).size);
      if (ext === ".css") {
        cssFiles.push(path);
      }
    }
  }
}

walk(root);

console.log("Assets in web/static:");
for (const [ext, bytes] of [...totals.entries()].sort(([a], [b]) => a.localeCompare(b))) {
  console.log(`  ${ext.padEnd(8)} ${(bytes / MB).toFixed(2)} MB (${bytes} bytes)`);
}

let failed = false;

console.log("\nBudgets:");
for (const budget of BUDGETS) {
  const bytes = [...totals.entries()]
    .filter(([ext]) => budget.match(ext))
    .reduce((sum, [, size]) => sum + size, 0);
  const status = bytes <= budget.limit ? "ok" : "FAILED";
  console.log(
    `  ${budget.name.padEnd(10)} ${(bytes / MB).toFixed(2)} MB / ${(budget.limit / MB).toFixed(
      2
    )} MB ${status}`
  );
  if (bytes > budget.limit) {
    failed = true;
  }
}

const daisyUiSelectors = [];
for (const file of cssFiles) {
  const css = readFileSync(file, "utf8");
  const matches = css.match(/\.du-[a-z0-9-]+/g);
  if (matches) {
    daisyUiSelectors.push(`${file}: ${[...new Set(matches)].slice(0, 5).join(", ")}`);
  }
}

console.log("\ndaisyUI selectors (du-*):");
if (daisyUiSelectors.length > 0) {
  for (const entry of daisyUiSelectors) {
    console.log(`  FAILED ${entry}`);
  }
  failed = true;
} else {
  console.log("  ok, none found");
}

if (failed) {
  console.error("\nAsset check failed.");
  process.exit(1);
}

console.log("\nAsset check passed.");
