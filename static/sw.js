const CACHE_PREFIX = "paisa-shell-";
const FALLBACK_VERSION = "unknown";

const SHELL_URLS = [
  "/",
  "/index.html",
  "/manifest.webmanifest",
  "/icons/icon-192.png",
  "/icons/icon-512.png",
  "/icons/maskable-512.png",
  "/apple-touch-icon.png"
];

let cacheNamePromise = null;

function resolveCacheName() {
  return fetch("/_app/version.json", { cache: "no-store" })
    .then((response) => {
      if (!response.ok) {
        return `${CACHE_PREFIX}${FALLBACK_VERSION}`;
      }
      return response.json().then((data) => {
        const version = data && typeof data.version === "string" ? data.version.trim() : "";
        return `${CACHE_PREFIX}${version || FALLBACK_VERSION}`;
      });
    })
    .catch(() => `${CACHE_PREFIX}${FALLBACK_VERSION}`);
}

function getCacheName() {
  if (!cacheNamePromise) {
    cacheNamePromise = resolveCacheName();
  }
  return cacheNamePromise;
}

async function cacheFirst(request) {
  const cache = await caches.open(await getCacheName());
  const cached = await cache.match(request);
  if (cached) {
    return cached;
  }

  try {
    const response = await fetch(request);
    if (response.ok) {
      await cache.put(request, response.clone());
    }
    return response;
  } catch {
    return Response.error();
  }
}

async function networkFirst(request) {
  const cache = await caches.open(await getCacheName());
  try {
    const response = await fetch(request);
    if (response.ok) {
      await cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await cache.match(request);
    return cached || Response.error();
  }
}

async function networkFirstNavigation(request) {
  const cache = await caches.open(await getCacheName());
  try {
    return await fetch(request);
  } catch {
    return (await cache.match("/index.html")) || (await cache.match("/")) || Response.error();
  }
}

function isShellAsset(pathname) {
  return (
    pathname === "/manifest.webmanifest" ||
    pathname === "/apple-touch-icon.png" ||
    pathname.startsWith("/icons/")
  );
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(await getCacheName());
      await cache.addAll(SHELL_URLS.map((url) => new Request(url, { cache: "reload" })));
    })()
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const current = await getCacheName();
      const names = await caches.keys();
      await Promise.all(
        names
          .filter((name) => name.startsWith(CACHE_PREFIX) && name !== current)
          .map((name) => caches.delete(name))
      );
      await self.clients.claim();
    })()
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) {
    return;
  }

  // Finance data, auth and every other API response always come from the
  // network. Do not read, write or fall back to a cache for these requests.
  if (url.pathname === "/api" || url.pathname.startsWith("/api/")) {
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkFirstNavigation(request));
    return;
  }

  if (url.pathname.startsWith("/_app/immutable/")) {
    event.respondWith(cacheFirst(request));
    return;
  }

  if (isShellAsset(url.pathname)) {
    event.respondWith(networkFirst(request));
  }
});

self.addEventListener("message", (event) => {
  if (event.data && event.data.type === "SKIP_WAITING") {
    self.skipWaiting();
  }
});
