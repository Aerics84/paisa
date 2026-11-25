import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = "web/static";
const pngSignature = "89504e470d0a1a0a";
const errors = [];

function fail(message) {
  errors.push(message);
}

function pngSize(relativePath) {
  const path = join(root, relativePath.replace(/^\/+/, ""));
  if (!existsSync(path)) {
    fail(`missing ${path}`);
    return null;
  }

  const bytes = readFileSync(path);
  if (bytes.length < 24 || bytes.subarray(0, 8).toString("hex") !== pngSignature) {
    fail(`${path} is not a PNG file`);
    return null;
  }
  if (bytes.subarray(12, 16).toString("ascii") !== "IHDR") {
    fail(`${path} has no PNG IHDR header`);
    return null;
  }

  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

const manifestPath = join(root, "manifest.webmanifest");
let manifest = null;

if (!existsSync(manifestPath)) {
  fail(`missing ${manifestPath}`);
} else {
  try {
    manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  } catch (error) {
    fail(`${manifestPath} is not valid JSON: ${error.message}`);
  }
}

if (manifest) {
  for (const field of [
    "name",
    "short_name",
    "start_url",
    "scope",
    "display",
    "background_color",
    "theme_color"
  ]) {
    if (typeof manifest[field] !== "string" || manifest[field].length === 0) {
      fail(`manifest field "${field}" is missing`);
    }
  }
  if (manifest.start_url !== "/")
    fail(`manifest start_url is "${manifest.start_url}", expected "/"`);
  if (manifest.scope !== "/") fail(`manifest scope is "${manifest.scope}", expected "/"`);
  if (manifest.display !== "standalone") {
    fail(`manifest display is "${manifest.display}", expected "standalone"`);
  }
  if (manifest.background_color !== "#ffffff") {
    fail(`manifest background_color is "${manifest.background_color}", expected "#ffffff"`);
  }
  if (manifest.theme_color !== "#3d5afe") {
    fail(`manifest theme_color is "${manifest.theme_color}", expected "#3d5afe"`);
  }
  if (!Array.isArray(manifest.icons)) {
    fail('manifest field "icons" is missing or not an array');
  }
}

const expectedIcons = [
  { src: "/icons/icon-192.png", size: 192, maskable: false },
  { src: "/icons/icon-512.png", size: 512, maskable: false },
  { src: "/icons/maskable-512.png", size: 512, maskable: true }
];

for (const expected of expectedIcons) {
  if (manifest && Array.isArray(manifest.icons)) {
    const icon = manifest.icons.find((entry) => entry.src === expected.src);
    if (!icon) {
      fail(`manifest is missing icon ${expected.src}`);
    } else {
      const size = `${expected.size}x${expected.size}`;
      if (icon.sizes !== size)
        fail(`${expected.src} declares sizes "${icon.sizes}", expected "${size}"`);
      if (icon.type !== "image/png")
        fail(`${expected.src} declares type "${icon.type}", expected "image/png"`);
      if (expected.maskable && icon.purpose !== "maskable") {
        fail(`${expected.src} declares purpose "${icon.purpose}", expected "maskable"`);
      }
    }
  }

  const dimensions = pngSize(expected.src);
  if (dimensions && (dimensions.width !== expected.size || dimensions.height !== expected.size)) {
    fail(
      `${expected.src} is ${dimensions.width}x${dimensions.height}, expected ${expected.size}x${expected.size}`
    );
  }
}

const appleIcon = pngSize("apple-touch-icon.png");
if (appleIcon && appleIcon.width !== appleIcon.height) {
  fail(`apple-touch-icon.png is not square (${appleIcon.width}x${appleIcon.height})`);
}

if (!existsSync(join(root, "sw.js"))) {
  fail(`missing ${join(root, "sw.js")}`);
}

if (errors.length > 0) {
  console.error("PWA check failed:");
  for (const error of errors) {
    console.error(`  FAILED ${error}`);
  }
  process.exit(1);
}

console.log(`PWA manifest: ${manifestPath}`);
for (const icon of manifest.icons) {
  const dimensions = pngSize(icon.src);
  console.log(`  ${icon.src} ${dimensions.width}x${dimensions.height} (${icon.sizes})`);
}
console.log(`  /apple-touch-icon.png ${appleIcon.width}x${appleIcon.height}`);
console.log("pwa OK");
