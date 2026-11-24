import { readFileSync } from "node:fs";

const svelteConfig = readFileSync("svelte.config.shared.js", "utf8");
const packageJson = readFileSync("package.json", "utf8");

const failures = [];
if (/startsWith\((["'])a11y[-_]/.test(svelteConfig)) {
  failures.push("svelte.config.shared.js suppresses a11y warnings");
}
if (/a11y[-_][a-z_-]+:ignore/.test(packageJson)) {
  failures.push("package.json suppresses a11y compiler warnings");
}
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("a11y policy OK");
