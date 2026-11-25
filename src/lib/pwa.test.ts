import { afterEach, expect, mock, test } from "bun:test";
import { applyUpdate, isCacheable } from "./pwa";

const originalServiceWorker = (globalThis.navigator as any)?.serviceWorker;

afterEach(() => {
  if (originalServiceWorker === undefined) {
    delete (globalThis.navigator as any).serviceWorker;
  } else {
    (globalThis.navigator as any).serviceWorker = originalServiceWorker;
  }
});

test("api requests are never cached", () => {
  expect(isCacheable("/api/config")).toBe(false);
  expect(isCacheable("/api/import/statement")).toBe(false);
});

test("immutable assets and shell are cacheable", () => {
  expect(isCacheable("/_app/immutable/chunks/app.js")).toBe(true);
  expect(isCacheable("/")).toBe(true);
  expect(isCacheable("/index.html")).toBe(true);
});

test("user pages are not precached", () => {
  expect(isCacheable("/ledger/import")).toBe(false);
  expect(isCacheable("/login")).toBe(false);
});

test("manifest and icons are cacheable", () => {
  expect(isCacheable("/manifest.webmanifest")).toBe(true);
  expect(isCacheable("/icons/icon-192.png")).toBe(true);
  expect(isCacheable("/apple-touch-icon.png")).toBe(true);
});

test("applyUpdate asks the waiting worker to skip waiting", async () => {
  const postMessage = mock(() => {});
  (globalThis.navigator as any).serviceWorker = {
    getRegistration: async () => ({ waiting: { postMessage } })
  };

  await applyUpdate();
  expect(postMessage).toHaveBeenCalledWith({ type: "SKIP_WAITING" });
});
