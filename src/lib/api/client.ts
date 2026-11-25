import dayjs from "dayjs";
import { error } from "@sveltejs/kit";
import { loading } from "../../store";

export type RequestOptions = RequestInit & {
  background?: boolean;
};

const DATE_KEY_PATTERN = /Date|date|time|now/;
const DATE_VALUE_PATTERN =
  /^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$/;

/**
 * Parses an API timestamp into a dayjs instance.
 *
 * API timestamps carry a timezone but the UI only ever needs the calendar date,
 * so the time component is intentionally dropped.
 */
export function parseDate(value: string): dayjs.Dayjs {
  return dayjs(value.slice(0, 10));
}

/**
 * Recursively converts date-like wire strings into dayjs instances.
 *
 * Only the legacy `ajax` wrapper uses this. The strict `request` client returns
 * raw wire JSON so that typed domain clients can map explicitly.
 */
export function reviveDates<T>(value: T): T {
  if (dayjs.isDayjs(value)) {
    return value;
  }

  if (Array.isArray(value)) {
    return value.map((item) => reviveDates(item)) as unknown as T;
  }

  if (value !== null && typeof value === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, nested] of Object.entries(value)) {
      if (
        typeof nested === "string" &&
        DATE_KEY_PATTERN.test(key) &&
        DATE_VALUE_PATTERN.test(nested)
      ) {
        result[key] = parseDate(nested);
      } else {
        result[key] = reviveDates(nested);
      }
    }
    return result as T;
  }

  return value;
}

export interface RawResponse {
  status: number;
  body: string;
}

/**
 * Shared request path used by both the strict client and the legacy wrapper.
 *
 * It never throws for HTTP error statuses (except the 401 redirect); callers
 * decide how to treat the returned body.
 */
export async function rawRequest(
  route: string,
  options?: RequestOptions,
  params?: Record<string, string>
): Promise<RawResponse> {
  const { background = false, ...init } = options ?? {};
  const path = substituteParams(route, params);

  if (!background) {
    loading.set(true);
  }

  try {
    const response = await fetch(path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...init.headers
      },
      credentials: init.credentials ?? "same-origin"
    });
    const body = await response.text();

    if (response.status === 401 && !isUnauthorizedExempt(path)) {
      await clearSession();
      await redirectToLogin();
      error(401, "Unauthorized");
    }

    return { status: response.status, body };
  } finally {
    if (!background) {
      loading.set(false);
    }
  }
}

/**
 * Strict API client. Sends credentials, resolves with the parsed JSON body and
 * throws for non-OK responses. Dates stay as wire strings.
 */
export async function request<T>(
  route: string,
  options?: RequestOptions,
  params?: Record<string, string>
): Promise<T> {
  const { status, body } = await rawRequest(route, options, params);

  if (status < 200 || status >= 300) {
    throw new Error(body || `Request failed with status ${status}`);
  }

  if (!body) {
    return undefined as T;
  }

  return JSON.parse(body) as T;
}

/**
 * Strict request that rewrites a failed JSON response body into a readable
 * message. Validation endpoints answer with {"error": "..."} and that message
 * should reach the user instead of the raw JSON body.
 */
export async function requestApi<T>(
  route: string,
  options?: RequestOptions,
  params?: Record<string, string>
): Promise<T> {
  try {
    return await request<T>(route, options, params);
  } catch (error) {
    throw new Error(apiErrorMessage(error), { cause: error });
  }
}

/** Extracts a human readable message from a failed request. */
export function apiErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error);

  try {
    const parsed = JSON.parse(raw) as { error?: unknown; message?: unknown };
    if (typeof parsed?.error === "string" && parsed.error) {
      return parsed.error;
    }
    if (typeof parsed?.message === "string" && parsed.message) {
      return parsed.message;
    }
  } catch {
    // The body was not JSON; keep the original text.
  }

  return raw;
}

function substituteParams(route: string, params?: Record<string, string>) {
  if (!params) {
    return route;
  }

  for (const [key, value] of Object.entries(params)) {
    route = route.replace(`:${key}`, value);
  }

  return route;
}

function isUnauthorizedExempt(route: string) {
  return route === "/api/ping" || route.startsWith("/api/auth/");
}

async function clearSession() {
  try {
    await fetch("/api/auth/logout", {
      method: "POST",
      credentials: "same-origin"
    });
  } catch {
    // Clearing the session is best effort. The cookie is HttpOnly, so the
    // client cannot remove it directly.
  }
}

async function redirectToLogin() {
  try {
    const { goto } = await import("$app/navigation");
    await goto("/login");
    return;
  } catch {
    // Fall back to a hard navigation when app navigation is unavailable.
  }

  if (typeof window !== "undefined") {
    window.location.href = "/login";
  }
}
