#!/usr/bin/env node

/**
 * Preview parity guard.
 *
 * Every pair in scripts/preview-mirror-map.json is a page that the preview and
 * (app) route trees are supposed to share verbatim. The guard fails when a
 * change touches exactly one side of a mapped pair.
 *
 * Base ref resolution (first usable commit wins):
 *   1. PREVIEW_PARITY_BASE_SHA — explicit override. If it is set to something
 *      that cannot be resolved, the run fails with exit 2 (a typo here should
 *      never be swallowed).
 *   2. PRETTIER_BASE_SHA — CI-provided base. Invalid or all-zero values (the
 *      "before" SHA of a new branch) are ignored.
 *   3. HEAD~1 — local default.
 *   If none of these resolve (e.g. a single-commit checkout), the guard warns
 *   and exits 0: it cannot compare anything, but that is not a code error.
 *
 * Exit codes: 0 = in sync or skipped, 1 = drift, 2 = unusable explicit
 * override or broken map file.
 */

import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const APP_DIR = "src/routes/(app)";
const PREVIEW_DIR = "src/routes/preview";
const MAP_FILE = "scripts/preview-mirror-map.json";

const ZERO_SHA = /^0+$/;

function git(args) {
  return execFileSync("git", ["-c", "core.quotePath=false", ...args], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"]
  });
}

function hasCommit(ref) {
  try {
    git(["rev-parse", "--verify", "--quiet", `${ref}^{commit}`]);
    return true;
  } catch {
    return false;
  }
}

function normalize(file) {
  return file.replace(/\\/g, "/");
}

function fail(code, message) {
  console.error(message);
  process.exit(code);
}

function pickBase() {
  const explicit = (process.env.PREVIEW_PARITY_BASE_SHA || "").trim();
  const ciBase = (process.env.PRETTIER_BASE_SHA || "").trim();

  if (explicit && !ZERO_SHA.test(explicit)) {
    if (!hasCommit(explicit)) {
      fail(
        2,
        `preview parity: PREVIEW_PARITY_BASE_SHA="${explicit}" is not a valid commit.\n` +
          "Unset it or point it at a commit that exists in this clone."
      );
    }
    return explicit;
  }

  if (ciBase && !ZERO_SHA.test(ciBase)) {
    if (hasCommit(ciBase)) {
      return ciBase;
    }
    console.warn(`preview parity: ignoring PRETTIER_BASE_SHA="${ciBase}" (not a commit).`);
  }

  if (hasCommit("HEAD~1")) {
    return "HEAD~1";
  }

  console.warn(
    "preview parity skipped: no usable base commit " +
      `(PREVIEW_PARITY_BASE_SHA, PRETTIER_BASE_SHA, HEAD~1 all unavailable).`
  );
  return null;
}

const base = pickBase();
if (!base) {
  process.exit(0);
}

let map;
try {
  map = JSON.parse(readFileSync(MAP_FILE, "utf8"));
} catch (error) {
  fail(2, `preview parity: cannot read ${MAP_FILE}\n${error.message}`);
}

if (!map || typeof map !== "object" || Array.isArray(map)) {
  fail(2, `preview parity: ${MAP_FILE} must be an object of app -> preview paths.`);
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
      `Edit both files together, or drop the pair from ${MAP_FILE}`,
      "if the routes are intentionally allowed to diverge."
    ].join("\n")
  );
  process.exit(1);
}

console.log(`preview parity OK (${Object.keys(map).length} mirrored pairs, base ${base})`);
