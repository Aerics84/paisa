import { writable } from "svelte/store";

export type ConfirmVariant = "default" | "danger";

export interface ConfirmOptions {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: ConfirmVariant;
}

export interface ConfirmRequest extends ConfirmOptions {
  id: number;
  resolve: (confirmed: boolean) => void;
}

/**
 * The request currently shown by GlobalConfirmDialog, or null when no dialog
 * is open. Only one request is active at a time; further calls are queued.
 */
export const confirmRequest = writable<ConfirmRequest | null>(null);

const queue: ConfirmRequest[] = [];
let active: ConfirmRequest | null = null;
let nextId = 0;

function activateNext() {
  active = queue.shift() ?? null;
  confirmRequest.set(active);
}

/**
 * Opens an in-app confirmation dialog and resolves with the user's choice.
 * Concurrent calls are serialized in FIFO order.
 */
export function askConfirm(options: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    queue.push({ ...options, id: ++nextId, resolve });
    if (!active) {
      activateNext();
    }
  });
}

/** Answers the currently active request and shows the next queued one. */
export function resolveConfirm(confirmed: boolean) {
  const request = active;
  if (!request) return;

  active = null;
  // Show the next request before resolving, so a then() handler that asks
  // again cannot jump ahead of requests that were already queued.
  activateNext();
  request.resolve(confirmed);
}
