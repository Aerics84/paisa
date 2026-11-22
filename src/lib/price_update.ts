import * as toast from "bulma-toast";
import { ajax, type PriceUpdateStatus, type SyncResponse } from "./utils";
import { hasUnsavedWork, refresh } from "../store";

const FAST_INTERVAL = 2000;
const SLOW_INTERVAL = 5000;
const IDLE_INTERVAL = 60000;
const FAST_PHASE = 30000;
const MAX_WATCH_MS = 15 * 60 * 1000;

let watchTimer: NodeJS.Timeout | null = null;
let idleTimer: NodeJS.Timeout | null = null;
let watchActive = false;
let watchStartedAt = 0;
let lastSeenJobID = 0;

function clearIdle(): void {
  if (idleTimer !== null) {
    clearTimeout(idleTimer);
    idleTimer = null;
  }
}

function clearWatch(): void {
  if (watchTimer !== null) {
    clearTimeout(watchTimer);
    watchTimer = null;
  }
  watchActive = false;
}

export function stopPriceUpdateWatcher(): void {
  clearWatch();
  clearIdle();
}

export function jobIsComplete(status: PriceUpdateStatus, jobID: number): boolean {
  if (status.last_job_id !== jobID) return status.last_job_id > jobID;
  return !status.running || status.job_id > jobID;
}

export function shouldRefreshAfter(status: PriceUpdateStatus): boolean {
  return !status.running && status.last_job_success;
}

async function finish(status: PriceUpdateStatus): Promise<void> {
  if (status.last_job_success) {
    toast.toast({ message: "Prices updated", type: "is-success" });
  } else if (status.last_partial_success_at) {
    toast.toast({
      message: `Prices partially updated: ${(status.failed_commodities || []).join(", ")}`,
      type: "is-warning"
    });
  } else {
    toast.toast({
      message: `Price update failed: ${status.last_error || "unknown error"}`,
      type: "is-danger"
    });
  }

  if (shouldRefreshAfter(status)) {
    if (!hasUnsavedWork() && document.visibilityState === "visible") {
      await refresh();
    } else {
      toast.toast({
        message: "Prices updated – visible on next navigation",
        type: "is-info"
      });
    }
  }
}

export function watchJob(jobID: number): void {
  if (watchActive) return;
  watchActive = true;
  watchStartedAt = Date.now();
  clearIdle();

  function scheduleNext(): void {
    if (!watchActive) return;
    const elapsed = Date.now() - watchStartedAt;
    if (elapsed >= MAX_WATCH_MS) {
      clearWatch();
      return;
    }
    const interval = elapsed < FAST_PHASE ? FAST_INTERVAL : SLOW_INTERVAL;
    watchTimer = setTimeout(poll, interval);
  }

  async function poll(): Promise<void> {
    if (!watchActive) return;
    let status: PriceUpdateStatus;
    try {
      status = await ajax("/api/sync/status", { background: true });
    } catch {
      scheduleNext();
      return;
    }

    if (jobIsComplete(status, jobID)) {
      clearWatch();
      if (status.last_job_id !== jobID) {
        return; // a newer job has taken over; stop silently
      }
      await finish(status);
      return;
    }

    if (status.last_job_id !== jobID) {
      clearWatch(); // newer job has taken over; stop silently
      return;
    }

    scheduleNext();
  }

  scheduleNext();
}

export function startIdlePoll(): void {
  if (idleTimer !== null) return;

  async function tick(): Promise<void> {
    idleTimer = null;
    if (document.visibilityState !== "visible") {
      idleTimer = setTimeout(tick, IDLE_INTERVAL);
      return;
    }

    let status: PriceUpdateStatus;
    try {
      status = await ajax("/api/sync/status", { background: true });
    } catch {
      idleTimer = setTimeout(tick, IDLE_INTERVAL);
      return;
    }

    if (lastSeenJobID === 0) {
      lastSeenJobID = status.last_job_id;
    }

    if (status.running) {
      if (status.job_id > 0) {
        watchJob(status.job_id);
        return;
      }
      // A synchronous/foreign run is active with no job id; keep idling.
    } else if (status.last_job_id > lastSeenJobID) {
      lastSeenJobID = status.last_job_id;
      await finish(status);
    }

    idleTimer = setTimeout(tick, IDLE_INTERVAL);
  }

  idleTimer = setTimeout(tick, IDLE_INTERVAL);
}

export async function startPriceUpdate(full = true): Promise<void> {
  if (watchActive) return;

  let response: SyncResponse;
  try {
    response = await ajax("/api/sync", {
      method: "POST",
      body: JSON.stringify({ prices: true, background: true, full }),
      background: true
    });
  } catch (e) {
    toast.toast({
      message: `Failed to start price update: ${e instanceof Error ? e.message : String(e)}`,
      type: "is-danger"
    });
    return;
  }

  if (response.status?.running) {
    toast.toast({ message: "Updating prices in the background", type: "is-info" });
    watchJob(response.status.job_id);
  } else if (response.started === false) {
    toast.toast({ message: "A price update is already running", type: "is-info" });
  }
}

export async function checkPriceUpdate(): Promise<void> {
  let status: PriceUpdateStatus;
  try {
    status = await ajax("/api/sync/status", { background: true });
  } catch {
    startIdlePoll();
    return;
  }

  if (status.running && status.job_id > 0) {
    watchJob(status.job_id);
  } else if (status.running && status.job_id === 0) {
    startIdlePoll();
  } else if (status.should_update) {
    await startPriceUpdate(false);
  } else {
    startIdlePoll();
  }
}
