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
let lastSeenJobIDSeeded = false;
let generation = 0;

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
  generation += 1;
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

export function watchInterval(elapsedMs: number): number {
  return elapsedMs < FAST_PHASE ? FAST_INTERVAL : SLOW_INTERVAL;
}

export type PriceUpdateJobOutcome = "success" | "partial" | "failure";

export function jobOutcome(status: PriceUpdateStatus): PriceUpdateJobOutcome {
  if (status.last_job_success) return "success";
  if (status.last_job_partial) return "partial";
  return "failure";
}

async function finish(status: PriceUpdateStatus, gen: number): Promise<void> {
  const outcome = jobOutcome(status);
  if (outcome === "success") {
    if (shouldRefreshAfter(status) && !hasUnsavedWork() && document.visibilityState === "visible") {
      toast.toast({ message: "Prices updated", type: "is-success" });
      await refresh();
    } else {
      toast.toast({ message: "Prices updated – visible on next navigation", type: "is-info" });
    }
  } else if (outcome === "partial") {
    toast.toast({
      message: `Prices partially updated: ${(status.failed_commodities || []).join(", ")}`,
      type: "is-warning",
      duration: 10000
    });
  } else {
    toast.toast({
      message: `Price update failed: ${status.last_error || "unknown error"}`,
      type: "is-danger",
      duration: 10000
    });
  }

  lastSeenJobID = status.last_job_id;
  lastSeenJobIDSeeded = true;

  if (gen !== generation) return;

  startIdlePoll();
}

export function watchJob(jobID: number): void {
  if (watchActive) return;
  watchActive = true;
  watchStartedAt = Date.now();
  clearIdle();
  const gen = generation;

  function scheduleNext(): void {
    if (!watchActive || gen !== generation) return;
    const elapsed = Date.now() - watchStartedAt;
    if (elapsed >= MAX_WATCH_MS) {
      clearWatch();
      startIdlePoll();
      return;
    }
    watchTimer = setTimeout(() => void poll(), watchInterval(elapsed));
  }

  async function poll(): Promise<void> {
    if (gen !== generation) return;
    let status: PriceUpdateStatus;
    try {
      status = await ajax("/api/sync/status", { background: true });
    } catch {
      scheduleNext();
      return;
    }

    if (gen !== generation) return;

    if (status.last_job_id !== jobID) {
      clearWatch();
      if (status.running && status.job_id > 0) {
        watchJob(status.job_id);
      } else if (!status.running && status.last_job_id > jobID) {
        await finish(status, gen);
      } else {
        lastSeenJobID = status.last_job_id;
        lastSeenJobIDSeeded = true;
        startIdlePoll();
      }
      return;
    }

    if (jobIsComplete(status, jobID)) {
      clearWatch();
      await finish(status, gen);
      return;
    }

    scheduleNext();
  }

  scheduleNext();
}

export function startIdlePoll(): void {
  if (idleTimer !== null) return;

  async function tick(): Promise<void> {
    const gen = generation;
    idleTimer = null;

    if (document.visibilityState !== "visible") {
      idleTimer = setTimeout(() => void tick(), IDLE_INTERVAL);
      return;
    }

    let status: PriceUpdateStatus;
    try {
      status = await ajax("/api/sync/status", { background: true });
    } catch {
      if (gen === generation) {
        idleTimer = setTimeout(() => void tick(), IDLE_INTERVAL);
      }
      return;
    }

    if (gen !== generation) return;

    if (!lastSeenJobIDSeeded) {
      lastSeenJobID = status.last_job_id;
      lastSeenJobIDSeeded = true;
    }

    if (status.running) {
      if (status.job_id > 0) {
        watchJob(status.job_id);
        return;
      }
      // A synchronous/foreign run is active with no job id; keep idling.
    } else if (status.last_job_id > lastSeenJobID) {
      await finish(status, gen);
      return;
    }

    if (gen === generation) {
      idleTimer = setTimeout(() => void tick(), IDLE_INTERVAL);
    }
  }

  idleTimer = setTimeout(() => void tick(), IDLE_INTERVAL);
}

export async function startPriceUpdate(full = true): Promise<void> {
  if (watchActive) return;
  const gen = generation;

  let response: SyncResponse;
  try {
    response = await ajax("/api/sync", {
      method: "POST",
      body: JSON.stringify({ prices: true, background: true, full }),
      background: true
    });
  } catch (e) {
    if (gen === generation) {
      toast.toast({
        message: `Failed to start price update: ${e instanceof Error ? e.message : String(e)}`,
        type: "is-danger"
      });
    }
    return;
  }

  if (gen !== generation) return;

  if (response.status?.running) {
    if (response.status.job_id > 0) {
      toast.toast({ message: "Updating prices in the background", type: "is-info" });
      watchJob(response.status.job_id);
    } else {
      // A foreign/foreground run occupies the runner (job_id === 0); we cannot
      // watch it, so report it and keep the idle poll alive.
      toast.toast({ message: "A price update is already running", type: "is-info" });
      startIdlePoll();
    }
  } else if (response.started === false) {
    toast.toast({ message: "A price update is already running", type: "is-info" });
  }
}

export async function checkPriceUpdate(): Promise<void> {
  const gen = generation;
  let status: PriceUpdateStatus;
  try {
    status = await ajax("/api/sync/status", { background: true });
  } catch {
    if (gen === generation) {
      startIdlePoll();
    }
    return;
  }

  if (gen !== generation) return;

  lastSeenJobID = status.last_job_id;
  lastSeenJobIDSeeded = true;

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
