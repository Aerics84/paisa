import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, "..");
const srcDir = path.join(rootDir, "src");
const budgetPath = path.join(__dirname, "runes-budget.json");

/**
 * Legacy Svelte 4 patterns that the migration removes. The keys are also the
 * keys in scripts/runes-budget.json, so renaming one requires a budget update.
 */
const METRICS = [
  {
    key: "export let",
    pattern: /\bexport\s+let\b/g,
    hint: "use `let { prop = default } = $props()`"
  },
  {
    key: "$:",
    pattern: /^[ \t]*\$:/gm,
    hint: "use `$derived(...)` for values and `$effect(...)` for side effects"
  },
  {
    key: "on:click",
    pattern: /on:click/g,
    hint: "use the `onclick` attribute"
  },
  {
    key: "createEventDispatcher",
    pattern: /\bcreateEventDispatcher\b/g,
    hint: "use a callback prop such as `onchange?: (value: T) => void`"
  }
];

function listSvelteFiles(dir) {
  const files = [];
  const entries = readdirSync(dir, { withFileTypes: true }).sort((a, b) =>
    a.name.localeCompare(b.name)
  );
  for (const entry of entries) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...listSvelteFiles(entryPath));
    } else if (entry.isFile() && entry.name.endsWith(".svelte")) {
      files.push(entryPath);
    }
  }
  return files;
}

export function countLegacyUsage(files) {
  const counts = Object.fromEntries(METRICS.map((metric) => [metric.key, 0]));
  const perFile = Object.fromEntries(METRICS.map((metric) => [metric.key, new Map()]));

  for (const file of files) {
    const text = readFileSync(file, "utf8");
    for (const metric of METRICS) {
      const matches = text.match(metric.pattern)?.length ?? 0;
      counts[metric.key] += matches;
      if (matches > 0) {
        perFile[metric.key].set(file, matches);
      }
    }
  }

  return { counts, perFile };
}

function readBudgets() {
  let raw;
  try {
    raw = readFileSync(budgetPath, "utf8");
  } catch (error) {
    throw new Error(`cannot read ${path.relative(rootDir, budgetPath)}: ${error.message}`, {
      cause: error
    });
  }

  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error(`invalid JSON in ${path.relative(rootDir, budgetPath)}: ${error.message}`, {
      cause: error
    });
  }

  const budgets = parsed?.budgets;
  if (budgets === null || typeof budgets !== "object" || Array.isArray(budgets)) {
    throw new Error(`${path.relative(rootDir, budgetPath)} must contain a "budgets" object`);
  }

  const knownKeys = new Set(METRICS.map((metric) => metric.key));
  const unknownKeys = Object.keys(budgets).filter((key) => !knownKeys.has(key));
  if (unknownKeys.length > 0) {
    throw new Error(`unknown budget key(s): ${unknownKeys.join(", ")}`);
  }

  for (const metric of METRICS) {
    const value = budgets[metric.key];
    if (!Number.isInteger(value) || value < 0) {
      throw new Error(`budget "${metric.key}" must be a non-negative integer, received ${value}`);
    }
  }

  return budgets;
}

function writeBudgets(budgets) {
  const ordered = Object.fromEntries(METRICS.map((metric) => [metric.key, budgets[metric.key]]));
  const content = JSON.stringify(
    {
      description:
        "Ratchet budgets for legacy Svelte 4 syntax in src/**/*.svelte. " +
        "Never raise these values.",
      budgets: ordered
    },
    null,
    2
  );
  writeFileSync(budgetPath, `${content}\n`);
}

function displayPath(file) {
  return path.relative(rootDir, file).split(path.sep).join("/");
}

function printReport(files, { counts, perFile }, budgets) {
  const labelWidth = Math.max(...METRICS.map((metric) => metric.key.length));
  const countWidth = Math.max(
    ...METRICS.map((metric) => String(Math.max(counts[metric.key], budgets[metric.key])).length)
  );
  const fileLabel = files.length === 1 ? "file" : "files";

  console.log(`Svelte legacy syntax budget (src/**/*.svelte, ${files.length} ${fileLabel})`);

  const exceeded = [];
  for (const metric of METRICS) {
    const count = counts[metric.key];
    const budget = budgets[metric.key];
    const status = count > budget ? `FAILED (+${count - budget})` : "ok";
    console.log(
      `  ${metric.key.padEnd(labelWidth)} ` +
        `${String(count).padStart(countWidth)} / ${String(budget).padStart(countWidth)}  ${status}`
    );
    if (count > budget) {
      exceeded.push(metric);
    }
  }

  if (exceeded.length === 0) {
    console.log("\nrunes budget: passed");
    return;
  }

  console.error("\nrunes budget: failed");
  for (const metric of exceeded) {
    const count = counts[metric.key];
    const budget = budgets[metric.key];
    console.error(`  - ${metric.key}: ${count} measured, budget is ${budget} (+${count - budget})`);

    const topFiles = [...perFile[metric.key].entries()]
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      .slice(0, 5);
    for (const [file, matches] of topFiles) {
      console.error(`      ${displayPath(file)} (${matches})`);
    }
    console.error(`    ${metric.hint}`);
  }

  console.error(
    "\nMigrate the remaining legacy syntax instead of raising the budget.\n" +
      "See docs/reference/svelte5-conventions.md. After a migration batch, run:\n" +
      "  npm run check:runes -- --update-budget"
  );
}

function updateBudgets(counts, budgets) {
  const increases = METRICS.filter((metric) => counts[metric.key] > budgets[metric.key]);
  if (increases.length > 0) {
    console.error("runes budget: refusing to raise the budget");
    for (const metric of increases) {
      console.error(
        `  - ${metric.key}: ${counts[metric.key]} measured, budget is ${budgets[metric.key]}; ` +
          `migrate the extra usage instead (${metric.hint})`
      );
    }
    process.exitCode = 1;
    return;
  }

  const lowered = METRICS.filter((metric) => counts[metric.key] < budgets[metric.key]);
  if (lowered.length === 0) {
    console.log("runes budget: already at the current counts, nothing to update");
    return;
  }

  writeBudgets(counts);
  for (const metric of lowered) {
    console.log(`runes budget: ${metric.key} ${budgets[metric.key]} -> ${counts[metric.key]}`);
  }
  console.log("runes budget: budgets updated");
}

function main(argv) {
  let updateMode = false;
  for (const arg of argv) {
    switch (arg) {
      case "--update-budget":
        updateMode = true;
        break;
      case "--help":
      case "-h":
        console.log(
          "usage: node scripts/check-runes.mjs [--update-budget]\n\n" +
            "Counts legacy Svelte 4 syntax in src/**/*.svelte and compares it with\n" +
            "scripts/runes-budget.json.\n\n" +
            "  --update-budget  lower the budgets to the current counts\n" +
            "  --help           show this message"
        );
        return;
      default:
        console.error(`runes budget: unknown option "${arg}"`);
        process.exitCode = 2;
        return;
    }
  }

  let budgets;
  try {
    budgets = readBudgets();
  } catch (error) {
    console.error(`runes budget: ${error.message}`);
    process.exitCode = 2;
    return;
  }

  let files;
  try {
    files = listSvelteFiles(srcDir);
  } catch (error) {
    console.error(`runes budget: cannot scan ${path.relative(rootDir, srcDir)}: ${error.message}`);
    process.exitCode = 2;
    return;
  }

  const usage = countLegacyUsage(files);
  if (updateMode) {
    updateBudgets(usage.counts, budgets);
    return;
  }

  printReport(files, usage, budgets);
  if (METRICS.some((metric) => usage.counts[metric.key] > budgets[metric.key])) {
    process.exitCode = 1;
  }
}

if (import.meta.main) {
  main(process.argv.slice(2));
}
