import { defineConfig } from "@playwright/test";

const port = process.env.PAISA_E2E_PORT || "5758";
const seededPort = process.env.PAISA_E2E_SEEDED_PORT || "5759";
const baseURL = `http://127.0.0.1:${port}`;
const seededBaseURL = `http://127.0.0.1:${seededPort}`;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL,
    trace: "retain-on-failure"
  },
  projects: [
    {
      name: "app",
      testIgnore: ["**/rendering.spec.ts"]
    },
    {
      name: "rendering",
      testMatch: ["**/rendering.spec.ts"],
      use: {
        baseURL: seededBaseURL
      }
    }
  ],
  webServer: [
    {
      command: "node e2e/fixture-server.mjs",
      url: `${baseURL}/api/ping`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      stdout: "pipe",
      stderr: "pipe",
      env: {
        PAISA_E2E_PORT: port
      }
    },
    {
      command: "node e2e/fixture-server.mjs",
      url: `${seededBaseURL}/api/ping`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      stdout: "pipe",
      stderr: "pipe",
      env: {
        PAISA_E2E_PORT: seededPort,
        PAISA_E2E_SEED: "true"
      }
    }
  ]
});
