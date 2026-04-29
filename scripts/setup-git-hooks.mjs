import path from "path";
import { chmodSync, existsSync } from "fs";
import { fileURLToPath } from "url";
import { run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

const gitDirCheck = run(rootDir, "git", ["rev-parse", "--git-dir"], { captureOutput: true });
if (gitDirCheck.status !== 0) {
  process.exit(0);
}

const configResult = run(rootDir, "git", ["config", "core.hooksPath", ".githooks"]);
if (configResult.status !== 0) {
  process.exit(configResult.status ?? 1);
}

for (const hook of ["pre-commit", "pre-push"]) {
  const hookPath = path.join(rootDir, ".githooks", hook);
  if (existsSync(hookPath)) {
    chmodSync(hookPath, 0o755);
  }
}

console.log("Configured git hooks path to .githooks");
