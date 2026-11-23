import { afterEach, expect, test } from "bun:test";
import { get } from "svelte/store";
import { loading } from "../store";
import { ajax } from "./utils";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  loading.set(false);
});

test("clears the global loading flag when fetch rejects", async () => {
  globalThis.fetch = (() => Promise.reject(new Error("network down"))) as unknown as typeof fetch;

  await expect(ajax("/api/ping")).rejects.toThrow("network down");
  expect(get(loading)).toBe(false);
});
