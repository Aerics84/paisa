import { readFileSync } from "node:fs";

const read = (path) => readFileSync(path, "utf8");
const goMod = read("go.mod");
const readme = read("README.md");
const hooks = read("src/hooks.client.ts");
const mkdocs = read("mkdocs.yml");

const goVersion = goMod.match(/^go (\d+\.\d+)/m)?.[1];
if (!goVersion) throw new Error("go version missing in go.mod");

const failures = [];
if (!readme.includes(`go-${goVersion}-`) || !readme.includes(`Go ${goVersion}`)) {
  failures.push(`README must mention Go ${goVersion}`);
}
const staleGo = ["Go 1.24", "go-1.24"].filter((needle) => readme.includes(needle));
if (staleGo.length) {
  failures.push(`README must not mention ${staleGo.join(" or ")}`);
}
if (hooks.includes("ananthakumaran/paisa/issues")) {
  failures.push("error footer must point to the fork issue tracker");
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
