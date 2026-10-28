import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

function toPosix(file) {
  return file.split(path.sep).join("/");
}

function hashFile(file) {
  return `sha256:${crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex")}`;
}

function walk(target, files) {
  const stat = fs.statSync(target);
  if (stat.isDirectory()) {
    for (const entry of fs.readdirSync(target).sort()) {
      walk(path.join(target, entry), files);
    }
  } else if (stat.isFile()) {
    files.push(target);
  }
}

function sortPaths(paths) {
  return paths.sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
}

function hashPaths(paths) {
  const files = [];
  for (const target of paths) {
    if (!fs.existsSync(target)) {
      throw new Error(`path does not exist: ${target}`);
    }
    walk(target, files);
  }

  const manifest = {};
  const keys = sortPaths(
    files.map((file) => toPosix(path.relative(process.cwd(), path.resolve(file))))
  );
  for (const key of keys) {
    if (manifest[key]) continue;
    manifest[key] = hashFile(path.resolve(process.cwd(), key));
  }
  return { files: manifest };
}

function gitFiles(paths) {
  const args = ["ls-files", "-z"];
  if (paths.length > 0) args.push("--", ...paths);

  const result = spawnSync("git", args, {
    cwd: process.cwd(),
    encoding: "utf8",
    maxBuffer: 128 * 1024 * 1024
  });

  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(result.stderr?.trim() || "git ls-files failed");
  }

  return result.stdout
    .split("\0")
    .filter(Boolean)
    .map((file) => path.resolve(process.cwd(), file));
}

function compare(beforeFile, afterFile) {
  const read = (file) => {
    try {
      return JSON.parse(fs.readFileSync(file, "utf8"));
    } catch (error) {
      throw new Error(`cannot read manifest ${file}: ${error.message}`, { cause: error });
    }
  };

  const before = read(beforeFile).files;
  const after = read(afterFile).files;
  if (!before || !after) {
    throw new Error("manifest is missing the files object");
  }

  const paths = sortPaths([...new Set([...Object.keys(before), ...Object.keys(after)])]);
  const differences = [];
  for (const file of paths) {
    if (!(file in before)) differences.push(`added: ${file}`);
    else if (!(file in after)) differences.push(`removed: ${file}`);
    else if (before[file] !== after[file]) differences.push(`changed: ${file}`);
  }

  for (const difference of differences) console.log(difference);
  return differences.length === 0 ? 0 : 1;
}

function usage() {
  console.error(
    [
      "usage:",
      "  node scripts/hash-tree.mjs [--git] <path...>",
      "  node scripts/hash-tree.mjs --compare <before.json> <after.json>"
    ].join("\n")
  );
}

function main(argv) {
  if (argv.includes("--help") || argv.includes("-h")) {
    usage();
    return 0;
  }

  const compareIndex = argv.indexOf("--compare");
  if (compareIndex !== -1) {
    const before = argv[compareIndex + 1];
    const after = argv[compareIndex + 2];
    if (!before || !after || argv.length !== 3) {
      usage();
      return 1;
    }
    return compare(before, after);
  }

  const git = argv.includes("--git");
  const paths = argv.filter((arg) => arg !== "--git");

  if (paths.length === 0 && !git) {
    usage();
    return 1;
  }

  const files = git ? gitFiles(paths) : null;
  const manifest = git
    ? {
        files: Object.fromEntries(
          sortPaths(files.map((file) => toPosix(path.relative(process.cwd(), file)))).map((key) => [
            key,
            hashFile(path.resolve(process.cwd(), key))
          ])
        )
      }
    : hashPaths(paths);

  process.stdout.write(JSON.stringify(manifest, null, 2) + "\n");
  return 0;
}

try {
  process.exitCode = main(process.argv.slice(2));
} catch (error) {
  console.error(`hash-tree: ${error.message}`);
  process.exitCode = 1;
}
