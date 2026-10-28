import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

const DEFAULT_NPM_DIRS = [".", "tools/fontgen"];
const GOVULNCHECK_VERSION = "v1.9.0";
const SEVERITIES = ["info", "low", "moderate", "high", "critical", "total"];

function isWindows() {
  return process.platform === "win32";
}

function runNpm(args, options) {
  if (isWindows()) {
    return spawnSync(
      process.env.ComSpec ?? "cmd.exe",
      ["/d", "/s", "/c", `npm ${args.join(" ")}`],
      { ...options, windowsHide: true }
    );
  }
  return spawnSync("npm", args, options);
}

/**
 * Extract the vulnerability counts from `npm audit --json` output.
 * Throws when the output is not a usable audit document.
 */
export function summarizeNpmAudit(json) {
  const vulnerabilities = json?.metadata?.vulnerabilities;
  if (!vulnerabilities || typeof vulnerabilities !== "object") {
    throw new Error("npm audit output is missing metadata.vulnerabilities");
  }

  const summary = {};
  for (const severity of SEVERITIES) {
    const value = Number(vulnerabilities[severity] ?? 0);
    if (!Number.isFinite(value)) {
      throw new Error(`npm audit severity "${severity}" is not a number`);
    }
    summary[severity] = value;
  }
  return summary;
}

/**
 * Parse the newline-delimited JSON stream emitted by `govulncheck -json`.
 * Findings are deduplicated by OSV id; traces are merged so a called
 * occurrence cannot be hidden by a later not-called duplicate.
 */
function splitJsonObjects(text) {
  const input = String(text ?? "");
  const objects = [];
  let start = -1;
  let depth = 0;
  let inString = false;
  let escaped = false;

  for (let index = 0; index < input.length; index += 1) {
    const char = input[index];

    if (start === -1) {
      if (/\s/.test(char)) continue;
      if (char !== "{") {
        throw new Error(
          `govulncheck output has unexpected character ${JSON.stringify(char)} at position ${index}`
        );
      }
      start = index;
      depth = 1;
      inString = false;
      escaped = false;
      continue;
    }

    if (inString) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === '"') inString = false;
      continue;
    }

    if (char === '"') {
      inString = true;
    } else if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        objects.push(input.slice(start, index + 1));
        start = -1;
      }
    }
  }

  if (start !== -1) {
    throw new Error("govulncheck output ended inside a JSON object");
  }
  return objects;
}

export function parseGovulncheckStream(text) {
  const objects = splitJsonObjects(text);

  if (objects.length === 0) {
    throw new Error("govulncheck produced no JSON output");
  }

  let config = null;
  const findings = new Map();

  for (const [index, line] of objects.entries()) {
    let parsed;
    try {
      parsed = JSON.parse(line);
    } catch (error) {
      throw new Error(`govulncheck object ${index + 1} is not valid JSON: ${error.message}`, {
        cause: error
      });
    }

    if (parsed?.config && !config) {
      config = parsed.config;
    }

    const finding = parsed?.finding;
    if (!finding) continue;

    const id = finding.osv ?? finding.id;
    if (!id) {
      throw new Error(`govulncheck finding in object ${index + 1} has no OSV id`);
    }

    const existing = findings.get(id);
    if (existing) {
      existing.trace = [...(existing.trace ?? []), ...(finding.trace ?? [])];
    } else {
      findings.set(id, { ...finding, osv: id, trace: [...(finding.trace ?? [])] });
    }
  }

  if (!config) {
    throw new Error("govulncheck output contains no config object");
  }

  return { config, findings: [...findings.values()] };
}

function frameBelongsToModule(frame, mainModule) {
  if (!frame || !mainModule) return false;
  if (frame.module === mainModule) return true;
  return typeof frame.package === "string" && frame.package.startsWith(`${mainModule}/`);
}

/**
 * Split findings into reachable (`called`) and unreachable (`not called`)
 * findings. A duplicate OSV id is called when any of its traces reaches the
 * main module.
 */
export function classifyGovulncheckFindings(findings, mainModule) {
  const byId = new Map();

  for (const finding of findings ?? []) {
    const id = finding.osv ?? finding.id;
    if (!id) continue;

    const called = (finding.trace ?? []).some((frame) => frameBelongsToModule(frame, mainModule));
    const existing = byId.get(id);

    if (!existing) {
      byId.set(id, { finding: { ...finding, osv: id }, called });
    } else if (called) {
      existing.called = true;
    }
  }

  const called = [];
  const notCalled = [];
  for (const { finding, called: isCalled } of byId.values()) {
    (isCalled ? called : notCalled).push(finding);
  }

  const byOsv = (a, b) => String(a.osv).localeCompare(String(b.osv));
  return { called: called.sort(byOsv), notCalled: notCalled.sort(byOsv) };
}

function spawnResult(result) {
  return {
    exitCode: result.status,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? "",
    error: result.error
  };
}

export function defaultRunNpmAudit(dir) {
  const result = runNpm(["audit", "--json"], {
    cwd: path.resolve(process.cwd(), dir),
    encoding: "utf8",
    maxBuffer: 128 * 1024 * 1024
  });
  return spawnResult(result);
}

export function defaultRunGovulncheck() {
  const run = (cmd, args) =>
    spawnSync(cmd, args, {
      cwd: process.cwd(),
      encoding: "utf8",
      maxBuffer: 128 * 1024 * 1024
    });

  let result = run("govulncheck", ["-json", "./..."]);
  if (result.error?.code === "ENOENT") {
    result = run("go", [
      "run",
      `golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}`,
      "-json",
      "./..."
    ]);
  }
  return spawnResult(result);
}

export function defaultResolveMainModule() {
  const result = spawnSync("go", ["list", "-m", "-f", "{{.Path}}"], {
    cwd: process.cwd(),
    encoding: "utf8"
  });
  if (result.status !== 0 || !result.stdout?.trim()) {
    throw new Error(
      `cannot resolve the main Go module: ${result.stderr || result.error?.message || "go list failed"}`
    );
  }
  return result.stdout.trim();
}

function uniqueDirs(dirs) {
  const seen = new Set();
  const result = [];
  for (const dir of dirs) {
    const normalized = path.normalize(dir);
    if (!seen.has(normalized)) {
      seen.add(normalized);
      result.push(normalized);
    }
  }
  return result;
}

function writeReports(reportDir, summary, raw, exitCode) {
  fs.mkdirSync(reportDir, { recursive: true });

  const npmRaw = {};
  for (const entry of raw.npm) {
    npmRaw[entry.dir] = {
      exitCode: entry.exitCode,
      stdout: entry.stdout,
      stderr: entry.stderr
    };
  }

  fs.writeFileSync(path.join(reportDir, "npm-audit.json"), JSON.stringify(npmRaw, null, 2) + "\n");
  fs.writeFileSync(
    path.join(reportDir, "govulncheck.json"),
    JSON.stringify(
      {
        exitCode: raw.govulncheck?.exitCode ?? null,
        stdout: raw.govulncheck?.stdout ?? "",
        stderr: raw.govulncheck?.stderr ?? ""
      },
      null,
      2
    ) + "\n"
  );
  fs.writeFileSync(path.join(reportDir, "summary.json"), JSON.stringify(summary, null, 2) + "\n");

  const exitCodes = {
    npm: Object.fromEntries(raw.npm.map((entry) => [entry.dir, entry.exitCode])),
    govulncheck: raw.govulncheck?.exitCode ?? null,
    gate: exitCode
  };
  fs.writeFileSync(
    path.join(reportDir, "exit-codes.json"),
    JSON.stringify(exitCodes, null, 2) + "\n"
  );
}

/**
 * Run the npm + govulncheck gate. All tool invocations are injected so tests
 * never touch the network or the local toolchain.
 */
export async function runGate({
  npmDirs,
  runNpmAudit = defaultRunNpmAudit,
  runGovulncheck = defaultRunGovulncheckForGate,
  resolveMainModule = defaultResolveMainModule,
  mainModule = null,
  reportDir = null
} = {}) {
  const errors = [];
  const raw = { npm: [], govulncheck: null };

  const dirs = uniqueDirs(
    npmDirs ??
      DEFAULT_NPM_DIRS.filter(
        (dir) => dir === "." || fs.existsSync(path.join(dir, "package-lock.json"))
      )
  );

  for (const dir of dirs) {
    if (!fs.existsSync(path.join(dir, "package-lock.json"))) {
      errors.push(
        `npm project "${dir}" has no lockfile (package-lock.json missing; configuration/scanner error)`
      );
    }
  }

  if (errors.length > 0) {
    const summary = {
      generated_at: new Date().toISOString(),
      npm: [],
      govulncheck: null,
      errors
    };
    if (reportDir) writeReports(reportDir, summary, raw, 2);
    return { exitCode: 2, summary, raw };
  }

  const npm = [];
  let npmHasFinding = false;

  for (const dir of dirs) {
    let result;
    try {
      result = await runNpmAudit(dir);
    } catch (error) {
      errors.push(`npm audit failed in "${dir}": ${error.message}`);
      continue;
    }

    raw.npm.push({
      dir,
      exitCode: result?.exitCode ?? null,
      stdout: result?.stdout ?? "",
      stderr: result?.stderr ?? ""
    });

    if (result?.error) {
      errors.push(`npm audit could not run in "${dir}": ${result.error.message}`);
      continue;
    }

    let parsed;
    try {
      parsed = JSON.parse(result?.stdout ?? "");
      const vulnerabilities = summarizeNpmAudit(parsed);
      npm.push({ dir, exitCode: result?.exitCode ?? null, vulnerabilities });
      if (vulnerabilities.critical > 0 || vulnerabilities.high > 0) {
        npmHasFinding = true;
      }
    } catch (error) {
      errors.push(`npm audit in "${dir}" returned unusable output: ${error.message}`);
    }
  }

  let govulncheck = null;
  let govulncheckHasCalledFinding = false;

  try {
    const resolvedMainModule = mainModule ?? resolveMainModule();

    const result = await runGovulncheck();
    raw.govulncheck = {
      exitCode: result?.exitCode ?? null,
      stdout: result?.stdout ?? "",
      stderr: result?.stderr ?? ""
    };

    if (result?.error) {
      throw new Error(`govulncheck could not run: ${result.error.message}`);
    }

    const parsed = parseGovulncheckStream(result?.stdout ?? "");
    const classified = classifyGovulncheckFindings(parsed.findings, resolvedMainModule);

    govulncheck = {
      called: classified.called.length,
      not_called: classified.notCalled.length,
      db_last_modified: parsed.config.db_last_modified ?? null,
      go_version: parsed.config.go_version ?? null
    };
    govulncheckHasCalledFinding = classified.called.length > 0;
  } catch (error) {
    errors.push(`govulncheck scan failed: ${error.message}`);
  }

  const exitCode = errors.length > 0 ? 2 : npmHasFinding || govulncheckHasCalledFinding ? 1 : 0;

  const summary = {
    generated_at: new Date().toISOString(),
    npm,
    govulncheck,
    errors
  };

  if (reportDir) writeReports(reportDir, summary, raw, exitCode);

  return { exitCode, summary, raw };
}

async function defaultRunGovulncheckForGate() {
  return defaultRunGovulncheck();
}

function parseArgs(argv) {
  const options = { reportDir: null, npmDirs: [] };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--report") {
      options.reportDir = argv[index + 1];
      index += 1;
      if (!options.reportDir) throw new Error("--report requires a directory");
    } else if (arg === "--npm-dir") {
      const dir = argv[index + 1];
      index += 1;
      if (!dir) throw new Error("--npm-dir requires a directory");
      options.npmDirs.push(dir);
    } else if (arg === "--help" || arg === "-h") {
      options.help = true;
    } else {
      throw new Error(`unknown argument: ${arg}`);
    }
  }

  return options;
}

async function main() {
  let options;
  try {
    options = parseArgs(process.argv.slice(2));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 2;
    return;
  }

  if (options.help) {
    console.log("usage: node scripts/security-gate.mjs [--report <dir>] [--npm-dir <dir>]...");
    return;
  }

  const npmDirs = uniqueDirs([
    ".",
    ...options.npmDirs,
    ...DEFAULT_NPM_DIRS.filter(
      (dir) => dir !== "." && fs.existsSync(path.join(dir, "package-lock.json"))
    )
  ]);

  try {
    const result = await runGate({
      npmDirs,
      reportDir: options.reportDir
    });
    console.log(JSON.stringify(result.summary, null, 2));
    process.exitCode = result.exitCode;
  } catch (error) {
    console.error(`security gate failed: ${error.stack || error.message}`);
    process.exitCode = 2;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
