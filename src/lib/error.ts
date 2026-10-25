export function formatError(error: unknown): string {
  if (!error) {
    return "Unknown error";
  }

  if (typeof error === "string") {
    return error;
  }

  const candidate = error as { stack?: unknown; message?: unknown };
  if (typeof candidate.stack === "string" && candidate.stack) {
    return candidate.stack;
  }
  if (typeof candidate.message === "string" && candidate.message) {
    return candidate.message;
  }

  try {
    return String(error);
  } catch {
    return "Unknown error";
  }
}

export function isResizeObserverLoopError(message: unknown): boolean {
  return typeof message === "string" && message.includes("ResizeObserver loop");
}
