import path from "path";
import { fileURLToPath } from "url";
import { collectChangedFiles, existingFiles, gitDiffNames, run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

const options = {
  mode: "check",
  stagedOnly: false,
  restage: false,
  failOnPartialStaging: false
};

for (const arg of process.argv.slice(2)) {
  switch (arg) {
    case "--write":
      options.mode = "write";
      break;
    case "--staged":
      options.stagedOnly = true;
      break;
    case "--restage":
      options.restage = true;
      break;
    case "--fail-on-partial-staging":
      options.failOnPartialStaging = true;
      break;
    default:
      console.error(`Unknown option: ${arg}`);
      process.exit(1);
  }
}

const files = existingFiles(
  rootDir,
  collectChangedFiles(rootDir, { stagedOnly: options.stagedOnly })
);

if (options.failOnPartialStaging && options.stagedOnly && files.length > 0) {
  const unstagedFiles = new Set(gitDiffNames(rootDir));
  const partialFiles = files.filter((file) => unstagedFiles.has(file));

  if (partialFiles.length > 0) {
    console.error(
      "Prettier auto-fix skipped because these staged files also have unstaged changes:"
    );
    for (const file of partialFiles) {
      console.error(`- ${file}`);
    }
    console.error("Stage the full file or run `npm run format:changed` before committing.");
    process.exit(1);
  }
}

if (files.length === 0) {
  console.log(
    options.stagedOnly
      ? "No staged files to check with Prettier."
      : "No changed files to check with Prettier."
  );
  process.exit(0);
}

const prettierBin = path.join(rootDir, "node_modules", "prettier", "bin", "prettier.cjs");
const prettierResult = run(rootDir, process.execPath, [
  prettierBin,
  `--${options.mode}`,
  "--ignore-unknown",
  ...files
]);

if (prettierResult.status !== 0) {
  process.exit(prettierResult.status ?? 1);
}

if (options.restage) {
  const addResult = run(rootDir, "git", ["add", "--", ...files]);
  if (addResult.status !== 0) {
    process.exit(addResult.status ?? 1);
  }
}
