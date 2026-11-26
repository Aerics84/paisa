import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { loading } from "../../store";
import { ajax } from "../utils";
import { request, reviveDates } from "./client";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
  loading.set(false);
});

test("request sends json and returns parsed body", async () => {
  globalThis.fetch = mock(async (input, init) => {
    expect(input).toBe("/api/config");
    expect(init?.credentials).toBe("same-origin");
    expect(init?.headers).toEqual({ "Content-Type": "application/json" });
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  }) as unknown as typeof fetch;

  await expect(request("/api/config")).resolves.toEqual({ ok: true });
});

test("request surfaces server errors", async () => {
  globalThis.fetch = mock(
    async () => new Response("boom", { status: 500 })
  ) as unknown as typeof fetch;
  await expect(request("/api/config")).rejects.toThrow("boom");
});

test("request keeps wire dates as strings", async () => {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify({ date: "2026-11-22T12:00:00Z" }))
  ) as unknown as typeof fetch;
  const body = await request<{ date: string }>("/api/ledger");
  expect(body.date).toBe("2026-11-22T12:00:00Z");
});

test("reviveDates converts only date-like keys", () => {
  const body = reviveDates({ date: "2026-11-22T12:00:00Z", note: "2026-11-22T12:00:00Z" });
  expect(dayjs.isDayjs(body.date)).toBe(true);
  expect(body.note).toBe("2026-11-22T12:00:00Z");
});

test("reviveDates keeps the time of day for log records", () => {
  const value = "2026-11-26T17:22:06Z";
  const body = reviveDates({ time: value, date: value }) as unknown as {
    time: dayjs.Dayjs;
    date: dayjs.Dayjs;
  };
  expect(body.time.format("YYYY-MM-DD HH:mm:ss")).toBe(dayjs(value).format("YYYY-MM-DD HH:mm:ss"));
  // Ordinary date keys stay date-only so timezone offsets cannot shift the day.
  expect(body.date.format("HH:mm:ss")).toBe("00:00:00");
});

test("ajax keeps legacy behaviour: 400 body is returned and dates are revived", async () => {
  globalThis.fetch = mock(
    async () =>
      new Response(JSON.stringify({ error: "invalid config", date: "2026-11-22T12:00:00Z" }), {
        status: 400
      })
  ) as unknown as typeof fetch;

  const body = (await ajax("/api/config", { method: "POST" })) as unknown as {
    error: string;
    date: dayjs.Dayjs;
  };

  expect(body.error).toBe("invalid config");
  expect(dayjs.isDayjs(body.date)).toBe(true);
});
