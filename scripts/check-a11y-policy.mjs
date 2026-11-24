import { readFileSync } from "node:fs";

const svelteConfig = readFileSync("svelte.config.shared.js", "utf8");
const packageJson = readFileSync("package.json", "utf8");

// Comments and string literals may legitimately mention `onwarn` without
// overriding it, so strip them before looking for a real property or
// assignment. This keeps the check precise.
function stripCommentsAndStrings(source) {
  return source
    .replace(/"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`/g, '""')
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n\r]*/g, "");
}

const configCode = stripCommentsAndStrings(svelteConfig);

const failures = [];
// Any `onwarn` override drops compiler warnings — including a11y warnings —
// before svelte-check can see them, so the config must not define one at all.
if (/\bonwarn\s*[:=]/.test(configCode) || /(["'])onwarn\1\s*:/.test(svelteConfig)) {
  failures.push("svelte.config.shared.js overrides onwarn (suppresses compiler warnings)");
}
if (/a11y[-_][a-z_-]+:ignore/.test(packageJson)) {
  failures.push("package.json suppresses a11y compiler warnings");
}
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("a11y policy OK");
