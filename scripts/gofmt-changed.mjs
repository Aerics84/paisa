import path from "path";
import { fileURLToPath } from "url";
import { collectChangedFiles, existingFiles, run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

const files = existingFiles(rootDir, collectChangedFiles(rootDir), (file) => file.endsWith(".go"));

if (files.length === 0) {
  console.log("No changed Go files to check with gofmt.");
  process.exit(0);
}

const result = run(rootDir, "gofmt", ["-l", ...files], { captureOutput: true });
if (result.status !== 0) {
  process.stderr.write(result.stderr || "");
  process.exit(result.status ?? 1);
}

const unformattedFiles = (result.stdout || "")
  .split(/\r?\n/)
  .map((line) => line.trim())
  .filter(Boolean);

if (unformattedFiles.length > 0) {
  process.stdout.write(`${unformattedFiles.join("\n")}\n`);
  process.exit(1);
}
