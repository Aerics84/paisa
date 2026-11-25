const CACHE_PREFIX = "paisa-shell-";
const FALLBACK_VERSION = "unknown";
const FALLBACK_CACHE = `${CACHE_PREFIX}${FALLBACK_VERSION}`;

const SHELL_URLS = [
  "/",
  "/index.html",
  "/manifest.webmanifest",
  "/icons/icon-192.png",
  "/icons/icon-512.png",
  "/icons/maskable-512.png",
  "/apple-touch-icon.png"
];

// SvelteKit references its entry, chunk and stylesheet assets from
// index.html. Cache them during install so the first offline reload can boot
// the shell even though no controlled navigation has run yet.
const IMMUTABLE_ASSET_PATTERN = /(?:src|href)="(\/_app\/immutable\/[^"]+)"/g;

let cacheNamePromise = null;

async function versionedCacheName() {
  try {
    const response = await fetch("/_app/version.json", { cache: "no-store" });
    if (!response.ok) {
      return "";
    }
    const data = await response.json();
    const version = data && typeof data.version === "string" ? data.version.trim() : "";
    return version ? `${CACHE_PREFIX}${version}` : "";
  } catch {
    return "";
  }
}

async function existingCacheName() {
  try {
    const names = await caches.keys();
    const candidates = names.filter(
      (name) => name.startsWith(CACHE_PREFIX) && name !== FALLBACK_CACHE
    );
    // caches.keys() returns names in creation order, so the last entry is the
    // most recently installed shell.
    return candidates.length > 0 ? candidates[candidates.length - 1] : "";
  } catch {
    return "";
  }
}

async function resolveCacheName() {
  // Stay version-derived while online so a new build replaces the old cache.
  const versioned = await versionedCacheName();
  if (versioned) {
    return versioned;
  }

  // Offline the version file is unreachable. Reuse a cache this origin already
  // owns instead of inventing the fallback name, which would miss the shell.
  const existing = await existingCacheName();
  return existing || FALLBACK_CACHE;
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

function extractImmutableAssets(html) {
  const assets = new Set();
  for (const match of html.matchAll(IMMUTABLE_ASSET_PATTERN)) {
    assets.add(match[1]);
  }
  return Array.from(assets);
}

async function precacheEntryAssets(cache) {
  let response;
  try {
    response = await fetch("/index.html", { cache: "reload" });
  } catch (error) {
    console.warn("[paisa sw] could not fetch index.html for precache", error);
    return;
  }

  if (!response.ok) {
    console.warn(`[paisa sw] index.html precache fetch failed with ${response.status}`);
    return;
  }

  const html = await response.text();
  const headers = { "Content-Type": "text/html; charset=utf-8" };
  const origin = self.location.origin;
  // Keep the shell available under both URLs, whichever the offline
  // navigation fallback looks up first.
  await Promise.all([
    cache.put(`${origin}/`, new Response(html, { headers })),
    cache.put(`${origin}/index.html`, new Response(html, { headers }))
  ]);

  const assets = extractImmutableAssets(html);
  const results = await Promise.allSettled(
    assets.map((url) => cache.add(new Request(new URL(url, origin).href, { cache: "reload" })))
  );

  results.forEach((result, index) => {
    if (result.status === "rejected") {
      console.warn(`[paisa sw] could not precache ${assets[index]}`, result.reason);
    }
  });
}

async function cleanupCaches(current) {
  // An unresolved name must never delete a real versioned shell cache.
  if (current === FALLBACK_CACHE) {
    return;
  }

  const names = await caches.keys();
  await Promise.all(
    names
      .filter((name) => name.startsWith(CACHE_PREFIX) && name !== current)
      .map((name) => caches.delete(name))
  );
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(await getCacheName());
      await cache.addAll(SHELL_URLS.map((url) => new Request(url, { cache: "reload" })));
      await precacheEntryAssets(cache);
    })()
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      await cleanupCaches(await getCacheName());
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
