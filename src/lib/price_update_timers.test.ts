import { afterEach, beforeEach, describe, expect, jest, mock, test } from "bun:test";
import type { PriceUpdateStatus } from "./utils";

function status(overrides: Partial<PriceUpdateStatus> = {}): PriceUpdateStatus {
  return {
    enabled: true,
    managed: true,
    running: true,
    job_id: 1,
    last_job_id: 1,
    last_job_success: false,
    last_job_partial: false,
    stale: false,
    should_update: false,
    last_attempt_at: null,
    last_success_at: null,
    last_partial_success_at: null,
    last_error: "",
    last_warning: "",
    failed_commodities: [],
    next_run_at: null,
    ...overrides
  };
}

let current = status();
let calls = 0;

mock.module("./utils", () => ({
  ajax: async () => {
    calls += 1;
    return current;
  }
}));

mock.module("../store", () => ({
  hasUnsavedWork: () => true,
  refresh: async () => {}
}));

mock.module("bulma-toast", () => ({
  toast: () => {}
}));

const { watchJob, stopPriceUpdateWatcher } = await import("./price_update");

async function flush(): Promise<void> {
  for (let i = 0; i < 5; i += 1) {
    await Promise.resolve();
  }
}

describe("price update watcher timers", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    current = status();
    calls = 0;
  });

  afterEach(() => {
    stopPriceUpdateWatcher();
    jest.useRealTimers();
  });

  test("polls after the fast interval and stops on completion", async () => {
    watchJob(1);
    expect(calls).toBe(0);

    jest.advanceTimersByTime(2000);
    await flush();
    expect(calls).toBe(1);

    current = status({ running: false, last_job_success: true });
    jest.advanceTimersByTime(2000);
    await flush();
    expect(calls).toBe(2);
  });

  test("teardown prevents a later poll", async () => {
    watchJob(1);
    stopPriceUpdateWatcher();

    jest.advanceTimersByTime(2000);
    await flush();
    expect(calls).toBe(0);
  });
});
