import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

const read = (path) => readFileSync(path, "utf8");
const goMod = read("go.mod");
const readme = read("README.md");
const mkdocs = read("mkdocs.yml");

const goVersion = goMod.match(/^go (\d+\.\d+)/m)?.[1];
if (!goVersion) throw new Error("go version missing in go.mod");

const failures = [];
const badgeVersions = [...readme.matchAll(/go-(\d+\.\d+)-/g)].map((match) => match[1]);
const textVersions = [...readme.matchAll(/Go (\d+\.\d+)/g)].map((match) => match[1]);
if (!badgeVersions.length || !textVersions.length) {
  failures.push(`README must mention Go ${goVersion} in a badge and in text`);
}
const staleVersions = [...new Set([...badgeVersions, ...textVersions])].filter(
  (version) => version !== goVersion
);
if (staleVersions.length) {
  failures.push(
    `README mentions stale Go ${staleVersions.join(", Go ")}; expected Go ${goVersion}`
  );
}
const upstreamIssueLinks = readdirSync("src", { recursive: true })
  .filter((file) => /\.(ts|svelte)$/.test(file))
  .map((file) => join("src", file))
  .filter((file) => read(file).includes("ananthakumaran/paisa/issues"));
if (upstreamIssueLinks.length) {
  failures.push(
    `upstream issue link in ${upstreamIssueLinks.join(", ")}; use the fork issue tracker`
  );
}
if (!mkdocs.includes("github.com/Aerics84/paisa")) {
  failures.push("mkdocs repo_url must point to the fork");
}
if (!/^repo_name:\s*Aerics84\/paisa\s*$/m.test(mkdocs)) {
  failures.push("mkdocs repo_name must be Aerics84/paisa");
}
if (!/^site_url:\s*https:\/\/aerics84\.github\.io\/paisa\/\s*$/m.test(mkdocs)) {
  failures.push("mkdocs site_url must be https://aerics84.github.io/paisa/");
}
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("docs OK");
