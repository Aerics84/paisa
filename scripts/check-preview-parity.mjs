import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const APP_DIR = "src/routes/(app)";
const PREVIEW_DIR = "src/routes/preview";

const base = process.env.PREVIEW_PARITY_BASE_SHA || process.env.PRETTIER_BASE_SHA || "HEAD~1";

function git(args) {
  return execFileSync("git", ["-c", "core.quotePath=false", ...args], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"]
  });
}

function normalize(file) {
  return file.replace(/\\/g, "/");
}

function fail(code, message) {
  console.error(message);
  process.exit(code);
}

// A missing or misspelled base ref would otherwise surface as a confusing
// `git diff` failure. Resolve it first and fail loudly.
try {
  git(["rev-parse", "--verify", "--quiet", `${base}^{commit}`]);
} catch {
  fail(
    2,
    `preview parity: base ref "${base}" is not a valid commit.\n` +
      "Set PREVIEW_PARITY_BASE_SHA (or PRETTIER_BASE_SHA) to a commit that exists in this clone."
  );
}

let map;
try {
  map = JSON.parse(readFileSync("scripts/preview-mirror-map.json", "utf8"));
} catch (error) {
  fail(2, `preview parity: cannot read scripts/preview-mirror-map.json\n${error.message}`);
}

if (!map || typeof map !== "object" || Array.isArray(map)) {
  fail(
    2,
    "preview parity: scripts/preview-mirror-map.json must be an object of app -> preview paths."
  );
}
for (const [appPath, previewPath] of Object.entries(map)) {
  if (typeof previewPath !== "string" || !previewPath) {
    fail(2, `preview parity: map entry "${appPath}" must point at a non-empty preview path.`);
  }
}

let diff;
try {
  diff = git(["diff", "--name-only", "--no-renames", base, "--", APP_DIR, PREVIEW_DIR]);
} catch (error) {
  const stderr = error.stderr ? String(error.stderr).trim() : "";
  fail(2, `preview parity: git diff against "${base}" failed.\n${stderr || error.message}`);
}

const changed = new Set(
  diff
    .split(/\r?\n/)
    .map((line) => normalize(line.trim()))
    .filter(Boolean)
);

const missing = [];
for (const [appPath, previewPath] of Object.entries(map)) {
  const app = `${APP_DIR}/${normalize(appPath)}`;
  const preview = `${PREVIEW_DIR}/${normalize(previewPath)}`;
  const appChanged = changed.has(app);
  const previewChanged = changed.has(preview);
  if (appChanged !== previewChanged) {
    const source = appChanged ? app : preview;
    const mirror = appChanged ? preview : app;
    missing.push(`- ${source} changed without its mirror ${mirror}`);
  }
}

if (missing.length) {
  console.error(
    [
      `preview parity drift detected against "${base}":`,
      ...missing,
      "Edit both files together, or drop the pair from scripts/preview-mirror-map.json",
      "if the routes are intentionally allowed to diverge."
    ].join("\n")
  );
  process.exit(1);
}

console.log("preview parity OK");
