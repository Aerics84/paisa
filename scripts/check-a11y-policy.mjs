import { readFileSync } from "node:fs";

const svelteConfig = readFileSync("svelte.config.shared.js", "utf8");
const packageJson = readFileSync("package.json", "utf8");

const failures = [];
// Any textual occurrence of `onwarn` counts, including comments and strings.
// There is no legitimate `onwarn` hook in this config, and a bare mention is
// not worth the bypass risk (shorthand, computed keys, other warning APIs).
// Warnings are gated with `svelte-check --fail-on-warnings` instead.
if (svelteConfig.includes("onwarn")) {
  failures.push(
    "svelte.config.shared.js must not define or mention onwarn; use --fail-on-warnings instead"
  );
}
if (/a11y[-_][a-z_-]+:ignore/.test(packageJson)) {
  failures.push("package.json suppresses a11y compiler warnings");
}
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("a11y policy OK");
