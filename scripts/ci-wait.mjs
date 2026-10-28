import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

const DEFAULT_TIMEOUT_SECONDS = 1800;
const DEFAULT_INTERVAL_SECONDS = 10;

function defaultRunGh(args) {
  const result = spawnSync("gh", args, {
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `gh ${args.join(" ")} failed (${result.status}): ${result.stderr?.trim() || "unknown error"}`
    );
  }
  return result.stdout;
}

function defaultSleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function newestRunPerWorkflow(runs) {
  const newest = new Map();
  for (const run of runs) {
    const current = newest.get(run.workflowName);
    if (!current) {
      newest.set(run.workflowName, run);
      continue;
    }
    const currentTime = Date.parse(current.createdAt ?? "") || 0;
    const runTime = Date.parse(run.createdAt ?? "") || 0;
    if (runTime > currentTime || (runTime === currentTime && run.databaseId > current.databaseId)) {
      newest.set(run.workflowName, run);
    }
  }
  return newest;
}

async function listRuns({ commit, branch, workflow, repo, runGh }) {
  const args = [
    "run",
    "list",
    "--commit",
    commit,
    "--limit",
    "100",
    "--json",
    "databaseId,workflowName,status,conclusion,headBranch,createdAt"
  ];
  if (repo) args.push("--repo", repo);
  if (branch) args.push("--branch", branch);
  if (workflow) args.push("--workflow", workflow);

  const parsed = JSON.parse(await runGh(args));
  if (!Array.isArray(parsed)) {
    throw new Error("gh run list did not return an array");
  }
  return parsed;
}

async function readJobs(runGh, run, repo) {
  const args = ["run", "view", String(run.databaseId), "--json", "jobs"];
  if (repo) args.push("--repo", repo);
  const parsed = JSON.parse(await runGh(args));
  if (!Array.isArray(parsed?.jobs)) {
    throw new Error(`gh run view ${run.databaseId} did not return jobs`);
  }
  return parsed.jobs;
}

/**
 * Poll GitHub Actions until every expected workflow and matrix job for one
 * commit is completed successfully, or until the timeout expires.
 */
export async function waitForRuns({
  commit,
  branch = null,
  workflow = null,
  repo = null,
  expectWorkflows = [],
  expectJobs = [],
  timeoutMs = DEFAULT_TIMEOUT_SECONDS * 1000,
  intervalMs = DEFAULT_INTERVAL_SECONDS * 1000,
  runGh = defaultRunGh,
  sleep = defaultSleep,
  now = Date.now
}) {
  const startedAt = now();
  const summary = {
    commit,
    branch,
    repo,
    checked_at: null,
    runs: [],
    jobs: [],
    missing: [],
    failed: [],
    errors: [],
    timed_out: false
  };

  while (true) {
    summary.checked_at = new Date(now()).toISOString();
    summary.runs = [];
    summary.jobs = [];
    summary.missing = [];
    summary.failed = [];
    summary.errors = [];

    let runs;
    try {
      runs = await listRuns({ commit, branch, workflow, repo, runGh });
    } catch (error) {
      summary.errors.push(error.message);
      return { exitCode: 1, summary };
    }

    const newest = newestRunPerWorkflow(runs);
    const expectedRuns = [];

    for (const name of expectWorkflows) {
      const run = newest.get(name);
      if (!run) {
        summary.missing.push(name);
        continue;
      }
      expectedRuns.push(run);
      summary.runs.push({
        workflowName: name,
        databaseId: run.databaseId,
        status: run.status,
        conclusion: run.conclusion
      });
      if (run.status === "completed" && run.conclusion !== "success") {
        summary.failed.push({ workflow: name, conclusion: run.conclusion });
      }
    }

    if (summary.failed.length > 0) {
      return { exitCode: 1, summary };
    }

    const allCompleted =
      summary.missing.length === 0 && expectedRuns.every((run) => run.status === "completed");

    if (allCompleted) {
      let jobFailure = false;
      let jobPending = false;
      let jobMissing = false;

      for (const run of expectedRuns) {
        let jobs;
        try {
          jobs = await readJobs(runGh, run, repo);
        } catch (error) {
          summary.errors.push(error.message);
          return { exitCode: 1, summary };
        }

        for (const expected of expectJobs) {
          const matching = jobs.filter((job) => job.name === expected);
          if (matching.length === 0) {
            jobMissing = true;
            continue;
          }
          for (const job of matching) {
            summary.jobs.push({
              workflow: run.workflowName,
              name: job.name,
              status: job.status,
              conclusion: job.conclusion
            });
            if (job.status !== "completed") {
              jobPending = true;
            } else if (job.conclusion !== "success") {
              jobFailure = true;
            }
          }
        }
      }

      if (jobMissing) {
        for (const expected of expectJobs) {
          if (!summary.jobs.some((job) => job.name === expected)) {
            summary.missing.push(`job:${expected}`);
          }
        }
        return { exitCode: 1, summary };
      }
      if (jobFailure) return { exitCode: 1, summary };
      if (!jobPending) return { exitCode: 0, summary };
    }

    if (now() - startedAt >= timeoutMs) {
      summary.timed_out = true;
      return { exitCode: 1, summary };
    }

    await sleep(intervalMs);
  }
}

function parseArgs(argv) {
  const options = {
    commit: null,
    branch: null,
    workflow: null,
    repo: null,
    expectWorkflows: [],
    expectJobs: [],
    timeoutMs: DEFAULT_TIMEOUT_SECONDS * 1000,
    intervalMs: DEFAULT_INTERVAL_SECONDS * 1000
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    const value = () => {
      const next = argv[index + 1];
      index += 1;
      if (!next) throw new Error(`${arg} requires a value`);
      return next;
    };

    if (arg === "--commit") options.commit = value();
    else if (arg === "--branch") options.branch = value();
    else if (arg === "--workflow") options.workflow = value();
    else if (arg === "--repo") options.repo = value();
    else if (arg === "--expect-workflow") options.expectWorkflows.push(value());
    else if (arg === "--expect-job") options.expectJobs.push(value());
    else if (arg === "--timeout") options.timeoutMs = Number(value()) * 1000;
    else if (arg === "--interval") options.intervalMs = Number(value()) * 1000;
    else if (arg === "--help" || arg === "-h") options.help = true;
    else throw new Error(`unknown argument: ${arg}`);
  }

  return options;
}

async function main() {
  let options;
  try {
    options = parseArgs(process.argv.slice(2));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
    return;
  }

  if (options.help) {
    console.log(
      [
        "usage: node scripts/ci-wait.mjs --commit <sha> [--repo <owner/name>] [--branch <b>] [--workflow <name>]",
        "  --expect-workflow <name>... [--expect-job <name>...] [--timeout 1800] [--interval 10]"
      ].join("\n")
    );
    return;
  }

  if (!options.commit || options.expectWorkflows.length === 0) {
    console.error("--commit and at least one --expect-workflow are required");
    process.exitCode = 1;
    return;
  }

  const result = await waitForRuns(options);
  console.log(JSON.stringify(result.summary, null, 2));
  process.exitCode = result.exitCode;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
