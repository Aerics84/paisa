import { spawn } from "node:child_process";
import { cpSync, existsSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const source = join(root, "tests", "fixture", "germany-eu");
const target = join(root, ".cache", "e2e");
const port = process.env.PAISA_E2E_PORT || "5758";

const index = join(root, "web", "static", "index.html");
if (!existsSync(index)) {
  console.error("web/static/index.html is missing. Run `npm run build` before `npm run e2e`.");
  process.exit(1);
}

// Never touch the committed fixture. Each run gets a fresh copy, including the
// SQLite main database, WAL and SHM files so the database stays consistent.
rmSync(target, { recursive: true, force: true });
mkdirSync(target, { recursive: true });
cpSync(source, target, { recursive: true, preserveTimestamps: true });

const configPath = join(target, "paisa.yaml");

const child = spawn("go", ["run", ".", "serve", "--port", port], {
  cwd: root,
  stdio: "inherit",
  windowsHide: true,
  env: { ...process.env, PAISA_CONFIG: configPath }
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => child.kill(signal));
}

child.on("error", (error) => {
  console.error("failed to start paisa server:", error);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.exit(0);
  }
  process.exit(code ?? 0);
});
