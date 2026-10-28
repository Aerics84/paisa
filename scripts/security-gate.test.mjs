import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";

import {
  classifyGovulncheckFindings,
  parseGovulncheckStream,
  runGate,
  summarizeNpmAudit
} from "./security-gate.mjs";

const MAIN_MODULE = "github.com/ananthakumaran/paisa";

function tmpProject(name, withLockfile = true) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `paisa-gate-${name}-`));
  if (withLockfile) {
    fs.writeFileSync(path.join(dir, "package-lock.json"), "{}\n");
  }
  return dir;
}

function npmAudit(vulnerabilities, exitCode = 0) {
  return {
    exitCode,
    stdout: JSON.stringify({ metadata: { vulnerabilities } }),
    stderr: ""
  };
}

const noNpmFindings = {
  info: 0,
  low: 0,
  moderate: 0,
  high: 0,
  critical: 0,
  total: 0
};

function govulncheck(config, findings = [], exitCode = 0) {
  const lines = [{ config }, ...findings.map((finding) => ({ finding }))];
  return {
    exitCode,
    stdout: lines.map((line) => JSON.stringify(line)).join("\n") + "\n",
    stderr: ""
  };
}

const CONFIG = {
  protocol_version: "v1.0.0",
  scanner_name: "govulncheck",
  scanner_version: "v1.9.0",
  db: "https://vuln.go.dev",
  go_version: "go1.26.10",
  db_last_modified: "2026-10-27T00:00:00Z"
};

const calledFinding = {
  osv: "GO-2026-6617",
  trace: [
    {
      module: MAIN_MODULE,
      package: "github.com/ananthakumaran/paisa/internal/server",
      function: "Serve"
    }
  ]
};

const notCalledFinding = {
  osv: "GO-2026-6809",
  trace: [
    {
      module: "golang.org/x/net",
      package: "golang.org/x/net/http2",
      function: "ConfigureServer"
    }
  ]
};

async function gate(overrides = {}) {
  const dir = tmpProject("root");
  return runGate({
    npmDirs: [dir],
    mainModule: MAIN_MODULE,
    runNpmAudit: async () => npmAudit(noNpmFindings),
    runGovulncheck: async () => govulncheck(CONFIG),
    ...overrides
  });
}

test("npm audit with critical/high findings exits 1", async () => {
  const result = await gate({
    runNpmAudit: async () => npmAudit({ ...noNpmFindings, high: 2, critical: 1, total: 3 }, 1)
  });

  assert.equal(result.exitCode, 1);
  assert.equal(result.summary.npm[0].vulnerabilities.high, 2);
  assert.equal(result.summary.npm[0].vulnerabilities.critical, 1);
});

test("npm audit registry failure with invalid output exits 2", async () => {
  const result = await gate({
    runNpmAudit: async () => ({
      exitCode: 1,
      stdout: "",
      stderr: "EAI_AGAIN registry.npmjs.org"
    })
  });

  assert.equal(result.exitCode, 2);
  assert.match(result.summary.errors.join("\n"), /npm audit/);
});

test("govulncheck called finding exits 1", async () => {
  const result = await gate({
    runGovulncheck: async () => govulncheck(CONFIG, [calledFinding], 0)
  });

  assert.equal(result.exitCode, 1);
  assert.equal(result.summary.govulncheck.called, 1);
  assert.equal(result.summary.govulncheck.not_called, 0);
});

test("govulncheck not-called findings alone exit 0", async () => {
  const result = await gate({
    runGovulncheck: async () => govulncheck(CONFIG, [notCalledFinding], 0)
  });

  assert.equal(result.exitCode, 0);
  assert.equal(result.summary.govulncheck.called, 0);
  assert.equal(result.summary.govulncheck.not_called, 1);
});

test("missing govulncheck tool exits 2", async () => {
  const result = await gate({
    runGovulncheck: async () => ({
      exitCode: null,
      stdout: "",
      stderr: "",
      error: Object.assign(new Error("spawn govulncheck ENOENT"), {
        code: "ENOENT"
      })
    })
  });

  assert.equal(result.exitCode, 2);
  assert.match(result.summary.errors.join("\n"), /govulncheck/);
});

test("invalid or config-less govulncheck JSON exits 2", async () => {
  const invalid = await gate({
    runGovulncheck: async () => ({
      exitCode: 1,
      stdout: "not json\n",
      stderr: ""
    })
  });
  assert.equal(invalid.exitCode, 2);

  const noConfig = await gate({
    runGovulncheck: async () => ({
      exitCode: 1,
      stdout: JSON.stringify({ finding: calledFinding }) + "\n",
      stderr: ""
    })
  });
  assert.equal(noConfig.exitCode, 2);
});

test("both scans valid and clean exit 0", async () => {
  const result = await gate();

  assert.equal(result.exitCode, 0);
  assert.deepEqual(result.summary.errors, []);
  assert.equal(result.summary.govulncheck.go_version, "go1.26.10");
  assert.equal(result.summary.govulncheck.db_last_modified, "2026-10-27T00:00:00Z");
});

test("npm audit exit 1 due to findings is not a scanner error", async () => {
  const result = await gate({
    runNpmAudit: async () => npmAudit({ ...noNpmFindings, moderate: 1, total: 1 }, 1)
  });

  assert.equal(result.exitCode, 0);
  assert.deepEqual(result.summary.errors, []);
  assert.equal(result.raw.npm[0].exitCode, 1);
});

test("npm audit scanner failure with valid JSON exits 2", async () => {
  const result = await gate({
    runNpmAudit: async () => npmAudit(noNpmFindings, 2)
  });

  assert.equal(result.exitCode, 2);
  assert.match(result.summary.errors.join("\n"), /npm audit.*exit/i);
});

test("govulncheck scanner failure with valid output exits 2", async () => {
  const result = await gate({
    runGovulncheck: async () => govulncheck(CONFIG, [], 1)
  });

  assert.equal(result.exitCode, 2);
  assert.match(result.summary.errors.join("\n"), /govulncheck.*exit/i);
});

test("govulncheck error records exit 2", async () => {
  const result = await gate({
    runGovulncheck: async () => ({
      exitCode: 0,
      stdout:
        [JSON.stringify({ config: CONFIG }), JSON.stringify({ error: "scan interrupted" })].join(
          "\n"
        ) + "\n",
      stderr: ""
    })
  });

  assert.equal(result.exitCode, 2);
  assert.match(result.summary.errors.join("\n"), /govulncheck/);
});

test("multiple npm projects are summarized and a missing lockfile exits 2", async () => {
  const first = tmpProject("root");
  const second = tmpProject("fontgen");
  const missing = tmpProject("missing", false);

  const result = await gate({
    npmDirs: [first, second]
  });

  assert.equal(result.exitCode, 0);
  assert.equal(result.summary.npm.length, 2);
  assert.deepEqual(result.summary.npm.map((entry) => entry.dir).sort(), [first, second].sort());

  const broken = await gate({ npmDirs: [first, missing] });
  assert.equal(broken.exitCode, 2);
  assert.match(broken.summary.errors.join("\n"), /lockfile/);
});

test("summarizeNpmAudit reads metadata.vulnerabilities", () => {
  const summary = summarizeNpmAudit({
    metadata: {
      vulnerabilities: {
        info: 1,
        low: 2,
        moderate: 3,
        high: 4,
        critical: 5,
        total: 15
      }
    }
  });

  assert.deepEqual(summary, {
    info: 1,
    low: 2,
    moderate: 3,
    high: 4,
    critical: 5,
    total: 15
  });
});

test("parseGovulncheckStream handles concatenated pretty-printed objects", () => {
  const stream = [
    JSON.stringify({ config: CONFIG }, null, 2),
    JSON.stringify({ finding: calledFinding }, null, 2),
    JSON.stringify({ finding: notCalledFinding }, null, 2)
  ].join("\n");

  const parsed = parseGovulncheckStream(stream);

  assert.equal(parsed.config.go_version, "go1.26.10");
  assert.equal(parsed.findings.length, 2);
});

test("parseGovulncheckStream deduplicates findings by OSV id", () => {
  const stream = [
    { config: CONFIG },
    { finding: calledFinding },
    { finding: notCalledFinding },
    { finding: calledFinding }
  ]
    .map((line) => JSON.stringify(line))
    .join("\n");

  const parsed = parseGovulncheckStream(stream);

  assert.equal(parsed.config.go_version, "go1.26.10");
  assert.equal(parsed.findings.length, 2);
});

test("classifyGovulncheckFindings separates called from not-called", () => {
  const findings = [
    { osv: "GO-1", trace: [{ module: MAIN_MODULE }] },
    { osv: "GO-2", trace: [{ module: "golang.org/x/net" }] },
    { osv: "GO-1", trace: [{ module: "golang.org/x/net" }] }
  ];

  const classified = classifyGovulncheckFindings(findings, MAIN_MODULE);

  assert.deepEqual(
    classified.called.map((finding) => finding.osv),
    ["GO-1"]
  );
  assert.deepEqual(
    classified.notCalled.map((finding) => finding.osv),
    ["GO-2"]
  );
});
