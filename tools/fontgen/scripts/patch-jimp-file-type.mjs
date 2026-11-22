import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

// @jimp/core 0.22 still calls the removed file-type v16 API (`fromBuffer`).
// Modern file-type releases expose the same detection as `fileTypeFromBuffer`
// through either the module namespace or the CJS default wrapper. Support both
// until oslllo-svg2/Jimp move to a fixed file-type API.
const file = join("node_modules", "@jimp", "core", "dist", "utils", "image-bitmap.js");
const legacyCall = "_fileType.default.fromBuffer";
const compatibleCall =
  "((_fileType.default && (_fileType.default.fileTypeFromBuffer || _fileType.default.fromBuffer)) || _fileType.fileTypeFromBuffer)";

const source = readFileSync(file, "utf8");
if (source.includes(compatibleCall)) {
  process.exit(0);
}

if (!source.includes(legacyCall)) {
  console.warn("jimp file-type patch: expected call not found; skipping");
  process.exit(0);
}

writeFileSync(file, source.replaceAll(legacyCall, compatibleCall));
console.log("jimp file-type patch: applied");
