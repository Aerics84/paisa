import { afterEach, beforeEach, describe, expect, jest, test } from "bun:test";
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

const originalFetch = globalThis.fetch;
const originalVisibilityState = document.visibilityState;

// Stubbing `globalThis.fetch` keeps the watcher on the real ajax/store/toast
// modules. `mock.module` mocks are process-wide and permanent in Bun, so they
// would shadow ./utils and ../store for concurrently running test files.
function stubStatusFetch() {
  globalThis.fetch = (async () => {
    calls += 1;
    return new Response(JSON.stringify(current), { status: 200 });
  }) as unknown as typeof fetch;
}

const { watchJob, stopPriceUpdateWatcher } = await import("./price_update");

async function flush(): Promise<void> {
  for (let i = 0; i < 5; i += 1) {
    await Promise.resolve();
  }
}

describe("price update watcher timers", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    // The success path would otherwise wait for the global loading flag and
    // refresh the layout; the watcher timers are what this file asserts.
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "hidden"
    });
    current = status();
    calls = 0;
    stubStatusFetch();
  });

  afterEach(() => {
    stopPriceUpdateWatcher();
    jest.useRealTimers();
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: originalVisibilityState
    });
    globalThis.fetch = originalFetch;
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
