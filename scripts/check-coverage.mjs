import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

const DEFAULT_THRESHOLD = 75;
const TEST_ARGS = ["test", "--coverage", "--preload", "./src/happydom.ts", "src"];

function parseThreshold(value, label) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 100) {
    throw new Error(`${label} must be a number between 0 and 100, received "${value}"`);
  }
  return parsed;
}

export function parseArgs(argv) {
  const options = { lines: DEFAULT_THRESHOLD, functions: DEFAULT_THRESHOLD, help: false };

  for (let index = 0; index < argv.length; index += 1) {
    const [flag, inlineValue] = argv[index].split("=", 2);
    switch (flag) {
      case "--lines":
      case "--functions": {
        const value = inlineValue ?? argv[index + 1];
        if (value === undefined || value.trim() === "") {
          throw new Error(`${flag} requires a value`);
        }
        if (inlineValue === undefined) {
          index += 1;
        }
        options[flag.slice(2)] = parseThreshold(value, flag);
        break;
      }
      case "--help":
      case "-h":
        options.help = true;
        break;
      default:
        throw new Error(`Unknown option: ${argv[index]}`);
    }
  }

  return options;
}

function stripAnsi(text) {
  // eslint-disable-next-line no-control-regex
  return text.replace(/\u001b\[[0-9;]*m/g, "");
}

/**
 * Extract the aggregate coverage row that `bun test --coverage` prints:
 *
 *   All files            |   78.77 |   76.98 |
 *
 * The table columns are `% Funcs` and `% Lines`; Bun 1.4.4 reports no
 * statement or branch metrics.
 */
export function parseCoverageSummary(output) {
  const line = stripAnsi(output)
    .split(/\r?\n/)
    .find((candidate) => /^\s*All files\s*\|/.test(candidate));

  if (!line) {
    throw new Error("coverage output has no 'All files' row");
  }

  const cells = line.split("|").map((cell) => cell.trim());
  if (cells.length < 3) {
    throw new Error(`unexpected 'All files' row: ${line.trim()}`);
  }

  const functions = Number(cells[1]);
  const lines = Number(cells[2]);
  if (!Number.isFinite(functions) || !Number.isFinite(lines)) {
    throw new Error(`unexpected 'All files' row: ${line.trim()}`);
  }

  return { functions, lines };
}

export function evaluateCoverage(summary, thresholds) {
  const failures = [];
  if (summary.lines < thresholds.lines) {
    failures.push(`lines ${summary.lines}% is below the ${thresholds.lines}% threshold`);
  }
  if (summary.functions < thresholds.functions) {
    failures.push(
      `functions ${summary.functions}% is below the ${thresholds.functions}% threshold`
    );
  }
  return failures;
}

function writeResult(result) {
  if (result.stdout) process.stdout.write(result.stdout);
  if (result.stderr) process.stderr.write(result.stderr);
}

async function main() {
  let options;
  try {
    options = parseArgs(process.argv.slice(2));
  } catch (error) {
    console.error(`coverage gate: ${error.message}`);
    process.exitCode = 2;
    return;
  }

  if (options.help) {
    console.log("usage: node scripts/check-coverage.mjs [--lines <n>] [--functions <n>]");
    return;
  }

  let result;
  try {
    result = run(rootDir, "bun", TEST_ARGS, {
      captureOutput: true,
      maxBuffer: 64 * 1024 * 1024
    });
  } catch (error) {
    console.error(`coverage gate: could not run bun: ${error.message}`);
    process.exitCode = 2;
    return;
  }
  writeResult(result);

  if (result.status !== 0) {
    console.error(`\ncoverage gate: the test suite failed (exit ${result.status ?? 1})`);
    process.exitCode = result.status ?? 1;
    return;
  }

  let summary;
  try {
    summary = parseCoverageSummary(`${result.stdout ?? ""}\n${result.stderr ?? ""}`);
  } catch (error) {
    console.error(`\ncoverage gate: ${error.message}`);
    process.exitCode = 2;
    return;
  }

  const failures = evaluateCoverage(summary, options);
  console.log(
    `\ncoverage gate: lines ${summary.lines}% (threshold ${options.lines}%), ` +
      `functions ${summary.functions}% (threshold ${options.functions}%)`
  );

  if (failures.length > 0) {
    for (const failure of failures) {
      console.error(`coverage gate: ${failure}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log("coverage gate: passed");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
