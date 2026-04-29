import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const version = fs.readFileSync(path.join(root, "VERSION"), "utf8").trim();

if (!version) {
  throw new Error("VERSION is empty");
}

const wailsConfigPath = path.join(root, "desktop", "wails.json");
const wailsConfig = JSON.parse(fs.readFileSync(wailsConfigPath, "utf8"));
wailsConfig.Info.productVersion = version;
fs.writeFileSync(wailsConfigPath, `${JSON.stringify(wailsConfig, null, 2)}\n`);

const controlPath = path.join(root, "desktop", "build", "linux", "DEBIAN", "control");
const control = fs
  .readFileSync(controlPath, "utf8")
  .replace(/^Version:\s+.*$/m, `Version: ${version}`);
fs.writeFileSync(controlPath, control);
