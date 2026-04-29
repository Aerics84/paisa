import { existsSync } from "fs";
import path from "path";
import { spawnSync } from "child_process";

function resolveCommand(command) {
  if (process.platform !== "win32") {
    return command;
  }

  switch (command) {
    case "npm":
      return "npm.cmd";
    case "npx":
      return "npx.cmd";
    case "bun":
      return "bun.exe";
    case "git":
      return "git.exe";
    case "go":
      return "go.exe";
    case "gofmt":
      return "gofmt.exe";
    default:
      return command;
  }
}

export function run(rootDir, command, args, extra = {}) {
  const resolvedCommand = resolveCommand(command);
  const result = spawnSync(resolvedCommand, args, {
    cwd: rootDir,
    encoding: "utf8",
    stdio: extra.captureOutput ? ["ignore", "pipe", "pipe"] : "inherit",
    shell: process.platform === "win32" && resolvedCommand.endsWith(".cmd"),
    ...extra
  });

  if (result.error) {
    throw result.error;
  }

  return result;
}

export function gitOutput(rootDir, args, allowFailure = false) {
  const result = run(rootDir, "git", args, { captureOutput: true });
  if (result.status !== 0 && !allowFailure) {
    process.stderr.write(result.stderr || "");
    process.exit(result.status ?? 1);
  }
  return (result.stdout || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

export function gitDiffNames(rootDir, args = []) {
  return gitOutput(rootDir, ["diff", "--name-only", "--diff-filter=ACMRTUXB", ...args], true);
}

export function hasCommit(rootDir, commitish) {
  if (!commitish || commitish === "0000000000000000000000000000000000000000") {
    return false;
  }

  const result = run(rootDir, "git", ["cat-file", "-e", `${commitish}^{commit}`], {
    captureOutput: true
  });
  return result.status === 0;
}

export function hasHead(rootDir) {
  const result = run(rootDir, "git", ["rev-parse", "--verify", "HEAD"], {
    captureOutput: true
  });
  return result.status === 0;
}

export function collectChangedFiles(rootDir, { stagedOnly = false } = {}) {
  if (stagedOnly) {
    return gitDiffNames(rootDir, ["--cached"]);
  }

  const baseSha = process.env.PRETTIER_BASE_SHA || "";
  if (hasCommit(rootDir, baseSha)) {
    return gitDiffNames(rootDir, [`${baseSha}...HEAD`]);
  }

  if (hasHead(rootDir)) {
    return [...gitDiffNames(rootDir, ["HEAD"]), ...gitDiffNames(rootDir, ["--cached"])];
  }

  return gitOutput(rootDir, ["ls-files"]);
}

export function existingFiles(rootDir, files, predicate = () => true) {
  return [...new Set(files)]
    .sort()
    .filter((file) => existsSync(path.join(rootDir, file)))
    .filter(predicate);
}
