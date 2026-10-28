import assert from "node:assert/strict";
import { test } from "node:test";

import { waitForRuns } from "./ci-wait.mjs";

function run(id, workflowName, status, conclusion, createdAt = "2026-10-28T10:00:00Z") {
  return { databaseId: id, workflowName, status, conclusion, headBranch: "main", createdAt };
}

function job(name, status = "completed", conclusion = "success") {
  return { name, status, conclusion };
}

function ghStub(runs, jobsById = {}) {
  return async (args) => {
    if (args[0] === "run" && args[1] === "list") {
      return JSON.stringify(runs);
    }
    if (args[0] === "run" && args[1] === "view") {
      return JSON.stringify({ jobs: jobsById[Number(args[2])] ?? [] });
    }
    throw new Error(`unexpected gh args: ${args.join(" ")}`);
  };
}

function clock() {
  let value = 0;
  return {
    now: () => value,
    sleep: async (ms) => {
      value += ms;
    }
  };
}

async function wait(overrides = {}) {
  const { now, sleep } = clock();
  return waitForRuns({
    commit: "abc123",
    expectWorkflows: ["Test"],
    timeoutMs: 30_000,
    intervalMs: 10_000,
    now,
    sleep,
    runGh: ghStub([run(1, "Test", "completed", "success")], { 1: [job("test")] }),
    ...overrides
  });
}

test("all expected workflows and jobs green exits 0", async () => {
  const result = await wait();

  assert.equal(result.exitCode, 0);
  assert.equal(result.summary.runs.length, 1);
  assert.deepEqual(result.summary.missing, []);
});

test("missing workflow times out with exit 1", async () => {
  const result = await wait({
    expectWorkflows: ["Test", "Ledger Parsers"],
    runGh: ghStub([run(1, "Test", "completed", "success")], { 1: [job("test")] })
  });

  assert.equal(result.exitCode, 1);
  assert.deepEqual(result.summary.missing, ["Ledger Parsers"]);
});

test("a completed non-success run exits 1", async () => {
  const result = await wait({
    runGh: ghStub([run(1, "Test", "completed", "failure")], {
      1: [job("test", "completed", "failure")]
    })
  });

  assert.equal(result.exitCode, 1);
  assert.deepEqual(result.summary.failed, [{ workflow: "Test", conclusion: "failure" }]);
});

test("an expected job that failed exits 1", async () => {
  const result = await wait({
    expectJobs: ["test"],
    runGh: ghStub([run(1, "Test", "completed", "success")], {
      1: [job("test", "completed", "failure")]
    })
  });

  assert.equal(result.exitCode, 1);
  assert.equal(result.summary.jobs[0].conclusion, "failure");
});

test("an expected job that is absent exits 1", async () => {
  const result = await wait({
    expectJobs: ["matrix-job"],
    runGh: ghStub([run(1, "Test", "completed", "success")], { 1: [job("test")] })
  });

  assert.equal(result.exitCode, 1);
  assert.deepEqual(result.summary.missing, ["job:matrix-job"]);
});

test("passes --repo to every gh command when configured", async () => {
  const seen = [];
  const result = await wait({
    repo: "Aerics84/paisa",
    runGh: async (args) => {
      seen.push(args);
      if (args[0] === "run" && args[1] === "list") {
        return JSON.stringify([run(1, "Test", "completed", "success")]);
      }
      if (args[0] === "run" && args[1] === "view") {
        return JSON.stringify({ jobs: [job("test")] });
      }
      throw new Error(`unexpected gh args: ${args.join(" ")}`);
    }
  });

  assert.equal(result.exitCode, 0);
  assert.ok(seen.length >= 1);
  for (const args of seen) {
    assert.equal(args[args.indexOf("--repo") + 1], "Aerics84/paisa");
  }
});

test("the newest run for a commit wins", async () => {
  const result = await wait({
    runGh: ghStub(
      [
        run(2, "Test", "completed", "success", "2026-10-28T11:00:00Z"),
        run(1, "Test", "completed", "failure", "2026-10-28T10:00:00Z")
      ],
      { 2: [job("test")] }
    )
  });

  assert.equal(result.exitCode, 0);
  assert.equal(result.summary.runs[0].databaseId, 2);
});
