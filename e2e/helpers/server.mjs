import { spawn, execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import net from "node:net";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const cacheRoot = join(root, ".cache");

const STARTUP_TIMEOUT = 60_000;
const STOP_TIMEOUT = 10_000;

function sleep(ms) {
  return new Promise((resolveSleep) => setTimeout(resolveSleep, ms));
}

async function canListen(port) {
  return new Promise((resolveListen) => {
    const server = net.createServer();
    server.unref();
    server.once("error", () => resolveListen(false));
    server.listen(port, "127.0.0.1", () => server.close(() => resolveListen(true)));
  });
}

async function findFreePort(preferred) {
  const start = Number.isInteger(preferred) && preferred > 0 ? preferred : 5760;
  for (let port = start; port < start + 100; port += 1) {
    if (await canListen(port)) return port;
  }
  throw new Error(`no free port in range ${start}-${start + 99}`);
}

function yamlScalar(value) {
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "number") return String(value);
  if (value === null || value === undefined) return "null";
  // Single quotes are safe for hashes (argon2id$/sha256:) and paths.
  return `'${String(value).replaceAll("'", "''")}'`;
}

function yamlBlock(key, value) {
  if (Array.isArray(value)) {
    const lines = [`${key}:`];
    for (const item of value) {
      if (item && typeof item === "object" && !Array.isArray(item)) {
        const entries = Object.entries(item);
        entries.forEach(([itemKey, itemValue], index) => {
          const prefix = index === 0 ? "  - " : "    ";
          lines.push(`${prefix}${itemKey}: ${yamlScalar(itemValue)}`);
        });
      } else {
        lines.push(`  - ${yamlScalar(item)}`);
      }
    }
    return lines.join("\n");
  }
  if (value && typeof value === "object") {
    const lines = [`${key}:`];
    for (const [childKey, childValue] of Object.entries(value)) {
      lines.push(`  ${childKey}: ${yamlScalar(childValue)}`);
    }
    return lines.join("\n");
  }
  return `${key}: ${yamlScalar(value)}`;
}

/**
 * Apply top-level configuration overrides without a YAML dependency.
 *
 * Existing scalar keys are replaced in place; missing keys are appended. Array
 * and object values are written as indented YAML blocks (used for
 * `user_accounts`). The committed fixtures stay untouched because callers
 * always operate on a fresh copy.
 */
export function applyConfigOverrides(configPath, overrides) {
  let content = existsSync(configPath) ? readFileSync(configPath, "utf8") : "";
  const newline = content.includes("\r\n") ? "\r\n" : "\n";

  for (const [key, value] of Object.entries(overrides ?? {})) {
    const block = yamlBlock(key, value);
    const pattern = new RegExp(`^${key}\\s*:.*$`, "m");
    if (pattern.test(content)) {
      content = content.replace(pattern, block.replaceAll("\n", newline));
    } else {
      if (content.length > 0 && !content.endsWith("\n")) content += newline;
      content += block.replaceAll("\n", newline) + newline;
    }
  }

  writeFileSync(configPath, content);
  return content;
}

function removeDatabase(dir) {
  for (const suffix of ["", "-wal", "-shm", ".synced"]) {
    rmSync(join(dir, `paisa.db${suffix}`), { force: true });
  }
}

async function waitForPing(url, child, logs, deadline) {
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`paisa server exited with code ${child.exitCode} before /api/ping`);
    }
    try {
      const response = await fetch(`${url}/api/ping`, {
        signal: AbortSignal.timeout(1000)
      });
      if (response.ok) return;
    } catch {
      // Server is not ready yet.
    }
    await sleep(250);
  }
  throw new Error(`paisa server did not answer /api/ping within ${STARTUP_TIMEOUT}ms`);
}

function killTree(child) {
  if (!child || child.exitCode !== null || child.signalCode !== null) return;

  try {
    if (process.platform === "win32") {
      execFileSync("taskkill", ["/PID", String(child.pid), "/T", "/F"], {
        stdio: "ignore",
        windowsHide: true
      });
    } else {
      if (child.spawnargs?.includes("go")) {
        try {
          process.kill(-child.pid, "SIGTERM");
        } catch {
          child.kill("SIGTERM");
        }
      } else {
        child.kill("SIGTERM");
      }
    }
  } catch {
    // The process already exited; cleanup below is best effort.
  }
}

async function waitForExit(child, timeout) {
  if (!child || child.exitCode !== null || child.signalCode !== null) return;
  await Promise.race([
    new Promise((resolveExit) => child.once("exit", resolveExit)),
    sleep(timeout)
  ]);
}

/**
 * Start an isolated paisa server for one spec.
 *
 * @param {object} options
 * @param {string} options.name           cache directory suffix, e.g. "login"
 * @param {number} [options.port]         preferred port; a nearby free port is used
 * @param {object} [options.config]       top-level YAML overrides
 * @param {string|null} [options.fixture] fixture directory, default germany-eu
 * @param {(context: {dir: string, configPath: string}) => unknown} [options.seed]
 * @param {boolean} [options.resetDb]     drop copied paisa.db/WAL/SHM files
 * @param {boolean} [options.firstRun]    use the real first-run/onboarding path
 * @param {string[]} [options.args]       extra `serve` flags, e.g. ["--now", "2022-02-15"]
 * @param {Record<string, string>} [options.env]
 * @returns {Promise<{url: string, dir: string, configPath: string, port: number, logs: () => string, stop: () => Promise<void>}>}
 */
export async function startServer({
  name,
  port: preferredPort,
  config = {},
  fixture = "tests/fixture/germany-eu",
  seed,
  resetDb = false,
  firstRun = false,
  args = [],
  env = {}
}) {
  if (!name) throw new Error("startServer requires a name");

  const dir = join(cacheRoot, `e2e-${name}`);
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });

  let configPath;
  const serverEnv = { ...process.env, ...env, PAISA_DISABLE_LOG_FILE: "true" };

  if (firstRun) {
    // A missing PAISA_CONFIG currently exits before onboarding can run, so use
    // the supported first-run path: no config in cwd, isolated XDG Documents.
    if (existsSync(join(root, "paisa.yaml"))) {
      throw new Error(
        "firstRun requires the worktree root to be free of paisa.yaml; move it away before running onboarding e2e"
      );
    }
    delete serverEnv.PAISA_CONFIG;
    const documents = join(dir, "Documents");
    mkdirSync(documents, { recursive: true });
    serverEnv.XDG_DOCUMENTS_DIR = documents;
    configPath = join(documents, "paisa", "paisa.yaml");
  } else {
    if (fixture) {
      cpSync(join(root, fixture), dir, { recursive: true, preserveTimestamps: true });
    }
    if (resetDb) removeDatabase(dir);

    configPath = join(dir, "paisa.yaml");
    if (Object.keys(config).length > 0) {
      applyConfigOverrides(configPath, config);
    }
    serverEnv.PAISA_CONFIG = configPath;
  }

  if (seed) {
    await seed({ dir, configPath });
  }

  const port = await findFreePort(preferredPort);
  const url = `http://127.0.0.1:${port}`;
  const logs = [];
  const child = spawn("go", ["run", ".", "serve", "--port", String(port), ...args], {
    cwd: root,
    stdio: ["ignore", "pipe", "pipe"],
    windowsHide: true,
    detached: process.platform !== "win32",
    env: serverEnv
  });

  child.stdout?.on("data", (chunk) => logs.push(chunk.toString()));
  child.stderr?.on("data", (chunk) => logs.push(chunk.toString()));

  let stopped = false;
  const stop = async () => {
    if (stopped) return;
    stopped = true;
    killTree(child);
    await waitForExit(child, STOP_TIMEOUT);
    killTree(child);
    for (let attempt = 0; attempt < 5; attempt += 1) {
      try {
        rmSync(dir, { recursive: true, force: true });
        break;
      } catch {
        await sleep(200);
      }
    }
  };

  child.on("error", (error) => logs.push(`spawn error: ${error.message}\n`));

  try {
    await waitForPing(url, child, logs, Date.now() + STARTUP_TIMEOUT);
    const response = await fetch(`${url}/api/ping`);
    if (!response.ok) throw new Error(`/api/ping returned ${response.status}`);
  } catch (error) {
    await stop();
    throw new Error(
      `failed to start e2e server "${name}" on port ${port}: ${error.message}\n` +
        `--- server log ---\n${logs.join("")}`
    );
  }

  return {
    url,
    dir,
    configPath,
    port,
    logs: () => logs.join(""),
    stop
  };
}
