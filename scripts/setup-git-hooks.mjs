import path from "path";
import { chmodSync, existsSync } from "fs";
import { fileURLToPath } from "url";
import { run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

function safeRun(command, args, extra = {}) {
  try {
    return run(rootDir, command, args, extra);
  } catch {
    return { status: 1 };
  }
}

const gitDirCheck = safeRun("git", ["rev-parse", "--git-dir"], { captureOutput: true });
if (gitDirCheck.status !== 0) {
  process.exit(0);
}

const configResult = safeRun("git", ["config", "core.hooksPath", ".githooks"]);
if (configResult.status !== 0) {
  process.exit(0);
}

for (const hook of ["pre-commit", "pre-push"]) {
  const hookPath = path.join(rootDir, ".githooks", hook);
  if (existsSync(hookPath)) {
    chmodSync(hookPath, 0o755);
  }
}

console.log("Configured git hooks path to .githooks");
