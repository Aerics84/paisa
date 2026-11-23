import { describe, expect, test } from "bun:test";
import { jobIsComplete, jobOutcome, shouldRefreshAfter } from "./price_update";
import type { PriceUpdateStatus } from "./utils";

function status(overrides: Partial<PriceUpdateStatus> = {}): PriceUpdateStatus {
  return {
    enabled: true,
    managed: true,
    running: false,
    job_id: 0,
    last_job_id: 0,
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

describe("jobIsComplete", () => {
  test("detects the job even when it finishes before the first poll", () => {
    expect(jobIsComplete(status({ last_job_id: 7, last_job_success: true }), 7)).toBe(true);
  });

  test("does not complete while the same job is running", () => {
    expect(jobIsComplete(status({ running: true, job_id: 7, last_job_id: 7 }), 7)).toBe(false);
  });

  test("completes when a newer job has started and the old one is gone", () => {
    expect(jobIsComplete(status({ running: true, job_id: 8, last_job_id: 7 }), 7)).toBe(true);
  });
});

describe("shouldRefreshAfter", () => {
  test("refreshes after a successful completed job", () => {
    expect(shouldRefreshAfter(status({ last_job_success: true }))).toBe(true);
  });

  test("does not refresh after a failed job", () => {
    expect(shouldRefreshAfter(status({ last_job_success: false }))).toBe(false);
  });
});

describe("jobOutcome", () => {
  test("classifies a successful job", () => {
    expect(jobOutcome(status({ last_job_success: true }))).toBe("success");
  });

  test("classifies a partially successful job", () => {
    expect(jobOutcome(status({ last_job_success: false, last_job_partial: true }))).toBe("partial");
  });

  test("classifies a failed job even when an earlier partial success exists", () => {
    expect(
      jobOutcome(
        status({
          last_job_success: false,
          last_job_partial: false,
          last_partial_success_at: "2026-11-20T22:30:00Z"
        })
      )
    ).toBe("failure");
  });
});
